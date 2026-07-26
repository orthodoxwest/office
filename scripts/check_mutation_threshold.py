#!/usr/bin/env python3
"""Fail when a Gremlins JSON report falls below mutation-score floors."""

import argparse
import json
import math
import sys
from pathlib import Path


def _score(report, key):
    value = report.get(key)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError(f"report field {key!r} must be numeric")
    value = float(value)
    if not math.isfinite(value) or not 0 <= value <= 100:
        raise ValueError(f"report field {key!r} must be between 0 and 100")
    return value


def _minimum(value, name):
    if not math.isfinite(value) or not 0 <= value <= 100:
        raise ValueError(f"{name} must be between 0 and 100")
    return value


def assess(report, min_efficacy, min_mutant_coverage):
    """Return measured scores and any failed threshold descriptions."""
    efficacy = _score(report, "test_efficacy")
    mutant_coverage = _score(report, "mutations_coverage")
    min_efficacy = _minimum(min_efficacy, "minimum efficacy")
    min_mutant_coverage = _minimum(
        min_mutant_coverage, "minimum mutant coverage"
    )
    mutants_total = report.get("mutants_total")
    if (
        isinstance(mutants_total, bool)
        or not isinstance(mutants_total, int)
        or mutants_total <= 0
    ):
        raise ValueError("report must contain at least one runnable mutant")

    failures = []
    if efficacy < min_efficacy:
        failures.append(
            f"efficacy {efficacy:.2f}% is below {min_efficacy:.2f}%"
        )
    if mutant_coverage < min_mutant_coverage:
        failures.append(
            "mutant coverage "
            f"{mutant_coverage:.2f}% is below {min_mutant_coverage:.2f}%"
        )
    return efficacy, mutant_coverage, failures


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description="Check a Gremlins JSON report against mutation-score floors."
    )
    parser.add_argument("report", type=Path)
    parser.add_argument("--min-efficacy", required=True, type=float)
    parser.add_argument("--min-mutant-coverage", required=True, type=float)
    return parser.parse_args(argv)


def main(argv=None):
    args = parse_args(argv)
    try:
        report = json.loads(args.report.read_text(encoding="utf-8"))
        efficacy, mutant_coverage, failures = assess(
            report, args.min_efficacy, args.min_mutant_coverage
        )
    except (OSError, json.JSONDecodeError, ValueError) as error:
        print(f"Invalid mutation report: {error}", file=sys.stderr)
        return 2

    print(
        f"Mutation efficacy: {efficacy:.2f}% "
        f"(minimum {args.min_efficacy:.2f}%)"
    )
    print(
        f"Mutant coverage: {mutant_coverage:.2f}% "
        f"(minimum {args.min_mutant_coverage:.2f}%)"
    )
    if failures:
        for failure in failures:
            print(f"Mutation ratchet failed: {failure}", file=sys.stderr)
        return 1

    print("Mutation ratchet passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
