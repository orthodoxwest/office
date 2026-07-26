#!/usr/bin/env python3
"""Tests for the mutation-threshold checker."""

import unittest

from check_mutation_threshold import assess


class MutationThresholdTest(unittest.TestCase):
    def setUp(self):
        self.report = {
            "test_efficacy": 87.5,
            "mutations_coverage": 93.5,
            "mutants_total": 100,
        }

    def test_accepts_scores_equal_to_floors(self):
        _, _, failures = assess(self.report, 87.5, 93.5)

        self.assertEqual(failures, [])

    def test_rejects_efficacy_below_floor(self):
        _, _, failures = assess(self.report, 88, 93)

        self.assertEqual(
            failures, ["efficacy 87.50% is below 88.00%"]
        )

    def test_rejects_mutant_coverage_below_floor(self):
        _, _, failures = assess(self.report, 87, 94)

        self.assertEqual(
            failures, ["mutant coverage 93.50% is below 94.00%"]
        )

    def test_rejects_report_without_runnable_mutants(self):
        self.report["mutants_total"] = 0

        with self.assertRaisesRegex(
            ValueError, "at least one runnable mutant"
        ):
            assess(self.report, 0, 0)

    def test_rejects_non_numeric_score(self):
        self.report["test_efficacy"] = "87.5"

        with self.assertRaisesRegex(ValueError, "must be numeric"):
            assess(self.report, 0, 0)

    def test_rejects_out_of_range_floor(self):
        with self.assertRaisesRegex(ValueError, "must be between 0 and 100"):
            assess(self.report, 101, 0)


if __name__ == "__main__":
    unittest.main()
