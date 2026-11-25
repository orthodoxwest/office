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
HELD = []


def run(argv, production, testing=TESTING, uploads=(), tagged=(), held=(), internal=None, hold_refused=None):
    """main()'s production release for these tracks and uploads, or None when it sends nothing.

    `uploads` are version codes; every version code's commit is "c<code>", except 999's, which is
    not on master. `held` is the version codes on the hold track, and Internal testing has what
    Closed testing has unless `internal` says otherwise. The step outputs written go to OUTPUTS,
    the builds put on the hold track to HELD."""
    sent = []
    OUTPUTS.clear()
    HELD.clear()

    class Play:
        def __init__(self, _key):
            pass

        def track(self, name):
            if name == "internal":
                return testing if internal is None else internal
            return testing if name == "alpha" else production

        def held(self):
            return [{"status": "completed", "versionCodes": [str(c)]} for c in held]

        def hold(self, code):
            if hold_refused is None:
                HELD.append(code)
            return hold_refused

        def set_production(self, release):
            sent.append(release)

        def abandon(self):
            pass

    with mock.patch.object(PROMOTE, "Play", Play), \
            mock.patch.object(PROMOTE, "uploads", lambda _hours: [(c, f"c{c}") for c in uploads]), \
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


class AutoTest(unittest.TestCase):
    def test_a_tested_build_goes_to_a_fifth_with_the_notes_since_production_and_start(self):
        release = run(["auto", "--rollout", "0.2"], [{"status": "completed", "versionCodes": ["100"]}],
                      uploads=[120, 110])
        self.assertEqual(release, {"versionCodes": ["120"], "status": "inProgress", "userFraction": 0.2,
                                   "releaseNotes": notes("c100", "c120"), "name": "0.1.120 staged 2026-11-12T15:00Z"})
        # Production's completed build is handed to the tag step, not the release just sent for review.
        self.assertEqual(OUTPUTS, [("live", "100")])

    def test_already_tagged_live_build_skips_tag_output(self):
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["120"]}],
                              uploads=[120], tagged={"0.1.120"}))
        self.assertEqual(OUTPUTS, [])

    def test_a_day_old_build_kept_on_the_hold_track_is_promoted(self):
        # Merges land more than once a day, so the build on Closed testing now (130) is never a day
        # old; 120, uploaded a day ago, replaced there and kept on the hold track, goes out with its
        # own notes, and 130 is held in its place.
        later = [{"status": "completed", "versionCodes": ["130"]}]
        release = run(["auto"], [{"status": "completed", "versionCodes": ["100"]}], testing=later,
                      uploads=[120, 110], held=[120])
        self.assertEqual(release["versionCodes"], ["120"])
        self.assertEqual(release["releaseNotes"], notes("c100", "c120"))
        self.assertEqual(HELD, [130])

    def test_a_build_no_track_holds_is_never_sent(self):
        # Play's API cannot release a bundle every track has let go of (2058, Nov 25): the newest
        # day-old build a track still holds goes instead, or none, and Closed testing's is held.
        later = [{"status": "completed", "versionCodes": ["130"]}]
        production = [{"status": "completed", "versionCodes": ["100"]}]
        self.assertIsNone(run(["auto"], production, testing=later, uploads=[120, 110]))
        self.assertEqual(HELD, [130])
        release = run(["auto"], production, testing=later, uploads=[120, 110], held=[110])
        self.assertEqual(release["versionCodes"], ["110"])
        release = run(["auto"], production, testing=later, uploads=[120, 110], internal=[
            {"status": "completed", "versionCodes": ["120"]}])
        self.assertEqual(release["versionCodes"], ["120"])

    def test_the_held_build_stays_until_it_goes_out(self):
        # 125 is held but not yet a day old: a newer merge does not take its place, or it never ages.
        later = [{"status": "completed", "versionCodes": ["130"]}]
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["100"]}], testing=later,
                              uploads=[], held=[125]))
        self.assertEqual(HELD, [])

    def test_a_refused_hold_warns_and_does_not_fail(self):
        later = [{"status": "completed", "versionCodes": ["130"]}]
        self.assertIsNone(run(["auto"], [{"status": "completed", "versionCodes": ["100"]}], testing=later,
                              uploads=[], hold_refused="403"))
        self.assertEqual(HELD, [])

    def test_a_staged_rollout_waiting_still_holds_the_newest(self):
        self.assertIsNone(run(["auto"], [staged(110, 23)], uploads=[120]))
        self.assertEqual(HELD, [120])

    def test_the_first_release_carries_every_note(self):
        release = run(["auto"], [], testing=[], uploads=[120], held=[120])
        self.assertEqual(release["versionCodes"], ["120"])
        self.assertEqual(release["releaseNotes"], notes(None, "c120"))

    def test_a_staged_rollout_is_finished_once_it_has_run_its_time(self):
        rollout = staged(110, 48) | {"releaseNotes": notes("c100", "c110")}
        release = run(["auto"], [{"status": "completed", "versionCodes": ["100"]}, rollout], uploads=[120])
        self.assertEqual(release["status"], "completed")
        self.assertEqual(release["versionCodes"], ["110"])
        self.assertEqual(release["releaseNotes"], notes("c100", "c110"))
        self.assertNotIn("userFraction", release)
        # Finishing an already-reviewed staged rollout makes 110 live now, so it is tagged right away.
        self.assertEqual(OUTPUTS, [("live", "110")])

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
        ), [(120, "old"), (110, "older")])


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

    def test_dry_run_sends_and_tags_nothing(self):
        self.assertIsNone(run(["manual", "--dry-run"], [{"status": "completed", "versionCodes": ["100"]}]))
        self.assertEqual(OUTPUTS, [])
        self.assertIsNone(run(["auto", "--dry-run"], [{"status": "completed", "versionCodes": ["100"]}], uploads=[]))
        self.assertEqual(HELD, [])

    def test_manual_runs_hold_nothing(self):
        run(["manual"], [{"status": "completed", "versionCodes": ["100"]}])
        self.assertEqual(HELD, [])

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
