#!/usr/bin/env python3
"""Promote the newest Internal testing build to Closed testing on Google Play.

Reads the service account's JSON key from PLAY_SERVICE_ACCOUNT_JSON. Does nothing when Closed
testing already has that build, so a run with nothing new is harmless. Run by the Promote to
Closed testing workflow (.github/workflows/play-promote.yml); needs google-auth and requests.

    play-promote.py [--dry-run]
"""

import argparse
import json
import os
import sys

from google.auth.transport.requests import AuthorizedSession
from google.oauth2 import service_account

PACKAGE = "org.orthodoxwest.office"
API = f"https://androidpublisher.googleapis.com/androidpublisher/v3/applications/{PACKAGE}/edits"
SOURCE, TARGET = "internal", "alpha"  # alpha is Closed testing
NOTES = "Fixes and improvements from the past week."


def check(resp):
    if not resp.ok:
        sys.exit(f"{resp.request.method} {resp.url}: {resp.status_code} {resp.text}")
    return resp.json() if resp.content else {}


def newest_release(track):
    """The track's newest completed release, by its highest version code."""
    releases = [r for r in track.get("releases", []) if r.get("status") == "completed" and r.get("versionCodes")]
    return max(releases, key=lambda r: max(int(c) for c in r["versionCodes"]), default=None)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dry-run", action="store_true", help="say what would be promoted, change nothing")
    args = parser.parse_args()

    key = json.loads(os.environ["PLAY_SERVICE_ACCOUNT_JSON"])
    creds = service_account.Credentials.from_service_account_info(
        key, scopes=["https://www.googleapis.com/auth/androidpublisher"]
    )
    play = AuthorizedSession(creds)

    edit = check(play.post(API))["id"]
    source = newest_release(check(play.get(f"{API}/{edit}/tracks/{SOURCE}")))
    if source is None:
        play.delete(f"{API}/{edit}")
        sys.exit("Internal testing has no completed release to promote")
    codes = source["versionCodes"]
    target = check(play.get(f"{API}/{edit}/tracks/{TARGET}"))
    on_target = {c for r in target.get("releases", []) for c in r.get("versionCodes", [])}
    if set(codes) <= on_target:
        play.delete(f"{API}/{edit}")
        print(f"Closed testing already has {source.get('name', codes)}; nothing to promote")
        return
    if args.dry_run:
        play.delete(f"{API}/{edit}")
        print(f"Would promote {source.get('name', codes)} (version codes {', '.join(codes)}) to Closed testing")
        return

    release = {
        "name": source.get("name", ""),
        "versionCodes": codes,
        "status": "completed",
        "releaseNotes": [{"language": "en-US", "text": NOTES}],
    }
    check(play.put(f"{API}/{edit}/tracks/{TARGET}", json={"track": TARGET, "releases": [release]}))
    check(play.post(f"{API}/{edit}:commit"))
    print(f"Promoted {release['name'] or codes} (version codes {', '.join(codes)}) to Closed testing")


if __name__ == "__main__":
    main()
