#!/usr/bin/env python3
"""Tests for scripts/ci_gate.py's verdict and waiting, with GitHub stood in for, and for the
publishing workflows' use of it."""

import contextlib
import io
import os
import pathlib
import re
import sys
import unittest
import urllib.error
from unittest import mock

sys.path.insert(0, str(pathlib.Path(__file__).parent))
import ci_gate  # noqa: E402

WORKFLOWS = pathlib.Path(__file__).resolve().parent.parent / ".github/workflows"
GATE_JOB = "Wait for the commit's checks"
RELEASE_JOB = "Publish android-preview"


def run(name, file, status="completed", conclusion="success", id=1, sha="a" * 40,
        created="2026-11-27T04:00:00Z", updated="2026-11-27T04:20:00Z"):
    return {"id": id, "name": name, "path": f".github/workflows/{file}", "status": status,
            "conclusion": conclusion if status == "completed" else None, "html_url": f"https://x/{id}",
            "head_sha": sha, "created_at": created, "updated_at": updated}


def job(name, status="completed", conclusion="success"):
    return {"name": name, "status": status, "conclusion": conclusion if status == "completed" else None}


class Stand(ci_gate.Client):
    """GitHub as a list of rounds of the commit's runs, the jobs of each run, and later runs."""

    def __init__(self, rounds, jobs=None, later=None):
        self.rounds, self.job_lists, self.later = list(rounds), jobs or {}, later or {}

    def runs(self, sha):
        answer = self.rounds.pop(0) if len(self.rounds) > 1 else self.rounds[0]
        if isinstance(answer, Exception):
            raise answer
        return answer

    def jobs(self, run_id):
        return self.job_lists.get(run_id, [])

    def later_runs(self, run):
        return self.later.get(run["id"], [])


GREEN = [run("CI", "ci.yml", id=1), run("iOS", "ios.yml", id=2), run("Android", "android.yml", id=3)]
GATE = dict(own_run="9", exclude=["play.yml", "testflight.yml"], require=["ci.yml"],
            exclude_jobs=[GATE_JOB, RELEASE_JOB])


def verdict(runs, jobs=None, later=None):
    return ci_gate.verdict(Stand([runs], jobs, later), runs, **GATE)


class Verdict(unittest.TestCase):
    def test_all_passed_is_green(self):
        self.assertEqual(verdict(GREEN)[0], "green")

    def test_one_failure_is_red_even_while_others_run(self):
        runs = [run("CI", "ci.yml", status="in_progress"), run("iOS", "ios.yml", conclusion="failure", id=2)]
        self.assertEqual(verdict(runs, jobs={2: [job("Test", conclusion="failure")]})[0], "red")

    def test_failed_and_timed_out_are_red(self):
        for conclusion in ("failure", "timed_out", "action_required", "startup_failure"):
            runs = GREEN[:2] + [run("Android", "android.yml", conclusion=conclusion, id=3)]
            self.assertEqual(verdict(runs, jobs={3: [job("Build", conclusion=conclusion)]})[0], "red", conclusion)

    def test_a_failed_run_without_jobs_is_red(self):
        runs = GREEN[:2] + [run("Android", "android.yml", conclusion="startup_failure", id=3)]
        self.assertEqual(verdict(runs)[0], "red")

    def test_a_running_check_is_waited_on(self):
        runs = GREEN[:2] + [run("Android", "android.yml", status="queued", id=3)]
        self.assertEqual(verdict(runs)[0], "waiting")

    def test_own_run_and_publishers_are_not_waited_on(self):
        runs = GREEN + [
            run("Publish to Play", "play.yml", status="in_progress", id=4),
            run("iOS TestFlight", "testflight.yml", conclusion="failure", id=5),
            run("Android", "android.yml", status="in_progress", id=9),
        ]
        self.assertEqual(verdict(runs)[0], "green")

    def test_androids_gate_and_release_are_not_waited_on(self):
        android = run("Android", "android.yml", status="in_progress", id=3)
        jobs = {3: [job("Build preview APK"), job(GATE_JOB, status="in_progress")]}
        self.assertEqual(verdict(GREEN[:2] + [android], jobs)[0], "green")
        jobs = {3: [job("Build preview APK", status="in_progress"), job(GATE_JOB, status="in_progress")]}
        self.assertEqual(verdict(GREEN[:2] + [android], jobs)[0], "waiting")

    def test_a_failed_release_does_not_stop_the_others(self):
        android = run("Android", "android.yml", conclusion="failure", id=3)
        jobs = {3: [job("Build preview APK"), job(GATE_JOB), job(RELEASE_JOB, conclusion="failure")]}
        self.assertEqual(verdict(GREEN[:2] + [android], jobs)[0], "green")

    def test_a_run_between_jobs_is_not_taken_for_finished(self):
        # GitHub lists a job only once it can queue: iOS between its first job and the next.
        ios = run("iOS", "ios.yml", status="in_progress", id=2)
        self.assertEqual(verdict([GREEN[0], ios, GREEN[2]], {2: [job("Decide")]})[0], "waiting")

    def test_a_required_check_that_has_not_started_is_waited_on(self):
        state, lines = verdict(GREEN[1:])
        self.assertEqual(state, "waiting")
        self.assertIn("ci.yml: not started", lines)

    def test_no_runs_is_not_green(self):
        self.assertEqual(ci_gate.verdict(Stand([[]]), [], own_run="", exclude=[], require=[])[0], "waiting")


class Superseded(unittest.TestCase):
    """A run cancelled for a newer push is judged by the run that took its place."""

    cancelled = run("iOS", "ios.yml", conclusion="cancelled", id=2, updated="2026-11-27T04:20:00Z")

    def newer(self, conclusion="success", status="completed", id=20, created="2026-11-27T04:19:58Z",
              updated="2026-11-27T04:40:00Z"):
        return run("iOS", "ios.yml", status=status, conclusion=conclusion, id=id, sha="b" * 40,
                   created=created, updated=updated)

    def test_judged_by_the_newer_run(self):
        runs = [GREEN[0], self.cancelled, GREEN[2]]
        self.assertEqual(verdict(runs, later={2: [self.newer()]})[0], "green")
        failed = self.newer("failure")
        self.assertEqual(verdict(runs, jobs={20: [job("Test", conclusion="failure")]}, later={2: [failed]})[0], "red")
        self.assertEqual(verdict(runs, later={2: [self.newer(status="in_progress")]})[0], "waiting")

    def test_a_chain_of_cancels_is_followed(self):
        runs = [GREEN[0], self.cancelled, GREEN[2]]
        middle = self.newer("cancelled", id=20, updated="2026-11-27T04:30:01Z")
        last = self.newer(id=21, created="2026-11-27T04:30:00Z")
        later = {2: [middle], 20: [last]}
        self.assertEqual(verdict(runs, later=later)[0], "green")

    def test_cancelled_with_nothing_newer_is_red(self):
        self.assertEqual(verdict([GREEN[0], self.cancelled, GREEN[2]])[0], "red")

    def test_a_push_long_after_a_cancel_does_not_count(self):
        hours_later = self.newer(created="2026-11-27T09:00:00Z")
        self.assertEqual(verdict([GREEN[0], self.cancelled, GREEN[2]], later={2: [hours_later]})[0], "red")


class Waiting(unittest.TestCase):
    def gate(self, rounds, *args, jobs=None, env=None):
        out = io.StringIO()
        with mock.patch.object(ci_gate.time, "sleep"), contextlib.redirect_stdout(out), \
                mock.patch.dict(os.environ, env or {}, clear=False):
            os.environ.pop("GITHUB_REF", None) if env is None else None
            code = ci_gate.main(["--sha", "abc1234", "--require", "ci.yml", *args], client=Stand(rounds, jobs))
        return code, out.getvalue()

    def test_waits_until_green(self):
        running = GREEN[:2] + [run("Android", "android.yml", status="in_progress", id=3)]
        code, out = self.gate([running, running, GREEN])
        self.assertEqual(code, 0)
        self.assertIn("All checks passed", out)

    def test_stops_at_the_first_failure(self):
        failed = GREEN[:2] + [run("Android", "android.yml", conclusion="failure", id=3)]
        code, out = self.gate([failed], jobs={3: [job("Build", conclusion="failure")]})
        self.assertEqual(code, 1)
        self.assertIn("::error::", out)

    def test_a_busy_api_is_asked_again(self):
        self.assertEqual(self.gate([urllib.error.URLError("busy"), GREEN])[0], 0)

    def test_a_rate_limit_is_waited_out(self):
        limited = urllib.error.HTTPError("u", 403, "rate limited", {}, None)
        with mock.patch.object(ci_gate.time, "sleep") as sleep:
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                os.environ.pop("GITHUB_REF", None)
                code = ci_gate.main(["--sha", "abc1234"], client=Stand([limited, GREEN]))
        self.assertEqual(code, 0)
        sleep.assert_called_once_with(ci_gate.RATE_LIMIT_WAIT)

    def test_a_refusal_stops_at_once(self):
        code, out = self.gate([ci_gate.Refused("GitHub answered 401"), GREEN])
        self.assertEqual(code, 1)
        self.assertIn("401", out)

    def test_only_master_is_gated(self):
        code, out = self.gate([GREEN], env={"GITHUB_REF": "refs/heads/topic"})
        self.assertEqual(code, 1)
        self.assertIn("refs/heads/topic", out)

    def test_no_wait_reports_running(self):
        running = [run("CI", "ci.yml", status="in_progress")]
        self.assertEqual(self.gate([running], "--no-wait")[0], 2)

    def test_gives_up_when_nothing_ran(self):
        clock = iter(range(0, 10_000, 120))
        with mock.patch.object(ci_gate.time, "monotonic", side_effect=lambda: next(clock)):
            code, out = self.gate([[]])
        self.assertEqual(code, 1)
        self.assertIn("never started", out)

    def test_gives_up_at_the_timeout(self):
        clock = iter(range(0, 100_000, 600))
        running = GREEN[:2] + [run("Android", "android.yml", status="in_progress", id=3)]
        with mock.patch.object(ci_gate.time, "monotonic", side_effect=lambda: next(clock)):
            code, out = self.gate([running], "--timeout", "30")
        self.assertEqual(code, 1)
        self.assertIn("still running after 30 minutes", out)


class Workflows(unittest.TestCase):
    """Every job that publishes waits on the gate."""

    def job(self, workflow, name):
        """A job's block of the workflow's text, from its key to the next job's."""
        text = (WORKFLOWS / workflow).read_text()
        return re.search(rf"(?ms)^  {name}:\n(.*?)(?=^  [a-z-]+:\n|\Z)", text.split("\njobs:\n", 1)[1])[1]

    def test_publishers_need_the_gate(self):
        for workflow, job in [("play.yml", "publish"), ("testflight.yml", "build"), ("android.yml", "publish")]:
            needs = re.search(r"(?m)^    needs: (.*)$", self.job(workflow, job))[1]
            self.assertIn("checks", needs, f"{workflow} {job}")
            gate = self.job(workflow, "checks")
            self.assertIn(f"name: {GATE_JOB}", gate, workflow)
            command = re.search(r"(?m)^        run: (python3 scripts/ci_gate\.py .*)$", gate)[1]
            for part in ("--require ci.yml", "--exclude play.yml", "--exclude testflight.yml",
                         f'--exclude-job "{GATE_JOB}"', f'--exclude-job "{RELEASE_JOB}"'):
                self.assertIn(part, command, workflow)
            self.assertIn("      actions: read\n", gate, workflow)
        self.assertIn(f"name: {RELEASE_JOB}", self.job("android.yml", "publish"))

    def test_testflight_uploads_only_after_the_gate_passed(self):
        self.assertIn("needs.checks.result == 'success' || needs.checks.result == 'skipped'",
                      self.job("testflight.yml", "build"))


if __name__ == "__main__":
    unittest.main()
