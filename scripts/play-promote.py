#!/usr/bin/env python3
"""Promote a Closed testing build of the Android app to production on Google Play.

Run by .github/workflows/play-promote.yml; see apps/android/README.md ("Promoting to production").

  auto       Once a day. If production has a staged rollout this script started at least
             --stage-hours ago, finish it (all users); while it is younger, wait. Otherwise
             release to --rollout of users the newest build that was uploaded to Closed testing
             at least --min-age hours ago and is newer than production's, whether or not a later
             merge has since replaced it there (merges land more often than once a day, so the
             build on Closed testing now is seldom a day old). A halted
             rollout, or a staged one started elsewhere, is left alone, and nothing newer goes
             out until it is finished, resumed or replaced in Play Console.
  manual     Release --version-code (default: Closed testing's newest) to --rollout of users now.

Nothing is rebuilt: production gets a bundle Play already has, by its version code. Its release
notes are the note files under release-notes/ added since production's newest completed build
(scripts/release_notes.py): a staged, halted or replaced release may never reach everyone, so
its notes go again with the next. Each run writes that build's version code as the step output
`live`, and the workflow's next step tags it (android/0.1.<code>). A staged release is named with the time it
started ("0.1.120 staged 2026-11-12T15:00Z"), which is how a later run knows its age; Google's
review comes out of that time. --dry-run reads Play and says what it would do.

Needs PLAY_SERVICE_ACCOUNT_JSON (the Play upload's service account). In auto mode, the age of a
build comes from the Publish to Play workflow's runs (GITHUB_TOKEN, GITHUB_REPOSITORY): each push
run that succeeded uploaded the build whose version code is its commit's count (as
apps/android/app/build.gradle.kts numbers them), so the run's finishing time is when the build
reached Closed testing.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import subprocess
import sys
import urllib.request

import release_notes

PACKAGE = "org.orthodoxwest.office"
API = f"https://androidpublisher.googleapis.com/androidpublisher/v3/applications/{PACKAGE}"
TESTING = "alpha"  # Closed testing
PRODUCTION = "production"
STAGED = re.compile(r"staged (\d{4}-\d\d-\d\dT\d\d:\d\dZ)")


def log(msg: str) -> None:
    print(msg, flush=True)


def summary(msg: str) -> None:
    log(msg)
    path = os.environ.get("GITHUB_STEP_SUMMARY")
    if path:
        with open(path, "a", encoding="utf-8") as f:
            f.write(msg + "\n")


class Play:
    def __init__(self, key_json: str):
        # Imported here so the tests run without the Google client.
        from google.auth.transport.requests import AuthorizedSession
        from google.oauth2 import service_account

        creds = service_account.Credentials.from_service_account_info(
            json.loads(key_json), scopes=["https://www.googleapis.com/auth/androidpublisher"]
        )
        self.http = AuthorizedSession(creds)
        self.edit = self._call("POST", "/edits")["id"]

    def _call(self, method: str, path: str, body: dict | None = None) -> dict:
        r = self.http.request(method, API + path, json=body)
        if r.status_code >= 400:
            sys.exit(f"::error::Play API {method} {path}: {r.status_code} {r.text}")
        return r.json() if r.content else {}

    def track(self, name: str) -> list[dict]:
        return self._call("GET", f"/edits/{self.edit}/tracks/{name}").get("releases", [])

    def set_production(self, release: dict) -> None:
        self._call("PUT", f"/edits/{self.edit}/tracks/{PRODUCTION}", {"track": PRODUCTION, "releases": [release]})
        self._call("POST", f"/edits/{self.edit}:commit")

    def abandon(self) -> None:
        self.http.request("DELETE", API + f"/edits/{self.edit}")


def codes(release: dict) -> list[int]:
    return [int(c) for c in release.get("versionCodes", [])]


def newest(releases: list[dict], statuses: tuple[str, ...]) -> tuple[int, dict] | None:
    """The highest version code among releases in the given statuses, with its release."""
    best = None
    for r in releases:
        if r.get("status") in statuses:
            for c in codes(r):
                if best is None or c > best[0]:
                    best = (c, r)
    return best


def now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def staged_at(release: dict) -> dt.datetime | None:
    """When this script started a staged release, from its name; None for one started elsewhere."""
    m = STAGED.search(release.get("name", ""))
    return dt.datetime.strptime(m.group(1), "%Y-%m-%dT%H:%MZ").replace(tzinfo=dt.timezone.utc) if m else None


def uploads(min_age: float) -> list[tuple[int, str]]:
    """(version code, commit) of the builds Publish to Play's push runs uploaded at least min_age hours ago, newest first."""
    repo, token = os.environ["GITHUB_REPOSITORY"], os.environ["GITHUB_TOKEN"]
    url = (
        f"https://api.github.com/repos/{repo}/actions/workflows/play.yml/runs"
        "?branch=master&event=push&status=success&per_page=50"
    )
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"})
    with urllib.request.urlopen(req) as resp:
        runs = json.load(resp)["workflow_runs"]
    found = []
    for run in runs:
        # When the run last changed: its finish, or later if re-run, which only makes a build look younger.
        done = dt.datetime.fromisoformat(run["updated_at"].replace("Z", "+00:00"))
        if now() - done < dt.timedelta(hours=min_age):
            continue
        count = subprocess.run(["git", "rev-list", "--count", run["head_sha"]], capture_output=True, text=True)
        if count.returncode == 0:  # a commit no longer in master's history is skipped
            found.append((int(count.stdout.strip()), run["head_sha"]))
    return sorted(found, reverse=True)


def notes_between(shipped: int, commit: str | None) -> list[dict]:
    """Play's release notes for the build of `commit`: the notes added since `shipped`, production's
    newest completed build (0: none). A build not on master gets the standing line."""
    if commit is None:
        return [{"language": "en-US", "text": release_notes.DEFAULT_NOTES}]
    base = release_notes.commit_of(shipped) if shipped else None
    if shipped and base is None:
        base = release_notes.latest_tag("android", commit)
    return [{"language": "en-US", "text": release_notes.release_text("android", base, commit)}]


def release_of(code: int, fraction: float, notes: list[dict]) -> dict:
    """A production release of `code`."""
    rel: dict = {"versionCodes": [str(code)], "status": "completed"}
    if notes:
        rel["releaseNotes"] = notes
    if fraction < 1:
        rel["status"], rel["userFraction"] = "inProgress", fraction
        rel["name"] = f"0.1.{code} staged {now():%Y-%m-%dT%H:%MZ}"
    return rel


def output(name: str, value: str) -> None:
    path = os.environ.get("GITHUB_OUTPUT")
    if path:
        with open(path, "a", encoding="utf-8") as f:
            f.write(f"{name}={value}\n")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("mode", choices=["auto", "manual"])
    ap.add_argument("--min-age", type=float, default=24.0, help="hours since a build was uploaded to Closed testing (auto)")
    ap.add_argument("--rollout", type=float, default=0.2, help="share of users for a new release, above 0 and at most 1")
    ap.add_argument("--stage-hours", type=float, default=47.0, help="hours a staged rollout runs before it is finished (auto)")
    ap.add_argument("--version-code", type=int, default=0, help="the build to release (manual; default Closed testing's newest)")
    ap.add_argument("--notes", default="", help="What's new (manual); empty uses the notes in release-notes/ since production's last completed build")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()
    if not 0 < args.rollout <= 1:
        sys.exit(f"::error::rollout must be above 0 and at most 1, not {args.rollout}")
    custom_notes = args.notes.strip()
    if len(custom_notes) > release_notes.LIMITS["android"]:
        sys.exit(f"::error::Release notes are {len(custom_notes)} characters; Play takes at most {release_notes.LIMITS['android']}")

    play = Play(os.environ["PLAY_SERVICE_ACCOUNT_JSON"])
    testing, production = play.track(TESTING), play.track(PRODUCTION)
    live = newest(production, ("completed", "inProgress", "halted"))
    live_code = live[0] if live else 0
    # The notes count from what everyone has: a staged, halted or rejected release may not reach them.
    shipped = (newest(production, ("completed",)) or (0, None))[0]
    log(f"Closed testing: {[(r.get('status'), codes(r)) for r in testing]}")
    log(f"Production: {[(r.get('status'), codes(r), r.get('userFraction')) for r in production]}")

    def emit_tag(code: int) -> None:
        if code and not args.dry_run and not release_notes.has_tag("android", f"0.1.{code}"):
            output("live", str(code))

    release = None
    if args.mode == "manual":
        code = args.version_code or (newest(testing, ("completed", "inProgress")) or (0, None))[0]
        if not code:
            sys.exit("::error::Closed testing has no release to promote")
        if code < live_code:
            sys.exit(f"::error::Production already has {live_code}, newer than {code}")
        notes = ([{"language": "en-US", "text": custom_notes}] if custom_notes
                 else notes_between(shipped, release_notes.commit_of(code)))
        release = release_of(code, args.rollout, notes)
        message = f"Releasing {code} to {args.rollout:.0%} of production"
    else:
        halted = [r for r in production if r.get("status") == "halted"]
        staged = [r for r in production if r.get("status") == "inProgress"]
        if halted:
            summary(f"Production's rollout of {codes(halted[0])} is halted; nothing promoted until it is resumed or replaced.")
        elif staged:
            started = staged_at(staged[0])
            if started is None:
                summary(f"Production's staged rollout of {codes(staged[0])} was started outside this workflow; "
                        "finish or replace it in Play Console, or with a manual run, and promotion resumes.")
            elif now() - started < dt.timedelta(hours=args.stage_hours):
                summary(f"The staged rollout of {codes(staged[0])} started {started:%Y-%m-%d %H:%M} UTC; "
                        f"it is finished once it has run {args.stage_hours:g} hours.")
            else:
                release = {k: v for k, v in staged[0].items() if k != "userFraction"} | {"status": "completed"}
                message = f"Finishing the staged rollout of {codes(staged[0])}: all users now get it"
        else:
            # The newest build a day old, even one a later merge has replaced on Closed testing: merges
            # land more often than daily, so waiting for the build there now to age never promotes.
            # Every such build went to Internal testing unreviewed, and Google reviews the release.
            # Not checked against Play's bundle list, which left out replaced builds and so blocked
            # every one; a version code Play lacks fails the commit with Play's error.
            ready = [(c, sha) for c, sha in uploads(args.min_age) if c > live_code]
            log(f"Uploaded at least {args.min_age:g} hours ago, newer than production: {[c for c, _ in ready]}")
            if ready:
                code, sha = ready[0]
                release = release_of(code, args.rollout, notes_between(shipped, sha))
                message = f"Releasing {code} to {args.rollout:.0%} of production (uploaded at least {args.min_age:g} hours ago)"
            else:
                summary(f"Nothing to promote: production has {live_code or 'nothing'}, and no newer build was uploaded {args.min_age:g} hours ago or more.")

    if release is None:
        play.abandon()
        emit_tag(shipped)
        return
    if args.dry_run:
        summary(f"Dry run: {message}")
        log(json.dumps(release, indent=2))
        play.abandon()
        return
    play.set_production(release)
    new = live_code not in codes(release)
    # A staged rollout was already reviewed when started; finishing it reaches everyone now.
    emit_tag(max(codes(release)) if (not new and release.get("status") == "completed") else shipped)
    summary(f"{message}." + (" Google reviews it before it reaches anyone." if new else ""))
    if release.get("releaseNotes"):
        summary("What's new:\n\n" + release["releaseNotes"][0]["text"])


if __name__ == "__main__":
    main()
