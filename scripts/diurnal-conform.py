#!/usr/bin/env python3
"""Re-apply corpus section conventions to pipeline-written sections and re-attest them.

Readers copy the book's small-caps openings and sometimes drop an ornamental
initial; conventions can also improve after a sweep has already written text.
This pass rewrites only sections whose SOURCE line the pipeline itself wrote
("# SOURCE: diurnal p. N") and whose provenance row was recorded by the pipeline,
then refreshes the hash-bound attestation with the same page.
"""
from __future__ import annotations

import argparse
import csv
import importlib
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
transcribe = importlib.import_module("diurnal-transcribe")


def section_file(key: str) -> Path | None:
    parts = key.split("/")
    if len(parts) != 3:
        return None
    return ROOT / "data" / "texts" / parts[0] / f"{parts[1]}.txt"


def raw_section(path: Path, section: str) -> list[str] | None:
    lines = path.read_text(encoding="utf-8").splitlines()
    out, inside = [], False
    for line in lines:
        if line.startswith("[") and line.rstrip().endswith("]"):
            if inside:
                break
            inside = line.strip() == f"[{section}]"
            continue
        if inside:
            if line.startswith("# [") :
                break
            out.append(line)
    return out if inside else None


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    rows = list(csv.DictReader(open(ROOT / "data" / "review" / "provenance.csv", encoding="utf-8")))
    changed = 0
    for row in rows:
        if row.get("status") != "verified" or "page image" not in row.get("notes", ""):
            continue
        key = row["key"]
        path = section_file(key)
        if path is None or not path.exists():
            continue
        section = key.rsplit("/", 1)[-1]
        raw = raw_section(path, section)
        if raw is None or not any(l.startswith("# SOURCE: diurnal p.") for l in raw):
            continue
        source = next(l for l in raw if l.startswith("# SOURCE: ")).removeprefix("# SOURCE: ").strip()
        body = "\n".join(l for l in raw if not l.startswith("#")).strip() + "\n"
        conformed = transcribe.conform_section_body(key, body)
        if conformed == body:
            continue
        changed += 1
        print(f"{key}: {body.splitlines()[0][:60]!r} -> {conformed.splitlines()[0][:60]!r}")
        if args.dry_run:
            continue
        tmp = ROOT / "output" / "conform" / f"{transcribe.safe_key(key)}.txt"
        tmp.parent.mkdir(parents=True, exist_ok=True)
        tmp.write_text(conformed, encoding="utf-8")
        transcribe.run_office(["corpus", "put", key, "--file", str(tmp), "--source", source])
        transcribe.run_office(["review", "attest", "--source", "diurnal", "--page", row["page"],
                               "--note", row["notes"], "--replace", key, row["reviewer"]])
    print(f"{changed} section(s) {'would change' if args.dry_run else 'rewritten and re-attested'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
