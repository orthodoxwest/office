#!/usr/bin/env python3
"""Regression tests for the unit-test coverage gate."""

import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from check_coverage_threshold import assess, main, read_coverage


class CoverageThresholdTest(unittest.TestCase):
    def test_weights_statements_instead_of_blocks_or_hit_counts(self):
        coverage = read_coverage("mode: count\nexample/p/a.go:1.1,2.2 9 0\nexample/p/a.go:3.1,4.2 1 50\n")
        self.assertEqual(coverage, {"example/p": (1, 10)})
        self.assertTrue(assess(coverage, {"example/p": 11})[1])

    def test_duplicate_blocks_count_once_and_merge_hits(self):
        coverage = read_coverage("mode: atomic\nexample/p/a.go:1.1,2.2 2 0\nexample/p/a.go:1.1,2.2 2 1\n")
        self.assertEqual(coverage, {"example/p": (2, 2)})

    def test_does_not_round_up_to_floor(self):
        self.assertTrue(assess({"p": (89999, 100000)}, {"p": 90})[1])
        self.assertEqual(assess({"p": (9, 10)}, {"p": 90})[1], [])

    def test_other_packages_cannot_hide_a_regression(self):
        _, failures = assess({"a": (1, 10), "b": (1000, 1000)}, {"a": 90, "b": 90})
        self.assertEqual(len(failures), 1)
        self.assertIn("a:", failures[0])

    def test_missing_empty_and_uncovered_packages_fail(self):
        for packages in ({}, {"p": (0, 0)}, {"p": (0, 10)}):
            with self.subTest(packages=packages):
                self.assertTrue(assess(packages, {"p": 90})[1])

    def test_rejects_invalid_profiles(self):
        for profile in ("", "mode: unknown", "mode: set\nbad", "mode: set\np.go:0.1,2.1 1 0",
                        "mode: set\np.go:3.1,2.1 1 0", "mode: set\np.go:1.1,2.1 1 -1",
                        "mode: set\np.go:1.1,2.1 1 0\np.go:1.1,2.1 2 0"):
            with self.subTest(profile=profile), self.assertRaises(ValueError):
                read_coverage(profile)

    def test_rejects_invalid_floors(self):
        for floors in ({}, [], {"p": True}, {"p": "90"}, {"p": -1}, {"p": 101}, {"p": float("nan")}, {"p": float("inf")}):
            with self.subTest(floors=floors), self.assertRaises(ValueError):
                assess({}, floors)

    def test_cli_exit_codes_and_ci_summary(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            profile, floors, summary = (root / name for name in ("unit.out", "floors.json", "summary.md"))
            floors.write_text(json.dumps({"p": 90}))
            args = [str(profile), "--floors", str(floors)]
            with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(summary)}), contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(main(args), 2)
                profile.write_text("mode: set\np/a.go:1.1,2.2 1 1\n")
                self.assertEqual(main(args), 0)
                self.assertIn("PASS", summary.read_text())
                profile.write_text("mode: set\np/a.go:1.1,2.2 1 0\n")
                self.assertEqual(main(args), 1)
                self.assertIn("FAIL", summary.read_text())
                profile.write_text("mode: set\n")
                self.assertEqual(main(args), 1)
                self.assertIn("MISSING", summary.read_text())


if __name__ == "__main__":
    unittest.main()
