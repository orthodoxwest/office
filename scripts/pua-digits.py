#!/usr/bin/env python3
"""Recover digits from the AWRV cycle draft PDFs.

The four 2025 cycle PDFs encode AJensonPro old-style digits as Unicode
private-use characters.  This wrapper runs ``pdftotext -layout`` and replaces
those characters using the committed ``pua-digits.json`` map.

Usage:
    scripts/pua-digits.py BOOK.pdf
    scripts/pua-digits.py BOOK.pdf /tmp/book.txt
    scripts/pua-digits.py --derive BOOK1.pdf BOOK2.pdf ...
    scripts/pua-digits.py --derive --write-map /tmp/map.json BOOK1.pdf ...

Extraction goes to stdout unless an output path is supplied.  Derivation uses
Psalm headings whose English first lines were matched to data/texts/psalms;
the corpus uses Coverdale/Hebrew numbers, so the expected headings are
converted to Vulgate numbering before glyph constraints are solved.

Only the Python standard library and Poppler's ``pdftotext`` are required.
Unmapped PUA characters are left unchanged and summarized on stderr.  In
these books they are normally chant-font glyphs, not missing digits.
"""

import argparse
import json
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
import re
import shutil
import subprocess
import sys
import unicodedata


DEFAULT_MAP = Path(__file__).with_name("pua-digits.json")
PSALM_RE = re.compile(r"\bPsalm\s+([\ue000-\uf8ff]+)\.\s*([^\n]*)")


@dataclass(frozen=True)
class PsalmAnchor:
    """A corpus-verified Psalm heading used to constrain digit glyphs."""

    coverdale: int
    latin_incipit: str
    english_marker: str


# Together these headings exercise all ten digits.  The markers are snippets
# from the corresponding files in data/texts/psalms, not transcriptions of the
# heading number itself.
PSALM_ANCHORS = (
    PsalmAnchor(51, "Miserere mei, Deus", "Have mercy upon me, O God"),
    PsalmAnchor(118, "Confitemini Domino", "O give thanks unto the Lord"),
    PsalmAnchor(63, "Deus, Deus meus", "O God, thou art my God"),
    PsalmAnchor(132, "Memento, Domine", "Lord, remember David"),
    PsalmAnchor(148, "Laudate Dominum", "praise the Lord of heaven"),
    PsalmAnchor(100, "Jubilate Deo", "O be joyful in the Lord"),
)


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--derive",
        action="store_true",
        help="derive the digit map from corpus-verified Psalm headings",
    )
    parser.add_argument(
        "--write-map",
        metavar="PATH",
        type=Path,
        help="write a derived map to PATH instead of stdout",
    )
    parser.add_argument(
        "--mapping",
        metavar="PATH",
        type=Path,
        default=DEFAULT_MAP,
        help=f"mapping for extraction (default: {DEFAULT_MAP})",
    )
    parser.add_argument(
        "--pdftotext",
        metavar="COMMAND",
        default="pdftotext",
        help="pdftotext executable (default: pdftotext)",
    )
    parser.add_argument(
        "paths",
        nargs="+",
        type=Path,
        help="input PDF(s), followed by an optional output path for extraction",
    )
    args = parser.parse_args()
    if args.write_map and not args.derive:
        parser.error("--write-map requires --derive")
    if not args.derive and len(args.paths) > 2:
        parser.error("extraction accepts one input PDF and at most one output path")
    return args


def pdftotext(command, pdf_path):
    """Return UTF-8 text from Poppler with its layout and page breaks intact."""
    executable = shutil.which(command)
    if executable is None:
        raise RuntimeError(
            f"{command!r} was not found; install Poppler's pdftotext or pass "
            "--pdftotext COMMAND"
        )
    try:
        result = subprocess.run(
            [executable, "-layout", "-enc", "UTF-8", str(pdf_path), "-"],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except subprocess.CalledProcessError as exc:
        detail = exc.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"pdftotext failed for {pdf_path}: {detail}") from exc
    return result.stdout.decode("utf-8")


def normalized_letters(text):
    """Normalize prose markers while ignoring accents, spacing, and notation."""
    decomposed = unicodedata.normalize("NFKD", text)
    return "".join(ch.lower() for ch in decomposed if "a" <= ch.lower() <= "z")


def coverdale_to_vulgate(number):
    """Convert an unambiguous whole-Psalm Coverdale number to Vulgate."""
    if 1 <= number <= 8 or 148 <= number <= 150:
        return number
    if 11 <= number <= 113 or 117 <= number <= 146:
        return number - 1
    raise ValueError(
        f"Coverdale Psalm {number} crosses a Vulgate split/merge boundary"
    )


def derive_mapping(command, pdf_paths):
    """Solve PUA-to-digit constraints from verified Psalm contexts."""
    observations = []
    for pdf_path in pdf_paths:
        lines = pdftotext(command, pdf_path).splitlines()
        for line_number, line in enumerate(lines):
            for match in PSALM_RE.finditer(line):
                glyphs, incipit = match.groups()
                window = "\n".join(lines[line_number : line_number + 16])
                normalized_window = normalized_letters(window)
                for anchor in PSALM_ANCHORS:
                    normalized_incipit = normalized_letters(anchor.latin_incipit)
                    if normalized_incipit not in normalized_letters(incipit):
                        continue
                    if normalized_letters(anchor.english_marker) not in normalized_window:
                        continue
                    observations.append((glyphs, anchor))

    constraints = {}
    used_anchors = set()
    for glyphs, anchor in observations:
        expected = str(coverdale_to_vulgate(anchor.coverdale))
        if len(glyphs) != len(expected):
            raise RuntimeError(
                f"heading {anchor.latin_incipit!r} has {len(glyphs)} glyphs; "
                f"expected {expected}"
            )
        used_anchors.add(anchor)
        for glyph, digit in zip(glyphs, expected):
            previous = constraints.setdefault(glyph, digit)
            if previous != digit:
                raise RuntimeError(
                    f"conflicting observations for U+{ord(glyph):04X}: "
                    f"{previous} and {digit}"
                )

    missing_digits = sorted(set("0123456789") - set(constraints.values()))
    if missing_digits:
        found = ", ".join(sorted(a.latin_incipit for a in used_anchors)) or "none"
        raise RuntimeError(
            "could not derive every digit; missing "
            + ", ".join(missing_digits)
            + f" (matched anchors: {found}). Include all four cycle PDFs."
        )

    if len(constraints) != 10:
        raise RuntimeError(
            f"expected ten distinct digit glyphs, found {len(constraints)}"
        )

    anchors = []
    for anchor in PSALM_ANCHORS:
        if anchor in used_anchors:
            anchors.append(
                {
                    "coverdale": anchor.coverdale,
                    "vulgate": coverdale_to_vulgate(anchor.coverdale),
                    "incipit": anchor.latin_incipit,
                }
            )
    return {
        "schema": 1,
        "description": "PUA replacements for the four AWRV 2025 cycle draft PDFs",
        "derivation": {
            "method": (
                "corpus-verified Psalm headings with "
                "Coverdale-to-Vulgate conversion"
            ),
            "anchors": anchors,
        },
        "digits": {
            f"U+{ord(glyph):04X}": digit
            for glyph, digit in sorted(
                constraints.items(), key=lambda item: ord(item[0])
            )
        },
        # U+F650 visibly prints as '#': it marks unresolved draft cross-references.
        "literals": {"U+F650": "#"},
    }


def codepoint(key):
    match = re.fullmatch(r"U\+([0-9A-Fa-f]{4,6})", key)
    if not match:
        raise ValueError(f"invalid mapping key {key!r}")
    return chr(int(match.group(1), 16))


def load_mapping(path):
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"cannot read mapping {path}: {exc}") from exc

    digits = data.get("digits", {})
    if set(digits.values()) != set("0123456789") or len(digits) != 10:
        raise RuntimeError(f"{path} must map ten distinct PUA characters to 0-9")
    replacements = {codepoint(key): value for key, value in digits.items()}
    replacements.update(
        {codepoint(key): value for key, value in data.get("literals", {}).items()}
    )
    return replacements


def is_private_use(character):
    value = ord(character)
    return (
        0xE000 <= value <= 0xF8FF
        or 0xF0000 <= value <= 0xFFFFD
        or 0x100000 <= value <= 0x10FFFD
    )


def codepoint_ranges(characters):
    """Compact sorted characters as U+XXXX and U+XXXX-U+YYYY ranges."""
    values = sorted(ord(ch) for ch in characters)
    ranges = []
    start = previous = values[0]
    for value in values[1:]:
        if value == previous + 1:
            previous = value
            continue
        ranges.append((start, previous))
        start = previous = value
    ranges.append((start, previous))
    return ", ".join(
        f"U+{start:04X}" if start == end else f"U+{start:04X}-U+{end:04X}"
        for start, end in ranges
    )


def recover_text(text, replacements):
    unmapped = Counter(
        ch for ch in text if is_private_use(ch) and ch not in replacements
    )
    recovered = text.translate(str.maketrans(replacements))
    return recovered, unmapped


def write_text(text, output_path):
    if output_path is None or str(output_path) == "-":
        sys.stdout.write(text)
        return
    try:
        output_path.write_text(text, encoding="utf-8")
    except OSError as exc:
        raise RuntimeError(f"cannot write {output_path}: {exc}") from exc


def main():
    args = parse_args()
    try:
        if args.derive:
            data = derive_mapping(args.pdftotext, args.paths)
            rendered = json.dumps(data, indent=2, ensure_ascii=False) + "\n"
            write_text(rendered, args.write_map)
            return 0

        input_path = args.paths[0]
        output_path = args.paths[1] if len(args.paths) == 2 else None
        replacements = load_mapping(args.mapping)
        extracted = pdftotext(args.pdftotext, input_path)
        recovered, unmapped = recover_text(extracted, replacements)
        write_text(recovered, output_path)
        if unmapped:
            print(
                "warning: left "
                f"{sum(unmapped.values())} occurrences of {len(unmapped)} unmapped "
                "PUA codepoints unchanged (normally chant glyphs): "
                + codepoint_ranges(unmapped),
                file=sys.stderr,
            )
        return 0
    except (RuntimeError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
