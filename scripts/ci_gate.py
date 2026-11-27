#!/usr/bin/env python3
"""Wait for a commit's CI to finish, and pass only if all of it passed.

Nothing goes out to readers from a commit whose checks are red or still running: the Play and
TestFlight uploads (play.yml, testflight.yml), the Android preview release (android.yml) and a
Fly deploy (`make deploy`) each run this first. It reads the workflow runs GitHub started for the
commit's push to master (CI, iOS, Android and the rest; which ones run depends on the paths the
commit touched) and waits until every one has finished. It exits 0 when all of them succeeded, and
1 as soon as one fails, is cancelled or times out, or when none has started for the commit.

  --sha SHA          The commit (default: $GITHUB_SHA).
  --exclude FILE     A workflow file (play.yml) not to wait on: the publishers, which wait on the
                     checks themselves, and so would wait on each other. Repeatable.
  --require FILE     A workflow file that must have run for the commit (ci.yml runs on every push
                     to master, so its absence means the commit was never checked). Repeatable.
  --timeout MIN      Give up after this many minutes (default 170).
  --no-wait          Report the state now instead of waiting: exit 0 green, 1 red, 2 still running.

The run this script is part of ($GITHUB_RUN_ID) is never waited on. GITHUB_TOKEN (or GH_TOKEN) is
used when set; the repository is public, so a local run can go without one.
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

API = "https://api.github.com"
PASSED = {"success", "skipped", "neutral"}
# How long to wait for a run that should exist (--require) before deciding it never started.
START_GRACE = 10 * 60


def runs_for(repo, sha, token):
    """The workflow runs GitHub started for sha's push, each in its latest attempt."""
    url = f"{API}/repos/{repo}/actions/runs?head_sha={sha}&event=push&per_page=100"
    headers = {"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=30) as resp:
        return json.load(resp)["workflow_runs"]


def workflow_file(run):
    """A run's workflow file name, such as ci.yml."""
    return run.get("path", "").split("@")[0].rsplit("/", 1)[-1]


def verdict(runs, own_run, exclude, require):
    """("green" | "red" | "waiting", lines describing each run waited on)."""
    watched = [r for r in runs if str(r["id"]) != str(own_run) and workflow_file(r) not in exclude]
    lines = []
    red = waiting = False
    for run in sorted(watched, key=lambda r: r["name"]):
        if run["status"] != "completed":
            state, waiting = run["status"], True
        else:
            state = run["conclusion"] or "unknown"
            red = red or state not in PASSED
        lines.append(f"{run['name']} ({workflow_file(run)}): {state}  {run['html_url']}")
    missing = sorted(set(require) - {workflow_file(r) for r in watched})
    lines += [f"{name}: not started" for name in missing]
    if red:
        return "red", lines
    if waiting or missing or not watched:
        return "waiting", lines
    return "green", lines


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--sha", default=os.environ.get("GITHUB_SHA"))
    parser.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY", "orthodoxwest/office"))
    parser.add_argument("--exclude", action="append", default=[])
    parser.add_argument("--require", action="append", default=[])
    parser.add_argument("--timeout", type=float, default=170)
    parser.add_argument("--interval", type=float, default=60)
    parser.add_argument("--no-wait", action="store_true")
    args = parser.parse_args(argv)
    if not args.sha:
        parser.error("--sha is required outside GitHub Actions")
    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")
    own_run = os.environ.get("GITHUB_RUN_ID", "")

    start = time.monotonic()
    last = None
    while True:
        try:
            state, lines = verdict(runs_for(args.repo, args.sha, token), own_run, args.exclude, args.require)
        except (urllib.error.URLError, TimeoutError, ValueError) as err:
            # A slow or busy API is no verdict; ask again on the next round.
            state, lines = "waiting", [f"Could not read the runs: {err}"]
        if lines != last:
            print(f"Checks for {args.sha[:7]}:", *(f"  {line}" for line in lines), sep="\n", flush=True)
            last = lines
        elapsed = time.monotonic() - start
        if state == "green":
            print("All checks passed.")
            return 0
        if state == "red":
            print(f"::error::A check for {args.sha[:7]} did not pass, so nothing is published from it.")
            return 1
        if args.no_wait:
            print("Checks are still running.")
            return 2
        if not lines and elapsed > START_GRACE:
            print(f"::error::No checks ran for {args.sha[:7]}; is it on master?")
            return 1
        if any(line.endswith(": not started") for line in lines) and elapsed > START_GRACE:
            print(f"::error::A required check never started for {args.sha[:7]}.")
            return 1
        if elapsed > args.timeout * 60:
            print(f"::error::The checks for {args.sha[:7]} were still running after {args.timeout:g} minutes.")
            return 1
        time.sleep(args.interval)


if __name__ == "__main__":
    sys.exit(main())
