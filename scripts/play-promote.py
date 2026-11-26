#!/usr/bin/env python3
"""Move the Android app up Google Play's tracks: Internal testing, Closed testing, production.

Run by .github/workflows/play-promote.yml; see apps/android/README.md ("Promoting to production").
Every merge goes to Internal testing (play.yml); this script moves builds on from there.

  auto       Once a day. If production has a staged rollout this script started at least
             --stage-hours ago, finish it (all users); while it is younger, wait. Otherwise, if
             Closed testing's build is newer than production's and has been there --min-age
             hours, release it to --rollout of users. A halted rollout, or a staged one started
             elsewhere, is left alone, and nothing newer goes out until it is finished, resumed
             or replaced in Play Console. Then, once production has Closed testing's build (or
             Closed testing is empty), Internal testing's newest build moves up to Closed testing,
             so each build has a day with testers before the public gets it.
  manual     Release --version-code (default: Closed testing's newest) to --rollout of users now.

Nothing is rebuilt: each track gets a bundle Play already has, by its version code. Play's API can
release only a bundle some track still holds, which is why a build waits its day on Closed testing
rather than being picked out of older uploads: a merge replaces the build on Internal testing.

Production's release notes are the note files under release-notes/ added since production's newest
completed build (scripts/release_notes.py): a staged, halted or replaced release may never reach
everyone, so its notes go again with the next. Closed testing gets Internal testing's notes. Each
run writes production's completed build as the step output `live`, and the workflow's next step
tags it (android/0.1.<code>). A release is named with the time this script made it ("0.1.120 staged
2026-11-12T15:00Z" in production, "0.1.120 tested from 2026-11-12T15:00Z" on Closed testing),
which is how a later run knows its age; a build put on Closed testing some other way counts as a
day old. Google's review comes out of those times. --dry-run reads Play and says what it would do.

Needs PLAY_SERVICE_ACCOUNT_JSON (the Play upload's service account).
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import sys

import release_notes

PACKAGE = "org.orthodoxwest.office"
API = f"https://androidpublisher.googleapis.com/androidpublisher/v3/applications/{PACKAGE}"
TESTING = "alpha"  # Closed testing
INTERNAL = "internal"
PRODUCTION = "production"
STAGED = re.compile(r"staged (\d{4}-\d\d-\d\dT\d\d:\d\dZ)")
TESTED = re.compile(r"tested from (\d{4}-\d\d-\d\dT\d\d:\d\dZ)")


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

    def put(self, track: str, release: dict) -> None:
        self._call("PUT", f"/edits/{self.edit}/tracks/{track}", {"track": track, "releases": [release]})

    def commit(self) -> None:
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


def named_at(release: dict, pattern: re.Pattern) -> dt.datetime | None:
    """When this script made a release, from its name; None for one made elsewhere."""
    m = pattern.search(release.get("name", ""))
    return dt.datetime.strptime(m.group(1), "%Y-%m-%dT%H:%MZ").replace(tzinfo=dt.timezone.utc) if m else None


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
    ap.add_argument("--min-age", type=float, default=24.0, help="hours a build is on Closed testing before production gets it (auto)")
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
        if code and not args.dry_run and not release_notes.has_tag("android", f"0.1.{code}", code):
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
            started = named_at(staged[0], STAGED)
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
            tested = newest(testing, ("completed",))
            if tested and tested[0] > live_code:
                code, on_testing = tested
                since = named_at(on_testing, TESTED)  # None: put there by hand, or before this script
                if since is None or now() - since >= dt.timedelta(hours=args.min_age):
                    release = release_of(code, args.rollout, notes_between(shipped, release_notes.commit_of(code)))
                    message = f"Releasing {code} to {args.rollout:.0%} of production after its day on Closed testing"
                else:
                    summary(f"{code} reached Closed testing {since:%Y-%m-%d %H:%M} UTC; production gets it once it "
                            f"has been there {args.min_age:g} hours.")
            else:
                summary(f"Nothing to promote: production has {live_code or 'nothing'}, and Closed testing nothing newer.")

    # Moving up: Internal testing's newest build goes to Closed testing once production has Closed
    # testing's (or it has none). Waiting until then gives each build its whole day there.
    step = None
    if args.mode == "auto":
        out = max([live_code, *codes(release or {})])
        on_testing = (newest(testing, ("completed", "inProgress", "halted")) or (0, None))[0]
        internal = newest(play.track(INTERNAL), ("completed",))
        log(f"Internal testing: {internal[0] if internal else 'nothing'}")
        if internal and on_testing <= out and internal[0] > max(on_testing, out):
            code, source = internal
            step = {"versionCodes": [str(code)], "status": "completed", "name": f"0.1.{code} tested from {now():%Y-%m-%dT%H:%MZ}"}
            if source.get("releaseNotes"):
                step["releaseNotes"] = source["releaseNotes"]

    if release is None and step is None:
        play.abandon()
        emit_tag(shipped)
        return
    if args.dry_run:
        if release:
            summary(f"Dry run: {message}")
            log(json.dumps(release, indent=2))
        if step:
            summary(f"Dry run: moving {codes(step)[0]} up to Closed testing")
            log(json.dumps(step, indent=2))
        play.abandon()
        return
    if release:
        play.put(PRODUCTION, release)
    if step:
        play.put(TESTING, step)
    play.commit()
    if step:
        summary(f"Moved {codes(step)[0]} up to Closed testing; production gets it after {args.min_age:g} hours there.")
    if release is None:
        emit_tag(shipped)
        return
    new = live_code not in codes(release)
    # A staged rollout was already reviewed when started; finishing it reaches everyone now.
    emit_tag(max(codes(release)) if (not new and release.get("status") == "completed") else shipped)
    summary(f"{message}." + (" Google reviews it before it reaches anyone." if new else ""))
    if release.get("releaseNotes"):
        summary("What's new:\n\n" + release["releaseNotes"][0]["text"])

if __name__ == "__main__":
    main()
