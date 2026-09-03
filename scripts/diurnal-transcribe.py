#!/usr/bin/env python3
"""Transcribe cached Diurnal page images, compare them, and optionally apply."""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import difflib
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile
import time
import unicodedata
from collections import Counter
from pathlib import Path
from typing import Callable, Iterable


ROOT = Path(__file__).resolve().parents[1]
SCHEMA = Path(__file__).with_name("diurnal-transcription-schema.json")
TRANSCRIBE_ROOT = ROOT / "output" / "transcribe"
DEFAULT_PAGE_KEY = "monastic-diurnal"
DEFAULT_MODEL = "gpt-5.6-luna"
MAX_OUTPUT_BYTES = 1024 * 1024
DEFAULT_TIMEOUT = 300
DEFAULT_MAX_ATTEMPTS = 3
NAMESPACES = {"proper", "commons", "seasonal", "ordinary", "shared", "psalms", "canticles"}
CANTICLE_DESCRIPTIONS = {
    "habakkuk-3": "Canticle of Habakkuk (Hab. 3)",
    "isaiah-38": "Canticle of Isaiah (Is. 38)",
    "isaiah-12": "Canticle of Isaiah (Is. 12)",
    "hannah": "Canticle of Hannah (1 Sam. 2)",
}
LEADING_LABEL_RE = re.compile(
    r"^\s*(?:(?:Ant\.\s+on\s+Magnificat\.)|Ant\.|Antiphon\.?|Collect\.?|Chapter\.?|"
    r"Hymn\.?|Prayer\.?|Memorial\.?)\s*",
    re.I,
)
COLLECT_CONCLUSION_RE = re.compile(
    r"(?:^|\s)(?:through\s+our\s+lord\b.*|through\s+jesus\s+christ\b.*|"
    r"who\s+livest\b.*|who\s+liveth\b.*|in\s+the\s+unity\s+of\s+the\s+same\b.*|"
    r"through\s+the\s+same|through)\s*$",
    re.I | re.S,
)
SOURCE_PAGE_RE = re.compile(
    r"(?i)(?:(?:printed)\s+|(?<!PDF\s))(?<![A-Za-z])p{1,2}\.\s*"
    r"([ivxlcdm]+|[0-9]+\*?)(?:\s*[-–]\s*([ivxlcdm]+|[0-9]+\*?))?"
)

PAGES_SPEC = importlib.util.spec_from_file_location("diurnal_pages", Path(__file__).with_name("diurnal-pages.py"))
diurnal_pages = importlib.util.module_from_spec(PAGES_SPEC)
assert PAGES_SPEC.loader
PAGES_SPEC.loader.exec_module(diurnal_pages)


class RunOptions:
    def __init__(self, run_dir: Path, page_key: str = DEFAULT_PAGE_KEY, provider: str = "codex",
                 model: str = DEFAULT_MODEL, timeout: int = DEFAULT_TIMEOUT,
                 max_output_bytes: int = MAX_OUTPUT_BYTES, dry_run: bool = False,
                 apply: bool = False, max_attempts: int = DEFAULT_MAX_ATTEMPTS):
        if max_attempts < 1:
            raise ValueError("max_attempts must be positive")
        self.run_dir = run_dir
        self.page_key = page_key
        self.provider = provider
        self.model = model
        self.timeout = timeout
        self.max_output_bytes = max_output_bytes
        self.dry_run = dry_run
        self.apply = apply
        self.max_attempts = max_attempts


class ProviderError(RuntimeError):
    pass


def is_collect_key(key: str) -> bool:
    slot = key.rsplit("/", 1)[-1]
    return (
        "collect" in slot
        and "conclusion" not in slot
        and slot not in {"collect-intro", "post-collect", "triduum-collect-rubric"}
        and "versicle" not in slot
    )


def normalize_text(text: str, key: str = "") -> str:
    """Normalize only comparison-level typography and layout differences."""
    text = LEADING_LABEL_RE.sub("", text, count=1)
    text = text.replace("\u00ad", "")
    text = re.sub(r"(?<=\w)[\-‐‑]\s*\n\s*(?=\w)", "", text)
    text = text.translate(str.maketrans({
        "‘": "'", "’": "'", "‚": "'", "‛": "'",
        "“": '"', "”": '"', "„": '"', "‟": '"',
        "æ": "ae", "Æ": "AE", "œ": "oe", "Œ": "OE",
        "℣": "V.", "℟": "R.", "Ꝟ": "V.",
    }))
    text = unicodedata.normalize("NFKC", text)
    text = re.sub(r"\s*[†*‡]\s*", " ", text)
    normalized_lines = []
    for line in text.splitlines():
        line = re.sub(r"^\s*[vV][./:]+\s*", "V. ", line)
        line = re.sub(r"^\s*[rR][./:]+\s*", "R. ", line)
        line = re.sub(r"[\s.,;:!?…–—]+$", "", line.strip())
        if line:
            normalized_lines.append(line)
    normalized = " ".join(" ".join(normalized_lines).split()).casefold()
    if is_collect_key(key):
        normalized = COLLECT_CONCLUSION_RE.sub("", normalized)
        normalized = re.sub(r"[\s.,;:!?…–—]+$", "", normalized)
    return normalized


def similarity(left: str, right: str, key: str = "") -> float:
    return difflib.SequenceMatcher(
        None, normalize_text(left, key), normalize_text(right, key), autojunk=False,
    ).ratio()


def classify_transcription(corpus_text: str, transcription: dict, key: str = "") -> tuple[str, float]:
    if not transcription.get("found") or not str(transcription.get("text", "")).strip():
        return "not-found", 0.0
    if transcription.get("confidence") == "low":
        return "low-confidence", 0.0
    observed = str(transcription["text"])
    if corpus_text.replace("\r\n", "\n").strip() == observed.replace("\r\n", "\n").strip():
        return "exact", 1.0
    score = similarity(corpus_text, observed, key)
    if normalize_text(corpus_text, key) == normalize_text(observed, key) or score >= 0.985:
        return "near", score
    return "different", score


def apply_decision(key: str, classification: str, first: dict, second: dict | None = None,
                   corpus_text: str | None = None) -> str:
    if classification in {"exact", "near"}:
        return "attest"
    if classification != "different":
        return "record-only"
    if second is None:
        return "needs-human"
    if not second.get("found") or second.get("confidence") == "low":
        return "needs-human"
    # The first-wave ingestion guardrail forbids automatic psalter writes.
    if key.startswith("psalms/"):
        return "needs-human"
    if corpus_text is None or similarity(str(first.get("text", "")), corpus_text, key) < 0.6:
        return "needs-human"
    return "replace-and-attest" if similarity(
        str(first.get("text", "")), str(second.get("text", "")), key,
    ) >= 0.985 else "needs-human"


def safe_key(key: str) -> str:
    return re.sub(r"[^A-Za-z0-9._-]+", "__", key)


def parse_feast_names(data_dir: Path) -> dict[str, str]:
    names: dict[str, str] = {}
    feasts_dir = data_dir / "feasts"
    for path in sorted(feasts_dir.glob("*.txt")):
        current = None
        for raw in path.read_text(encoding="utf-8").splitlines():
            line = raw.strip()
            match = re.fullmatch(r"\[([^]]+)\]", line)
            if match:
                current = match.group(1)
            elif current and line.startswith("Name") and "=" in line:
                names[current] = line.split("=", 1)[1].strip()
    return names


def humanize(value: str) -> str:
    words = value.replace("-", " ").split()
    replacements = {"bvm": "BVM", "i": "I", "ii": "II"}
    return " ".join(replacements.get(word, word) for word in words)


def describe_key(key: str, data_dir: Path, feast_names: dict[str, str] | None = None) -> str:
    parts = key.split("/")
    if len(parts) < 2 or parts[0] not in NAMESPACES:
        raise ValueError(f"unsupported corpus key: {key}")
    slot = humanize(parts[-1])
    if parts[0] == "canticles" and len(parts) == 2:
        return CANTICLE_DESCRIPTIONS.get(parts[1], f"the canticle {slot}")
    if parts[0] == "proper" and len(parts) >= 3:
        names = feast_names if feast_names is not None else parse_feast_names(data_dir)
        owner = names.get(parts[1], humanize(parts[1]))
        return f"the {slot} for {owner}"
    owner = humanize(" ".join(parts[1:-1])) if len(parts) > 2 else humanize(parts[0])
    return f"the {slot} in {owner}"


CORPUS_GRAMMAR = """Corpus output grammar:
- Return only the requested corpus entry, without surrounding page headings, rubrics, or explanatory prose. Retain only a title or incipit required by the entry forms below.
- Omit printed entry labels such as `Ant.`, `Ant. on Magnificat.`, `Collect`, `Chapter`, `Hymn`, `Prayer`, and `Memorial`. When a printed ℣./℟. labels dialogue, use the corpus `V. ` / `R. ` markers below instead of copying the printed label.
- Preserve the book's archaic spelling and capitalization. Expand nothing and silently correct nothing.
- Render every printed flex or mediant mark (`†`, `*`, or `‡`) as the corpus marker ` * `.
- A psalm/canticle entry keeps its corpus title line and `!Reference`, then a blank line before the verses. Use `[section: Heading]` for a printed canticle section break.
- Use `!Reference` for a printed scripture reference that belongs to a chapter or other block.
- Use `V. ` and `R. ` at the start of versicle and response lines (the corpus renderers display ℣./℟.).
- Keep the `Blessing. ` and `All: ` sigils when those speakers are printed.
- For a collect, the transcription must stop before its conclusion cue (for example `Through` or `Who livest`).
- Psalm/canticle verses use one verse per line, an optional `N. ` number, and ` * ` at the mediant.
- In hymns, keep each verse line and put one blank line between stanzas; a lone Latin incipit may stand above a blank line.
- Preserve meaningful blank lines. Do not include page headers, page numbers, rubrics, Latin parallel-column text, or adjacent slots."""


def build_prompt(key: str, description: str, printed_page: str, pages: list[dict], *, paths_for_read: bool = False) -> str:
    page_summary = ", ".join(
        f"printed {page.get('printed_page') or '?'} / PDF {page['pdf_page']} ({page['png']})"
        for page in pages
    )
    image_instruction = (
        "Use the Read tool to inspect the PNG paths named below. " if paths_for_read else
        "Inspect the attached PNG images. "
    )
    return f"""You are a literal reader of a printed Monastic Diurnal page image.
{image_instruction}Transcribe {description}, corpus key `{key}`. It is expected on printed page {printed_page}; the following PDF page is supplied only in case the section continues.

Images: {page_summary}

{CORPUS_GRAMMAR}

Never infer, reconstruct, modernize, or borrow wording from memory or another edition. If the requested section is absent or unreadable, set found=false and text="". When it spans the two supplied pages, join only that single section.

Return exactly one JSON object with keys: found (boolean), text (string), printed_page (string), pdf_page (integer), confidence (high|medium|low), and notes (string). Notes must describe visibility or location only; do not repeat the transcription there."""


def command_output(command: list[str], timeout: int, max_bytes: int) -> str:
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        try:
            process = subprocess.Popen(command, cwd=ROOT, stdout=stdout, stderr=stderr)
        except FileNotFoundError as exc:
            raise ProviderError(f"provider command not found: {command[0]}") from exc
        try:
            returncode = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired as exc:
            process.kill()
            process.wait()
            raise ProviderError(f"provider timed out after {timeout}s") from exc
        payload_size = os.fstat(stdout.fileno()).st_size + os.fstat(stderr.fileno()).st_size
        if payload_size > max_bytes:
            raise ProviderError(f"provider output exceeded {max_bytes} bytes")
        stdout.seek(0)
        stderr.seek(0)
        output = stdout.read()
        error_output = stderr.read()
    if returncode != 0:
        detail = error_output.decode("utf-8", errors="replace").strip()
        raise ProviderError(f"provider exited {returncode}: {detail[:1000]}")
    return output.decode("utf-8", errors="replace")


def find_result_object(value) -> dict | None:
    if isinstance(value, dict):
        if {"found", "text", "printed_page", "pdf_page", "confidence", "notes"} <= value.keys():
            return value
        for key in ("structured_output", "result", "output", "message", "content", "text", "item"):
            if key in value:
                found = find_result_object(value[key])
                if found:
                    return found
    elif isinstance(value, list):
        for item in reversed(value):
            if found := find_result_object(item):
                return found
    elif isinstance(value, str):
        try:
            return find_result_object(json.loads(value))
        except json.JSONDecodeError:
            match = re.search(r"\{.*\}", value, re.S)
            if match:
                try:
                    return find_result_object(json.loads(match.group(0)))
                except json.JSONDecodeError:
                    pass
    return None


def parse_provider_output(output: str) -> dict:
    candidates = []
    try:
        candidates.append(json.loads(output))
    except json.JSONDecodeError:
        for line in output.splitlines():
            try:
                candidates.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    for candidate in reversed(candidates):
        result = find_result_object(candidate)
        if result:
            expected_types = {
                "found": bool, "text": str, "printed_page": str,
                "pdf_page": int, "confidence": str, "notes": str,
            }
            if any(not isinstance(result[field], expected) for field, expected in expected_types.items()):
                raise ProviderError("provider returned invalid transcription field types")
            if result["confidence"] not in {"high", "medium", "low"} or result["pdf_page"] < 1:
                raise ProviderError("provider returned invalid transcription field values")
            return result
    raise ProviderError("provider output contained no transcription JSON")


class ProviderRunner:
    def __init__(self, timeout: int = DEFAULT_TIMEOUT, max_bytes: int = MAX_OUTPUT_BYTES,
                 execute: Callable[[list[str], int, int], str] = command_output):
        self.timeout = timeout
        self.max_bytes = max_bytes
        self.execute = execute

    def transcribe(self, provider: str, model: str, prompt: str, images: list[Path]) -> tuple[dict, float]:
        return self.read_json(provider, model, prompt, images, SCHEMA, parse_provider_output)

    def read_json(self, provider: str, model: str, prompt: str, images: list[Path],
                  schema: Path, parser: Callable[[str], dict]) -> tuple[dict, float]:
        """Run one bounded image-reading call with a caller-supplied JSON schema."""
        started = time.monotonic()
        if provider == "codex":
            command = ["codex", "exec", "--ephemeral", "--json", "--output-schema", str(schema)]
            for image in images:
                command.extend(["-i", str(image)])
            command.extend(["--sandbox", "read-only", "-m", model, prompt])
        elif provider == "claude":
            read_prompt = build_read_prompt(prompt, images)
            command = [
                "claude", "-p", read_prompt, "--model", model, "--output-format", "json",
                "--json-schema", schema.read_text(encoding="utf-8"),
                "--allowedTools", "Read", "--no-session-persistence",
            ]
        else:
            raise ProviderError(f"unsupported provider: {provider}")
        output = self.execute(command, self.timeout, self.max_bytes)
        return parser(output), time.monotonic() - started


def build_read_prompt(prompt: str, images: list[Path]) -> str:
    paths = "\n".join(f"- {path}" for path in images)
    return f"{prompt}\n\nRead these image files before answering:\n{paths}"


class PageResolver:
    def __init__(self, root: Path = ROOT):
        self.root = root
        self._indexes: dict[str, dict] = {}

    def _index(self, key: str) -> dict:
        if key not in self._indexes:
            index_path = self.root / "output" / "pages" / key / "index.json"
            index = json.loads(index_path.read_text(encoding="utf-8"))
            index["pages"] = diurnal_pages.repair_label_runs(index.get("pages", []))
            self._indexes[key] = index
        return self._indexes[key]

    def resolve(self, key: str, printed_page: str) -> list[dict]:
        index = self._index(key)
        located = diurnal_pages.locate_page(index, printed_page)
        return self._pages_from_pdf(key, located["pdf_page"])

    def resolve_pdf(self, key: str, cited_page: str) -> list[dict]:
        if not cited_page.isdigit() or int(cited_page) < 1:
            raise ValueError(f"cited page is not a PDF page number: {cited_page}")
        return self._pages_from_pdf(key, int(cited_page))

    def find(self, key: str, query: str, limit: int = 3) -> list[list[dict]]:
        index = self._index(key)
        return [self._pages_from_pdf(key, item["pdf_page"])
                for item in diurnal_pages.find_candidates(index, query, limit)]

    def page(self, key: str, pdf_page: int) -> dict:
        index = self._index(key)
        for page in index.get("pages", []):
            if page["pdf_page"] == pdf_page:
                return self._page_record(key, page, index)
        raise LookupError(f"PDF page {pdf_page} not found")

    def _pages_from_pdf(self, key: str, pdf_page: int) -> list[dict]:
        index = self._index(key)
        by_pdf = {page["pdf_page"]: page for page in index["pages"]}
        if pdf_page not in by_pdf:
            raise LookupError(f"PDF page {pdf_page} not found")
        selected = by_pdf[pdf_page]
        result = [self._page_record(key, selected, index)]
        if selected["pdf_page"] + 1 in by_pdf:
            result.append(self._page_record(key, by_pdf[selected["pdf_page"] + 1], index))
        return result

    def _page_record(self, key: str, page: dict, index: dict) -> dict:
        png = Path(page["png"])
        absolute = png if png.is_absolute() else self.root / png
        return {
            "pdf_page": page["pdf_page"], "printed_page": page.get("printed_page"),
            "inferred": bool(page.get("inferred", False)), "png": str(absolute.resolve()),
            "page_key": key, "pdf_sha256": index.get("pdf_sha256", ""),
            "ocr_route": "pdftotext+pdftotext-layout",
            "ocr_text_sha256": hashlib.sha256(
                (page.get("text", "") + "\0" + page.get("layout_text", "")).encode("utf-8")
            ).hexdigest(),
        }


def run_office(args: list[str], *, capture: bool = True) -> str:
    command = [str(ROOT / "office"), *args]
    result = subprocess.run(command, cwd=ROOT, capture_output=capture, text=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(f"{' '.join(command)} failed: {result.stderr.strip()}")
    return result.stdout if capture else ""


def corpus_text(key: str) -> str:
    return run_office(["corpus", "show", key]).rstrip("\n")


def corpus_source_lines(key: str, data_dir: Path = ROOT / "data") -> list[str]:
    """Return SOURCE comments belonging to a corpus key's live section."""
    parts = key.split("/")
    plain_path = data_dir / "texts" / Path(*parts).with_suffix(".txt")
    if plain_path.is_file():
        section_lines = plain_path.read_text(encoding="utf-8").splitlines()
    else:
        if len(parts) < 2:
            return []
        section_path = data_dir / "texts" / Path(*parts[:-1]).with_suffix(".txt")
        if not section_path.is_file():
            return []
        lines = section_path.read_text(encoding="utf-8").splitlines()
        header = re.compile(rf"^\s*\[{re.escape(parts[-1])}\]\s*$")
        start = next((index + 1 for index, line in enumerate(lines) if header.match(line)), None)
        if start is None:
            return []
        section_lines = []
        for line in lines[start:]:
            if re.match(r"^\s*#?\s*\[[a-z0-9][a-z0-9-]*\]\s*$", line, re.I):
                break
            section_lines.append(line)
    result = []
    for line in section_lines:
        match = re.match(r"^\s*#\s*SOURCE:\s*(.*)$", line, re.I)
        if match:
            result.append(match.group(1).strip())
    return result


def corpus_source_pages(key: str, data_dir: Path = ROOT / "data") -> list[str]:
    """Extract printed Diurnal page starts from a corpus section's sources."""
    pages = []
    for source in corpus_source_lines(key, data_dir):
        if not re.search(r"diurnal|monastic", source, re.I):
            continue
        for match in SOURCE_PAGE_RE.finditer(source):
            page = diurnal_pages.canonical_label(match.group(1))
            if page and page not in pages:
                pages.append(page)
    return pages


def source_page_candidates(resolver: PageResolver, page_key: str, key: str) -> list[list[dict]]:
    """Resolve each cited printed source page, preserving citation order."""
    candidates = []
    for page in corpus_source_pages(key):
        try:
            candidates.append(resolver.resolve(page_key, page))
        except (AttributeError, OSError, ValueError, LookupError, json.JSONDecodeError):
            continue
    return candidates


def unified_diff(corpus: str, transcription: str) -> str:
    return "".join(difflib.unified_diff(
        corpus.splitlines(keepends=True), transcription.splitlines(keepends=True),
        fromfile="corpus", tofile="diurnal",
    ))


def extract_printed_page(raw: str) -> str | None:
    for match in re.finditer(r"(?i)(?<![A-Za-z0-9])([ivxlcdm]+|[0-9]+\*?)(?![A-Za-z0-9])", raw):
        if label := diurnal_pages.canonical_label(match.group(1)):
            return label
    return None


def split_field(raw: str) -> list[str]:
    return [part.strip() for part in raw.split(";") if part.strip()]


def row_page(row: dict) -> str | None:
    sources, pages = split_field(row.get("source", "")), split_field(row.get("page", ""))
    if len(sources) == len(pages):
        for source, page in zip(sources, pages):
            if re.search(r"diurnal|monastic", source, re.I):
                if found := extract_printed_page(page):
                    return found
    return extract_printed_page(row.get("page", ""))


def corpus_search_query(text: str, word_limit: int = 8) -> str:
    """Build a short OCR locator from corpus wording, without grammar sigils."""
    content = []
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("!") or (line.startswith("[") and line.endswith("]")):
            continue
        line = LEADING_LABEL_RE.sub("", line, count=1)
        line = re.sub(r"^(?:[VR][./:]|Blessing\.|All:)\s*", "", line, flags=re.I)
        line = re.sub(r"^\d{1,3}\.\s*", "", line)
        content.append(line)
    words = diurnal_pages.normalize_search(" ".join(content)).split()
    return " ".join(words[:word_limit])


def feast_slot_query(key: str, description: str, feast_names: dict[str, str]) -> str:
    parts = key.split("/")
    if len(parts) >= 3 and parts[0] == "proper":
        return f"{feast_names.get(parts[1], humanize(parts[1]))} {humanize(parts[-1])}"
    return description


def is_default_candidate(row: dict) -> bool:
    return bool(re.search(r"diurnal|monastic", row.get("source", ""), re.I) and row_page(row))


def load_queue(path: Path | None, start: int, years: int) -> list[dict]:
    if path:
        handle = path.open(newline="", encoding="utf-8")
        try:
            return list(csv.DictReader(handle))
        finally:
            handle.close()
    output = run_office(["review", "provenance-queue", "-start", str(start), "-years", str(years)])
    return list(csv.DictReader(output.splitlines()))


def selected_rows(rows: Iterable[dict], all_rows: bool, keys: set[str],
                  statuses: set[str] | None = None) -> list[dict]:
    statuses = statuses or {"needs-review", "source-unknown"}
    selected = []
    seen = set()
    for row in rows:
        key = row.get("key", "")
        if key in seen:
            continue
        if keys and key not in keys:
            continue
        if not keys and row.get("status", "") not in statuses:
            continue
        if not keys and not all_rows and not (
                is_default_candidate(row) or row.get("status") == "source-unknown"):
            continue
        selected.append(row)
        seen.add(key)
    missing = keys - seen
    if missing:
        raise ValueError("keys absent from queue: " + ", ".join(sorted(missing)))
    return selected


def write_jsonl(path: Path, record: dict) -> None:
    if "packet_sha256" not in record:
        encoded = json.dumps(record, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        record = {**record, "packet_sha256": hashlib.sha256(encoded.encode("utf-8")).hexdigest()}
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n")


def attest(key: str, printed_page: str, png: str) -> None:
    png_path = Path(png)
    try:
        note_path = png_path.resolve().relative_to(ROOT)
    except ValueError:
        note_path = png_path
    note = f"Word-for-word after normalization; page image {note_path}"
    run_office(["review", "attest", "--source", "diurnal", "--page", printed_page,
                "--note", note, "--replace", key, "codex"])


def replace_and_attest(options: RunOptions, key: str, printed_page: str, png: str, text: str) -> None:
    bodies = options.run_dir / "bodies"
    bodies.mkdir(exist_ok=True)
    body_path = bodies / f"{safe_key(key)}.txt"
    body_path.write_text(text.rstrip() + "\n", encoding="utf-8")
    run_office(["corpus", "put", key, "--file", str(body_path), "--source", f"diurnal p. {printed_page}"])
    attest(key, printed_page, png)


def process_row(row: dict, options: RunOptions, resolver: PageResolver,
                provider_runner: ProviderRunner, feast_names: dict[str, str]) -> dict:
    key = row["key"]
    cited_page = row_page(row)
    base = {
        "key": key, "cited_page": cited_page, "printed_page": None,
        "representative_url": row.get("representative_url", ""),
        "provider": options.provider, "model": options.model, "queue_status": row.get("status", ""),
        "work_type": row.get("work_type", ""), "source": row.get("source", ""),
        "locator": row.get("locator", ""),
    }
    empty_result = {
        "corpus_text": None, "first": None, "second": None,
        "diff": "", "similarity": 0.0, "timing_seconds": {"first": None, "second": None},
        "locate_strategy": None, "locate_attempts": [],
    }
    description = describe_key(key, ROOT / "data", feast_names)

    # A dry run prepares only the first resolvable prompt; without a reader it
    # cannot determine whether a later locating strategy is necessary.
    if options.dry_run:
        dry_strategies = () if not cited_page else (
            ("printed-page", lambda: [resolver.resolve(options.page_key, cited_page)]),
            ("source-page", lambda: source_page_candidates(resolver, options.page_key, key)),
            ("pdf-page", lambda: [resolver.resolve_pdf(options.page_key, cited_page)]),
        )
        pages = None
        locate_strategy = None
        errors = []
        for strategy, locate in dry_strategies:
            try:
                candidates = locate()
                if not candidates:
                    continue
                pages = candidates[0]
                locate_strategy = strategy
                break
            except (AttributeError, OSError, ValueError, LookupError, json.JSONDecodeError) as exc:
                errors.append(f"{strategy}: {exc}")
        if pages is None:
            try:
                dry_corpus = corpus_text(key)
                dry_ocr_strategies = (
                    ("corpus-ocr", lambda: resolver.find(
                        options.page_key, corpus_search_query(dry_corpus), limit=3)),
                    ("feast-slot-ocr", lambda: resolver.find(
                        options.page_key, feast_slot_query(key, description, feast_names), limit=3)),
                )
                for strategy, locate in dry_ocr_strategies:
                    candidates = locate()
                    if candidates:
                        pages = candidates[0]
                        locate_strategy = strategy
                        break
            except (AttributeError, OSError, RuntimeError, ValueError, LookupError,
                    json.JSONDecodeError) as exc:
                errors.append(f"OCR fallback: {exc}")
        if pages is None:
            return {**base, **empty_result, "classification": "not-found", "decision": "needs-human",
                    "notes": "page resolution failed: " + "; ".join(errors), "pages": []}
        prompt_page = pages[0].get("printed_page") or cited_page or "unknown"
        prompt = build_prompt(key, description, prompt_page, pages)
        prompts = options.run_dir / "prompts"
        prompts.mkdir(exist_ok=True)
        prompt_path = prompts / f"{safe_key(key)}.txt"
        prompt_path.write_text(prompt + "\n", encoding="utf-8")
        return {**base, "prompt": str(prompt_path), "pages": pages, "dry_run": True,
                "locate_strategy": locate_strategy}

    existing = None
    try:
        existing = corpus_text(key)
    except (OSError, RuntimeError) as exc:
        return {**base, **empty_result, "classification": "not-found", "decision": "needs-human",
                "notes": f"corpus read failed: {exc}", "pages": []}

    page_strategies = () if not cited_page else (
        ("printed-page", lambda: [resolver.resolve(options.page_key, cited_page)]),
        ("source-page", lambda: source_page_candidates(resolver, options.page_key, key)),
        ("pdf-page", lambda: [resolver.resolve_pdf(options.page_key, cited_page)]),
    )
    strategies = page_strategies + (
        ("corpus-ocr", lambda: resolver.find(
            options.page_key, corpus_search_query(existing), limit=3)),
        ("feast-slot-ocr", lambda: resolver.find(
            options.page_key, feast_slot_query(key, description, feast_names), limit=3)),
    )
    attempts = 0
    attempted_pdf_pages = set()
    locate_attempts = []
    locate_errors = []
    first = None
    first_seconds = 0.0
    pages: list[dict] = []
    locate_strategy = None
    result_pages: list[dict] = []
    result_strategy = None
    result_classification = None
    result_score = 0.0
    stop = False
    for strategy, locate in strategies:
        try:
            candidates = locate()
        except (AttributeError, OSError, ValueError, LookupError, json.JSONDecodeError) as exc:
            locate_errors.append(f"{strategy}: {exc}")
            continue
        for candidate_pages in candidates:
            if attempts >= options.max_attempts:
                stop = True
                break
            primary_pdf = candidate_pages[0]["pdf_page"]
            if primary_pdf in attempted_pdf_pages:
                continue
            attempted_pdf_pages.add(primary_pdf)
            attempts += 1
            pages = candidate_pages
            locate_strategy = strategy
            prompt_page = pages[0].get("printed_page") or cited_page or "unknown"
            prompt = build_prompt(key, description, prompt_page, pages)
            try:
                result, seconds = provider_runner.transcribe(
                    options.provider, options.model, prompt, [Path(page["png"]) for page in pages]
                )
                first_seconds += seconds
                first = result
                result_pages = candidate_pages
                result_strategy = strategy
                candidate_classification, candidate_score = classify_transcription(existing, result, key)
                wrong_page = bool(
                    result.get("found") and str(result.get("text", "")).strip()
                    and similarity(existing, str(result["text"]), key) < 0.5
                )
                result_classification = "wrong-page" if wrong_page else candidate_classification
                result_score = candidate_score
                locate_attempts.append({
                    "strategy": strategy, "pdf_page": primary_pdf,
                    "found": bool(result.get("found")), "reader": result,
                    "classification": result_classification,
                    "similarity": round(candidate_score, 6),
                })
            except (OSError, RuntimeError, ProviderError) as exc:
                locate_attempts.append({
                    "strategy": strategy, "pdf_page": primary_pdf, "found": False,
                    "error": str(exc),
                })
                continue
            if wrong_page:
                break
            if result.get("found"):
                stop = True
                break
        if stop:
            break

    if first is not None:
        pages = result_pages
        locate_strategy = result_strategy

    if first is None:
        notes = "first reader failed"
        if locate_errors:
            notes += "; " + "; ".join(locate_errors)
        return {**base, **empty_result, "corpus_text": existing, "classification": "low-confidence",
                "decision": "needs-human", "notes": notes, "pages": pages,
                "locate_strategy": locate_strategy, "locate_attempts": locate_attempts}

    if result_classification == "wrong-page":
        classification, score = "not-found", result_score
    else:
        classification, score = classify_transcription(existing, first, key)
    decision = apply_decision(key, classification, first, corpus_text=existing)
    found_page = None
    if first.get("found") and result_classification != "wrong-page":
        try:
            found_page = resolver.page(options.page_key, first["pdf_page"])
        except (AttributeError, OSError, ValueError, LookupError, json.JSONDecodeError):
            found_page = next((page for page in pages if page["pdf_page"] == first["pdf_page"]), None)
        if found_page is None or not found_page.get("printed_page"):
            decision = "needs-human"

    second = None
    second_seconds = None
    second_error = None
    if options.apply and classification == "different" and attempts < options.max_attempts:
        attempts += 1
        prompt_page = pages[0].get("printed_page") or cited_page or "unknown"
        second_prompt = build_prompt(key, description, prompt_page, pages, paths_for_read=True)
        try:
            second, second_seconds = provider_runner.transcribe(
                "claude", "sonnet", second_prompt, [Path(page["png"]) for page in pages]
            )
            decision = apply_decision(key, classification, first, second, corpus_text=existing)
        except (OSError, RuntimeError, ProviderError) as exc:
            decision = "needs-human"
            second_error = str(exc)
    elif options.apply and classification == "different":
        decision = "needs-human"

    if (first.get("found") and result_classification != "wrong-page"
            and (found_page is None or not found_page.get("printed_page"))):
        decision = "needs-human"
    actual_printed_page = found_page.get("printed_page") if found_page else None
    record = {
        **base, "printed_page": actual_printed_page, "classification": classification,
        "similarity": round(score, 6), "decision": decision, "pages": pages,
        "corpus_text": existing, "first": first, "second": second,
        "diff": unified_diff(existing, str(first.get("text", ""))),
        "locate_strategy": locate_strategy, "locate_attempts": locate_attempts,
        "timing_seconds": {"first": round(first_seconds, 3), "second": None if second_seconds is None else round(second_seconds, 3)},
    }
    if locate_errors:
        record["locate_errors"] = locate_errors
    if second_error:
        record["second_error"] = second_error
    if first.get("found") and result_classification != "wrong-page" and actual_printed_page is None:
        record["notes"] = "found PDF page has no detected or inferred printed label"
    if options.apply:
        try:
            if decision == "attest":
                attest(key, actual_printed_page, found_page["png"])
            elif decision == "replace-and-attest":
                replace_and_attest(options, key, actual_printed_page, found_page["png"], str(first["text"]))
        except RuntimeError as exc:
            record["decision"] = "needs-human"
            record["apply_error"] = str(exc)
    return record


def parse_keys(values: list[str]) -> set[str]:
    return {key.strip() for value in values for key in value.split(",") if key.strip()}


def default_run_id() -> str:
    return dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def run_command(args: argparse.Namespace) -> int:
    if args.apply and args.dry_run:
        raise ValueError("--apply and --dry-run are mutually exclusive")
    if args.max_attempts < 1:
        raise ValueError("--max-attempts must be positive")
    run_dir = TRANSCRIBE_ROOT / (args.run_id or default_run_id())
    run_dir.mkdir(parents=True, exist_ok=False)
    options = RunOptions(
        run_dir=run_dir, page_key=args.page_key, provider=args.provider, model=args.model,
        timeout=args.timeout, max_output_bytes=args.max_output_bytes,
        dry_run=args.dry_run, apply=args.apply, max_attempts=args.max_attempts,
    )
    keys = parse_keys(args.keys)
    rows = selected_rows(
        load_queue(args.queue, args.start, args.years), args.all, keys,
        set(args.status or ("needs-review", "source-unknown")),
    )
    resolver = PageResolver()
    provider_runner = ProviderRunner(options.timeout, options.max_output_bytes)
    feast_names = parse_feast_names(ROOT / "data")
    results_path = run_dir / "results.jsonl"
    prompt_records = run_dir / "prompts.jsonl"
    for row in rows:
        record = process_row(row, options, resolver, provider_runner, feast_names)
        if options.dry_run:
            write_jsonl(prompt_records, record)
        else:
            write_jsonl(results_path, record)
    print(json.dumps({"run": str(run_dir), "rows": len(rows), "dry_run": options.dry_run, "apply": options.apply}))
    return 0


def resolve_run_path(value: str) -> Path:
    candidate = Path(value)
    if candidate.is_dir():
        return candidate
    return TRANSCRIBE_ROOT / value


def report_command(value: str) -> int:
    run_dir = resolve_run_path(value)
    path = run_dir / "results.jsonl"
    if not path.is_file():
        raise FileNotFoundError(path)
    records = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]
    counts = Counter(record.get("classification", "unknown") for record in records)
    print(f"# Diurnal transcription report: {run_dir.name}\n")
    print("## Classifications\n")
    for name in ("exact", "near", "different", "not-found", "low-confidence"):
        print(f"- {name}: {counts.get(name, 0)}")
    needs = [record for record in records if record.get("decision") == "needs-human"]
    print("\n## Needs human review\n")
    if not needs:
        print("None.")
    for record in needs:
        key = record["key"]
        url = record.get("representative_url")
        label = f"[{key}]({url})" if url else f"`{key}`"
        print(f"- {label} — {record.get('classification', 'unknown')}, printed p. {record.get('printed_page') or '?'}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    run = subparsers.add_parser("run", help="transcribe selected provenance queue rows")
    run.add_argument("--queue", type=Path, help="read queue CSV instead of invoking office")
    run.add_argument("--start", type=int, default=2026)
    run.add_argument("--years", type=int, default=1)
    run.add_argument("--all", action="store_true", help="include all queue rows that have pages")
    run.add_argument(
        "--status", action="append", choices=("needs-review", "source-unknown"),
        default=None, help="queue status to include (repeatable; default: both)",
    )
    run.add_argument("--keys", action="append", default=[], metavar="KEY[,KEY]", help="limit to named queue keys")
    run.add_argument("--page-key", default=DEFAULT_PAGE_KEY)
    run.add_argument("--provider", choices=("codex", "claude"), default="codex")
    run.add_argument("--model", default=DEFAULT_MODEL)
    run.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT)
    run.add_argument("--max-output-bytes", type=int, default=MAX_OUTPUT_BYTES)
    run.add_argument("--max-attempts", type=int, default=DEFAULT_MAX_ATTEMPTS,
                     help="maximum reader calls per key across locating and adjudication")
    run.add_argument("--run-id")
    run.add_argument("--dry-run", action="store_true", help="write prompts without invoking a provider")
    run.add_argument("--apply", action="store_true", help="attest matches and gate agreed replacements")
    report = subparsers.add_parser("report", help="print a markdown report for a completed run")
    report.add_argument("run")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return report_command(args.run) if args.command == "report" else run_command(args)
    except (OSError, ValueError, RuntimeError, csv.Error, json.JSONDecodeError) as exc:
        print(f"diurnal-transcribe: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
