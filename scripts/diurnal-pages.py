#!/usr/bin/env python3
"""Render source PDFs into a content-bound page cache and search their OCR."""

from __future__ import annotations

import argparse
from collections import Counter
import difflib
import hashlib
import json
import re
import subprocess
import sys
import unicodedata
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
PAGES_ROOT = ROOT / "output" / "pages"
KEY_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")
LABEL_RE = re.compile(r"^[\s\-–—·.]*([0-9]+\s*\*?|[ivxlcdm]+)[\s\-–—·.]*$", re.I)
ROMAN_VALUES = {"i": 1, "v": 5, "x": 10, "l": 50, "c": 100, "d": 500, "m": 1000}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def roman_to_int(value: str) -> int | None:
    value = value.casefold()
    if not value or any(char not in ROMAN_VALUES for char in value):
        return None
    total = 0
    previous = 0
    for char in reversed(value):
        current = ROMAN_VALUES[char]
        total += -current if current < previous else current
        previous = max(previous, current)
    if int_to_roman(total) != value:
        return None
    return total


def int_to_roman(value: int) -> str:
    if value <= 0 or value >= 4000:
        return ""
    parts: list[str] = []
    for number, token in (
        (1000, "m"), (900, "cm"), (500, "d"), (400, "cd"),
        (100, "c"), (90, "xc"), (50, "l"), (40, "xl"),
        (10, "x"), (9, "ix"), (5, "v"), (4, "iv"), (1, "i"),
    ):
        while value >= number:
            parts.append(token)
            value -= number
    return "".join(parts)


def canonical_label(value: str) -> str | None:
    match = LABEL_RE.fullmatch(value.strip())
    if not match:
        return None
    token = re.sub(r"\s+", "", match.group(1)).casefold()
    if token.endswith("*"):
        number = int(token[:-1])
        return f"{number}*" if number > 0 else None
    if token.isdigit():
        number = int(token)
        return str(number) if number > 0 else None
    return token if roman_to_int(token) is not None else None


def edge_lines(text: str, count: int = 4) -> list[str]:
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    if len(lines) <= count * 2:
        return lines
    return lines[:count] + lines[-count:]


def detect_printed_label(*texts: str) -> str | None:
    """Return the sole page-like token at the top/bottom, or None if ambiguous."""
    labels = {
        label
        for text in texts
        for line in edge_lines(text)
        if (label := canonical_label(line)) is not None
    }
    return next(iter(labels)) if len(labels) == 1 else None


def label_series(label: str) -> tuple[str, int] | None:
    if label.endswith("*") and label[:-1].isdigit():
        return "star", int(label[:-1])
    if label.isdigit():
        return "arabic", int(label)
    value = roman_to_int(label)
    return ("roman", value) if value is not None else None


def format_series(kind: str, value: int) -> str:
    if kind == "star":
        return f"{value}*"
    if kind == "roman":
        return int_to_roman(value)
    return str(value)


def interpolate_labels(pages: list[dict]) -> list[dict]:
    """Fill bounded monotonic gaps without crossing printed-numbering series."""
    result = [dict(page) for page in pages]
    known = [i for i, page in enumerate(result) if page.get("printed_page")]
    for left_index, right_index in zip(known, known[1:]):
        if right_index == left_index + 1:
            continue
        left = label_series(result[left_index]["printed_page"])
        right = label_series(result[right_index]["printed_page"])
        if not left or not right or left[0] != right[0]:
            continue
        distance = right_index - left_index
        if right[1] - left[1] != distance:
            continue
        for offset in range(1, distance):
            page = result[left_index + offset]
            page["printed_page"] = format_series(left[0], left[1] + offset)
            page["inferred"] = True
    return result


def ocr_label_candidates(page: dict) -> set[str]:
    """Return page-like OCR tokens at an edge, including ambiguous ones."""
    candidates = set()
    for text in (page.get("text", ""), page.get("layout_text", "")):
        for line in edge_lines(text):
            if label := canonical_label(line):
                candidates.add(label)
            elif re.fullmatch(r"[Il]\s*\*", line.strip()):
                # The appendix begins at 1*, which OCR commonly reads as l*.
                candidates.add("1*")
    return candidates


def page_label_candidates(page: dict) -> set[str]:
    candidates = ocr_label_candidates(page)
    if label := page.get("printed_page"):
        if canonical := canonical_label(str(label)):
            candidates.add(canonical)
    return candidates


def repair_label_runs(pages: list[dict]) -> list[dict]:
    """Recover dominant numbered runs and reject sequence-breaking OCR labels.

    A printed run has a constant difference between its numeric label and PDF
    page. Requiring the same offset on at least two pages lets clean anchors
    repair long OCR gaps while excluding plausible-looking errors such as
    ``mi`` for ``xxxi``.
    """
    cleaned = [dict(page) for page in pages]
    for page in cleaned:
        current = page.get("printed_page")
        if current is not None:
            page["printed_page"] = canonical_label(str(current))
            if page["printed_page"] is None:
                page["inferred"] = False
    result = interpolate_labels(cleaned)
    candidates: dict[str, list[tuple[int, int, str]]] = {"roman": [], "star": []}
    page_candidates: dict[int, set[str]] = {}
    for index, source_page in enumerate(pages):
        labels = page_label_candidates(source_page)
        page_candidates[index] = ocr_label_candidates(source_page)
        for label in labels:
            series = label_series(label)
            if series and series[0] in candidates:
                candidates[series[0]].append((index, series[1], label))

    for kind, anchors in candidates.items():
        offsets = Counter(value - result[index]["pdf_page"] for index, value, _ in anchors)
        supported = [(count, offset) for offset, count in offsets.items() if count >= 2]
        if not supported:
            continue
        _, offset = max(supported, key=lambda item: (item[0], -abs(item[1])))
        coherent = [
            (index, value) for index, value, _ in anchors
            if value - result[index]["pdf_page"] == offset
        ]
        first = min(index for index, _ in coherent)
        last = max(index for index, _ in coherent)

        # Once a coherent run is known, same-series labels elsewhere are OCR
        # outliers rather than independent numbering runs in this book.
        for index, page in enumerate(result):
            current = page.get("printed_page")
            if current and (series := label_series(current)) and series[0] == kind:
                expected = page["pdf_page"] + offset
                if not (first <= index <= last and series[1] == expected):
                    page["printed_page"] = None
                    page["inferred"] = False

        for index in range(first, last + 1):
            page = result[index]
            value = page["pdf_page"] + offset
            if value <= 0:
                continue
            expected = format_series(kind, value)
            directly_seen = expected in page_candidates[index]
            already_exact = page.get("printed_page") == expected and not page.get("inferred", False)
            page["printed_page"] = expected
            page["inferred"] = not (directly_seen or already_exact)
    return result


def normalize_search(text: str) -> str:
    text = unicodedata.normalize("NFKD", text).casefold()
    text = "".join(char for char in text if not unicodedata.combining(char))
    return " ".join(re.findall(r"[a-z0-9]+", text))


def fuzzy_score(query: str, text: str) -> float:
    query_norm, text_norm = normalize_search(query), normalize_search(text)
    if not query_norm or not text_norm:
        return 0.0
    sequence = difflib.SequenceMatcher(None, query_norm, text_norm, autojunk=False).ratio()
    query_tokens, text_tokens = set(query_norm.split()), set(text_norm.split())
    overlap = len(query_tokens & text_tokens) / len(query_tokens)
    substring = 1.0 if query_norm in text_norm else 0.0
    return max(sequence, 0.72 * overlap + 0.28 * substring)


def find_candidates(index: dict, query: str, limit: int = 10) -> list[dict]:
    candidates = []
    for page in index.get("pages", []):
        score = max(fuzzy_score(query, page.get("text", "")),
                    fuzzy_score(query, page.get("layout_text", "")))
        if score <= 0:
            continue
        candidates.append({
            "pdf_page": page["pdf_page"],
            "printed_page": page.get("printed_page"),
            "png": page["png"],
            "inferred": bool(page.get("inferred", False)),
            "score": round(score, 6),
        })
    candidates.sort(key=lambda item: (-item["score"], item["pdf_page"]))
    return candidates[:limit]


def cache_dir(key: str) -> Path:
    if not KEY_RE.fullmatch(key):
        raise ValueError("key must contain only letters, digits, dot, underscore, or hyphen")
    return PAGES_ROOT / key


def run_checked(command: list[str], *, text: bool = False) -> str | bytes:
    try:
        completed = subprocess.run(command, check=True, capture_output=True, text=text)
    except FileNotFoundError as exc:
        raise RuntimeError(f"required command not found: {command[0]}") from exc
    except subprocess.CalledProcessError as exc:
        detail = exc.stderr.strip() if isinstance(exc.stderr, str) else exc.stderr.decode(errors="replace").strip()
        raise RuntimeError(f"{command[0]} failed: {detail}") from exc
    return completed.stdout


def pdf_page_count(source: Path) -> int:
    output = run_checked(["pdfinfo", str(source)], text=True)
    match = re.search(r"^Pages:\s+(\d+)\s*$", output, re.M)
    if not match:
        raise RuntimeError("pdfinfo did not report a page count")
    return int(match.group(1))


def extract_page_text(source: Path, page: int, layout: bool) -> str:
    command = ["pdftotext", "-f", str(page), "-l", str(page)]
    if layout:
        command.append("-layout")
    command.extend([str(source), "-"])
    output = run_checked(command)
    return output.decode("utf-8", errors="replace")


def render_pdf(source: Path, key: str, dpi: int) -> dict:
    if dpi <= 0:
        raise ValueError("dpi must be positive")
    source = source.resolve()
    if not source.is_file():
        raise FileNotFoundError(source)
    destination = cache_dir(key)
    destination.mkdir(parents=True, exist_ok=True)
    digest = sha256_file(source)
    manifest_path = destination / "manifest.json"
    expected = {"version": 1, "key": key, "source_pdf": str(source), "pdf_sha256": digest, "dpi": dpi}
    if manifest_path.exists():
        current = json.loads(manifest_path.read_text(encoding="utf-8"))
        identity_fields = ("version", "key", "pdf_sha256", "dpi")
        if any(current.get(field) != expected[field] for field in identity_fields):
            raise RuntimeError(f"cache metadata mismatch for {key}; use a new key or remove {destination}")
    elif any(destination.iterdir()):
        raise RuntimeError(f"unbound cache artifacts exist for {key}; remove {destination} before rendering")

    count = pdf_page_count(source)
    manifest = {**expected, "page_count": count}
    # Bind the directory before the first expensive render. An interrupted run
    # can then resume safely, while unbound leftover PNGs are never reused.
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")

    index_path = destination / "index.json"
    if index_path.exists():
        index = json.loads(index_path.read_text(encoding="utf-8"))
        if (index.get("pdf_sha256") == digest and index.get("dpi") == dpi and
                index.get("page_count") == count and len(index.get("pages", [])) == count and
                all((destination / f"{page:04d}.png").is_file() for page in range(1, count + 1))):
            return index

    for page in range(1, count + 1):
        png = destination / f"{page:04d}.png"
        if png.exists():
            continue
        prefix = destination / f".{page:04d}"
        run_checked([
            "pdftoppm", "-f", str(page), "-l", str(page), "-singlefile",
            "-r", str(dpi), "-png", str(source), str(prefix),
        ])
        temporary = Path(str(prefix) + ".png")
        temporary.replace(png)

    pages = []
    for page in range(1, count + 1):
        plain = extract_page_text(source, page, False)
        layout = extract_page_text(source, page, True)
        pages.append({
            "pdf_page": page,
            "png": str((destination / f"{page:04d}.png").relative_to(ROOT)),
            "text": plain,
            "layout_text": layout,
            "printed_page": detect_printed_label(plain, layout),
            "inferred": False,
        })
    pages = repair_label_runs(pages)
    index = {**expected, "page_count": count, "pages": pages}
    index_path.write_text(json.dumps(index, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return index


def load_index(key: str) -> dict:
    path = cache_dir(key) / "index.json"
    if not path.is_file():
        raise FileNotFoundError(f"page index not found: {path}")
    index = json.loads(path.read_text(encoding="utf-8"))
    index["pages"] = repair_label_runs(index.get("pages", []))
    return index


def locate_page(index: dict, printed_page: str) -> dict:
    target = canonical_label(printed_page)
    if target is None:
        raise ValueError(f"invalid printed page label: {printed_page}")
    matches = [page for page in repair_label_runs(index.get("pages", [])) if page.get("printed_page") == target]
    if len(matches) != 1:
        if not matches:
            raise LookupError(f"printed page {target} not found")
        raise LookupError(f"printed page {target} is ambiguous ({len(matches)} PDF pages)")
    page = matches[0]
    return {"pdf_page": page["pdf_page"], "png": page["png"], "inferred": bool(page.get("inferred", False))}


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    render = subparsers.add_parser("render", help="render and index one source PDF")
    render.add_argument("source_pdf", type=Path)
    render.add_argument("--key", required=True)
    render.add_argument("--dpi", type=int, default=150)
    locate = subparsers.add_parser("locate", help="resolve a printed page label")
    locate.add_argument("key")
    locate.add_argument("printed_page")
    find = subparsers.add_parser("find", help="rank OCR pages matching words")
    find.add_argument("key")
    find.add_argument("words")
    find.add_argument("--limit", type=int, default=10)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "render":
            index = render_pdf(args.source_pdf, args.key, args.dpi)
            print(json.dumps({"key": args.key, "pages": index["page_count"], "index": str(cache_dir(args.key) / "index.json")}))
        elif args.command == "locate":
            print(json.dumps(locate_page(load_index(args.key), args.printed_page)))
        else:
            print(json.dumps(find_candidates(load_index(args.key), args.words, args.limit), indent=2))
    except (OSError, ValueError, LookupError, RuntimeError, json.JSONDecodeError) as exc:
        print(f"diurnal-pages: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
