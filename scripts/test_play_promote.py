#!/usr/bin/env python3
"""Tests for the decisions of scripts/play-promote.py, with Google Play and GitHub stood in for."""

import contextlib
import importlib.util
import io
import os
import pathlib
import sys
import unittest
from unittest import mock

SCRIPT = pathlib.Path(__file__).with_name("play-promote.py")
SPEC = importlib.util.spec_from_file_location("play_promote", SCRIPT)
PROMOTE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = PROMOTE
SPEC.loader.exec_module(PROMOTE)

NOTES = [{"language": "en-US", "text": "Feast pages redesigned."}]
TESTING = [{"status": "completed", "versionCodes": ["120"], "releaseNotes": NOTES}]


NOW = PROMOTE.dt.datetime(2026, 11, 12, 15, 0, tzinfo=PROMOTE.dt.timezone.utc)


def run(argv, production, testing=TESTING, uploads=()):
    """main()'s production release for these tracks and uploads, or None when it sends nothing."""
    sent = []

    class Play:
        def __init__(self, _key):
            pass

        def track(self, name):
            return testing if name == "alpha" else production

        def set_production(self, release):
            sent.append(release)

        def abandon(self):
            pass

    with mock.patch.object(PROMOTE, "Play", Play), \
            mock.patch.object(PROMOTE, "uploads", lambda _hours: list(uploads)), \
            mock.patch.object(PROMOTE, "now", lambda: NOW), \
            mock.patch.dict(os.environ, {"PLAY_SERVICE_ACCOUNT_JSON": "{}"}), \
            mock.patch.object(sys, "argv", ["play-promote.py", *argv]), \
            contextlib.redirect_stdout(io.StringIO()):
        PROMOTE.main()
    return sent[0] if sent else None


def staged(code, hours_ago, name=None):
    at = NOW - PROMOTE.dt.timedelta(hours=hours_ago)
    return {"status": "inProgress", "versionCodes": [str(code)], "userFraction": 0.2,
            "name": name if name is not None else f"0.1.{code} staged {at:%Y-%m-%dT%H:%MZ}"}


class AutoTest(unittest.TestCase):
    def test_a_tested_build_goes_to_a_fifth_with_its_notes_and_start(self):
        release = run(["auto", "--rollout", "0.2"], [{"status": "completed", "versionCodes": ["100"]}],
                      uploads=[120, 110])
        self.assertEqual(release, {"versionCodes": ["120"], "status": "inProgress", "userFraction": 0.2,
                                   "releaseNotes": NOTES, "name": "0.1.120 staged 2026-11-12T15:00Z"})

    def test_only_the_build_on_testing_now(self):
        # 110 was replaced on Closed testing (perhaps while in review), so no tester has it.
        self.assertIsNone(run(["auto"], [], uploads=[110]))
        self.assertIsNone(run(["auto"], [], testing=[{"status": "inProgress", "versionCodes": ["120"]}], uploads=[120]))

    def test_a_staged_rollout_is_finished_once_it_has_run_its_time(self):
        release = run(["auto"], [{"status": "completed", "versionCodes": ["100"]}, staged(110, 48)], uploads=[120])
        self.assertEqual(release["status"], "completed")
        self.assertEqual(release["versionCodes"], ["110"])
        self.assertNotIn("userFraction", release)

    def test_a_young_staged_rollout_waits_and_holds_back_newer_builds(self):
        self.assertIsNone(run(["auto"], [staged(110, 23)], uploads=[120]))

    def test_a_staged_rollout_started_elsewhere_is_left_alone(self):
        self.assertIsNone(run(["auto"], [staged(110, 100, name="")], uploads=[120]))

    def test_a_halted_rollout_stops_promotion(self):
        self.assertIsNone(run(["auto"], [{"status": "halted", "versionCodes": ["110"], "userFraction": 0.2}],
                              uploads=[120]))

    def test_nothing_when_production_is_current(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["120"]}], uploads=[120]))

    def test_nothing_until_a_build_is_old_enough(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["100"]}], uploads=[]))


class UploadsTest(unittest.TestCase):
    def runs(self, *runs):
        body = io.BytesIO(PROMOTE.json.dumps({"workflow_runs": list(runs)}).encode())
        counts = {"old": "120", "older": "110"}

        def rev_list(cmd, **_kw):
            sha = cmd[-1]
            return mock.Mock(returncode=0 if sha in counts else 128, stdout=counts.get(sha, ""))

        with mock.patch.object(PROMOTE.urllib.request, "urlopen", lambda _req: contextlib.nullcontext(body)), \
                mock.patch.object(PROMOTE.subprocess, "run", rev_list), \
                mock.patch.object(PROMOTE, "now", lambda: NOW), \
                mock.patch.dict(os.environ, {"GITHUB_REPOSITORY": "o/r", "GITHUB_TOKEN": "t"}):
            return PROMOTE.uploads(24)

    def test_age_cutoff_count_and_order(self):
        self.assertEqual(self.runs(
            {"head_sha": "older", "updated_at": "2026-11-10T15:00:00Z"},
            {"head_sha": "new", "updated_at": "2026-11-12T01:00:00Z"},
            {"head_sha": "old", "updated_at": "2026-11-11T14:59:00Z"},
            {"head_sha": "gone", "updated_at": "2026-11-01T00:00:00Z"},
        ), [120, 110])


class ManualTest(unittest.TestCase):
    def test_testings_newest_goes_to_everyone(self):
        release = run(["manual", "--rollout", "1"], [{"status": "completed", "versionCodes": ["100"]}])
        self.assertEqual(release, {"versionCodes": ["120"], "status": "completed", "releaseNotes": NOTES})

    def test_a_named_build_at_a_share(self):
        release = run(["manual", "--rollout", "0.5", "--version-code", "115"], [])
        self.assertEqual(release, {"versionCodes": ["115"], "status": "inProgress", "userFraction": 0.5,
                                   "name": "0.1.115 staged 2026-11-12T15:00Z"})

    def test_never_older_than_production(self):
        with self.assertRaises(SystemExit), contextlib.redirect_stdout(io.StringIO()):
            run(["manual", "--version-code", "90"], [{"status": "completed", "versionCodes": ["100"]}])

    def test_dry_run_sends_nothing(self):
        self.assertIsNone(run(["manual", "--dry-run"], []))

    def test_rollout_out_of_range(self):
        with self.assertRaises(SystemExit):
            run(["manual", "--rollout", "1.5"], [])


if __name__ == "__main__":
    unittest.main()
