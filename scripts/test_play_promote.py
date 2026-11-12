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


def run(argv, production, testing=TESTING, uploads=(), notes=()):
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
            mock.patch.object(PROMOTE, "notes_at", lambda _sha: list(notes)), \
            mock.patch.dict(os.environ, {"PLAY_SERVICE_ACCOUNT_JSON": "{}"}), \
            mock.patch.object(sys, "argv", ["play-promote.py", *argv]), \
            contextlib.redirect_stdout(io.StringIO()):
        PROMOTE.main()
    return sent[0] if sent else None


class AutoTest(unittest.TestCase):
    def test_a_tested_build_goes_to_a_fifth_with_its_notes(self):
        release = run(["auto", "--rollout", "0.2"], [{"status": "completed", "versionCodes": ["100"]}],
                      uploads=[(120, "c120"), (110, "c110")])
        self.assertEqual(release, {"versionCodes": ["120"], "status": "inProgress", "userFraction": 0.2,
                                   "releaseNotes": NOTES})

    def test_a_build_since_replaced_on_testing_takes_the_notes_it_was_uploaded_with(self):
        old = [{"language": "en-US", "text": "Older notes."}]
        release = run(["auto"], [], uploads=[(118, "c118")], notes=old)
        self.assertEqual(release["versionCodes"], ["118"])
        self.assertEqual(release["releaseNotes"], old)

    def test_a_staged_rollout_is_finished_before_anything_newer(self):
        release = run(["auto"], [{"status": "completed", "versionCodes": ["100"]},
                                 {"status": "inProgress", "versionCodes": ["110"], "userFraction": 0.2}],
                      uploads=[(120, "c120")])
        self.assertEqual(release, {"versionCodes": ["110"], "status": "completed"})

    def test_a_halted_rollout_stops_promotion(self):
        self.assertIsNone(run(["auto"], [{"status": "halted", "versionCodes": ["110"], "userFraction": 0.2}],
                              uploads=[(120, "c120")]))

    def test_nothing_when_production_is_current(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["120"]}],
                              uploads=[(120, "c120")]))

    def test_nothing_until_a_build_is_old_enough(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["100"]}], uploads=[]))

    def test_a_build_never_on_testing_is_not_promoted(self):
        self.assertIsNone(run(["auto"], [], uploads=[(121, "c121")]))


class ManualTest(unittest.TestCase):
    def test_testings_newest_goes_to_everyone(self):
        release = run(["manual", "--rollout", "1"], [{"status": "completed", "versionCodes": ["100"]}])
        self.assertEqual(release, {"versionCodes": ["120"], "status": "completed", "releaseNotes": NOTES})

    def test_a_named_build_at_a_share(self):
        release = run(["manual", "--rollout", "0.5", "--version-code", "115"], [])
        self.assertEqual(release, {"versionCodes": ["115"], "status": "inProgress", "userFraction": 0.5})

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
