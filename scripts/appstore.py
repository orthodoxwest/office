#!/usr/bin/env python3
"""The iOS app's version numbers and its App Store releases, through the App Store Connect API.

Run by .github/workflows/testflight.yml and .github/workflows/appstore-promote.yml; see
apps/ios/README.md ("TestFlight", "The App Store").

  version    Print the marketing version (CFBundleShortVersionString) master's next TestFlight
             upload takes: project.yml's MARKETING_VERSION, or the highest version TestFlight
             already has builds under if that is higher, or, if the App Store has that version
             or a higher one submitted, in review, pending release or on sale, one patch above
             the highest such. App Store Connect closes a version to new builds once Apple
             approves it, and a build uploaded while its version is in review could never be
             released, so a version moves on as soon as it is submitted. project.yml stays the
             floor: raising its minor or major version there is how a person starts 1.1.0.
  auto       Once a week. Unless a version is already in review or waiting to be released, or
             Apple rejected one (a person answers that in App Store Connect), take the newest
             build that has passed beta review and gone to the external testers, was uploaded
             at least --min-age hours ago, and has a higher version than the App Store's, and
             submit it for App Review as a new App Store version.
  manual     The same now, for --build (default: the newest build with the external testers,
             of any age).

Submitting means: create the App Store version for the build's version string (or reuse one
being prepared, renumbered), attach the build, set "What's New", release automatically once
approved, in Apple's seven-day phased release, and send it for review through a review
submission. Everything else (the listing, screenshots, App Privacy) App Store Connect carries
over from the previous version. Each step looks before it acts, so a re-run after a failure
picks up where it stopped. --dry-run reads App Store Connect and says what it would do.

"What's New" is --notes if given, otherwise apps/android/whatsnew.txt as of the build's commit
(the notes Google Play shows; lines starting with # dropped), otherwise DEFAULT_NOTES. The build
number counts commits, as testflight.yml numbers it, so the commit is found from it.

Needs ASC_KEY_ID, ASC_ISSUER_ID and ASC_KEY (the .p8's contents). The token is an ES256 JWT,
signed with the openssl command, so the script needs nothing beyond the standard library.
"""

from __future__ import annotations

import argparse
import base64
import datetime as dt
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

BUNDLE_ID = "org.orthodoxwest.office"
API = "https://api.appstoreconnect.apple.com"
ROOT = pathlib.Path(__file__).resolve().parent.parent
PROJECT = ROOT / "apps/ios/project.yml"
NOTES_FILE = "apps/android/whatsnew.txt"
DEFAULT_NOTES = "Corrections to the texts and improvements to the app."

# App Store version states (appVersionState, and the older appStoreState it replaces; Apple's
# AppVersionState and AppStoreVersionState). A version in one of these has been submitted:
# Apple is reviewing it, it waits to be released, or it is or was on sale.
IN_FLIGHT = {
    "READY_FOR_REVIEW",
    "WAITING_FOR_EXPORT_COMPLIANCE",
    "WAITING_FOR_REVIEW",
    "IN_REVIEW",
    "ACCEPTED",
    "PENDING_APPLE_RELEASE",
    "PENDING_DEVELOPER_RELEASE",
    "PENDING_CONTRACT",
    "PROCESSING_FOR_DISTRIBUTION",
    "PROCESSING_FOR_APP_STORE",
}
RELEASED = {
    "READY_FOR_DISTRIBUTION",
    "READY_FOR_SALE",
    "PREORDER_READY_FOR_SALE",
    "REPLACED_WITH_NEW_VERSION",
    "DEVELOPER_REMOVED_FROM_SALE",
    "REMOVED_FROM_SALE",
}
SUBMITTED = IN_FLIGHT | RELEASED
# Apple turned it down: a person reads why in App Store Connect and answers there.
REJECTED = {"REJECTED", "METADATA_REJECTED", "INVALID_BINARY"}
# Being prepared, or taken back out of review by the developer: this script may fill it in.
EDITABLE = {"PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED"}
# Review submissions (ReviewSubmissionState) that are with Apple or wait on a person.
SUBMISSION_BUSY = {"WAITING_FOR_REVIEW", "IN_REVIEW", "UNRESOLVED_ISSUES", "CANCELING", "COMPLETING"}
SUBMISSION_DRAFT = "READY_FOR_REVIEW"
# A build's beta state once beta review has passed and it is with the external group.
WITH_TESTERS = {"IN_BETA_TESTING"}


def log(msg: str) -> None:
    print(msg, file=sys.stderr, flush=True)


def summary(msg: str) -> None:
    log(msg)
    path = os.environ.get("GITHUB_STEP_SUMMARY")
    if path:
        with open(path, "a", encoding="utf-8") as f:
            f.write(msg + "\n")


def now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


# --- Versions -----------------------------------------------------------------------------


def parse_version(s: str) -> tuple[int, int, int]:
    """'1.0' -> (1, 0, 0); App Store version strings are up to three dot-separated integers."""
    parts = [int(p) for p in s.strip().split(".")]
    if not 1 <= len(parts) <= 3:
        raise ValueError(f"not a version: {s!r}")
    return tuple(parts + [0] * (3 - len(parts)))  # type: ignore[return-value]


def show(v: tuple[int, int, int]) -> str:
    return ".".join(map(str, v))


def floor_version(project: pathlib.Path = PROJECT) -> str:
    m = re.search(r'^\s*MARKETING_VERSION:\s*"?([0-9.]+)"?\s*$', project.read_text(), re.M)
    if not m:
        sys.exit(f"::error::No MARKETING_VERSION in {project}")
    return m.group(1)


def state(version: dict) -> str:
    a = version.get("attributes", {})
    return a.get("appVersionState") or a.get("appStoreState") or ""


def store_versions(versions: list[dict], states: set[str]) -> list[tuple[int, int, int]]:
    return [parse_version(v["attributes"]["versionString"]) for v in versions if state(v) in states]


def next_marketing_version(floor: str, versions: list[dict], prerelease: list[dict]) -> str:
    """The marketing version master's next upload takes (see the module's `version`)."""
    best = parse_version(floor)
    for p in prerelease:
        best = max(best, parse_version(p["attributes"]["version"]))
    closed = store_versions(versions, SUBMITTED)
    if closed and max(closed) >= best:
        top = max(closed)
        best = (top[0], top[1], top[2] + 1)
    return show(best)


# --- Builds -------------------------------------------------------------------------------


def timestamp(s: str) -> dt.datetime:
    return dt.datetime.fromisoformat(s.replace("Z", "+00:00"))


def builds_of(response: dict) -> list[dict]:
    """GET /v1/builds?include=preReleaseVersion,buildBetaDetail flattened: one dict per build."""
    included = {(i["type"], i["id"]): i.get("attributes", {}) for i in response.get("included", [])}
    out = []
    for b in response.get("data", []):
        a, rel = b.get("attributes", {}), b.get("relationships", {})

        def linked(name: str) -> dict:
            d = (rel.get(name) or {}).get("data")
            return included.get((d["type"], d["id"]), {}) if d else {}

        pre, beta = linked("preReleaseVersion"), linked("buildBetaDetail")
        if not pre.get("version"):
            continue
        out.append({
            "id": b["id"],
            "number": int(a["version"]),
            "version": pre["version"],
            "uploaded": timestamp(a["uploadedDate"]),
            "processing": a.get("processingState"),
            "expired": bool(a.get("expired")),
            "external": beta.get("externalBuildState"),
            "encryption": a.get("usesNonExemptEncryption"),
        })
    return out


def choose_build(builds: list[dict], live: tuple[int, int, int] | None, at: dt.datetime,
                 min_age: float, number: int = 0) -> dict | None:
    """The build to submit: --build's, or the newest tested one, valid and above the App Store's version."""
    def releasable(b: dict) -> bool:
        return (b["processing"] == "VALID" and not b["expired"]
                and (live is None or parse_version(b["version"]) > live))

    if number:
        return next((b for b in builds if b["number"] == number and releasable(b)), None)
    ready = [b for b in builds if releasable(b) and b["external"] in WITH_TESTERS
             and at - b["uploaded"] >= dt.timedelta(hours=min_age)]
    return max(ready, key=lambda b: (parse_version(b["version"]), b["number"]), default=None)


# --- The decision -------------------------------------------------------------------------


def decide(versions: list[dict], submissions: list[dict], builds: list[dict], at: dt.datetime,
           min_age: float, number: int = 0) -> tuple[dict | None, str]:
    """What to submit, or None and why not. The plan names the build, the version to reuse
    (None: create one), and whether a release exists to carry over from."""
    busy = [v for v in versions if state(v) in IN_FLIGHT]
    if busy:
        v = busy[0]["attributes"]
        return None, (f"Version {v['versionString']} is {state(busy[0]).lower().replace('_', ' ')}; "
                      "nothing more is submitted until Apple has released it.")
    rejected = [v for v in versions if state(v) in REJECTED]
    if rejected:
        v = rejected[0]["attributes"]
        return None, (f"Apple rejected version {v['versionString']}; answer in App Store Connect "
                      "(resubmit, or remove the version) and promotion resumes.")
    open_subs = [s for s in submissions if s.get("attributes", {}).get("state") in SUBMISSION_BUSY]
    if open_subs:
        return None, (f"A review submission is {open_subs[0]['attributes']['state'].lower().replace('_', ' ')}; "
                      "nothing more is submitted until it is settled in App Store Connect.")

    released = store_versions(versions, RELEASED)
    live = max(released) if released else None
    build = choose_build(builds, live, at, min_age, number)
    if build is None:
        if number:
            return None, (f"Build {number} cannot be submitted: it is not a valid, unexpired build with a "
                          f"version above the App Store's {show(live) if live else '(none)'}.")
        return None, (f"Nothing to submit: the App Store has {show(live) if live else 'no release'}, and no "
                      f"newer build has been with the external testers {min_age:g} hours.")

    editable = [v for v in versions if state(v) in EDITABLE]
    reuse = editable[0] if editable else None
    if reuse and parse_version(reuse["attributes"]["versionString"]) > parse_version(build["version"]):
        return None, (f"Version {reuse['attributes']['versionString']} is being prepared in App Store Connect, "
                      f"above build {build['number']}'s {build['version']}; it is left for whoever made it.")
    draft = next((s for s in submissions if s.get("attributes", {}).get("state") == SUBMISSION_DRAFT), None)
    return {"build": build, "reuse": reuse, "draft": draft, "update": live is not None}, ""


def notes_at(commit: str | None) -> str:
    """apps/android/whatsnew.txt at the commit (HEAD if unknown), without its # lines."""
    r = subprocess.run(["git", "-C", str(ROOT), "show", f"{commit or 'HEAD'}:{NOTES_FILE}"],
                       capture_output=True, text=True)
    if r.returncode != 0:
        return ""
    return "\n".join(line for line in r.stdout.splitlines() if not line.startswith("#")).strip()


def commit_of(number: int) -> str | None:
    """master's commit whose commit count is the build number (testflight.yml's numbering)."""
    chain = subprocess.run(["git", "-C", str(ROOT), "rev-list", "--first-parent", "HEAD"],
                           capture_output=True, text=True).stdout.split()

    def count(i: int) -> int:
        return int(subprocess.run(["git", "-C", str(ROOT), "rev-list", "--count", chain[i]],
                                  capture_output=True, text=True).stdout)

    lo, hi = 0, len(chain) - 1  # counts fall strictly from chain[0] (HEAD) back
    while lo <= hi:
        mid = (lo + hi) // 2
        c = count(mid)
        if c == number:
            return chain[mid]
        lo, hi = (mid + 1, hi) if c > number else (lo, mid - 1)
    return None


# --- App Store Connect --------------------------------------------------------------------


def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def der_to_raw(sig: bytes, size: int = 32) -> bytes:
    """An ECDSA signature from DER (what openssl writes) to JWS's r || s."""
    def length(i: int) -> tuple[int, int]:
        n = sig[i]
        if n < 0x80:
            return n, i + 1
        k = n & 0x7F
        return int.from_bytes(sig[i + 1:i + 1 + k], "big"), i + 1 + k

    if sig[0] != 0x30:
        raise ValueError("not a DER sequence")
    _, i = length(1)
    out = b""
    for _ in range(2):
        if sig[i] != 0x02:
            raise ValueError("not a DER integer")
        n, i = length(i + 1)
        out += sig[i:i + n].lstrip(b"\0").rjust(size, b"\0")
        i += n
    return out


def jwt(key_id: str, issuer: str, key_pem: str, at: int | None = None) -> str:
    """App Store Connect's ES256 token, valid 15 minutes (Apple allows at most 20)."""
    at = int(time.time()) if at is None else at
    head = b64url(json.dumps({"alg": "ES256", "kid": key_id, "typ": "JWT"}).encode())
    body = b64url(json.dumps({"iss": issuer, "iat": at, "exp": at + 900, "aud": "appstoreconnect-v1"}).encode())
    signing = f"{head}.{body}".encode()
    with tempfile.TemporaryDirectory() as tmp:
        key = pathlib.Path(tmp, "key.p8")
        key.touch(mode=0o600)
        key.write_text(key_pem.strip() + "\n")
        r = subprocess.run(["openssl", "dgst", "-sha256", "-sign", str(key)], input=signing, capture_output=True)
    if r.returncode != 0:
        sys.exit(f"::error::openssl could not sign with ASC_KEY: {r.stderr.decode().strip()}")
    return f"{head}.{body}.{b64url(der_to_raw(r.stdout))}"


class Api:
    def __init__(self, key_id: str, issuer: str, key_pem: str, dry_run: bool = False):
        self.creds, self.dry_run = (key_id, issuer, key_pem), dry_run
        self.token, self.made = "", 0.0

    def call(self, method: str, path: str, body: dict | None = None) -> dict:
        if method != "GET" and self.dry_run:
            sys.exit(f"::error::Dry run, yet asked to {method} {path}")
        if time.time() - self.made > 600:
            self.token, self.made = jwt(*self.creds), time.time()
        url = path if path.startswith("https://") else API + path
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(url, data=data, method=method, headers={
            "Authorization": f"Bearer {self.token}", "Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(req) as resp:
                raw = resp.read()
        except urllib.error.HTTPError as e:
            sys.exit(f"::error::App Store Connect {method} {path}: {e.code} {e.read().decode(errors='replace')}")
        return json.loads(raw) if raw else {}

    def get_all(self, path: str) -> dict:
        """A list endpoint's every page, data and included joined."""
        out: dict = {"data": [], "included": []}
        url: str | None = path
        while url:
            page = self.call("GET", url)
            out["data"] += page.get("data", [])
            out["included"] += page.get("included", [])
            url = (page.get("links") or {}).get("next")
        return out


def app_id(api: Api) -> str:
    apps = api.call("GET", "/v1/apps?" + urllib.parse.urlencode({"filter[bundleId]": BUNDLE_ID}))["data"]
    if not apps:
        sys.exit(f"::error::App Store Connect has no app {BUNDLE_ID}")
    return apps[0]["id"]


def read_versions(api: Api, app: str) -> list[dict]:
    return api.get_all(f"/v1/apps/{app}/appStoreVersions?" + urllib.parse.urlencode(
        {"filter[platform]": "IOS", "limit": "200"}))["data"]


def read_prerelease(api: Api, app: str) -> list[dict]:
    return api.get_all("/v1/preReleaseVersions?" + urllib.parse.urlencode(
        {"filter[app]": app, "filter[platform]": "IOS", "limit": "200"}))["data"]


def read_submissions(api: Api, app: str) -> list[dict]:
    return api.get_all("/v1/reviewSubmissions?" + urllib.parse.urlencode(
        {"filter[app]": app, "filter[platform]": "IOS", "limit": "200",
         "filter[state]": ",".join(sorted(SUBMISSION_BUSY | {SUBMISSION_DRAFT}))}))["data"]


def read_builds(api: Api, app: str) -> list[dict]:
    # The newest 200 uploads cover every build TestFlight still holds (each lasts 90 days).
    return builds_of(api.call("GET", "/v1/builds?" + urllib.parse.urlencode({
        "filter[app]": app, "filter[preReleaseVersion.platform]": "IOS", "sort": "-uploadedDate",
        "limit": "200", "include": "preReleaseVersion,buildBetaDetail"})))


def credentials() -> tuple[str, str, str]:
    try:
        return os.environ["ASC_KEY_ID"], os.environ["ASC_ISSUER_ID"], os.environ["ASC_KEY"]
    except KeyError as e:
        sys.exit(f"::error::{e.args[0]} is not set")


def submit(api: Api, app: str, plan: dict, notes: str) -> str:
    """Carry out decide()'s plan; each step checks what is already there."""
    build, version = plan["build"], plan["reuse"]
    ref = {"type": "builds", "id": build["id"]}
    if version is None:
        version = api.call("POST", "/v1/appStoreVersions", {"data": {
            "type": "appStoreVersions",
            "attributes": {"platform": "IOS", "versionString": build["version"], "releaseType": "AFTER_APPROVAL"},
            "relationships": {"app": {"data": {"type": "apps", "id": app}}, "build": {"data": ref}},
        }})["data"]
        log(f"Created App Store version {build['version']}")
    else:
        a = version["attributes"]
        changes = {}
        if a.get("versionString") != build["version"]:
            changes["versionString"] = build["version"]
        if a.get("releaseType") != "AFTER_APPROVAL":
            changes["releaseType"] = "AFTER_APPROVAL"
        if changes:
            api.call("PATCH", f"/v1/appStoreVersions/{version['id']}", {"data": {
                "type": "appStoreVersions", "id": version["id"], "attributes": changes}})
            log(f"Updated version {a.get('versionString')}: {changes}")
        api.call("PATCH", f"/v1/appStoreVersions/{version['id']}/relationships/build", {"data": ref})
    vid = version["id"]

    # Info.plist says no non-exempt encryption, so App Store Connect normally has the answer already.
    if build["encryption"] is None:
        api.call("PATCH", f"/v1/builds/{build['id']}", {"data": {
            "type": "builds", "id": build["id"], "attributes": {"usesNonExemptEncryption": False}}})

    if plan["update"]:  # the first release has no "What's New" and no phased release
        for loc in api.get_all(f"/v1/appStoreVersions/{vid}/appStoreVersionLocalizations")["data"]:
            if loc["attributes"].get("whatsNew") != notes:
                api.call("PATCH", f"/v1/appStoreVersionLocalizations/{loc['id']}", {"data": {
                    "type": "appStoreVersionLocalizations", "id": loc["id"], "attributes": {"whatsNew": notes}}})
        phased = api.call("GET", f"/v1/appStoreVersions/{vid}/appStoreVersionPhasedRelease").get("data")
        if not phased:
            api.call("POST", "/v1/appStoreVersionPhasedReleases", {"data": {
                "type": "appStoreVersionPhasedReleases",
                "attributes": {"phasedReleaseState": "INACTIVE"},
                "relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": vid}}},
            }})

    draft = plan["draft"]
    if draft is None:
        draft = api.call("POST", "/v1/reviewSubmissions", {"data": {
            "type": "reviewSubmissions", "attributes": {"platform": "IOS"},
            "relationships": {"app": {"data": {"type": "apps", "id": app}}},
        }})["data"]
        items = []
    else:
        items = api.get_all(f"/v1/reviewSubmissions/{draft['id']}/items")["data"]
    held = {((i.get("relationships", {}).get("appStoreVersion") or {}).get("data") or {}).get("id") for i in items}
    if vid not in held:
        api.call("POST", "/v1/reviewSubmissionItems", {"data": {
            "type": "reviewSubmissionItems",
            "relationships": {
                "reviewSubmission": {"data": {"type": "reviewSubmissions", "id": draft["id"]}},
                "appStoreVersion": {"data": {"type": "appStoreVersions", "id": vid}},
            },
        }})
    api.call("PATCH", f"/v1/reviewSubmissions/{draft['id']}", {"data": {
        "type": "reviewSubmissions", "id": draft["id"], "attributes": {"submitted": True}}})
    return vid


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("mode", choices=["version", "auto", "manual"])
    ap.add_argument("--min-age", type=float, default=24.0, help="hours since a build's upload (auto)")
    ap.add_argument("--build", type=int, default=0, help="the build number to submit (manual)")
    ap.add_argument("--notes", default="", help="What's New; empty uses apps/android/whatsnew.txt")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    api = Api(*credentials(), dry_run=args.dry_run)
    app = app_id(api)
    versions = read_versions(api, app)
    log(f"App Store versions: {[(v['attributes']['versionString'], state(v)) for v in versions]}")

    if args.mode == "version":
        floor = floor_version()
        chosen = next_marketing_version(floor, versions, read_prerelease(api, app))
        if chosen != floor:
            log(f"::notice::Building version {chosen}: project.yml's {floor} is behind App Store Connect.")
        print(chosen)
        return

    submissions = read_submissions(api, app)
    builds = read_builds(api, app)
    log(f"Builds: {[(b['version'], b['number'], b['processing'], b['external']) for b in builds[:10]]}")
    plan, why = decide(versions, submissions, builds, now(),
                       args.min_age if args.mode == "auto" else 0.0, args.build)
    if plan is None:
        summary(why)
        return
    build = plan["build"]
    notes = args.notes.strip() or notes_at(commit_of(build["number"])) or DEFAULT_NOTES
    what = (f"Submitting build {build['number']} as version {build['version']} for App Review, released "
            f"automatically once approved" + (", phased over seven days" if plan["update"] else ""))
    if plan["reuse"] is not None:
        what += f" (in App Store Connect's version {plan['reuse']['attributes']['versionString']}, being prepared)"
    if args.dry_run:
        summary(f"Dry run: {what}. What's New:\n\n{notes}")
        return
    submit(api, app, plan, notes)
    summary(f"{what}. What's New:\n\n{notes}")


if __name__ == "__main__":
    main()
