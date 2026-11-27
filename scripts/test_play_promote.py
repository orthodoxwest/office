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

TESTING = [{"status": "completed", "versionCodes": ["120"]}]


def notes(base, head):
    """The release notes the stand-in release_notes gives for the notes added from base to head."""
    return [{"language": "en-US", "text": f"Notes {base}..{head}"}]


NOW = PROMOTE.dt.datetime(2026, 11, 12, 15, 0, tzinfo=PROMOTE.dt.timezone.utc)


OUTPUTS = []
MOVED = []


def tested(code, hours_ago, name=None):
    """Closed testing's release of `code`, moved up by this script `hours_ago`."""
    at = NOW - PROMOTE.dt.timedelta(hours=hours_ago)
    return [{"status": "completed", "versionCodes": [str(code)],
             "name": name if name is not None else f"0.1.{code} tested from {at:%Y-%m-%dT%H:%MZ}"}]


def run(argv, production, testing=TESTING, internal=(), tagged=()):
    """main()'s production release for these tracks, or None when it sends nothing.

    `internal` is Internal testing's version codes. Every version code's commit is "c<code>",
    except 999's, which is not on master. The step outputs written go to OUTPUTS, and what moved up
    to Closed testing to MOVED."""
    sent = []
    OUTPUTS.clear()
    MOVED.clear()
    staged_edit = {}

    class Play:
        def __init__(self, _key):
            pass

        def track(self, name):
            if name == "internal":
                return [{"status": "completed", "versionCodes": [str(c)], "releaseNotes": notes("i", f"c{c}")}
                        for c in internal]
            return testing if name == "alpha" else production

        def put(self, track, release):
            staged_edit[track] = release

        def commit(self):
            if "production" in staged_edit:
                sent.append(staged_edit["production"])
            if "alpha" in staged_edit:
                MOVED.append(staged_edit["alpha"])

        def abandon(self):
            staged_edit.clear()

    with mock.patch.object(PROMOTE, "Play", Play), \
            mock.patch.object(PROMOTE.release_notes, "commit_of", lambda n: None if n == 999 else f"c{n}"), \
            mock.patch.object(PROMOTE.release_notes, "has_tag", lambda _p, v, _b: v in tagged), \
            mock.patch.object(PROMOTE.release_notes, "release_text", lambda _p, base, head: notes(base, head)[0]["text"]), \
            mock.patch.object(PROMOTE, "output", lambda k, v: OUTPUTS.append((k, v))), \
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


def moved():
    return [r["versionCodes"][0] for r in MOVED]


COMPLETED_100 = [{"status": "completed", "versionCodes": ["100"]}]


class AutoTest(unittest.TestCase):
    def test_a_day_on_closed_testing_then_a_fifth_with_the_notes_since_production(self):
        release = run(["auto", "--rollout", "0.2"], COMPLETED_100, testing=tested(120, 24))
        self.assertEqual(release, {"versionCodes": ["120"], "status": "inProgress", "userFraction": 0.2,
                                   "releaseNotes": notes("c100", "c120"), "name": "0.1.120 staged 2026-11-12T15:00Z"})
        # Production's completed build is handed to the tag step, not the release just sent for review.
        self.assertEqual(OUTPUTS, [("live", "100")])

    def test_a_build_younger_than_a_day_on_closed_testing_waits(self):
        self.assertIsNone(run(["auto"], COMPLETED_100, testing=tested(120, 23), internal=[130]))
        # It keeps its place: a newer merge would otherwise restart its day, every day.
        self.assertEqual(moved(), [])

    def test_a_build_put_on_closed_testing_elsewhere_counts_as_a_day_old(self):
        release = run(["auto"], COMPLETED_100, testing=tested(120, 1, name=""))
        self.assertEqual(release["versionCodes"], ["120"])

    def test_internals_newest_moves_up_once_production_has_closed_testings(self):
        release = run(["auto"], COMPLETED_100, testing=tested(120, 30), internal=[130])
        self.assertEqual(release["versionCodes"], ["120"])
        self.assertEqual(MOVED, [{"versionCodes": ["130"], "status": "completed",
                                  "name": "0.1.130 tested from 2026-11-12T15:00Z",
                                  "releaseNotes": notes("i", "c130")}])
        # Nothing for production, but Closed testing's build is already out: the next one moves up.
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["120"]}],
                              testing=tested(120, 50), internal=[130]))
        self.assertEqual(moved(), ["130"])
        # An empty Closed testing takes Internal's newest.
        self.assertIsNone(run(["auto"], COMPLETED_100, testing=[], internal=[130]))
        self.assertEqual(moved(), ["130"])

    def test_nothing_moves_up_when_internal_has_nothing_newer(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["120"]}],
                              testing=tested(120, 50), internal=[120]))
        self.assertEqual(MOVED, [])

    def test_already_tagged_live_build_skips_tag_output(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["120"]}],
                              testing=tested(120, 50), tagged={"0.1.120"}))
        self.assertEqual(OUTPUTS, [])

    def test_the_first_release_carries_every_note(self):
        release = run(["auto"], [], testing=tested(120, 30))
        self.assertEqual(release["versionCodes"], ["120"])
        self.assertEqual(release["releaseNotes"], notes(None, "c120"))

    def test_a_staged_rollout_is_finished_once_it_has_run_its_time(self):
        rollout = staged(110, 48) | {"releaseNotes": notes("c100", "c110")}
        release = run(["auto"], [COMPLETED_100[0], rollout], testing=tested(120, 30))
        self.assertEqual(release["status"], "completed")
        self.assertEqual(release["versionCodes"], ["110"])
        self.assertEqual(release["releaseNotes"], notes("c100", "c110"))
        self.assertNotIn("userFraction", release)
        # Finishing an already-reviewed staged rollout makes 110 live now, so it is tagged right away.
        self.assertEqual(OUTPUTS, [("live", "110")])

    def test_a_young_staged_rollout_waits_and_holds_back_newer_builds(self):
        self.assertIsNone(run(["auto"], [staged(110, 23)], testing=tested(120, 30)))

    def test_a_staged_rollout_lets_the_next_build_start_its_day(self):
        # 110 is out to a fifth and still on Closed testing, so 130 starts its day there meanwhile.
        self.assertIsNone(run(["auto"], [staged(110, 23)], testing=tested(110, 50), internal=[130]))
        self.assertEqual(moved(), ["130"])

    def test_a_staged_rollout_started_elsewhere_is_left_alone(self):
        self.assertIsNone(run(["auto"], [staged(110, 100, name="")], testing=tested(120, 30)))

    def test_a_halted_rollout_stops_promotion(self):
        self.assertIsNone(run(["auto"], [{"status": "halted", "versionCodes": ["110"], "userFraction": 0.2}],
                              testing=tested(120, 30)))

    def test_nothing_when_production_is_current(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["120"]}], testing=tested(120, 30)))

    def test_dry_run_changes_nothing(self):
        self.assertIsNone(run(["auto", "--dry-run"], COMPLETED_100, testing=tested(120, 30), internal=[130]))
        self.assertEqual((MOVED, OUTPUTS), ([], []))


class ManualTest(unittest.TestCase):
    def test_testings_newest_goes_to_everyone(self):
        release = run(["manual", "--rollout", "1"], [{"status": "completed", "versionCodes": ["100"]}])
        self.assertEqual(release, {"versionCodes": ["120"], "status": "completed", "releaseNotes": notes("c100", "c120")})

    def test_manual_notes_override(self):
        release = run(["manual", "--rollout", "1", "--notes", "Custom release note."],
                      [{"status": "completed", "versionCodes": ["100"]}])
        self.assertEqual(release["releaseNotes"], [{"language": "en-US", "text": "Custom release note."}])

    def test_manual_notes_over_limit_rejected(self):
        with self.assertRaises(SystemExit):
            run(["manual", "--rollout", "1", "--notes", "x" * 501], [])

    def test_a_named_build_at_a_share(self):
        release = run(["manual", "--rollout", "0.5", "--version-code", "115"], [])
        self.assertEqual(release, {"versionCodes": ["115"], "status": "inProgress", "userFraction": 0.5,
                                   "releaseNotes": notes(None, "c115"), "name": "0.1.115 staged 2026-11-12T15:00Z"})

    def test_never_older_than_production(self):
        with self.assertRaises(SystemExit), contextlib.redirect_stdout(io.StringIO()):
            run(["manual", "--version-code", "90"], [{"status": "completed", "versionCodes": ["100"]}])

    def test_manual_runs_move_nothing_up(self):
        run(["manual"], COMPLETED_100, internal=[130])
        self.assertEqual(MOVED, [])

    def test_dry_run_sends_and_tags_nothing(self):
        self.assertIsNone(run(["manual", "--dry-run"], [{"status": "completed", "versionCodes": ["100"]}]))
        self.assertEqual(OUTPUTS, [])

    def test_replacing_a_staged_or_halted_release_carries_its_notes_again(self):
        # 110 reached a fifth of users (or none, if Google rejected it); 120's notes count from 100.
        for status in ("inProgress", "halted"):
            production = [{"status": "completed", "versionCodes": ["100"]},
                          {"status": status, "versionCodes": ["110"], "userFraction": 0.2}]
            release = run(["manual", "--rollout", "1"], production)
            self.assertEqual(release["releaseNotes"], notes("c100", "c120"), status)

    def test_a_build_not_on_master_gets_the_standing_line(self):
        release = run(["manual", "--version-code", "999"], [{"status": "completed", "versionCodes": ["100"]}])
        self.assertEqual(release["releaseNotes"][0]["text"], PROMOTE.release_notes.DEFAULT_NOTES)

    def test_rollout_out_of_range(self):
        with self.assertRaises(SystemExit):
            run(["manual", "--rollout", "1.5"], [])


if __name__ == "__main__":
    unittest.main()
