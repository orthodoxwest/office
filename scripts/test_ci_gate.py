#!/usr/bin/env python3
"""Tests for scripts/ci_gate.py's verdict and waiting, with GitHub stood in for, and for the
publishing workflows' use of it."""

import contextlib
import io
import pathlib
import re
import sys
import unittest
from unittest import mock

sys.path.insert(0, str(pathlib.Path(__file__).parent))
import ci_gate  # noqa: E402

WORKFLOWS = pathlib.Path(__file__).resolve().parent.parent / ".github/workflows"


def run(name, file, status="completed", conclusion="success", id=1):
    return {"id": id, "name": name, "path": f".github/workflows/{file}", "status": status,
            "conclusion": conclusion if status == "completed" else None, "html_url": f"https://x/{id}"}


GREEN = [run("CI", "ci.yml", id=1), run("iOS", "ios.yml", id=2), run("Android", "android.yml", id=3)]
GATE = dict(own_run="9", exclude=["play.yml", "testflight.yml"], require=["ci.yml"])


class Verdict(unittest.TestCase):
    def test_all_passed_is_green(self):
        self.assertEqual(ci_gate.verdict(GREEN, **GATE)[0], "green")

    def test_one_failure_is_red_even_while_others_run(self):
        runs = [run("CI", "ci.yml", status="in_progress"), run("iOS", "ios.yml", conclusion="failure", id=2)]
        self.assertEqual(ci_gate.verdict(runs, **GATE)[0], "red")

    def test_cancelled_and_timed_out_are_red(self):
        for conclusion in ("cancelled", "timed_out", "action_required", None):
            runs = GREEN[:2] + [run("Android", "android.yml", conclusion=conclusion, id=3)]
            self.assertEqual(ci_gate.verdict(runs, **GATE)[0], "red", conclusion)

    def test_a_running_check_is_waited_on(self):
        runs = GREEN[:2] + [run("Android", "android.yml", status="queued", id=3)]
        self.assertEqual(ci_gate.verdict(runs, **GATE)[0], "waiting")

    def test_own_run_and_publishers_are_not_waited_on(self):
        runs = GREEN + [
            run("Publish to Play", "play.yml", status="in_progress", id=4),
            run("iOS TestFlight", "testflight.yml", conclusion="failure", id=5),
            run("Android", "android.yml", status="in_progress", id=9),
        ]
        self.assertEqual(ci_gate.verdict(runs, **GATE)[0], "green")

    def test_a_required_check_that_has_not_started_is_waited_on(self):
        state, lines = ci_gate.verdict(GREEN[1:], **GATE)
        self.assertEqual(state, "waiting")
        self.assertIn("ci.yml: not started", lines)

    def test_no_runs_is_not_green(self):
        self.assertEqual(ci_gate.verdict([], own_run="", exclude=[], require=[])[0], "waiting")


class Waiting(unittest.TestCase):
    def gate(self, rounds, *args):
        out = io.StringIO()
        with mock.patch.object(ci_gate, "runs_for", side_effect=rounds), \
                mock.patch.object(ci_gate.time, "sleep"), contextlib.redirect_stdout(out):
            code = ci_gate.main(["--sha", "abc1234", "--require", "ci.yml", *args])
        return code, out.getvalue()

    def test_waits_until_green(self):
        running = GREEN[:2] + [run("Android", "android.yml", status="in_progress", id=3)]
        code, out = self.gate([running, running, GREEN])
        self.assertEqual(code, 0)
        self.assertIn("All checks passed", out)

    def test_stops_at_the_first_failure(self):
        failed = GREEN[:2] + [run("Android", "android.yml", conclusion="failure", id=3)]
        code, out = self.gate([failed])
        self.assertEqual(code, 1)
        self.assertIn("::error::", out)

    def test_a_busy_api_is_asked_again(self):
        code, _ = self.gate([ci_gate.urllib.error.URLError("busy"), GREEN])
        self.assertEqual(code, 0)

    def test_no_wait_reports_running(self):
        running = [run("CI", "ci.yml", status="in_progress")]
        self.assertEqual(self.gate([running], "--no-wait")[0], 2)

    def test_gives_up_when_nothing_ran(self):
        clock = iter(range(0, 10_000, 120))
        with mock.patch.object(ci_gate.time, "monotonic", side_effect=lambda: next(clock)):
            code, out = self.gate([[]] * 20)
        self.assertEqual(code, 1)
        self.assertIn("never started", out)


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
            self.assertIn("python3 scripts/ci_gate.py --require ci.yml --exclude play.yml --exclude testflight.yml", gate, workflow)
            self.assertIn("      actions: read\n", gate, workflow)

    def test_testflight_builds_without_the_gate_only_when_it_is_skipped(self):
        self.assertIn("needs.checks.result != 'failure'", self.job("testflight.yml", "build"))


if __name__ == "__main__":
    unittest.main()
