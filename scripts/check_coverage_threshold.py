#!/usr/bin/env python3
"""Enforce package-local statement coverage from a Go coverprofile."""

import argparse
import json
import math
import os
import re
import sys
from fractions import Fraction
from pathlib import Path


BLOCK = re.compile(r"(.+\.go):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)")


def read_coverage(text):
    lines = text.splitlines()
    if not lines or lines[0] not in ("mode: set", "mode: count", "mode: atomic"):
        raise ValueError("missing or unsupported coverage mode")
    blocks = {}
    for number, line in enumerate(lines[1:], 2):
        match = BLOCK.fullmatch(line)
        if not match:
            raise ValueError(f"invalid coverage block on line {number}")
        filename, *values = match.groups()
        start_line, start_col, end_line, end_col, statements, count = map(
            int, values
        )
        if min(start_line, start_col, end_line, end_col) < 1 or (
            end_line, end_col
        ) < (start_line, start_col):
            raise ValueError(f"invalid source range on line {number}")
        key = (filename, start_line, start_col, end_line, end_col)
        if key in blocks:
            previous_statements, previous_count = blocks[key]
            if statements != previous_statements:
                raise ValueError(f"inconsistent statement count on line {number}")
            count = max(count, previous_count)
        blocks[key] = (statements, count)
    packages = {}
    for (filename, *_), (statements, count) in blocks.items():
        package = filename.rsplit("/", 1)[0]
        covered, total = packages.get(package, (0, 0))
        packages[package] = (
            covered + (statements if count else 0), total + statements
        )
    return packages


def assess(packages, floors):
    if not isinstance(floors, dict) or not floors:
        raise ValueError("coverage floors must be a nonempty package-to-percent object")
    rows = [
        "## Unit-test statement coverage",
        "",
        "| Package | Covered / total | Coverage | Floor | Result |",
        "| --- | ---: | ---: | ---: | --- |",
    ]
    failures = []
    for package, floor in sorted(floors.items()):
        if not isinstance(package, str) or not package:
            raise ValueError("coverage floor requires a package name")
        if (
            isinstance(floor, bool)
            or not isinstance(floor, (int, float))
            or not math.isfinite(floor)
            or not 0 <= floor <= 100
        ):
            raise ValueError(f"invalid coverage floor for {package}")
        covered, total = packages.get(package, (0, 0))
        if total == 0:
            failures.append(f"{package}: missing coverage or no statements")
            rows.append(f"| `{package}` | — | — | {floor:.1f}% | MISSING |")
            continue
        percent = Fraction(100 * covered, total)
        failed = percent < Fraction(str(floor))
        if failed:
            failures.append(
                f"{package}: {float(percent):.4f}% is below {floor:.1f}%"
            )
        rows.append(
            f"| `{package}` | {covered} / {total} | {float(percent):.2f}% "
            f"| {floor:.1f}% | {'FAIL' if failed else 'PASS'} |"
        )
    return "\n".join(rows) + "\n", failures


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("profile", type=Path)
    parser.add_argument(
        "--floors", type=Path,
        default=Path(__file__).with_name("coverage-floors.json"),
    )
    args = parser.parse_args(argv)
    try:
        packages = read_coverage(args.profile.read_text(encoding="utf-8"))
        floors = json.loads(args.floors.read_text(encoding="utf-8"))
        summary, failures = assess(packages, floors)
        print(summary)
        if summary_path := os.environ.get("GITHUB_STEP_SUMMARY"):
            with Path(summary_path).open("a", encoding="utf-8") as output:
                output.write(summary)
    except (OSError, ValueError) as error:
        print(f"Invalid coverage input: {error}", file=sys.stderr)
        return 2
    for failure in failures:
        print(f"Coverage gate failed: {failure}", file=sys.stderr)
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
