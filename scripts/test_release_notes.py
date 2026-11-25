#!/usr/bin/env python3
"""Tests for scripts/release_notes.py, on a scratch git repository."""

import contextlib
import io
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, str(pathlib.Path(__file__).parent))
import release_notes as RN  # noqa: E402


class ParseTest(unittest.TestCase):
    def test_both_platforms_by_default_and_lines_joined(self):
        self.assertEqual(RN.parse("The ordo\nscrolls to today.\n\nAnd more.\n"),
                         (frozenset({"android", "ios"}), "The ordo scrolls to today.\n\nAnd more."))

    def test_a_byte_order_mark_does_not_hide_the_platforms_line(self):
        self.assertEqual(RN.parse("\ufeffPlatforms: ios\n\nA fix."), (frozenset({"ios"}), "A fix."))

    def test_platforms_line(self):
        self.assertEqual(RN.parse("Platforms: Android\n\nA fix.")[0], frozenset({"android"}))
        self.assertEqual(RN.parse("platforms: ios, android\nA fix.")[0], frozenset({"android", "ios"}))

    def test_bad_notes(self):
        for text in ("Platforms: web\n\nA fix.", "Platforms:\n\nA fix.", "Platforms: ios\n", "  \n"):
            with self.assertRaises(RN.NoteError, msg=text):
                RN.parse(text)


class ComposeTest(unittest.TestCase):
    def test_none_is_the_standing_line(self):
        self.assertEqual(RN.compose([], 500), RN.DEFAULT_NOTES)

    def test_all_that_fit(self):
        self.assertEqual(RN.compose(["New.", "Old."], 500), "New.\n\nOld.")

    def test_older_notes_that_do_not_fit_give_way(self):
        new, old = "N" * 300, "O" * 300
        self.assertEqual(RN.compose([new, old], 500), new + "\n\n" + RN.MORE)
        self.assertEqual(RN.compose([new, old], 4000), new + "\n\n" + old)


class RepoTest(unittest.TestCase):
    """Notes added across a few commits of a scratch repository."""

    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)
        self.root = pathlib.Path(self.dir.name)
        patch = mock.patch.object(RN, "ROOT", self.root)
        patch.start()
        self.addCleanup(patch.stop)
        self.time = 1_700_000_000
        self.git("init", "-q", "-b", "master")
        self.git("config", "user.email", "t@example.com")
        self.git("config", "user.name", "T")
        self.git("config", "tag.gpgSign", "false")
        self.git("config", "commit.gpgSign", "false")

    def git(self, *args):
        env = os.environ | {"GIT_AUTHOR_DATE": f"{self.time} +0000", "GIT_COMMITTER_DATE": f"{self.time} +0000"}
        return subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True,
                              text=True, env=env).stdout.strip()

    def commit(self, files=None, remove=()):
        self.time += 60
        for name, text in (files or {}).items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text)
            self.git("add", name)
        for name in remove:
            self.git("rm", "-q", name)
        self.git("commit", "-q", "--allow-empty", "-m", "c")
        return self.git("rev-parse", "HEAD")

    def test_a_note_goes_out_once_on_each_platform(self):
        first = self.commit({"app.txt": "x"})
        a = self.commit({"release-notes/tap.txt": "Tap to explain."})
        b = self.commit({"release-notes/fix.txt": "Platforms: android\n\nA fix."})
        c = self.commit({"release-notes/ordo.txt": "The ordo scrolls."})
        # Android shipped b; iOS still has first.
        self.assertEqual(RN.notes_for("android", b, c), ["The ordo scrolls."])
        self.assertEqual(RN.notes_for("ios", first, c), ["The ordo scrolls.", "Tap to explain."])
        self.assertEqual(RN.notes_for("android", None, c), ["The ordo scrolls.", "A fix.", "Tap to explain."])
        self.assertEqual(RN.notes_for("android", c, c), [])
        self.assertEqual(RN.release_text("android", c, c), RN.DEFAULT_NOTES)
        self.assertEqual(RN.notes_for("ios", a, b), [])

    def test_a_note_removed_before_release_never_appears_and_an_edited_one_does_not_return(self):
        base = self.commit({"release-notes/tap.txt": "Tap to explain."})
        self.commit({"release-notes/tap.txt": "Tap to explain, now better."})
        self.commit({"release-notes/gone.txt": "Withdrawn."})
        head = self.commit(remove=["release-notes/gone.txt"])
        self.assertEqual(RN.notes_for("ios", base, head), [])

    def test_a_renamed_note_is_not_new(self):
        base = self.commit({"release-notes/old.txt": "Old note."})
        self.git("mv", "release-notes/old.txt", "release-notes/renamed.txt")
        head = self.commit({"release-notes/new.txt": "New note."})
        self.assertEqual(RN.notes_for("ios", base, head), ["New note."])

    def test_deleting_a_shipped_note_and_adding_a_similar_one_does_not_drop_the_new_note(self):
        base = self.commit({"release-notes/old.txt": "Posture cues now appear in the little hours of the office."})
        head = self.commit(
            {"release-notes/new.txt": "Posture cues now appear in Vespers and Lauds of the office."},
            remove=["release-notes/old.txt"],
        )
        self.assertEqual(RN.notes_for("ios", base, head),
                         ["Posture cues now appear in Vespers and Lauds of the office."])

    def test_renaming_an_unshipped_note_keeps_its_sort_order(self):
        base = self.commit({"app.txt": "x"})
        self.commit({"release-notes/older.txt": "Older."})
        self.commit({"release-notes/draft.txt": "Newer."})
        self.git("mv", "release-notes/draft.txt", "release-notes/final.txt")
        head = self.commit()
        self.assertEqual(RN.notes_for("ios", base, head), ["Newer.", "Older."])

    def test_broadening_platforms_delivers_to_the_new_platform_only(self):
        a = self.commit({"release-notes/feat.txt": "Platforms: android\n\nA feature."})
        b = self.commit({"release-notes/feat.txt": "A feature."})
        self.assertEqual(RN.notes_for("ios", a, b), ["A feature."])
        self.assertEqual(RN.notes_for("android", a, b), [])

    def test_spaces_and_non_ascii_filenames(self):
        base = self.commit({"app.txt": "x"})
        head = self.commit({
            "release-notes/my feature.txt": "Has a space.",
            "release-notes/café.txt": "Has an accent.",
        })
        self.assertEqual(set(RN.notes_for("ios", base, head)), {"Has a space.", "Has an accent."})
        self.assertEqual(RN.check(), [])
        self.assertEqual(RN.check(head), [])

    def test_a_note_merged_from_a_branch_sorts_by_the_merge(self):
        start = self.commit()
        self.git("checkout", "-q", "-b", "side")
        self.commit({"release-notes/early.txt": "Written early."})
        self.git("checkout", "-q", "master")
        self.commit({"release-notes/later.txt": "Written later."})
        self.time += 60
        self.git("merge", "-q", "--no-ff", "--no-edit", "side")
        self.assertEqual(RN.notes_for("ios", start, "HEAD"), ["Written early.", "Written later."])

    def test_a_base_git_cannot_read_stops_the_run(self):
        head = self.commit({"release-notes/one.txt": "One."})
        with self.assertRaises(SystemExit):
            RN.notes_for("ios", "0" * 40, head)

    def test_commit_of_counts_commits(self):
        shas = [self.commit() for _ in range(5)]
        self.assertEqual(RN.commit_of(3), shas[2])
        self.assertIsNone(RN.commit_of(9))

    def test_tags_record_releases_and_set_the_default_base(self):
        a = self.commit({"release-notes/one.txt": "One."})
        b = self.commit({"release-notes/two.txt": "Two."})
        self.assertIsNone(RN.latest_tag("ios"))
        self.assertFalse(RN.has_tag("ios", "1.0.0"))
        with contextlib.redirect_stdout(io.StringIO()):
            RN.record("ios", "1.0.0", a, push=False)
            RN.record("ios", "1.0.0", b, push=False)  # left as it is, with a warning
        self.assertTrue(RN.has_tag("ios", "1.0.0"))
        self.assertEqual(RN.latest_tag("ios"), a)
        self.assertIsNone(RN.latest_tag("android"))
        self.assertEqual(RN.notes_for("ios", RN.latest_tag("ios"), b), ["Two."])

    def test_check(self):
        self.commit({"release-notes/README.md": "About.", "release-notes/ok.txt": "Fine.",
                     "release-notes/ios-ok.txt": "Platforms: ios\n\n" + "i" * 600,
                     "release-notes/ios-long.txt": "Platforms: ios\n\n" + "i" * 4001,
                     "release-notes/long.txt": "x" * 501, "release-notes/web.txt": "Platforms: web\n\nNo.",
                     "release-notes/wrong.md": "Not a note.", "release-notes/sub/deep.txt": "Nested."})
        (self.root / "release-notes/latin1.txt").write_bytes("Caf\xe9.".encode("latin-1"))
        self.git("add", "release-notes/latin1.txt")
        self.commit()
        for problems in (RN.check(), RN.check("HEAD")):
            self.assertEqual(len(problems), 6, problems)
            self.assertTrue(any("deep.txt" in p for p in problems))
            self.assertTrue(any("latin1.txt" in p and "UTF-8" in p for p in problems))
            self.assertTrue(any("long.txt" in p for p in problems))
            self.assertTrue(any("ios-long.txt" in p for p in problems))
            self.assertTrue(any("web.txt" in p for p in problems))
            self.assertTrue(any("wrong.md" in p for p in problems))

    def test_check_catches_uncommitted_notes(self):
        self.commit({"release-notes/ok.txt": "Fine."})
        self.assertEqual(RN.check(), [])
        (self.root / "release-notes/uncommitted.txt").write_text("Platforms: web\n\nBad.")
        problems = RN.check()
        self.assertEqual(len(problems), 1, problems)
        self.assertIn("uncommitted.txt", problems[0])
        self.assertEqual(RN.check("HEAD"), [])


class CheckedInTest(unittest.TestCase):
    def test_the_notes_checked_in_pass(self):
        self.assertEqual(RN.check(), [])


if __name__ == "__main__":
    unittest.main()
