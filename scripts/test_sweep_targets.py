#!/usr/bin/env python3
"""Focused tests for sweeping idle Cargo target directories."""

import importlib.util
import os
import pathlib
import subprocess
import sys
import tempfile
import time
import unittest


SCRIPT = pathlib.Path(__file__).with_name("sweep-targets.py")
SPEC = importlib.util.spec_from_file_location("sweep_targets", SCRIPT)
SWEEP = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = SWEEP
SPEC.loader.exec_module(SWEEP)

OLD = time.time() - 7 * 24 * 3600


def git(*args, cwd, date=OLD):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True,
                   env={**os.environ, "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t",
                        "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t",
                        "GIT_COMMITTER_DATE": f"{date:.0f} +0000"})


def age(path):
    """Backdate everything under path, including git metadata."""
    for dirpath, dirnames, filenames in os.walk(path):
        for name in [*dirnames, *filenames, "."]:
            os.utime(pathlib.Path(dirpath) / name, (OLD, OLD), follow_symlinks=False)


class SweepTargetsTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.main = pathlib.Path(self.tmp.name) / "main"
        self.main.mkdir()
        git("init", "-q", cwd=self.main)
        (self.main / "README").write_text("x")
        git("add", "README", cwd=self.main)
        git("commit", "-qm", "init", cwd=self.main)
        self.nested = self.main / "worktrees" / "nested"
        git("worktree", "add", "-q", "-b", "nested", str(self.nested), cwd=self.main)
        for checkout in (self.main, self.nested):
            (checkout / "target" / "debug" / "deps").mkdir(parents=True)
            (checkout / "target" / "debug" / "deps" / "bin").write_bytes(b"0" * 10)
        age(pathlib.Path(self.tmp.name))

    def tearDown(self):
        self.tmp.cleanup()

    def sweep(self):
        return SWEEP.sweep(self.main, hours=72, log=lambda _: None)

    def test_idle_checkouts_are_swept(self):
        self.assertEqual(self.sweep(), 20)
        self.assertFalse((self.main / "target").exists())
        self.assertFalse((self.nested / "target").exists())

    def test_recent_build_keeps_target(self):
        os.utime(self.nested / "target" / "debug" / "deps" / "bin")
        self.sweep()
        self.assertTrue((self.nested / "target").exists())
        self.assertFalse((self.main / "target").exists())

    def test_nested_worktree_edits_do_not_keep_parent_target(self):
        (self.nested / "README").write_text("edited")
        self.sweep()
        self.assertTrue((self.nested / "target").exists())
        self.assertFalse((self.main / "target").exists())

    def test_recent_commit_keeps_target(self):
        git("commit", "-q", "--allow-empty", "-m", "recent", cwd=self.nested, date=time.time())
        age(self.nested)  # only the reflog entry is recent
        self.sweep()
        self.assertTrue((self.nested / "target").exists())
        self.assertFalse((self.main / "target").exists())

    def test_dry_run_deletes_nothing(self):
        SWEEP.sweep(self.main, hours=72, dry_run=True, log=lambda _: None)
        self.assertTrue((self.main / "target").exists())


if __name__ == "__main__":
    unittest.main()
