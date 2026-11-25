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
  auto       Once a day. Unless a version is already with Apple or waiting to be released, or
             Apple rejected one (a person reads why first), take the newest valid build that
             was uploaded at least --min-age hours ago (a soak for master's CI after the merge)
             and has a higher version than the App Store's, and submit it for App Review as a
             new App Store version. TestFlight's beta review and testers play no part.
  manual     The same now, for --build (default: the newest valid build, of any age). After a rejection, it renumbers the rejected version to that build's
             version (uploads moved past the rejected number when it was submitted), attaches
             the build, marks Apple's item resolved and sends the submission again.

Submitting means: create the App Store version for the build's version string (or reuse one
being prepared, or left on an unsent review submission by a run that stopped, renumbered),
attach the build, set "What's New", release automatically once
approved, in Apple's seven-day phased release, and send it for review through a review
submission. Everything else (the listing, screenshots, App Privacy) App Store Connect carries
over from the previous version. Each step looks before it acts, so a re-run after a failure
picks up where it stopped. A review submission being prepared that holds anything else is left
alone. Reads are retried, with a timeout, when App Store Connect is slow or busy. --dry-run reads App Store Connect and says what it would do.

"What's New" is --notes if given, otherwise the note files under release-notes/ added between the
App Store's live build and this one (scripts/release_notes.py), so a note goes out once on iOS.
The build number counts commits, as testflight.yml numbers it, so the commits are found from it.
Each run writes the live version and its build number as the step outputs `version` and `build`,
and the workflow's next step tags that build ios/<version>.

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

import release_notes

BUNDLE_ID = "org.orthodoxwest.office"
API = "https://api.appstoreconnect.apple.com"
ROOT = pathlib.Path(__file__).resolve().parent.parent
PROJECT = ROOT / "apps/ios/project.yml"
DEFAULT_NOTES = release_notes.DEFAULT_NOTES

# App Store version states (appVersionState, and the older appStoreState it replaces; Apple's
# AppVersionState and AppStoreVersionState). A version in one of these has been sent to Apple:
# Apple is reviewing it, it waits to be released, or it is or was on sale.
IN_FLIGHT = {
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
# Apple turned it down: a person reads why in App Store Connect, and a manual run (or App Store
# Connect) resubmits it with a newer build.
REJECTED = {"REJECTED", "METADATA_REJECTED"}
# Apple could not process the binary: only App Store Connect settles that.
INVALID = {"INVALID_BINARY"}
# A version on a review submission that was never sent (READY_FOR_REVIEW: a run that stopped
# between adding it and submitting), being prepared, or taken back out of review by the
# developer: this script may fill it in and send it.
READY = "READY_FOR_REVIEW"
EDITABLE = {"PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", READY}
# Review submissions (ReviewSubmissionState) that are with Apple or wait on a person.
SUBMISSION_BUSY = {"WAITING_FOR_REVIEW", "IN_REVIEW", "UNRESOLVED_ISSUES", "CANCELING", "COMPLETING"}
SUBMISSION_DRAFT = "READY_FOR_REVIEW"
SUBMISSION_REJECTED = "UNRESOLVED_ISSUES"
# How long a version may take to show it is on the submission before it is sent (as fastlane's
# deliver waits), and how often App Store Connect is asked again after a refusal or no answer.
POLLS, POLL_SECONDS = 10, 15
RETRIES, TIMEOUT = 4, 60
sleep = time.sleep


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
    """The build to submit: --build's, or the newest uploaded min_age hours ago; valid, unexpired and
    above the App Store's version. Whether TestFlight's beta review passed it does not matter."""
    def releasable(b: dict) -> bool:
        return (b["processing"] == "VALID" and not b["expired"]
                and (live is None or parse_version(b["version"]) > live))

    if number:
        return next((b for b in builds if b["number"] == number and releasable(b)), None)
    ready = [b for b in builds if releasable(b) and at - b["uploaded"] >= dt.timedelta(hours=min_age)]
    return max(ready, key=lambda b: (parse_version(b["version"]), b["number"]), default=None)


# --- The decision -------------------------------------------------------------------------


def words(s: str) -> str:
    return s.lower().replace("_", " ")


def decide(versions: list[dict], submissions: list[dict], builds: list[dict], at: dt.datetime,
           min_age: float, number: int = 0, manual: bool = False) -> tuple[dict | None, str]:
    """What to submit, or None and why not. The plan names the build, the version to reuse
    (None: create one), the draft submission to send (None: create one), whether this answers
    a rejection, and whether a release exists to carry over from."""
    busy = [v for v in versions if state(v) in IN_FLIGHT]
    if busy:
        v = busy[0]["attributes"]
        return None, (f"Version {v['versionString']} is {words(state(busy[0]))}; "
                      "nothing more is submitted until Apple has released it.")
    invalid = [v for v in versions if state(v) in INVALID]
    if invalid:
        return None, (f"Version {invalid[0]['attributes']['versionString']} has an invalid binary; "
                      "settle it in App Store Connect and promotion resumes.")
    rejected = [v for v in versions if state(v) in REJECTED]
    if rejected and not manual:
        v = rejected[0]["attributes"]
        return None, (f"Apple rejected version {v['versionString']}. Read why in App Store Connect; once "
                      "the cause is fixed in a newer build, a manual run of this "
                      "workflow resubmits the version with it (apps/ios/README.md).")
    allowed = {SUBMISSION_REJECTED} if rejected else set()
    open_subs = [s for s in submissions
                 if s.get("attributes", {}).get("state") in SUBMISSION_BUSY - allowed]
    if open_subs:
        return None, (f"A review submission is {words(open_subs[0]['attributes']['state'])}; "
                      "nothing more is submitted until it is settled in App Store Connect.")

    released = store_versions(versions, RELEASED)
    live = max(released) if released else None
    build = choose_build(builds, live, at, min_age, number)
    if build is None:
        if number:
            return None, (f"Build {number} cannot be submitted: it is not a valid, unexpired build with a "
                          f"version above the App Store's {show(live) if live else '(none)'}.")
        return None, (f"Nothing to submit: the App Store has {show(live) if live else 'no release'}, and no "
                      f"newer valid build was uploaded {min_age:g} hours ago or more.")

    if rejected:
        # The rejected version is renumbered to the newer build's version and sent again on the
        # submission Apple answered (a version string is editable while rejected, as fastlane's
        # deliver relies on). Uploads moved past its number when it was submitted.
        reuse = rejected[0]
        draft = next((s for s in submissions if s["attributes"].get("state") == SUBMISSION_REJECTED), None)
    else:
        editable = [v for v in versions if state(v) in EDITABLE]
        reuse = editable[0] if editable else None
        draft = next((s for s in submissions if s.get("attributes", {}).get("state") == SUBMISSION_DRAFT), None)
    if reuse and parse_version(reuse["attributes"]["versionString"]) > parse_version(build["version"]):
        return None, (f"Version {reuse['attributes']['versionString']} is {words(state(reuse))} in App Store "
                      f"Connect, above build {build['number']}'s {build['version']}; it is left for whoever made it.")
    return {"build": build, "reuse": reuse, "draft": draft, "resubmit": bool(rejected),
            "update": live is not None}, ""


def notes_between(live_build: int | None, commit: str | None) -> str:
    """What's New for the build of `commit`: the notes added since the App Store's live build."""
    if commit is None:
        return DEFAULT_NOTES
    base = release_notes.commit_of(live_build) if live_build else None
    if base is None:
        base = release_notes.latest_tag("ios", commit)
    return release_notes.release_text("ios", base, commit)


commit_of = release_notes.commit_of


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


class ApiError(Exception):
    def __init__(self, message: str, status: int | None = None):
        super().__init__(message)
        self.status = status


class Api:
    def __init__(self, key_id: str, issuer: str, key_pem: str, dry_run: bool = False):
        self.creds, self.dry_run = (key_id, issuer, key_pem), dry_run
        self.token, self.made = "", 0.0

    def call(self, method: str, path: str, body: dict | None = None) -> dict:
        if method != "GET" and self.dry_run:
            sys.exit(f"::error::Dry run, yet asked to {method} {path}")
        url = path if path.startswith("https://") else API + path
        data = json.dumps(body).encode() if body is not None else None
        for attempt in range(RETRIES + 1):
            if time.time() - self.made > 600:
                self.token, self.made = jwt(*self.creds), time.time()
            req = urllib.request.Request(url, data=data, method=method, headers={
                "Authorization": f"Bearer {self.token}", "Content-Type": "application/json"})
            try:
                with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
                    raw = resp.read()
                return json.loads(raw) if raw else {}
            except urllib.error.HTTPError as e:
                # A refusal for rate is never carried out, so any request may be sent again; a
                # server error or no answer may have been, so only a read is.
                again = e.code == 429 or (e.code >= 500 and method == "GET")
                problem, status = f"{e.code} {e.read().decode(errors='replace')}", e.code
            except (urllib.error.URLError, TimeoutError, ConnectionError) as e:
                again, problem, status = method == "GET", str(getattr(e, "reason", e)), None
            if not again or attempt == RETRIES:
                raise ApiError(f"App Store Connect {method} {path}: {problem}", status)
            log(f"App Store Connect {method} {path}: {problem}; asking again")
            sleep(2 ** (attempt + 1))
        raise AssertionError("unreachable")

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


def live_build(api: Api, versions: list[dict]) -> tuple[str, int] | None:
    """The App Store's live version and its build number, or None before the first release."""
    released = [v for v in versions if state(v) in RELEASED]
    if not released:
        return None
    top = max(released, key=lambda v: parse_version(v["attributes"]["versionString"]))
    try:
        build = api.call("GET", f"/v1/appStoreVersions/{top['id']}/build").get("data") or {}
    except ApiError as e:
        log(f"::warning::Could not read version {top['attributes']['versionString']}'s build ({e}); "
            "the release notes count from the newest ios/ tag")
        return None
    number = (build.get("attributes") or {}).get("version")
    return (top["attributes"]["versionString"], int(number)) if number else None


def output(**values: str) -> None:
    path = os.environ.get("GITHUB_OUTPUT")
    if path:
        with open(path, "a", encoding="utf-8") as f:
            f.writelines(f"{k}={v}\n" for k, v in values.items())


def credentials() -> tuple[str, str, str]:
    try:
        return os.environ["ASC_KEY_ID"], os.environ["ASC_ISSUER_ID"], os.environ["ASC_KEY"]
    except KeyError as e:
        sys.exit(f"::error::{e.args[0]} is not set")


def items_of(api: Api, submission: str) -> list[dict]:
    return api.get_all(f"/v1/reviewSubmissions/{submission}/items?include=appStoreVersion")["data"]


def item_version(item: dict) -> str | None:
    return ((item.get("relationships", {}).get("appStoreVersion") or {}).get("data") or {}).get("id")


def submit(api: Api, app: str, plan: dict, notes: str) -> str | None:
    """Carry out decide()'s plan; each step checks what is already there. None, with a summary,
    when the draft submission holds something else, which is left for whoever put it there."""
    build, version, draft = plan["build"], plan["reuse"], plan["draft"]
    ref = {"type": "builds", "id": build["id"]}
    items = items_of(api, draft["id"]) if draft else []
    mine = version["id"] if version else None
    foreign = [i for i in items if item_version(i) != mine]
    if foreign:
        summary(f"The review submission being prepared in App Store Connect holds {len(foreign)} item(s) "
                "other than this version (an in-app event, or another version); nothing is submitted "
                "until it is sent or emptied there.")
        return None
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
        locs = api.get_all(f"/v1/appStoreVersions/{vid}/appStoreVersionLocalizations")["data"]
        if not locs:
            log(f"::warning::Version {build['version']} has no localizations, so What's New is not set; "
                "App Review may refuse it until it is filled in in App Store Connect.")
        for loc in locs:
            if loc["attributes"].get("whatsNew") != notes:
                api.call("PATCH", f"/v1/appStoreVersionLocalizations/{loc['id']}", {"data": {
                    "type": "appStoreVersionLocalizations", "id": loc["id"], "attributes": {"whatsNew": notes}}})
        # A version with no phased release yet may answer 404 rather than empty data, as
        # fastlane's deliver finds ("returns no data error so need to rescue").
        try:
            phased = api.call("GET", f"/v1/appStoreVersions/{vid}/appStoreVersionPhasedRelease").get("data")
        except ApiError as e:
            if e.status != 404:
                raise
            phased = None
        if not phased:
            api.call("POST", "/v1/appStoreVersionPhasedReleases", {"data": {
                "type": "appStoreVersionPhasedReleases",
                "attributes": {"phasedReleaseState": "INACTIVE"},
                "relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": vid}}},
            }})

    if draft is None:
        draft = api.call("POST", "/v1/reviewSubmissions", {"data": {
            "type": "reviewSubmissions", "attributes": {"platform": "IOS"},
            "relationships": {"app": {"data": {"type": "apps", "id": app}}},
        }})["data"]
    if not items:
        api.call("POST", "/v1/reviewSubmissionItems", {"data": {
            "type": "reviewSubmissionItems",
            "relationships": {
                "reviewSubmission": {"data": {"type": "reviewSubmissions", "id": draft["id"]}},
                "appStoreVersion": {"data": {"type": "appStoreVersions", "id": vid}},
            },
        }})
    if plan["resubmit"]:
        # Apple's answer is dealt with: the rejected item is marked resolved, and the
        # submission is sent again.
        for item in items:
            if item.get("attributes", {}).get("state") in {"REJECTED", "UNRESOLVED_ISSUES"}:
                api.call("PATCH", f"/v1/reviewSubmissionItems/{item['id']}", {"data": {
                    "type": "reviewSubmissionItems", "id": item["id"], "attributes": {"resolved": True}}})
    else:
        # App Store Connect takes a moment to show the version as on the submission, and refuses
        # to send it before (fastlane's deliver waits the same way).
        for attempt in range(POLLS):
            current = api.call("GET", f"/v1/appStoreVersions/{vid}")["data"]
            if state(current) == READY:
                break
            if attempt == POLLS - 1:
                raise ApiError(f"Version {build['version']} is still {words(state(current))}, not ready for "
                               f"review, after {POLLS * POLL_SECONDS} seconds; the next run sends it.")
            sleep(POLL_SECONDS)
    api.call("PATCH", f"/v1/reviewSubmissions/{draft['id']}", {"data": {
        "type": "reviewSubmissions", "id": draft["id"], "attributes": {"submitted": True}}})
    return vid


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("mode", choices=["version", "auto", "manual"])
    ap.add_argument("--min-age", type=float, default=24.0, help="hours since a build's upload (auto)")
    ap.add_argument("--build", type=int, default=0, help="the build number to submit (manual)")
    ap.add_argument("--notes", default="", help="What's New; empty uses the notes in release-notes/ since the live build")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    try:
        run(args)
    except ApiError as e:
        # testflight.yml warns and falls back when the version cannot be read.
        sys.exit(str(e) if args.mode == "version" else f"::error::{e}")


def run(args: argparse.Namespace) -> None:
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

    live = live_build(api, versions)
    if live and not args.dry_run and not release_notes.has_tag("ios", live[0]):
        # The workflow's next step, which alone holds a token that can push, tags it ios/<version>.
        output(version=live[0], build=str(live[1]))
    submissions = read_submissions(api, app)
    builds = read_builds(api, app)
    log(f"Builds: {[(b['version'], b['number'], b['processing'], b['external']) for b in builds[:10]]}")
    manual = args.mode == "manual"
    plan, why = decide(versions, submissions, builds, now(), 0.0 if manual else args.min_age, args.build, manual)
    if plan is None:
        summary(why)
        return
    build = plan["build"]
    commit = commit_of(build["number"])
    notes = args.notes.strip() or notes_between(live[1] if live else None, commit)
    what = (f"Submitting build {build['number']} as version {build['version']} for App Review, released "
            f"automatically once approved" + (", phased over seven days" if plan["update"] else ""))
    if plan["resubmit"]:
        what += f" (resubmitting rejected version {plan['reuse']['attributes']['versionString']})"
    elif plan["reuse"] is not None:
        what += (f" (in App Store Connect's version {plan['reuse']['attributes']['versionString']}, "
                 f"{words(state(plan['reuse']))})")
    if args.dry_run:
        summary(f"Dry run: {what}. What's New:\n\n{notes}")
        return
    if submit(api, app, plan, notes) is not None:
        summary(f"{what}. What's New:\n\n{notes}")


if __name__ == "__main__":
    main()
