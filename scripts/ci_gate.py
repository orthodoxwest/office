#!/usr/bin/env python3
"""Wait for a commit's CI to finish, and pass only if all of it passed.

Nothing goes out to readers from a commit whose checks are red or still running: the Play and
TestFlight uploads (play.yml, testflight.yml), the Android preview release (android.yml) and a
Fly deploy (`make deploy`) each run this first. It reads the workflow runs GitHub started for the
commit's push to master (CI, iOS, Android and the rest; which ones run depends on the paths the
commit touched) and waits until every one has finished. It exits 0 when all of them succeeded, and
1 as soon as one fails or times out, or when none has started for the commit.

A run that is still going counts by its jobs, less the --exclude-job ones: Android's run holds its
own gate and preview release, which say nothing about whether the commit is sound, so the other
publishers need not wait for them, nor stop when the release fails.

A run cancelled because a newer push to master took its place (iOS cancels its master runs; CI
and Android replace a queued one) is judged by the run that took its place: that commit holds
this one, so its checks cover this one's. A run cancelled with nothing newer is red.

  --sha SHA          The commit (default: $GITHUB_SHA).
  --exclude FILE     A workflow file (play.yml) not to wait on: the publishers, which wait on the
                     checks themselves, and so would wait on each other. Repeatable.
  --exclude-job NAME A job not to wait on in the runs that are waited on. Repeatable.
  --require FILE     A workflow file that must have run for the commit (ci.yml runs on every push
                     to master, so its absence means the commit was never checked). Repeatable.
  --timeout MIN      Give up after this many minutes (default 170).
  --no-wait          Report the state now instead of waiting: exit 0 green, 1 red, 2 still running.

The run this script is part of ($GITHUB_RUN_ID) is never waited on. In GitHub Actions it refuses
at once to gate anything but master ($GITHUB_REF). GITHUB_TOKEN (or GH_TOKEN) is used when set;
the repository is public, so a local run can go without one, at GitHub's 60 requests an hour.
"""

import argparse
import datetime as dt
import json
import os
import sys
import time
import urllib.error
import urllib.request

API = "https://api.github.com"
MASTER = "refs/heads/master"
PASSED = {"success", "skipped", "neutral"}
# How long to wait for a run that should exist (--require) before deciding it never started.
START_GRACE = 10 * 60
# How long to wait after GitHub refuses for rate limiting before asking again.
RATE_LIMIT_WAIT = 5 * 60


class Refused(Exception):
    """GitHub refused the request in a way waiting will not mend (bad token, unknown repository)."""


class Client:
    def __init__(self, repo, token):
        self.repo, self.token = repo, token

    def get(self, path):
        headers = {"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        request = urllib.request.Request(f"{API}/repos/{self.repo}/{path}", headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=30) as resp:
                return json.load(resp)
        except urllib.error.HTTPError as err:
            if err.code in (401, 404):
                raise Refused(f"GitHub answered {err.code} for {path}") from err
            raise

    def runs(self, sha):
        """The workflow runs GitHub started for sha's push, each in its latest attempt."""
        return self.get(f"actions/runs?head_sha={sha}&event=push&per_page=100")["workflow_runs"]

    def jobs(self, run_id):
        return self.get(f"actions/runs/{run_id}/jobs?per_page=100")["jobs"]

    def later_runs(self, run):
        """Runs of run's workflow for pushes to master after it, oldest first."""
        file = workflow_file(run)
        found = self.get(f"actions/workflows/{file}/runs?branch=master&event=push&per_page=30")["workflow_runs"]
        return sorted((r for r in found if r["created_at"] > run["created_at"]), key=lambda r: r["created_at"])


def stamp(text):
    return dt.datetime.fromisoformat(text.replace("Z", "+00:00"))


def workflow_file(run):
    """A run's workflow file name, such as ci.yml."""
    return run.get("path", "").split("@")[0].rsplit("/", 1)[-1]


def successor(client, run):
    """The run that took a cancelled run's place, or None if nothing newer was pushed. A newer
    one cancelled in turn hands on to its own successor."""
    # Concurrency cancels a run as the newer one is queued, so a run that took its place was
    # created by the time it ended; one created later did not, and the cancel was a person's.
    ended = stamp(run["updated_at"]) + dt.timedelta(minutes=2)
    later = [r for r in client.later_runs(run) if stamp(r["created_at"]) <= ended]
    for candidate in later:
        if candidate["status"] != "completed" or candidate["conclusion"] != "cancelled":
            return candidate
    return later[-1] if later else None


def judge(client, run, exclude_jobs, depth=0):
    """("passed" | "failed" | "running", what it is) for one run."""
    if run["status"] == "completed" and run["conclusion"] in PASSED:
        return "passed", run["conclusion"]
    if run["status"] == "completed" and run["conclusion"] == "cancelled":
        newer = successor(client, run) if depth < 5 else None
        if newer is None:
            return "failed", "cancelled"
        state, what = judge(client, newer, exclude_jobs, depth + 1)
        return state, f"cancelled for {newer['head_sha'][:7]}, whose run is {what}  {newer['html_url']}"
    every = client.jobs(run["id"])
    jobs = [j for j in every if j["name"] not in exclude_jobs]
    bad = [f"{j['name']} {j['conclusion']}" for j in jobs if j["status"] == "completed" and j["conclusion"] not in PASSED]
    if bad:
        return "failed", ", ".join(bad)
    # Judging by jobs needs an excluded job in the run: GitHub lists a job only once its
    # dependencies let it queue, so a run of checks alone can show every listed job finished while
    # the next has yet to appear. The excluded jobs (the gate, the release) come last or alone.
    excluded = len(jobs) < len(every)
    if run["status"] == "completed":
        if excluded:
            return "passed", f"{run['conclusion']} only in {', '.join(sorted(exclude_jobs))}"
        return "failed", run["conclusion"] or "unknown"
    if excluded and jobs and all(j["status"] == "completed" for j in jobs):
        return "passed", "its checks passed"
    return "running", run["status"]


def verdict(client, runs, own_run, exclude, require, exclude_jobs=()):
    """("green" | "red" | "waiting", lines describing each run waited on)."""
    watched = [r for r in runs if str(r["id"]) != str(own_run) and workflow_file(r) not in exclude]
    lines = []
    red = waiting = False
    for run in sorted(watched, key=lambda r: r["name"]):
        state, what = judge(client, run, set(exclude_jobs))
        red = red or state == "failed"
        waiting = waiting or state == "running"
        lines.append(f"{run['name']} ({workflow_file(run)}): {what}  {run['html_url']}")
    missing = sorted(set(require) - {workflow_file(r) for r in watched})
    lines += [f"{name}: not started" for name in missing]
    if red:
        return "red", lines
    if waiting or missing or not watched:
        return "waiting", lines
    return "green", lines


def main(argv=None, client=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--sha", default=os.environ.get("GITHUB_SHA"))
    parser.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY", "orthodoxwest/office"))
    parser.add_argument("--exclude", action="append", default=[])
    parser.add_argument("--exclude-job", action="append", default=[])
    parser.add_argument("--require", action="append", default=[])
    parser.add_argument("--timeout", type=float, default=170)
    parser.add_argument("--interval", type=float, default=60)
    parser.add_argument("--no-wait", action="store_true")
    args = parser.parse_args(argv)
    if not args.sha:
        parser.error("--sha is required outside GitHub Actions")
    ref = os.environ.get("GITHUB_REF", MASTER)
    if ref != MASTER:
        print(f"::error::Only master's commits are published, not {ref}'s.")
        return 1
    client = client or Client(args.repo, os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN"))
    own_run = os.environ.get("GITHUB_RUN_ID", "")

    start = time.monotonic()
    last = None
    while True:
        pause = args.interval
        try:
            state, lines = verdict(client, client.runs(args.sha), own_run, args.exclude, args.require, args.exclude_job)
        except Refused as err:
            print(f"::error::{err}; check the token and repository.")
            return 1
        except urllib.error.HTTPError as err:
            # 403 and 429 are GitHub's rate limits; anything else, a busy API. Neither is a verdict.
            state, lines = "waiting", [f"Could not read the runs: {err}"]
            if err.code in (403, 429):
                pause = max(pause, RATE_LIMIT_WAIT)
        except (urllib.error.URLError, TimeoutError, ValueError, KeyError) as err:
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
        time.sleep(pause)


if __name__ == "__main__":
    sys.exit(main())
