#!/usr/bin/env python3
"""Build disposable, page-aware review packets from local office books.

The extracted source text is deliberately written only below ``output/``
(which is gitignored). The checked-in artifact is this parser, not a copy of
the source books.

Typical use::

    make review-sources
    scripts/source-reconcile.py show SR-0001-01234567

Discovery packets stay under ``output/``. Writable classes can be turned
into an apply-queue; ``office review apply`` is the only writer that
touches ``data/texts``. That write is not a human attestation.
"""

from __future__ import annotations

import argparse
import csv
import difflib
import functools
import hashlib
import json
import math
import pathlib
import re
import subprocess
import sys
import unicodedata
import zipfile
import xml.etree.ElementTree as ET
from dataclasses import asdict, dataclass


W_NS = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
ROOT = pathlib.Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT = pathlib.Path("output/source-reconcile")
DECISION_FIELDS = ("candidate_id", "decision", "note")


def require_output_path(path: pathlib.Path) -> pathlib.Path:
    """Confine source-derived artifacts to the repository's ignored output."""
    resolved = path.resolve()
    allowed = (ROOT / "output").resolve()
    try:
        resolved.relative_to(allowed)
    except ValueError as exc:
        raise ValueError(f"source reconciliation artifacts must stay below {allowed}") from exc
    return resolved

MASTER_BOOKS = (
    ("lauds", "00. Lauds for Sundays & Major Feasts*.docx"),
    ("vespers", "00. Vespers for Saturdays & Major Feasts*.docx"),
)

BENEDICTUS_TARGETS = (
    *(
        (
            f"{number} Sunday after Epiphany",
            f"proper/epiphany-sunday-{number}/benedictus-antiphon",
        )
        for number in range(2, 7)
    ),
    *(
        (
            f"{number} Sunday after Pentecost",
            f"proper/pentecost-sunday-{number}/benedictus-antiphon",
        )
        for number in range(3, 25)
    ),
)

MAGNIFICAT_TARGETS = (
    (
        "Saturdays Throughout the Year",
        "ordinary/vespers/magnificat-antiphon-saturday",
    ),
    (
        "Saturday before Septuagesima Sunday",
        "proper/septuagesima/magnificat-antiphon-first",
    ),
    (
        "Saturday before Sexagesima Sunday",
        "proper/sexagesima/magnificat-antiphon-first",
    ),
    (
        "Saturday before Quinquagesima Sunday",
        "proper/quinquagesima/magnificat-antiphon-first",
    ),
    *(
        (
            f"Saturday before the {number} Sunday after Pentecost",
            f"proper/pentecost-sunday-{number}/magnificat-antiphon-first",
        )
        for number in range(3, 12)
    ),
    *(
        (
            f"Saturday before the {week} Sunday of {month.title()}",
            f"proper/historia-{month}-{week}/magnificat-antiphon-first",
        )
        for month in ("august", "september", "october", "november")
        for week in range(1, 6)
    ),
)

STANDALONE_CANTICLE_BOOKS = (
    (
        "lauds",
        "Benedictus",
        "Antiphons on the Benedictus for Sunday Lauds Throughout the Year.docx",
        BENEDICTUS_TARGETS,
    ),
    (
        "vespers",
        "Magnificat",
        "Antiphons on the Magnificat for Saturday Vespers Throughout the Year.docx",
        MAGNIFICAT_TARGETS,
    ),
)

FERIAL_LAUDS_BOOK = "Copy of OCP Ferial Lauds.pdf"

# The 2025 Sunday/feast master already supplies newer witnesses for Wednesday.
# These twenty slots are the remaining per-annum Monday, Tuesday, Thursday,
# and Friday antiphons whose corpus provenance still points only to a generic
# Divinum Officium seed.  Each target page contains the printed antiphon before
# and/or after its psalm; extraction below requires every occurrence on that
# page to agree.
FERIAL_LAUDS_ANTIPHON_TARGETS = (
    ("Monday", 1, 7),
    ("Monday", 2, 8),
    ("Monday", 3, 9),
    ("Monday", 4, 10),
    ("Monday", 5, 11),
    ("Tuesday", 1, 23),
    ("Tuesday", 2, 24),
    ("Tuesday", 3, 25),
    ("Tuesday", 4, 26),
    ("Tuesday", 5, 28),
    ("Thursday", 1, 56),
    ("Thursday", 2, 57),
    ("Thursday", 3, 58),
    ("Thursday", 4, 60),
    ("Thursday", 5, 62),
    ("Friday", 1, 76),
    ("Friday", 2, 77),
    ("Friday", 3, 78),
    ("Friday", 4, 79),
    ("Friday", 5, 82),
)

HEADINGS = {
    "chapter": "THE CHAPTER",
    "short-responsory": "THE SHORT RESPONSORY",
    "hymn": "THE HYMN",
    "versicle": "THE VERSICLE",
    "gospel": "THE GOSPEL CANTICLE",
    "prayers": "THE PRAYERS",
}

BOILERPLATE_PREFIXES = (
    "after the last collect",
    "all is said as in",
    "here follow any special commem",
    "praise be to thee",
    "this collect is followed",
    "then, unless another office",
    "with this, the office is ended",
    "v. the lord be with you",
    "v. o lord, hear my prayer",
    "v. let us bless the lord",
    "v. may the",
    "our father is said secretly",
    "or, if not a priest",
    ", deliver us",
    "o , deliver us",
)

COMMON_OWNER_PATTERNS = (
    (r"common of (?:feasts of )?(?:the )?blessed virgin mary", "commons/blessed-virgin"),
    (r"common of (?:a )?confessor (?:and|&) bishop", "commons/confessor-bishop"),
    (r"common of (?:a )?confessor not a bishop", "commons/confessor"),
    (r"common of (?:the )?dedication", "commons/dedication"),
    (r"common of holy women", "commons/holy-woman"),
    (r"common of virgins", "commons/virgin"),
    (r"common of (?:apostles|apostles and evangelists).*out of paschaltide", "commons/apostle"),
    (r"common of (?:apostles|apostles and evangelists).*in paschaltide", "commons/apostle-paschal"),
    (r"common of one martyr out of paschaltide", "commons/martyr"),
    (r"common of one martyr in paschaltide", "commons/martyr-paschal"),
    (r"common of many martyrs", "commons/martyrs"),
)

PROPER_OWNER_PATTERNS = (
    (r"christ the king", "proper/christ-the-king"),
    (r"most holy body of christ", "proper/corpus-christi"),
)

OWNER_ALIASES = {
    # The lower-ranked duplicate record shares the principal feast's proper.
    "comm-extra-01-25-the-conversion-of-st-paul-the-apostle": "conversion-st-paul",
}


@dataclass(frozen=True)
class Paragraph:
    page: int
    text: str


@dataclass
class OfficeSection:
    source: str
    hour: str
    title: str
    variant: str
    start_page: int
    end_page: int
    paragraphs: list[Paragraph]


@dataclass
class SourceCandidate:
    source: str
    source_page: int
    hour: str
    office_title: str
    office_variant: str
    slot: str
    latin_incipit: str
    source_text: str
    candidate_id: str = ""
    corpus_key: str = ""
    current_text: str = ""
    text_similarity: float = 0.0
    title_similarity: float = 0.0
    confidence: str = "unmatched"
    review_flags: str = ""
    leverage_score: int = 0
    provenance_status: str = ""
    # Diurnal-ingest witness and runtime-resolution fields.  They default to
    # empty values so the established SR packet format and its stable IDs keep
    # their exact meaning for the existing book workflows.
    source_sha256: str = ""
    printed_page: str = ""
    source_bbox: str = ""
    source_column: str = ""
    source_offset: str = ""
    extractor: str = ""
    extraction_confidence: str = ""
    raw_text_sha256: str = ""
    canonical_owner: str = ""
    runtime_slot: str = ""
    runtime_target: str = ""
    resolution_tier: str = ""
    resolution_reason: str = ""
    discovery_classification: str = ""
    mapping_confidence: str = ""
    discovery_note: str = ""
    source_page_last: int = 0
    page_slices: str = ""


@dataclass(frozen=True)
class CorpusEntry:
    key: str
    file: str
    section: str
    text: str


def collapse_space(value: str) -> str:
    return re.sub(r"\s+", " ", value).strip()


def paragraph_text(node: ET.Element) -> str:
    return collapse_space("".join(t.text or "" for t in node.iter(W_NS + "t")))


def read_docx_paragraphs(path: pathlib.Path) -> list[Paragraph]:
    """Return non-empty Word paragraphs with last-rendered page numbers."""
    with zipfile.ZipFile(path) as archive:
        root = ET.fromstring(archive.read("word/document.xml"))

    page = 1
    paragraphs = []
    for node in root.iter(W_NS + "p"):
        text = paragraph_text(node)
        if text:
            paragraphs.append(Paragraph(page, text))
        page += sum(1 for _ in node.iter(W_NS + "lastRenderedPageBreak"))
    return paragraphs


def read_pdf_pages(path: pathlib.Path) -> dict[int, str]:
    """Return searchable PDF text keyed by one-based rendered page number."""
    result = subprocess.run(
        ["pdftotext", "-layout", str(path), "-"],
        check=True,
        text=True,
        capture_output=True,
    )
    return {
        number: page
        for number, page in enumerate(result.stdout.split("\f"), start=1)
        if page.strip()
    }


def extract_ferial_lauds_antiphons(
    source: str,
    pages: dict[int, str],
    targets: tuple[tuple[str, int, int], ...],
) -> list[SourceCandidate]:
    """Extract fixed weekday antiphon slots from the searchable OCP booklet."""
    candidates = []
    for weekday, slot, page_number in targets:
        page = pages.get(page_number, "")
        marker = re.compile(rf"Antiphon\s+{slot}:\s*(.+?)\s*$", re.I | re.M)
        source_forms = {
            collapse_space(match.group(1)).replace("†", "*")
            for match in marker.finditer(page)
        }
        if len(source_forms) != 1:
            raise ValueError(
                f"{source}: expected one distinct Antiphon {slot} form on "
                f"page {page_number}, found {sorted(source_forms)!r}"
            )
        source_text = collapse_space(source_forms.pop())
        weekday_key = weekday.lower()
        corpus_key = f"ordinary/lauds/psalm-antiphon-{slot}-{weekday_key}"
        candidates.append(
            SourceCandidate(
                source=source,
                source_page=page_number,
                hour="lauds",
                office_title=f"{weekday} Ferial Lauds",
                office_variant="per annum",
                slot=f"psalm-antiphon-{slot}-{weekday_key}",
                latin_incipit="",
                source_text=source_text,
                corpus_key=corpus_key,
            )
        )
    return candidates


def clean_canticle_underlay(paragraphs: list[Paragraph]) -> str:
    """Turn a standalone book's syllabified English chant underlay into prose."""
    lines = clean_lines(paragraphs)
    text = collapse_space(" ".join(lines))
    # These standalone books use hyphens and en/em dashes only to divide sung
    # syllables; their prose underlay contains no lexical hyphen compounds.
    text = re.sub(r"(?<=\w)\s*[-–—]\s*(?=\w)", "", text)
    text = re.sub(r"^([A-Z])\s+([a-z])", r"\1\2", text)
    # Word drop caps are stored in separate runs without a separating space.
    text = re.sub(r"\bI(?=(?:beseech|dwell|have|might|saw)\b)", "I ", text)
    text = re.sub(r"^As\.\s+(?=Jesus\b)", "As ", text)
    text = text.replace("highplaces", "high places")
    # A dagger in these antiphon books is the mediation mark represented by an
    # asterisk in the corpus.
    text = collapse_space(text.replace("†", "*"))
    return text


def extract_standalone_canticles(
    source: str,
    hour: str,
    canticle: str,
    paragraphs: list[Paragraph],
    targets: tuple[tuple[str, str], ...],
) -> list[SourceCandidate]:
    pattern = re.compile(rf"Antiphon on {re.escape(canticle)}\.\s*(.*)", re.I)
    markers = [
        (index, pattern.search(paragraph.text))
        for index, paragraph in enumerate(paragraphs)
        if pattern.search(paragraph.text)
    ]
    if len(markers) != len(targets):
        raise ValueError(
            f"{source}: expected {len(targets)} {canticle} antiphons, found {len(markers)}"
        )

    candidates = []
    for (start, match), (title, corpus_key) in zip(markers, targets):
        stop = start + 1
        while stop < len(paragraphs):
            if paragraphs[stop].text.upper().startswith("THE GOSPEL CANTICLE"):
                break
            stop += 1
        source_text = clean_canticle_underlay(paragraphs[start + 1 : stop])
        if not source_text:
            raise ValueError(f"{source}: empty underlay after {match.group(0)!r}")
        candidates.append(
            SourceCandidate(
                source=source,
                source_page=paragraphs[start].page,
                hour=hour,
                office_title=title,
                office_variant="first" if canticle.lower() == "magnificat" else "",
                slot=corpus_key.rsplit("/", 1)[1],
                latin_incipit=collapse_space(match.group(1)),
                source_text=source_text,
                corpus_key=corpus_key,
            )
        )
    return candidates


def is_chant_code(text: str) -> bool:
    private_use = sum(unicodedata.category(ch) == "Co" for ch in text)
    if private_use:
        return True
    compact = re.sub(r"\s+", "", text)
    if len(compact) < 25:
        return False
    chant_chars = sum(ch.lower() in "bcdfghjklmnoprstuvxyz" for ch in compact)
    punctuation = sum(ch in "cvxz{}[]<>" for ch in compact.lower())
    return punctuation > 8 and chant_chars / len(compact) > 0.55


def is_artifact(text: str) -> bool:
    text = text.strip()
    if not text or text == "\\" or re.fullmatch(r"\d+", text):
        return True
    if re.fullmatch(
        r"(?:(?:[ivx]+(?:\.(?:\d+|c))?|T\.P\.)\s*)+", text, re.I
    ):
        return True
    if is_chant_code(text):
        return True
    # Word running headers are often concatenated with their page number.
    if re.match(r"^\d{2,}.*(.{12,})\1", text):
        return True
    return False


def is_title_candidate(text: str) -> bool:
    lowered = collapse_space(text).lower()
    if is_artifact(text) or len(text) > 140:
        return False
    if lowered.startswith(BOILERPLATE_PREFIXES):
        return False
    if text == text.lower():
        # Calendar running heads such as "wednesday after the ii sunday in
        # paschaltide" sit immediately above the real, title-cased office.
        return False
    if lowered.startswith(("r. ", "v. ", "let us pray", "through ", "who with ")):
        return False
    if re.fullmatch(
        r"(?:january|february|march|april|may|june|july|august|"
        r"september|october|november|december)\s+\d+",
        lowered,
    ):
        return False
    if text.upper() in HEADINGS.values() or text.upper().startswith("AT "):
        return False
    if lowered in ("the temporal cycle", "the sanctoral cycle", "the commons"):
        return False
    return bool(re.search(r"[A-Za-z]", text))


def infer_title_and_variant(history: list[Paragraph], hour: str) -> tuple[str, str]:
    recent = history[-24:]
    variant = ""
    for paragraph in reversed(recent):
        upper = paragraph.text.upper()
        if upper.startswith("AT ") or "AT II VESPERS" in upper:
            if "II VESPERS" in upper:
                variant = "second"
            elif "I VESPERS" in upper:
                variant = "first"

    date_re = re.compile(
        r"^(?:january|february|march|april|may|june|july|august|september|"
        r"october|november|december)\s+\d+$",
        re.I,
    )
    last_date = -1
    for i, paragraph in enumerate(recent):
        if date_re.fullmatch(collapse_space(paragraph.text)):
            last_date = i

    candidates: list[Paragraph] = []
    for paragraph in reversed(recent[last_date + 1 :]):
        if paragraph.text.upper().startswith("AT "):
            continue
        if is_title_candidate(paragraph.text):
            if not candidates or collapse_space(paragraph.text).lower() != collapse_space(
                candidates[-1].text
            ).lower():
                candidates.append(paragraph)
            continue
        if candidates and paragraph.text.lower().startswith(BOILERPLATE_PREFIXES):
            break
    if candidates:
        # Titles frequently span two Word paragraphs (for example "The
        # Nativity of Our Lord" / "& THE SUNDAY WITHIN THE OCTAVE"). Keep the
        # short tail of that run while excluding prior-office boilerplate.
        nearest = candidates[0]
        title_parts = [nearest.text]
        if len(candidates) > 1:
            previous = candidates[1]
            continuation = nearest.text.lstrip().startswith(
                ("&", "AND ", "in Paschaltide", "out of Paschaltide")
            )
            incomplete_previous = bool(
                re.search(r"\b(?:the|of|for|and|saints?)\s*$", previous.text, re.I)
            )
            if continuation or incomplete_previous:
                title_parts.insert(0, previous.text)
        title = collapse_space(" ".join(title_parts))
        lowered = title.lower()
        if hour == "vespers" and not variant:
            if "ii vespers" in lowered:
                variant = "second"
            elif "saturday before" in lowered or "i vespers" in lowered:
                variant = "first"
        return title, variant
    return "Unidentified office", variant


def find_offices(path: pathlib.Path, hour: str) -> list[OfficeSection]:
    paragraphs = read_docx_paragraphs(path)
    starts = [
        i
        for i, paragraph in enumerate(paragraphs)
        if paragraph.text.startswith("Our Father. Hail Mary. O God, make speed")
    ]
    offices = []
    for position, start in enumerate(starts):
        end = starts[position + 1] if position + 1 < len(starts) else len(paragraphs)
        title, variant = infer_title_and_variant(paragraphs[:start], hour)
        block = paragraphs[start:end]
        if not block:
            continue
        offices.append(
            OfficeSection(
                source=path.name,
                hour=hour,
                title=title,
                variant=variant,
                start_page=block[0].page,
                end_page=block[-1].page,
                paragraphs=block,
            )
        )
    return offices


def clean_lines(paragraphs: list[Paragraph]) -> list[str]:
    lines = []
    for paragraph in paragraphs:
        text = collapse_space(paragraph.text)
        if is_artifact(text):
            continue
        if text.upper() in HEADINGS.values():
            continue
        if re.fullmatch(r"(?:Psalm|Magnificat|Benedictus)\b.*", text, re.I):
            continue
        if re.fullmatch(r"Ant\.\s+.*&c\.", text, re.I):
            continue
        text = re.sub(r"^([A-Z])\s+([a-z])", r"\1\2", text)
        lines.append(text)

    merged = []
    i = 0
    while i < len(lines):
        if len(lines[i]) == 1 and lines[i].isalpha() and i + 1 < len(lines):
            merged.append(lines[i] + lines[i + 1])
            i += 2
            continue
        merged.append(lines[i])
        i += 1
    return merged


def make_candidate(
    office: OfficeSection,
    slot: str,
    paragraphs: list[Paragraph],
    latin_incipit: str = "",
) -> SourceCandidate | None:
    lines = clean_lines(paragraphs)
    title_norm = normalize_for_comparison(office.title)
    filtered = []
    for line in lines:
        line_norm = normalize_for_comparison(line)
        if len(line_norm) > 8 and (line_norm == title_norm or line_norm in title_norm):
            continue
        if filtered and line == filtered[-1]:
            continue
        filtered.append(line)
    lines = filtered
    if not lines:
        return None
    source_text = "\n".join(lines)
    if len(normalize_for_comparison(source_text)) < 8:
        return None
    return SourceCandidate(
        source=office.source,
        source_page=paragraphs[0].page if paragraphs else office.start_page,
        hour=office.hour,
        office_title=office.title,
        office_variant=office.variant,
        slot=slot,
        latin_incipit=latin_incipit,
        source_text=source_text,
    )


def heading_index(paragraphs: list[Paragraph], heading: str) -> int | None:
    heading = heading.upper()
    for i, paragraph in enumerate(paragraphs):
        if heading in paragraph.text.upper():
            return i
    return None


def extract_psalm_antiphons(office: OfficeSection) -> list[SourceCandidate]:
    paragraphs = office.paragraphs
    end = heading_index(paragraphs, HEADINGS["chapter"])
    if end is None:
        return []
    candidates = []
    marker_re = re.compile(r"Antiphon\s+([1-5])[\.:]\s*([^\n]*)", re.I)
    for i, paragraph in enumerate(paragraphs[:end]):
        marker = marker_re.search(paragraph.text)
        if not marker:
            continue
        stop = i + 1
        while stop < end:
            next_text = paragraphs[stop].text
            if (
                re.match(r"Psalm\s+\d", next_text, re.I)
                or re.match(r"(?:The Song|Benedicite)\b", next_text, re.I)
                or marker_re.search(next_text)
            ):
                break
            stop += 1
        if stop == end:
            continue
        candidate = make_candidate(
            office,
            f"psalm-antiphon-{marker.group(1)}",
            paragraphs[i + 1 : stop],
            collapse_space(marker.group(2)),
        )
        if candidate:
            candidates.append(candidate)
    return candidates


def extract_between_headings(
    office: OfficeSection, slot: str, start_heading: str, end_heading: str
) -> SourceCandidate | None:
    paragraphs = office.paragraphs
    start = heading_index(paragraphs, start_heading)
    end = heading_index(paragraphs, end_heading)
    if start is None or end is None or end <= start:
        return None
    return make_candidate(office, slot, paragraphs[start + 1 : end])


def extract_gospel_antiphon(office: OfficeSection) -> SourceCandidate | None:
    pattern = re.compile(r"Antiphon on (Benedictus|Magnificat)\.\s*(.*)", re.I)
    paragraphs = office.paragraphs
    for i, paragraph in enumerate(paragraphs):
        match = pattern.search(paragraph.text)
        if not match:
            continue
        canticle = match.group(1).lower()
        if canticle == "benedictus":
            slot = "benedictus-antiphon"
        elif office.variant == "first":
            slot = "magnificat-antiphon-first"
        else:
            slot = "magnificat-antiphon"
        stop = i + 1
        while stop < len(paragraphs):
            text = paragraphs[stop].text
            if text.upper().startswith("THE PRAYERS") or re.match(
                r"(?:Magnificat|Benedictus),?\s+tone", text, re.I
            ):
                break
            stop += 1
        return make_candidate(office, slot, paragraphs[i + 1 : stop], match.group(2))
    return None


def extract_collect(office: OfficeSection) -> SourceCandidate | None:
    paragraphs = office.paragraphs
    prayers = heading_index(paragraphs, HEADINGS["prayers"])
    start_at = prayers + 1 if prayers is not None else 0
    marker = None
    for i in range(start_at, len(paragraphs)):
        text = paragraphs[i].text.lower()
        if "let us pray" in text and "collect" in text:
            marker = i
            break
    if marker is None:
        return None
    stop = marker + 1
    while stop < len(paragraphs):
        if re.match(r"R\.\s*Amen", paragraphs[stop].text, re.I):
            stop += 1
            break
        stop += 1
    return make_candidate(office, "collect", paragraphs[marker + 1 : stop])


def extract_candidates(office: OfficeSection) -> list[SourceCandidate]:
    candidates = extract_psalm_antiphons(office)

    def hour_slot(base: str) -> str:
        if office.hour == "vespers" and office.variant == "first":
            return f"{base}-first-vespers"
        return f"{base}-{office.hour}"

    pairs = (
        (hour_slot("chapter"), HEADINGS["chapter"], HEADINGS["short-responsory"]),
        (
            hour_slot("short-responsory"),
            HEADINGS["short-responsory"],
            HEADINGS["hymn"],
        ),
        (hour_slot("hymn"), HEADINGS["hymn"], HEADINGS["versicle"]),
        (hour_slot("versicle"), HEADINGS["versicle"], HEADINGS["gospel"]),
    )
    for slot, start, end in pairs:
        candidate = extract_between_headings(office, slot, start, end)
        if candidate:
            candidates.append(candidate)
    gospel = extract_gospel_antiphon(office)
    if gospel:
        candidates.append(gospel)
    collect = extract_collect(office)
    if collect:
        candidates.append(collect)
    return candidates


def load_ini_file(path: pathlib.Path, root: pathlib.Path) -> list[CorpusEntry]:
    relative = path.relative_to(root).as_posix()
    stem = path.stem
    directory = pathlib.PurePosixPath(relative).parent.as_posix()
    if directory == ".":
        directory = ""

    entries = []
    current = ""
    lines: list[str] = []

    def flush() -> None:
        nonlocal lines
        if not current:
            return
        key = "/".join(part for part in (directory, stem, current) if part)
        entries.append(CorpusEntry(key, relative, current, "\n".join(lines).strip()))
        lines = []

    raw = path.read_text(errors="replace")
    sections = list(re.finditer(r"^\[([A-Za-z0-9-]+)\]\s*$", raw, re.M))
    if not sections:
        key = "/".join(part for part in (directory, stem) if part)
        return [CorpusEntry(key, relative, "", raw.strip())]

    for line in raw.splitlines():
        stripped = line.strip()
        match = re.fullmatch(r"\[([A-Za-z0-9-]+)\]", stripped)
        if match:
            flush()
            current = match.group(1)
        elif current and not stripped.startswith("#"):
            lines.append(line)
    flush()
    return entries


def load_corpus(data_dir: pathlib.Path) -> dict[str, CorpusEntry]:
    text_root = data_dir / "texts"
    entries = {}
    for path in sorted(text_root.rglob("*.txt")):
        for entry in load_ini_file(path, text_root):
            entries[entry.key] = entry

    resolved = {}

    def resolve(entry: CorpusEntry, trail: tuple[str, ...] = ()) -> str:
        match = re.fullmatch(r"@use\s+([^\s]+)", entry.text)
        if not match:
            return entry.text
        target_key = match.group(1)
        if target_key in trail or target_key not in entries:
            return entry.text
        return resolve(entries[target_key], trail + (entry.key,))

    for key, entry in entries.items():
        resolved[key] = CorpusEntry(entry.key, entry.file, entry.section, resolve(entry))
    return resolved


def load_feast_names(data_dir: pathlib.Path) -> dict[str, str]:
    names = {}
    for path in sorted((data_dir / "feasts").glob("*.txt")):
        current = ""
        for line in path.read_text(errors="replace").splitlines():
            section = re.fullmatch(r"\[([A-Za-z0-9-]+)\]", line.strip())
            if section:
                current = section.group(1)
                continue
            name = re.match(r"\s*Name\s*=\s*(.+?)\s*$", line)
            if current and name:
                names[current] = name.group(1)
    return names


@functools.lru_cache(maxsize=None)
def normalize_for_comparison(text: str) -> str:
    text = re.sub(r"^![^\n]*", " ", text, flags=re.M)
    text = text.replace("&", " and ").replace("æ", "ae").replace("Æ", "Ae")
    text = unicodedata.normalize("NFKD", text)
    text = "".join(ch for ch in text if not unicodedata.combining(ch))
    text = text.lower().replace("holy spirit", "holy ghost")
    text = re.sub(r"\b(?:bow|stand|sit)\.?\b", " ", text)
    return "".join(re.findall(r"[a-z]+", text))


@functools.lru_cache(maxsize=None)
def comparison_words(text: str) -> frozenset[str]:
    text = re.sub(r"^![^\n]*", " ", text, flags=re.M)
    text = text.replace("&", " and ").replace("æ", "ae").replace("Æ", "Ae")
    text = unicodedata.normalize("NFKD", text)
    text = "".join(ch for ch in text if not unicodedata.combining(ch))
    text = re.sub(r"(?<=[A-Za-z])[\-–—](?=[A-Za-z])", "", text)
    return frozenset(re.findall(r"[a-z]+", text.lower()))


def word_similarity(source: str, current: str) -> float:
    left = comparison_words(source)
    right = comparison_words(current)
    if not left or not right:
        return 0.0
    return len(left & right) / min(len(left), len(right))


def text_similarity(source: str, current: str) -> float:
    left = normalize_for_comparison(source)
    right = normalize_for_comparison(current)
    if not left or not right:
        return 0.0
    shorter, longer = sorted((left, right), key=len)
    if len(shorter) >= 30 and shorter in longer:
        return min(1.0, 0.94 + 0.06 * len(shorter) / len(longer))
    return difflib.SequenceMatcher(None, left, right).ratio()


def anchored_text_similarity(source: str, current: str) -> float:
    """Compare known-owner texts without discarding repeated hymn anchors."""
    left = normalize_for_comparison(source)
    right = normalize_for_comparison(current)
    if not left or not right:
        return 0.0
    shorter, longer = sorted((left, right), key=len)
    if len(shorter) >= 30 and shorter in longer:
        return min(1.0, 0.94 + 0.06 * len(shorter) / len(longer))
    # Hymns and responsories repeat enough short words for SequenceMatcher's
    # auto-junk heuristic to discard meaningful anchors in longer texts.
    return difflib.SequenceMatcher(None, left, right, autojunk=False).ratio()


def title_similarity(title: str, key: str, feast_names: dict[str, str]) -> float:
    parts = key.split("/")
    identifier = parts[1] if len(parts) > 2 and parts[0] in ("proper", "commons") else ""
    title_words = title_tokens(title)
    targets = [title_tokens(identifier.replace("-", " "))]
    if identifier in feast_names:
        targets.append(title_tokens(feast_names[identifier]))
    if not title_words:
        return 0.0

    scores = []
    for target_words in targets:
        if not target_words:
            continue
        overlap = len(title_words & target_words)
        target_coverage = overlap / len(target_words)
        jaccard = overlap / len(title_words | target_words)
        scores.append(0.5 * target_coverage + 0.5 * jaccard)
    return max(scores, default=0.0)


def title_tokens(value: str) -> set[str]:
    value = unicodedata.normalize("NFKD", value.lower())
    value = "".join(ch for ch in value if not unicodedata.combining(ch))
    value = re.sub(
        r"^(?:january|february|march|april|may|june|july|august|september|"
        r"october|november|december)\s+\d{1,2}\s*[–-]\s*"
        r"(?:i{1,2}|first|second)?\s*vespers\s+for\s+",
        "",
        value,
    )
    value = re.sub(r"\bb\.?\s*v\.?\s*m\.?\b", "blessed virgin mary", value)
    aliases = {
        "first": "1",
        "second": "2",
        "third": "3",
        "fourth": "4",
        "fifth": "5",
        "sixth": "6",
        "i": "1",
        "ii": "2",
        "iii": "3",
        "iv": "4",
        "v": "5",
        "vi": "6",
        "st": "saint",
    }
    stop = {
        "the",
        "of",
        "our",
        "and",
        "at",
        "in",
        "before",
        "after",
        "feast",
        "for",
        "vespers",
    }
    words = re.findall(r"[a-z]+|\d+", value)
    return {aliases.get(word, word) for word in words if word not in stop}


def slot_compatibility(slot: str, section: str) -> float:
    if slot == section:
        return 1.0
    if slot.endswith("-first-vespers"):
        generic = slot.removesuffix("-first-vespers") + "-vespers"
        if section == generic:
            return 0.90
    if slot == "psalm-antiphon-1" and section == "psalm-antiphon":
        return 0.96
    if slot.endswith(("-lauds", "-vespers")):
        base = slot.rsplit("-", 1)[0]
        if section == base:
            return 0.90
    if slot == "magnificat-antiphon-first" and section == "magnificat-antiphon":
        return 0.75
    if slot == "magnificat-antiphon" and section == "magnificat-antiphon-first":
        return 0.70
    return 0.0


def infer_owner(
    title: str, corpus: dict[str, CorpusEntry], feast_names: dict[str, str]
) -> tuple[str, float]:
    normalized_title = collapse_space(title).lower()
    for pattern, owner in COMMON_OWNER_PATTERNS:
        if re.search(pattern, normalized_title):
            return owner, 1.0
    for pattern, owner in PROPER_OWNER_PATTERNS:
        if re.search(pattern, normalized_title):
            return owner, 1.0

    proper_ids = {
        key.split("/")[1]
        for key in corpus
        if key.startswith("proper/") and len(key.split("/")) > 2
    } | set(feast_names)
    scored = [
        (
            title_similarity(title, f"proper/{identifier}/candidate", feast_names),
            identifier,
        )
        for identifier in proper_ids
    ]
    scored.sort(reverse=True)
    score, identifier = scored[0] if scored else (0.0, "")
    runner_up = scored[1][0] if len(scored) > 1 else 0.0
    if score >= 0.72 and score - runner_up >= 0.08:
        identifier = OWNER_ALIASES.get(identifier, identifier)
        return f"proper/{identifier}", score
    return "", score


def fallback_entries(
    owner: str, candidate: SourceCandidate, corpus: dict[str, CorpusEntry]
) -> list[tuple[float, CorpusEntry]]:
    """Return legal runtime fallback targets for an absent proper slot."""
    identifier = owner.removeprefix("proper/")
    seasonal = ""
    if identifier.startswith("advent-sunday-") or identifier == "vigil-nativity":
        seasonal = "advent"
    elif identifier.startswith("lent-sunday-") or identifier == "laetare-sunday":
        seasonal = "lent"

    possible: list[tuple[float, CorpusEntry]] = []
    if seasonal:
        prefix = f"seasonal/{seasonal}/"
        for key, entry in corpus.items():
            if not key.startswith(prefix):
                continue
            compatibility = slot_compatibility(candidate.slot, entry.section)
            if (
                seasonal == "advent"
                and candidate.slot == "magnificat-antiphon-first"
                and entry.section.startswith("magnificat-antiphon-december-")
            ):
                compatibility = 0.90
            if compatibility:
                possible.append((compatibility, entry))

    weekday = {"ash-wednesday": "wednesday"}.get(identifier, "")
    if weekday:
        prefix = f"ordinary/{candidate.hour}/"
        for key, entry in corpus.items():
            if not key.startswith(prefix):
                continue
            section = entry.section
            compatibility = slot_compatibility(candidate.slot, section)
            if section.endswith(f"-{weekday}"):
                compatibility = slot_compatibility(
                    candidate.slot, section.removesuffix(f"-{weekday}")
                )
            if compatibility:
                possible.append((compatibility, entry))

    return possible


def source_review_flags(candidate: SourceCandidate) -> list[str]:
    """Identify source blocks that need modeling rather than direct copying."""
    text = candidate.source_text
    flags = []
    if len(text) > 1800:
        flags.append("long extraction")
    if "¶" in text or re.search(
        r"\b(?:in|out of) paschaltide\b|\bP\.\s*T\.|\bT\.\s*P\.|"
        r"^for (?:a|the) (?:doctor|confessor|bishop|patron)|"
        r"^if\b(?!\s+i\s+be\s+a\s+man\s+of\s+god\b)|"
        r"\bmay be (?:said|used)\b|"
        r"\blast two lines\b|\bor else\b|\bsaturday before\b",
        text,
        re.I | re.M,
    ):
        flags.append("rubrical or seasonal variants")
    if re.search(
        r"\b(?:January|February|March|April|May|June|July|August|September|"
        r"October|November|December)\s+\d{1,2}\s+[–-]",
        text,
        re.I,
    ):
        flags.append("embedded page header")
    if len(re.findall(r"(?<=[A-Za-z])[–-](?=[A-Za-z])", text)) >= 4:
        flags.append("chant underlay requires cleanup")
    return flags


def classify_candidate(candidate: SourceCandidate, source_score: float) -> None:
    if source_score >= 0.985:
        confidence = "exact"
    elif source_score >= 0.84:
        confidence = "near"
    elif source_score >= 0.55:
        confidence = "different"
    else:
        confidence = "weak"
    flags = source_review_flags(candidate)
    candidate.review_flags = "; ".join(flags)
    if source_requires_modeling(flags) and confidence in ("near", "different"):
        confidence = "complex"
    candidate.confidence = confidence


def source_requires_modeling(flags: list[str]) -> bool:
    return bool(
        {"long extraction", "rubrical or seasonal variants"}.intersection(flags)
    )


def owner_entries(
    owner: str, candidate: SourceCandidate, corpus: dict[str, CorpusEntry]
) -> list[tuple[float, CorpusEntry]]:
    possible = []
    prefix = owner + "/"
    for key, entry in corpus.items():
        if not key.startswith(prefix):
            continue
        compatibility = slot_compatibility(candidate.slot, entry.section)
        if compatibility:
            possible.append((compatibility, entry))
    return possible


def load_review_queue(
    office_binary: pathlib.Path | None, start: int, years: int
) -> dict[str, dict]:
    if office_binary is None or not office_binary.exists():
        return {}
    command = [
        str(office_binary.resolve()),
        "review",
        "provenance-queue",
        "-start",
        str(start),
        "-years",
        str(years),
    ]
    result = subprocess.run(command, check=True, text=True, capture_output=True)
    queue = {}
    for row in csv.DictReader(result.stdout.splitlines()):
        queue[row["key"]] = {"score": int(row["score"]), "status": row["status"]}
    return queue


def reconcile(
    candidates: list[SourceCandidate],
    corpus: dict[str, CorpusEntry],
    feast_names: dict[str, str],
    queue: dict[str, dict],
) -> None:
    by_section: dict[str, list[CorpusEntry]] = {}
    for entry in corpus.values():
        by_section.setdefault(entry.section, []).append(entry)
    owners = {
        candidate.office_title: infer_owner(candidate.office_title, corpus, feast_names)
        for candidate in candidates
    }

    # A first-Vespers form deserves its own corpus key only when the books
    # also provide a divergent later-Vespers form. If the first form is the
    # only witness, compare it with the generic key instead of manufacturing
    # an unnecessary override.
    for candidate in candidates:
        if not candidate.slot.endswith("-first-vespers"):
            continue
        owner = owners[candidate.office_title][0]
        generic = candidate.slot.removesuffix("-first-vespers") + "-vespers"
        later = [
            other
            for other in candidates
            if other is not candidate
            and other.slot == generic
            and other.hour == "vespers"
            and owners[other.office_title][0] == owner
            and owner
        ]
        if not later or max(
            anchored_text_similarity(candidate.source_text, other.source_text)
            for other in later
        ) >= 0.84:
            candidate.slot = generic

    for candidate in candidates:
        # Standalone canticle books have a fixed liturgical sequence and Latin
        # incipits, so their corpus targets are assigned explicitly during
        # extraction rather than inferred through fuzzy title matching.
        if candidate.corpus_key:
            entry = corpus.get(candidate.corpus_key)
            review = queue.get(candidate.corpus_key, {})
            candidate.title_similarity = 1.0
            candidate.leverage_score = review.get("score", 0)
            candidate.provenance_status = review.get("status", "")
            if entry:
                candidate.current_text = entry.text
                score = anchored_text_similarity(candidate.source_text, entry.text)
                candidate.text_similarity = round(score, 3)
                classify_candidate(candidate, score)
            else:
                candidate.confidence = "missing"
                candidate.provenance_status = "missing"
                flags = source_review_flags(candidate)
                candidate.review_flags = "; ".join(flags)
                if source_requires_modeling(flags):
                    candidate.confidence = "complex"
            continue

        if (
            candidate.hour == "vespers"
            and "saturdays throughout the year" in candidate.office_title.lower()
        ):
            section = candidate.slot.removesuffix("-vespers")
            key = f"ordinary/vespers/{section}-saturday"
            candidate.corpus_key = key
            entry = corpus.get(key)
            review = queue.get(key, {})
            if entry:
                candidate.current_text = entry.text
                score = anchored_text_similarity(candidate.source_text, entry.text)
                candidate.text_similarity = round(score, 3)
                candidate.leverage_score = review.get("score", 0)
                candidate.provenance_status = review.get("status", "")
                classify_candidate(candidate, score)
            else:
                candidate.confidence = "missing"
                candidate.provenance_status = "missing"
                flags = source_review_flags(candidate)
                candidate.review_flags = "; ".join(flags)
                if source_requires_modeling(flags):
                    candidate.confidence = "complex"
                generic = f"ordinary/vespers/{section}"
                candidate.leverage_score = queue.get(generic, {}).get("score", 0)
            continue

        owner, owner_score = owners[candidate.office_title]
        if owner:
            owned = owner_entries(owner, candidate, corpus)
            if not owned:
                fallbacks = fallback_entries(owner, candidate, corpus)
                if fallbacks:
                    possible = []
                    for compatibility, entry in fallbacks:
                        source_score = anchored_text_similarity(
                            candidate.source_text, entry.text
                        )
                        combined = compatibility * source_score
                        possible.append((combined, source_score, entry))
                    _, source_score, entry = max(possible, key=lambda item: item[:2])
                    candidate.corpus_key = entry.key
                    candidate.current_text = entry.text
                    candidate.text_similarity = round(source_score, 3)
                    candidate.title_similarity = round(owner_score, 3)
                    review = queue.get(entry.key, {})
                    candidate.leverage_score = review.get("score", 0)
                    candidate.provenance_status = review.get("status", "")
                    classify_candidate(candidate, source_score)
                    continue
                candidate.corpus_key = f"{owner}/{candidate.slot}"
                candidate.title_similarity = round(owner_score, 3)
                candidate.confidence = "missing"
                candidate.provenance_status = "missing"
                flags = source_review_flags(candidate)
                candidate.review_flags = "; ".join(flags)
                if source_requires_modeling(flags):
                    candidate.confidence = "complex"
                candidate.leverage_score = max(
                    (
                        details["score"]
                        for key, details in queue.items()
                        if key.startswith(owner + "/")
                    ),
                    default=0,
                )
                continue

            possible = []
            for compatibility, entry in owned:
                source_score = anchored_text_similarity(
                    candidate.source_text, entry.text
                )
                heading_score = title_similarity(candidate.office_title, entry.key, feast_names)
                combined = compatibility * (0.82 * source_score + 0.18 * heading_score)
                possible.append((combined, source_score, heading_score, entry))
            combined, source_score, heading_score, entry = max(
                possible, key=lambda item: item[:3]
            )
            if (
                candidate.slot.endswith("-first-vespers")
                and entry.section
                == candidate.slot.removesuffix("-first-vespers") + "-vespers"
                and source_score < 0.84
            ):
                candidate.corpus_key = f"{owner}/{candidate.slot}"
                candidate.current_text = entry.text
                candidate.text_similarity = round(source_score, 3)
                candidate.title_similarity = round(owner_score, 3)
                candidate.confidence = "missing"
                candidate.provenance_status = "missing"
                flags = source_review_flags(candidate)
                candidate.review_flags = "; ".join(flags)
                if source_requires_modeling(flags):
                    candidate.confidence = "complex"
                candidate.leverage_score = max(
                    (
                        details["score"]
                        for key, details in queue.items()
                        if key.startswith(owner + "/")
                    ),
                    default=0,
                )
                continue
            candidate.corpus_key = entry.key
            candidate.current_text = entry.text
            candidate.text_similarity = round(source_score, 3)
            candidate.title_similarity = round(heading_score, 3)
            review = queue.get(entry.key, {})
            candidate.leverage_score = review.get("score", 0)
            candidate.provenance_status = review.get("status", "")
            classify_candidate(candidate, source_score)
            continue

        rough = []
        for section, entries in by_section.items():
            compatibility = slot_compatibility(candidate.slot, section)
            if not compatibility:
                continue
            for entry in entries:
                heading_score = title_similarity(candidate.office_title, entry.key, feast_names)
                token_score = word_similarity(candidate.source_text, entry.text)
                rough_score = compatibility * (0.78 * token_score + 0.22 * heading_score)
                rough.append((rough_score, compatibility, heading_score, entry))
        # Full sequence comparison is the expensive part. Word overlap and
        # office-title agreement safely narrow each slot to a small shortlist.
        shortlist = sorted(rough, key=lambda item: item[0], reverse=True)[:6]
        possible = []
        for _, compatibility, heading_score, entry in shortlist:
            source_score = text_similarity(candidate.source_text, entry.text)
            combined = compatibility * (0.82 * source_score + 0.18 * heading_score)
            possible.append((combined, source_score, heading_score, entry))
        if not possible:
            continue
        combined, source_score, heading_score, entry = max(possible, key=lambda item: item[:3])
        if combined < 0.28:
            continue
        candidate.corpus_key = entry.key
        candidate.current_text = entry.text
        candidate.text_similarity = round(source_score, 3)
        candidate.title_similarity = round(heading_score, 3)
        review = queue.get(entry.key, {})
        candidate.leverage_score = review.get("score", 0)
        candidate.provenance_status = review.get("status", "")
        classify_candidate(candidate, source_score)


def candidate_fingerprint(candidate: SourceCandidate) -> str:
    raw = "\x1f".join(
        (
            candidate.source,
            str(candidate.source_page),
            candidate.office_title,
            candidate.slot,
            candidate.latin_incipit,
        )
    )
    return hashlib.sha1(raw.encode()).hexdigest()[:8]


def assign_candidate_ids(candidates: list[SourceCandidate]) -> None:
    ordered = sorted(
        candidates,
        key=lambda candidate: (
            candidate.source,
            candidate.source_page,
            candidate.office_title,
            candidate.slot,
            candidate.latin_incipit,
        ),
    )
    for number, candidate in enumerate(ordered, 1):
        candidate.candidate_id = f"SR-{number:04d}-{candidate_fingerprint(candidate)}"


def deduplicated_actionable(
    candidates: list[SourceCandidate], decided: set[str] | None = None
) -> list[SourceCandidate]:
    decided = decided or set()
    best: dict[str, SourceCandidate] = {}
    for candidate in candidates:
        if candidate.candidate_id in decided or not candidate.corpus_key or candidate.confidence not in (
            "missing",
            "near",
            "different",
        ):
            continue
        previous = best.get(candidate.corpus_key)
        if previous is None or (
            candidate.leverage_score,
            candidate.text_similarity,
        ) > (
            previous.leverage_score,
            previous.text_similarity,
        ):
            best[candidate.corpus_key] = candidate
    return sorted(
        best.values(),
        key=lambda candidate: (
            -candidate.leverage_score,
            {"missing": 0, "near": 1, "different": 2}[candidate.confidence],
            -candidate.text_similarity,
            candidate.corpus_key,
        ),
    )


def fenced(text: str) -> str:
    return "```text\n" + text.strip() + "\n```"


def render_candidate(candidate: SourceCandidate) -> str:
    lines = [
        f"## {candidate.candidate_id} — `{candidate.corpus_key or 'unmatched'}`",
        "",
        f"- Source: `{candidate.source}`, rendered page {candidate.source_page}",
        f"- Office: {candidate.office_title} "
        f"({candidate.hour}{', ' + candidate.office_variant if candidate.office_variant else ''})",
        f"- Slot: `{candidate.slot}`; text similarity {candidate.text_similarity:.3f}; "
        f"leverage {candidate.leverage_score}; class `{candidate.confidence}`",
    ]
    if candidate.review_flags:
        lines.append(f"- Deferred-modeling flags: {candidate.review_flags}")
    if candidate.latin_incipit:
        lines.append(f"- Latin incipit: {candidate.latin_incipit}")
    lines.extend(("", "Source extraction:", "", fenced(candidate.source_text)))
    if candidate.current_text:
        lines.extend(("", "Current corpus:", "", fenced(candidate.current_text)))
    lines.extend(
        (
            "",
            "Pairing decision: retain current / replace from source / edit manually / defer.",
            "",
        )
    )
    return "\n".join(lines)


def load_decisions(output_dir: pathlib.Path) -> dict[str, dict[str, str]]:
    path = output_dir / "decisions.csv"
    if not path.exists():
        return {}
    with path.open(newline="") as handle:
        return {
            row["candidate_id"]: row
            for row in csv.DictReader(handle)
            if row.get("candidate_id")
        }


def write_decisions(output_dir: pathlib.Path, decisions: dict[str, dict[str, str]]) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    path = output_dir / "decisions.csv"
    temporary = path.with_suffix(".csv.tmp")
    with temporary.open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=DECISION_FIELDS)
        writer.writeheader()
        for identifier in sorted(decisions):
            writer.writerow(decisions[identifier])
    temporary.replace(path)


def write_outputs(
    output_dir: pathlib.Path,
    candidates: list[SourceCandidate],
    offices: list[OfficeSection],
    batch_size: int,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    batches_dir = output_dir / "batches"
    batches_dir.mkdir(exist_ok=True)
    for old_batch in batches_dir.glob("batch-*.md"):
        old_batch.unlink()

    payload = {
        "offices": [
            {
                "source": office.source,
                "hour": office.hour,
                "title": office.title,
                "variant": office.variant,
                "start_page": office.start_page,
                "end_page": office.end_page,
            }
            for office in offices
        ],
        "candidates": [asdict(candidate) for candidate in candidates],
    }
    (output_dir / "candidates.json").write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n"
    )

    fields = (
        "candidate_id",
        "confidence",
        "review_flags",
        "leverage_score",
        "provenance_status",
        "corpus_key",
        "text_similarity",
        "title_similarity",
        "source",
        "source_page",
        "hour",
        "office_title",
        "office_variant",
        "slot",
        "latin_incipit",
    )
    with (output_dir / "candidates.csv").open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fields)
        writer.writeheader()
        for candidate in candidates:
            row = asdict(candidate)
            writer.writerow({field: row[field] for field in fields})

    decisions = load_decisions(output_dir)
    actionable = deduplicated_actionable(candidates, set(decisions))
    counts = {}
    for candidate in candidates:
        counts[candidate.confidence] = counts.get(candidate.confidence, 0) + 1

    overview = [
        "# Disposable source reconciliation",
        "",
        "Generated source excerpts live only in this gitignored output directory.",
        "Nothing in this report is an automatic edit or attestation.",
        "",
        f"- Parsed offices: {len(offices)}",
        f"- Extracted candidates: {len(candidates)}",
        f"- Unique actionable corpus keys: {len(actionable)}",
        f"- Locally recorded decisions: {len(decisions)}",
        "- Candidate classes: "
        + ", ".join(f"{key}={value}" for key, value in sorted(counts.items())),
        "",
        "Review the numbered files under `batches/`, then ask Codex to process the listed candidate IDs.",
        "Source blocks with rubrical or seasonal alternatives are listed in `complex.md` and kept out of the low-risk batches.",
        "Record retain/defer/applied judgments with the `decide` command so they stay out of regenerated batches.",
        "",
        "## First actionable candidates",
        "",
    ]
    for candidate in actionable[:20]:
        overview.append(
            f"- `{candidate.candidate_id}` → `{candidate.corpus_key}` "
            f"({candidate.confidence}, similarity {candidate.text_similarity:.3f}, "
            f"leverage {candidate.leverage_score})"
        )
    (output_dir / "README.md").write_text("\n".join(overview) + "\n")

    complex_candidates = [
        candidate
        for candidate in candidates
        if candidate.confidence == "complex" and candidate.candidate_id not in decisions
    ]
    complex_lines = [
        "# Deferred source blocks",
        "",
        "These candidates contain alternatives or rubrics that need explicit data/engine modeling; do not copy them as one corpus value.",
        "",
    ]
    for candidate in sorted(
        complex_candidates,
        key=lambda item: (-item.leverage_score, item.corpus_key, item.candidate_id),
    ):
        complex_lines.append(
            f"- `{candidate.candidate_id}` → `{candidate.corpus_key or 'unmatched'}` "
            f"({candidate.review_flags}; page {candidate.source_page})"
        )
    (output_dir / "complex.md").write_text("\n".join(complex_lines) + "\n")

    for index in range(0, len(actionable), batch_size):
        batch = actionable[index : index + batch_size]
        number = index // batch_size + 1
        header = [
            f"# Source reconciliation batch {number}",
            "",
            "Review each source/current pair. No edits have been applied.",
            "",
        ]
        body = "\n".join(render_candidate(candidate) for candidate in batch)
        (batches_dir / f"batch-{number:02d}.md").write_text("\n".join(header) + body)


DISCOVERY_CLASSES = (
    "verify-existing",
    "missing-override",
    "fallback-equal",
    "existing-different",
    "unmodeled-slot",
    "known-owner-unobserved",
    "unknown-feast",
    "ambiguous-owner/context",
    "rubrical-complex",
)


def load_json(path: pathlib.Path) -> object:
    """Read a JSON artifact with an error that names the artifact."""
    try:
        return json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        raise ValueError(f"invalid JSON in {path}: {exc}") from exc


def sha256_file(path: pathlib.Path) -> str:
    """Return the full digest for a materialized, non-directory input."""
    if not path.is_file():
        raise ValueError(f"dependency is not a file: {path}")
    return hashlib.sha256(path.read_bytes()).hexdigest()


def sha256_tree(path: pathlib.Path) -> str:
    """Hash a directory's file names and contents, including an empty tree."""
    root = path.resolve()
    if not root.is_dir():
        raise ValueError(f"dependency is not a directory: {path}")
    digest = hashlib.sha256()
    for child in sorted(item for item in root.rglob("*") if item.is_file()):
        relative = confined_dependency_path(child, root, "dependency tree entry")
        digest.update(relative.encode())
        digest.update(b"\0")
        digest.update(bytes.fromhex(sha256_file(child)))
    return digest.hexdigest()


def confined_dependency_path(
    path: pathlib.Path, root: pathlib.Path, label: str
) -> str:
    """Return a stable relative locator, rejecting traversal and symlinks.

    Discovery reports can move between worktrees, so host-absolute paths are
    both non-portable and an unnecessary disclosure.  The only paths that may
    be declared are files genuinely below either the repository or the intake
    run supplied to this invocation.
    """
    resolved_root = root.resolve()
    resolved_path = path.resolve()
    try:
        relative = resolved_path.relative_to(resolved_root)
    except ValueError as exc:
        raise ValueError(f"{label}: dependency lies outside its allowed root") from exc
    if not relative.parts or any(part in {"", ".", ".."} for part in relative.parts):
        raise ValueError(f"{label}: malformed dependency path")
    if not resolved_path.is_file():
        raise ValueError(f"{label}: dependency is not a file")
    return relative.as_posix()


def declared_intake_artifact_paths(
    intake_dir: pathlib.Path, records: list[dict]
) -> list[pathlib.Path]:
    """Return every canonical file declared by the intake run.

    The report never copies extracted text.  Hashing the manifest/page records
    and their declared text artifacts instead lets a later process prove the
    exact materialized inputs have not changed.
    """
    paths: set[pathlib.Path] = set()
    manifest = intake_dir / "manifest.json"
    if manifest.exists():
        confined_dependency_path(manifest, intake_dir, "intake manifest")
        paths.add(manifest.resolve())

    for page_record in sorted((intake_dir / "pages").glob("*/page.json")):
        confined_dependency_path(page_record, intake_dir, "intake page record")
        paths.add(page_record.resolve())

    for record in records:
        for field in ("text_path", "ocr_path", "native_path"):
            value = record.get(field)
            if value in (None, ""):
                continue
            if not isinstance(value, str):
                raise ValueError(f"intake {field}: dependency path must be a string")
            declared = pathlib.PurePosixPath(value)
            if declared.is_absolute() or ".." in declared.parts or not declared.parts:
                raise ValueError(f"intake {field}: dependency lies outside its allowed root")
            artifact = intake_dir.joinpath(*declared.parts)
            confined_dependency_path(artifact, intake_dir, f"intake {field}")
            paths.add(artifact.resolve())
    return sorted(paths, key=lambda path: confined_dependency_path(path, intake_dir, "intake artifact"))


def discovery_dependency_manifest(
    *,
    intake_dir: pathlib.Path,
    intake_records: list[dict],
    profile_path: pathlib.Path | None,
    inventory_path: pathlib.Path | None,
    data_dir: pathlib.Path,
) -> dict:
    """Build the complete, portable dependency contract for discovery.

    Entries contain only structural metadata, root-relative artifact locators,
    and full SHA-256s. Locators may retain intake page-directory names, but the
    manifest omits source identifiers, bounding boxes, and extracted book text.
    """
    entries: list[dict[str, str]] = []

    if not intake_dir.is_dir():
        raise ValueError("intake root: dependency root is not a directory")
    if not data_dir.is_dir():
        raise ValueError("data root: dependency root is not a directory")
    try:
        data_dir.resolve().relative_to(ROOT.resolve())
    except ValueError as exc:
        raise ValueError("data root: dependency lies outside its allowed root") from exc
    try:
        intake_locator = intake_dir.resolve().relative_to((ROOT / "output").resolve())
    except ValueError as exc:
        raise ValueError("intake root: dependency lies outside repository output") from exc

    def add(role: str, path: pathlib.Path, root: pathlib.Path) -> None:
        entries.append({
            "kind": "file",
            "role": role,
            "root": "intake" if root.resolve() == intake_dir.resolve() else "repository",
            "path": confined_dependency_path(path, root, role),
            "sha256": sha256_file(path.resolve()),
        })

    def add_tree(role: str, path: pathlib.Path, root: pathlib.Path) -> None:
        resolved = path.resolve()
        try:
            relative = resolved.relative_to(root.resolve())
        except ValueError as exc:
            raise ValueError(f"{role}: dependency lies outside its allowed root") from exc
        entries.append({
            "kind": "tree",
            "role": role,
            "root": "intake" if root.resolve() == intake_dir.resolve() else "repository",
            "path": relative.as_posix() or ".",
            "sha256": sha256_tree(resolved),
        })

    if profile_path is not None:
        add("profile", profile_path, ROOT)
    if inventory_path is not None:
        add("resolution-inventory", inventory_path, ROOT)

    for path in declared_intake_artifact_paths(intake_dir, intake_records):
        add("intake-artifact", path, intake_dir)

    text_root = data_dir / "texts"
    feasts_root = data_dir / "feasts"
    for path in sorted(text_root.rglob("*.txt")):
        add("corpus-text", path, ROOT)
    for path in sorted(feasts_root.glob("*.txt")):
        add("feast-metadata", path, ROOT)

    # Individual hashes identify the exact materialized inputs. Tree hashes
    # additionally detect a later file addition or deletion that is absent
    # from this frozen list.
    add_tree("intake-tree", intake_dir, intake_dir)
    add_tree("corpus-tree", text_root, ROOT)
    add_tree("feast-metadata-tree", feasts_root, ROOT)

    entries.sort(
        key=lambda entry: (
            entry["root"], entry["role"], entry["kind"], entry["path"]
        )
    )
    return {
        "schema_version": 1,
        "roots": {
            "repository": ".",
            "intake": (pathlib.PurePosixPath("output") / intake_locator).as_posix(),
        },
        "entries": entries,
    }


def intake_text(intake_dir: pathlib.Path, record: dict) -> str:
    """Read the selected intake witness safely, falling back to embedded text."""
    embedded = record.get("canonical_text", record.get("selected_text", record.get("text", "")))
    for field in ("text_path", "ocr_path", "native_path"):
        value = record.get(field)
        if not isinstance(value, str) or not value:
            continue
        path = (intake_dir / value).resolve()
        try:
            path.relative_to(intake_dir.resolve())
        except ValueError:
            continue
        if path.is_file():
            return path.read_text(encoding="utf-8", errors="replace")
    return str(embedded)


def normalize_column_records(intake_dir: pathlib.Path, records: list[dict]) -> list[dict]:
    """Expand only explicitly declared column witnesses; fail closed on bad maps."""
    normalized: list[dict] = []
    saw_columns = any(record.get("columns") is not None for record in records)
    for record in records:
        columns = record.get("columns")
        if columns is None:
            if saw_columns:
                raise ValueError("column intake cannot mix page and column witnesses")
            normalized.append(record)
            continue
        if not isinstance(columns, list) or not columns:
            raise ValueError("intake page declares columns but has no column witnesses")
        for column in columns:
            if not isinstance(column, dict) or not isinstance(column.get("column"), str):
                raise ValueError("intake column witness must declare source_column")
            bbox = column.get("bbox")
            if not isinstance(bbox, list) or len(bbox) != 4:
                raise ValueError("intake column witness must preserve bbox")
            item = dict(record)
            item.update({
                "source_column": column["column"],
                "bbox": bbox,
                "text_path": column.get("text_path", record.get("text_path", "")),
                "raw_text_sha256": column.get("raw_text_sha256", ""),
                "extractor": column.get("extractor", ""),
                "canonical_text": column.get("canonical_text", ""),
            })
            if not item["text_path"] or not item["raw_text_sha256"]:
                raise ValueError("column witness requires its own text artifact and SHA-256")
            path = (intake_dir / item["text_path"]).resolve()
            try:
                path.relative_to(intake_dir.resolve())
            except ValueError as exc:
                raise ValueError("column text artifact escapes intake root") from exc
            if not path.is_file():
                raise ValueError("column text artifact is missing")
            actual = hashlib.sha256(path.read_bytes()).hexdigest()
            if actual != str(item["raw_text_sha256"]).lower():
                raise ValueError("column text artifact SHA-256 does not match witness")
            item["canonical_text"] = path.read_text(encoding="utf-8", errors="replace")
            normalized.append(item)
    if saw_columns:
        return sorted(normalized, key=lambda item: (str(item.get("source_column", "")), int(item.get("page", item.get("page_number", 0)) or 0)))
    return normalized


def load_diurnal_profile(path: pathlib.Path | None) -> dict:
    """Load the deliberately small edition-specific mapping profile.

    Profiles only suggest headings, owners, and slots; uncertain material is
    emitted for review rather than becoming a corpus edit.
    """
    if path is None:
        return {}
    payload = load_json(path)
    if not isinstance(payload, dict):
        raise ValueError(f"{path}: profile must be a JSON object")
    return payload


def profile_include_pages(profile: dict) -> set[int] | None:
    """Validate the explicit PDF-page allowlist used before normalization."""
    if "include_pages" not in profile:
        return None
    values = profile["include_pages"]
    if not isinstance(values, list) or not values:
        raise ValueError("profile include_pages must be a non-empty list")
    pages: set[int] = set()
    for value in values:
        if isinstance(value, bool) or not isinstance(value, int) or value < 1:
            raise ValueError("profile include_pages must contain positive integers")
        if value in pages:
            raise ValueError(f"profile include_pages repeats page {value}")
        pages.add(value)
    return pages


def intake_page_records(intake_dir: pathlib.Path, include_pages: set[int] | None = None) -> list[dict]:
    """Return normalized page records from a diurnal-intake run.

    The intake command writes one JSON artifact per page.  This reader is
    intentionally tolerant of the two useful layouts (a manifest ``pages``
    list or individual ``page*.json`` records) so a later extractor revision
    does not invalidate review evidence already on disk.
    """
    records: list[dict] = []
    manifest = intake_dir / "manifest.json"
    if manifest.exists():
        payload = load_json(manifest)
        if not isinstance(payload, dict):
            raise ValueError(f"{manifest}: manifest must be a JSON object")
        pages = payload.get("pages", [])
        if isinstance(pages, list):
            records.extend(item for item in pages if isinstance(item, dict))
        elif isinstance(pages, dict):
            records.extend(item for item in pages.values() if isinstance(item, dict))
        else:
            raise ValueError(f"{manifest}: pages must be a list or object")

        declared_pages = {
            int(record.get("page", record.get("page_number", 0)) or 0): record
            for record in records
        }
        source_hashes = {
            str(record.get("source_sha256", record.get("document_sha256", "")))
            for record in records
            if record.get("source_sha256", record.get("document_sha256", ""))
        }
        manifest_source = str(payload.get("source_sha256", ""))
        if manifest_source:
            source_hashes.add(manifest_source)
        if len(source_hashes) > 1:
            raise ValueError(f"{manifest}: page records mix source documents")
        expected_source = next(iter(source_hashes), "")
        run_id = str(payload.get("run_id", ""))
        for record in records:
            record_source = str(record.get("source_sha256", record.get("document_sha256", "")))
            if expected_source and record_source != expected_source:
                raise ValueError(f"{manifest}: page record has a foreign source hash")
            record_id = str(record.get("id", ""))
            if run_id and record_id and not record_id.startswith(run_id + "-P"):
                raise ValueError(f"{manifest}: page record has a foreign witness ID")

        # A run manifest is authoritative. Page artifacts are redundant
        # evidence, so validate them rather than merging arbitrary JSON found
        # below the run directory into the same review packet.
        for path in sorted((intake_dir / "pages").glob("*/page.json")):
            artifact = load_json(path)
            if not isinstance(artifact, dict):
                raise ValueError(f"{path}: page artifact must be a JSON object")
            page_number = int(artifact.get("page", artifact.get("page_number", 0)) or 0)
            if page_number not in declared_pages:
                raise ValueError(f"{path}: page is not declared by the run manifest")
            artifact_source = str(artifact.get("source_sha256", artifact.get("document_sha256", "")))
            if expected_source and artifact_source != expected_source:
                raise ValueError(f"{path}: page artifact belongs to a foreign source document")
            artifact_id = str(artifact.get("id", ""))
            if run_id and artifact_id and not artifact_id.startswith(run_id + "-P"):
                raise ValueError(f"{path}: page artifact has a foreign witness ID")
            declared_hash = str(declared_pages[page_number].get("raw_text_sha256", ""))
            artifact_hash = str(artifact.get("raw_text_sha256", ""))
            if declared_hash and artifact_hash and artifact_hash != declared_hash:
                raise ValueError(f"{path}: page artifact disagrees with the run manifest")
    else:
        # Legacy/synthetic runs without a manifest remain readable, but each
        # page.json is still the sole accepted record type.
        for path in sorted((intake_dir / "pages").glob("*/page.json")):
            payload = load_json(path)
            if isinstance(payload, dict):
                records.append(payload)
    if include_pages is not None:
        available = {
            int(record.get("page", record.get("page_number", 0)) or 0)
            for record in records
        }
        missing = sorted(include_pages - available)
        if missing:
            raise ValueError("profile include_pages requested absent PDF page(s): " + ", ".join(map(str, missing)))
        records = [
            record for record in records
            if int(record.get("page", record.get("page_number", 0)) or 0) in include_pages
        ]
    # A manifest may also point to page artifacts; retain only one record per
    # page/text witness deterministically.
    unique: dict[tuple[str, str, str], dict] = {}
    for record in records:
        page = str(record.get("page", record.get("page_number", "")))
        text = str(record.get("canonical_text", record.get("selected_text", record.get("text", ""))))
        witness = str(record.get("raw_text_sha256", record.get("text_sha256", "")))
        unique[(page, witness, text)] = record
    def record_sort_key(record: dict) -> tuple[int, str, str]:
        value = record.get("page", record.get("page_number", 0))
        try:
            page = int(value)
        except (TypeError, ValueError):
            page = 0
        return page, str(record.get("source_sha256", "")), str(record.get("source_key", ""))

    normalized = normalize_column_records(intake_dir, list(unique.values()))
    if any(record.get("source_column") for record in normalized):
        return normalized
    return sorted(normalized, key=record_sort_key)


def profile_match(text: str, mappings: object) -> tuple[str, str]:
    """Return a mapped value and confidence from profile regex mappings."""
    if not isinstance(mappings, list):
        return "", ""
    matches = []
    for mapping in mappings:
        if not isinstance(mapping, dict):
            continue
        pattern = mapping.get("pattern")
        value = mapping.get("owner", mapping.get("slot", mapping.get("value", "")))
        if isinstance(pattern, str) and isinstance(value, str) and re.search(pattern, text, re.I | re.M):
            matches.append((value, str(mapping.get("confidence", "profile"))))
    if len({item[0] for item in matches}) == 1 and matches:
        return matches[0]
    if len(matches) > 1:
        return "", "ambiguous"
    return "", ""


@dataclass(frozen=True)
class ProfileBoundary:
    """A profile match with its source span retained for page segmentation."""

    start: int
    end: int
    mapping: dict
    text: str


def profile_boundaries(text: str, mappings: object, value_field: str) -> list[ProfileBoundary]:
    """Return every usable profile-regex match in deterministic source order.

    ``profile_match`` remains the deliberately conservative page-level helper
    used by older callers.  Diurnal pages, however, often contain several
    offices and slots, so the discovery path needs every occurrence and its
    offset rather than a single whole-page answer.
    """
    if not isinstance(mappings, list):
        return []
    boundaries: list[ProfileBoundary] = []
    for order, raw_mapping in enumerate(mappings):
        if not isinstance(raw_mapping, dict):
            continue
        pattern = raw_mapping.get("pattern")
        value = raw_mapping.get(value_field)
        if not isinstance(pattern, str) or not isinstance(value, str) or not value:
            continue
        try:
            matches = re.finditer(pattern, text, re.I | re.M)
        except re.error as exc:
            raise ValueError(f"invalid profile {value_field} pattern {pattern!r}: {exc}") from exc
        mapping = dict(raw_mapping)
        mapping["_profile_order"] = order
        for match in matches:
            # Empty regexes would otherwise manufacture zero-width witnesses.
            if match.start() == match.end():
                continue
            boundaries.append(ProfileBoundary(match.start(), match.end(), mapping, match.group(0)))
    return sorted(
        boundaries,
        key=lambda item: (item.start, item.end, item.mapping["_profile_order"], item.mapping[value_field]),
    )


def boundary_groups(boundaries: list[ProfileBoundary], value_field: str) -> list[tuple[int, int, list[ProfileBoundary]]]:
    """Coalesce aliases beginning at the same position into one boundary.

    A specific and a general alias frequently begin at the same heading.  They
    are one physical boundary, not two zero-length candidates.  Different
    mapped values at that position remain explicitly ambiguous instead of
    being chosen by profile ordering.
    """
    grouped: dict[int, list[ProfileBoundary]] = {}
    for boundary in boundaries:
        grouped.setdefault(boundary.start, []).append(boundary)
    result = []
    for start in sorted(grouped):
        members = grouped[start]
        # The longest match is the natural body start for synonymous aliases.
        end = max(member.end for member in members)
        result.append((start, end, sorted(members, key=lambda item: (item.mapping[value_field], item.mapping["_profile_order"]))))
    return result


def mapping_context(mapping: dict, default_hour: str) -> tuple[str, str, str]:
    """Read optional contextual fields without making them corpus assertions."""
    hour = str(mapping.get("hour", default_hour) or "")
    variant = str(mapping.get("variant", "") or "")
    confidence = str(mapping.get("confidence", "profile") or "profile")
    return hour, variant, confidence


def segment_witness_hash(raw_witness: str, start: int, end: int, body: str) -> str:
    """Hash one immutable page slice, including offsets to avoid collisions."""
    material = "\x1f".join((raw_witness, str(start), str(end), body))
    return hashlib.sha256(material.encode("utf-8")).hexdigest()


def composite_slice_hashes(hashes: list[str]) -> str:
    """Bind a continued witness to its ordered slice hashes.

    A one-slice witness retains that exact hash. Multiple hashes are joined by
    ASCII unit separator before SHA-256 so page boundaries remain unambiguous.
    """
    normalized = [value.lower() for value in hashes]
    if not normalized or any(not re.fullmatch(r"[0-9a-f]{64}", value) for value in normalized):
        raise ValueError("slice hashes must be full SHA-256 values")
    if len(normalized) == 1:
        return normalized[0]
    return hashlib.sha256("\x1f".join(normalized).encode("ascii")).hexdigest()


def profile_page_alias(page: object, mappings: object) -> str:
    """Return an edition's printed folio for an extracted PDF page, if known."""
    value = str(page)
    if not isinstance(mappings, list):
        return ""
    for mapping in mappings:
        if not isinstance(mapping, dict) or not isinstance(mapping.get("pattern"), str):
            continue
        match = re.search(mapping["pattern"], value)
        printed = mapping.get("printed_page")
        if match and isinstance(printed, str):
            return match.expand(printed)
    return ""


def diurnal_candidate_id(candidate: SourceCandidate) -> str:
    source_hash = (candidate.source_sha256 or hashlib.sha256(candidate.source.encode()).hexdigest())[:12]
    witness = candidate.raw_text_sha256 or hashlib.sha256(candidate.source_text.encode()).hexdigest()
    column = f"-C{candidate.source_column}" if candidate.source_column else ""
    page = candidate.source_page or 0
    last = candidate.source_page_last or page
    if last and last != page:
        return f"DI-{source_hash}{column}-P{page}-P{last}-{witness[:12]}"
    return f"DI-{source_hash}{column}-P{page}-{witness[:12]}"


def candidates_from_intake(
    intake_dir: pathlib.Path, profile: dict, corpus: dict[str, CorpusEntry], feast_names: dict[str, str]
) -> list[SourceCandidate]:
    """Create page-slice witnesses from intake artifacts, never touching data/.

    Each slot alias starts a new witness. Heading aliases establish the owner
    context for following slots and, when unambiguous, may continue onto the
    next PDF page. An open mapped slot may continue across consecutive pages
    of the same document and extractor until a new heading or slot appears.
    """
    candidates: list[SourceCandidate] = []
    carried_owner = ""
    carried_title = ""
    carried_hour = ""
    carried_variant = ""
    carried_confidence = ""
    carried_source_sha256 = ""
    carried_source_column = ""
    open_slot: dict | None = None

    def normalized_owner(value: str) -> str:
        return value.removeprefix("proper/") if value.startswith("proper/") else value

    def span_cap(slot: str) -> int:
        if "canticle" in (slot or "") or slot in {"benedictus", "magnificat", "nunc-dimittis", "benedicite"}:
            return 12
        return 3

    def emit(
        record: dict,
        page: int,
        full_text: str,
        start: int,
        end: int,
        slot: str,
        slot_confidence: str,
        owner: str,
        owner_title: str,
        owner_confidence: str,
        hour: str,
        variant: str,
        raw_witness: str,
    ) -> None:
        # Whitespace does not belong to the witness body, but preserve source
        # offsets relative to the original extracted page for review tooling.
        body = full_text[start:end]
        left = len(body) - len(body.lstrip())
        right = len(body.rstrip())
        body_start, body_end = start + left, start + right
        body = body.strip()
        if not body:
            return
        title = str(record.get("heading", record.get("title", ""))).strip() or owner_title or "Unidentified office"
        ambiguous = "ambiguous" in (owner_confidence, slot_confidence)
        candidate = SourceCandidate(
            source=str(record.get("source", record.get("source_key", intake_dir.name))),
            source_page=page,
            hour=hour or str(record.get("hour", profile.get("default_hour", ""))),
            office_title=title,
            office_variant=variant or str(record.get("variant", "")),
            slot=slot,
            latin_incipit=str(record.get("latin_incipit", "")),
            source_text=body,
            source_sha256=str(record.get("source_sha256", record.get("document_sha256", ""))),
            printed_page=str(record.get("printed_page", record.get("folio", ""))) or profile_page_alias(page, profile.get("page_aliases")),
            source_bbox=json.dumps(record.get("bbox", ""), sort_keys=True) if record.get("bbox") else "",
            source_column=str(record.get("source_column", "")),
            source_offset=f"{body_start}:{body_end}",
            extractor=str(record.get("extractor", record.get("route", ""))),
            extraction_confidence=str(record.get("confidence", record.get("route", ""))),
            raw_text_sha256=segment_witness_hash(raw_witness, body_start, body_end, body),
            canonical_owner=normalized_owner(owner),
            mapping_confidence="ambiguous" if ambiguous else (owner_confidence or slot_confidence or "unmapped"),
        )
        candidate.candidate_id = diurnal_candidate_id(candidate)
        candidates.append(candidate)

    def flush_open() -> None:
        nonlocal open_slot
        if not open_slot:
            return
        slices = open_slot["slices"]
        joined = "\n".join(item["body"] for item in slices if item["body"])
        if not joined.strip():
            open_slot = None
            return
        first = slices[0]
        last = slices[-1]
        candidate = SourceCandidate(
            source=str(first["record"].get("source", first["record"].get("source_key", intake_dir.name))),
            source_page=first["page"],
            hour=open_slot["hour"],
            office_title=open_slot["title"],
            office_variant=open_slot["variant"],
            slot=open_slot["slot"],
            latin_incipit="",
            source_text=joined,
            source_sha256=open_slot["source_sha256"],
            source_column=open_slot["source_column"],
            source_bbox=json.dumps(first["record"].get("bbox"), sort_keys=True) if first["record"].get("bbox") else "",
            printed_page=str(first["record"].get("printed_page", first["record"].get("folio", "")))
            or profile_page_alias(first["page"], profile.get("page_aliases")),
            source_offset=";".join(item["offset"] for item in slices),
            extractor=open_slot["extractor"],
            extraction_confidence=str(first["record"].get("confidence", first["record"].get("route", ""))),
            raw_text_sha256=composite_slice_hashes([item["hash"] for item in slices]),
            canonical_owner=open_slot["owner"],
            mapping_confidence=open_slot["confidence"],
            source_page_last=last["page"],
            page_slices=json.dumps(
                [
                    {
                        "page": item["page"],
                        "printed_page": str(item["record"].get("printed_page", item["record"].get("folio", ""))) or profile_page_alias(item["page"], profile.get("page_aliases")) or str(item["page"]),
                        "raw_text_sha256": item["hash"],
                        "offset": item["offset"],
                        "extractor": open_slot["extractor"],
                        "source_column": item["record"].get("source_column", open_slot["source_column"]),
                        "bbox": item["record"].get("bbox", ""),
                    }
                    for item in slices
                ],
                sort_keys=True,
            ),
        )
        if last["page"] != first["page"]:
            last_printed = str(last["record"].get("printed_page", last["record"].get("folio", ""))) or str(last["page"])
            first_printed = candidate.printed_page or str(first["page"])
            if last_printed and last_printed != first_printed:
                candidate.printed_page = f"{first_printed}-{last_printed}"
        candidate.candidate_id = diurnal_candidate_id(candidate)
        candidates.append(candidate)
        open_slot = None

    def append_open(record: dict, page: int, full_text: str, start: int, end: int, raw_witness: str) -> None:
        nonlocal open_slot
        if not open_slot:
            return
        body = full_text[start:end]
        left = len(body) - len(body.lstrip())
        right = len(body.rstrip())
        body_start, body_end = start + left, start + right
        body = body.strip()
        if not body:
            return
        if len(open_slot["slices"]) >= span_cap(open_slot["slot"]):
            meta = open_slot
            flush_open()
            emit(
                record,
                page,
                full_text,
                start,
                end,
                meta["slot"],
                meta["confidence"],
                meta["owner"],
                meta["title"],
                meta["confidence"],
                meta["hour"],
                meta["variant"],
                raw_witness,
            )
            return
        open_slot["slices"].append(
            {
                "record": record,
                "page": page,
                "body": body,
                "offset": f"{body_start}:{body_end}",
                "hash": segment_witness_hash(raw_witness, body_start, body_end, body),
            }
        )
        open_slot["last_page"] = page

    def begin_open(
        record: dict,
        page: int,
        full_text: str,
        start: int,
        end: int,
        raw_witness: str,
        slot: str,
        slot_confidence: str,
        owner: str,
        owner_title: str,
        owner_confidence: str,
        hour: str,
        variant: str,
        extractor: str,
        source_sha256: str,
    ) -> None:
        nonlocal open_slot
        flush_open()
        open_slot = {
            "owner": normalized_owner(owner),
            "title": owner_title or "Unidentified office",
            "slot": slot,
            "hour": hour,
            "variant": variant,
            "confidence": "ambiguous" if "ambiguous" in (owner_confidence, slot_confidence) else (owner_confidence or slot_confidence or "unmapped"),
            "extractor": extractor,
            "source_sha256": source_sha256,
            "source_column": str(record.get("source_column", "")),
            "last_page": page,
            "slices": [],
        }
        append_open(record, page, full_text, start, end, raw_witness)
        if not open_slot or not open_slot["slices"]:
            open_slot = None

    for record in intake_page_records(intake_dir, profile_include_pages(profile)):
        source_sha256 = str(record.get("source_sha256", record.get("document_sha256", "")))
        extractor = str(record.get("extractor", record.get("route", "")))
        page = int(record.get("page", record.get("page_number", 0)) or 0)
        if carried_source_sha256 and source_sha256 != carried_source_sha256:
            flush_open()
            carried_owner = carried_title = carried_hour = carried_variant = carried_confidence = ""
        source_column = str(record.get("source_column", ""))
        if source_column != carried_source_column:
            flush_open()
            carried_owner = carried_title = carried_hour = carried_variant = carried_confidence = ""
        if open_slot is not None and (
            source_sha256 != open_slot["source_sha256"]
            or page != open_slot["last_page"] + 1
            or extractor != open_slot["extractor"]
            or str(record.get("source_column", "")) != open_slot["source_column"]
        ):
            flush_open()
        carried_source_sha256 = source_sha256
        carried_source_column = source_column
        text = intake_text(intake_dir, record)
        if not text.strip():
            continue
        default_hour = str(record.get("hour", profile.get("default_hour", "")) or "")
        title = str(record.get("heading", record.get("title", ""))).strip()
        raw_witness = str(record.get("raw_text_sha256", record.get("text_sha256", ""))) or hashlib.sha256(text.encode()).hexdigest()

        heading_groups = boundary_groups(
            profile_boundaries(text, profile.get("heading_aliases"), "owner"), "owner"
        )
        slot_groups = boundary_groups(
            profile_boundaries(text, profile.get("slot_aliases"), "slot"), "slot"
        )

        # A profile with no slot aliases retains the old one-page witness
        # behavior.  Keep its supplied raw hash so pre-existing DI packets do
        # not need to be regenerated solely by this segmentation enhancement.
        if not slot_groups:
            first_heading_at = heading_groups[0][0] if heading_groups else None
            emit_start = 0
            if open_slot is not None and first_heading_at == 0:
                flush_open()
            elif open_slot is not None:
                cut = first_heading_at if first_heading_at is not None else len(text)
                append_open(record, page, text, 0, cut, raw_witness)
                if first_heading_at is None:
                    continue
                flush_open()
                emit_start = first_heading_at
            owner = ""
            owner_title = title
            owner_hour = default_hour
            owner_variant = str(record.get("variant", ""))
            owner_confidence = ""
            final_context = None
            for _start, _end, members in heading_groups:
                values = {str(member.mapping["owner"]) for member in members}
                if len(values) != 1:
                    final_context = ("", "", "", "", "ambiguous")
                    continue
                selected = max(
                    members,
                    key=lambda member: (
                        member.end - member.start,
                        -member.mapping["_profile_order"],
                    ),
                )
                heading_hour, heading_variant, heading_confidence = mapping_context(
                    selected.mapping, default_hour
                )
                heading_title = str(
                    selected.mapping.get("title", "") or selected.text
                ).strip()
                final_context = (
                    normalized_owner(next(iter(values))),
                    heading_title,
                    heading_hour,
                    heading_variant,
                    heading_confidence,
                )

            if len(heading_groups) == 1 and final_context is not None:
                (
                    owner,
                    owner_title,
                    owner_hour,
                    owner_variant,
                    owner_confidence,
                ) = final_context
            elif len(heading_groups) > 1:
                # The whole page spans several offices and is not a safe
                # single-owner witness. The final heading may still provide
                # unambiguous carry-over context for the next page.
                owner_confidence = "ambiguous"
            else:
                owner, _score = infer_owner(
                    title or text.splitlines()[0], corpus, feast_names
                )
                owner_confidence = "inferred" if owner else ""
            slot = str(record.get("slot", "")).strip()
            emit(
                record,
                page,
                text,
                emit_start,
                len(text),
                slot,
                "",
                owner,
                owner_title,
                owner_confidence,
                owner_hour,
                owner_variant,
                raw_witness,
            )
            if final_context is not None:
                (
                    carried_owner,
                    carried_title,
                    carried_hour,
                    carried_variant,
                    carried_confidence,
                ) = final_context
            elif owner_confidence == "ambiguous":
                carried_owner = carried_title = carried_hour = carried_variant = carried_confidence = ""
            elif owner:
                carried_owner, carried_title, carried_hour, carried_variant, carried_confidence = (
                    normalized_owner(owner),
                    owner_title,
                    owner_hour,
                    owner_variant,
                    owner_confidence,
                )
            continue

        # Build a state timeline from heading events.  An ambiguous event
        # clears context, rather than allowing a preceding feast to leak into
        # a later page or slot.
        timeline: list[tuple[int, str, str, str, str, str]] = []
        for start, _end, members in heading_groups:
            values = {str(member.mapping["owner"]) for member in members}
            if len(values) != 1:
                timeline.append((start, "", "", "", "", "ambiguous"))
                continue
            selected = max(members, key=lambda member: (member.end - member.start, -member.mapping["_profile_order"]))
            heading_hour, heading_variant, heading_confidence = mapping_context(selected.mapping, default_hour)
            heading_title = str(selected.mapping.get("title", "") or selected.text).strip()
            timeline.append((start, normalized_owner(next(iter(values))), heading_title, heading_hour, heading_variant, heading_confidence))

        context = (carried_owner, carried_title, carried_hour, carried_variant, carried_confidence)
        timeline_index = 0
        if open_slot is not None:
            first_heading = timeline[0][0] if timeline else None
            first_slot = slot_groups[0][0] if slot_groups else None
            event_positions = [pos for pos in (first_heading, first_slot) if pos is not None]
            first_event = min(event_positions) if event_positions else len(text)
            if first_event > 0:
                append_open(record, page, text, 0, first_event, raw_witness)
            if open_slot is not None and first_heading is not None and first_heading <= first_event:
                heading_owner, heading_conf = timeline[0][1], timeline[0][5]
                if heading_conf == "ambiguous" or heading_owner != open_slot["owner"]:
                    flush_open()
            if open_slot is not None and first_slot is not None and first_slot <= first_event:
                flush_open()
        for index, (start, end, members) in enumerate(slot_groups):
            while timeline_index < len(timeline) and timeline[timeline_index][0] <= start:
                _at, context_owner, context_title, context_hour, context_variant, context_confidence = timeline[timeline_index]
                context = (context_owner, context_title, context_hour, context_variant, context_confidence)
                timeline_index += 1
            values = {str(member.mapping["slot"]) for member in members}
            if len(values) == 1:
                selected = max(members, key=lambda member: (member.end - member.start, -member.mapping["_profile_order"]))
                slot = next(iter(values))
                slot_hour, slot_variant, slot_confidence = mapping_context(selected.mapping, default_hour)
            else:
                slot, slot_hour, slot_variant, slot_confidence = "", default_hour, "", "ambiguous"
            next_start = slot_groups[index + 1][0] if index + 1 < len(slot_groups) else len(text)
            # A new office heading terminates the preceding slot even when the
            # next mapped slot occurs later. This prevents the next feast's
            # title/rubrics from contaminating the prior witness body.
            following_headings = [event[0] for event in timeline if event[0] > end]
            if following_headings:
                next_start = min(next_start, following_headings[0])
            owner, owner_title, owner_hour, owner_variant, owner_confidence = context
            hour = slot_hour or owner_hour or default_hour
            variant = slot_variant or owner_variant
            extends_to_eof = next_start >= len(text)
            heading_after = any(event[0] > end for event in timeline)
            if (
                index == len(slot_groups) - 1
                and extends_to_eof
                and not heading_after
                and slot
                and "ambiguous" not in (owner_confidence, slot_confidence)
            ):
                begin_open(
                    record, page, text, end, next_start, raw_witness,
                    slot, slot_confidence, owner, owner_title or title, owner_confidence,
                    hour, variant, extractor, source_sha256,
                )
            else:
                emit(
                    record, page, text, end, next_start, slot, slot_confidence,
                    owner, owner_title or title, owner_confidence,
                    hour, variant,
                    raw_witness,
                )

        # Apply headings after the final slot so their context may continue to
        # the next page.  An explicit ambiguous heading always wins and clears
        # any prior context.
        while timeline_index < len(timeline):
            _at, context_owner, context_title, context_hour, context_variant, context_confidence = timeline[timeline_index]
            context = (context_owner, context_title, context_hour, context_variant, context_confidence)
            timeline_index += 1
        carried_owner, carried_title, carried_hour, carried_variant, carried_confidence = context

    flush_open()
    ordered = sorted(candidates, key=lambda item: (item.source_column, item.source, item.source_page, item.source_offset, item.slot, item.candidate_id))
    seen: dict[str, SourceCandidate] = {}
    for candidate in ordered:
        prior = seen.get(candidate.candidate_id)
        if prior is not None and (prior.source_column, prior.source_offset, prior.slot) != (candidate.source_column, candidate.source_offset, candidate.slot):
            raise ValueError(f"candidate ID collision: {candidate.candidate_id}")
        seen[candidate.candidate_id] = candidate
    return ordered


def load_resolution_inventory(path: pathlib.Path | None) -> list[dict]:
    if path is None or not path.exists():
        return []
    payload = load_json(path)
    if isinstance(payload, dict):
        payload = payload.get("rows", payload.get("entries", []))
    if not isinstance(payload, list):
        raise ValueError(f"{path}: resolution inventory must contain a rows list")
    return [item for item in payload if isinstance(item, dict)]


def inventory_index(rows: list[dict]) -> dict[tuple[str, str, str], list[dict]]:
    indexed: dict[tuple[str, str, str], list[dict]] = {}
    for row in rows:
        owner = str(row.get("owner_id", row.get("owner", ""))).removeprefix("proper/")
        hour = str(row.get("hour", ""))
        slot = str(row.get("slot_ref", row.get("slot", row.get("runtime_slot", ""))))
        if owner and hour and slot:
            indexed.setdefault((owner, hour, slot), []).append(row)
    return indexed


def proper_target(owner: str, slot: str) -> str:
    if not owner or not re.fullmatch(r"[a-z0-9][a-z0-9-]*", owner):
        return ""
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", slot):
        return ""
    return f"proper/{owner}/{slot}"


# Bare hour-shared names the engine also resolves. Apply targets the
# hour-qualified corpus section (chapter-lauds, hymn-vespers, …).
HOUR_QUALIFIED_SLOTS = frozenset({"chapter", "hymn", "short-responsory", "versicle"})


def qualify_slot(slot: str, hour: str) -> str:
    slot = (slot or "").strip()
    hour = (hour or "").strip()
    if not slot or not hour:
        return slot
    if slot in HOUR_QUALIFIED_SLOTS:
        return f"{slot}-{hour}"
    return slot


def classify_discovery(
    candidates: list[SourceCandidate], corpus: dict[str, CorpusEntry], feast_names: dict[str, str], inventory: list[dict]
) -> None:
    """Classify printed witnesses against traced runtime resolution.

    This deliberately treats the printed source as authoritative evidence even
    where the selected inventory span has not observed the owner yet.
    """
    indexed = inventory_index(inventory)
    known_owners = set(feast_names) | {
        key.split("/")[1] for key in corpus if key.startswith("proper/") and key.count("/") >= 2
    }
    for candidate in candidates:
        owner, slot = candidate.canonical_owner, candidate.slot
        if candidate.mapping_confidence == "ambiguous":
            candidate.discovery_classification = "ambiguous-owner/context"
            candidate.discovery_note = "profile or heading mapping had multiple matches"
            continue
        if source_requires_modeling(source_review_flags(candidate)):
            candidate.discovery_classification = "rubrical-complex"
            candidate.discovery_note = "printed block contains alternatives or rubrics"
            continue
        if not slot:
            candidate.discovery_classification = "unmodeled-slot"
            candidate.discovery_note = "no slot mapping; add an edition profile alias"
            continue
        if not owner or owner not in known_owners:
            candidate.discovery_classification = "unknown-feast"
            candidate.discovery_note = "printed heading did not map to a known calendar owner"
            continue
        qualified = qualify_slot(slot, candidate.hour)
        target = proper_target(owner, qualified)
        if not target:
            candidate.discovery_classification = "unmodeled-slot"
            candidate.discovery_note = "slot is not a safe corpus section name"
            continue
        candidate.runtime_slot = qualified
        rows = indexed.get((owner, candidate.hour, qualified), [])
        if not rows and qualified != slot:
            rows = indexed.get((owner, candidate.hour, slot), [])
        if not rows:
            candidate.discovery_classification = "known-owner-unobserved"
            candidate.discovery_note = "known calendar owner was not observed in the selected inventory span"
            candidate.runtime_target = target if target in corpus else ""
            continue
        row = sorted(rows, key=lambda item: json.dumps(item, sort_keys=True))[0]
        selected = str(row.get("selected_ref", row.get("selected_key", row.get("runtime_target", row.get("source_ref", "")))))
        candidate.runtime_target = selected
        candidate.resolution_tier = str(row.get("selected_tier", row.get("tier", "")))
        candidate.resolution_reason = str(row.get("reason", ""))
        comparison_key = target if target in corpus else (selected if selected in corpus else "")
        current = corpus.get(comparison_key) if comparison_key else None
        candidate.corpus_key = comparison_key
        candidate.current_text = current.text if current else ""
        similarity = anchored_text_similarity(candidate.source_text, current.text) if current else 0.0
        candidate.text_similarity = round(similarity, 3)
        direct_existing = row.get("direct_existing", [])
        if isinstance(direct_existing, str):
            direct_existing = [direct_existing]
        direct_exists = target in corpus or bool(direct_existing)
        if direct_exists:
            candidate.discovery_classification = "verify-existing" if similarity >= 0.985 else "existing-different"
        elif selected:
            candidate.discovery_classification = "fallback-equal" if similarity >= 0.985 else "missing-override"
        else:
            candidate.discovery_classification = "missing-override"
        candidate.discovery_note = f"runtime selected {selected or 'no text'}"


def write_proper_discovery(
    output_dir: pathlib.Path,
    candidates: list[SourceCandidate],
    inventory: list[dict],
    dependencies: dict[str, dict[str, str]] | None = None,
    dependency_manifest: dict | None = None,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    ordered = sorted(candidates, key=lambda item: (item.discovery_classification, item.candidate_id))
    payload = {
        "schema_version": 2 if dependency_manifest is not None else 1,
        "dependencies": dependencies or {},
        "candidates": [asdict(item) for item in ordered],
    }
    if dependency_manifest is not None:
        payload["dependency_manifest"] = dependency_manifest
    (output_dir / "proper-discovery.json").write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n")
    fields = tuple(SourceCandidate.__dataclass_fields__)
    with (output_dir / "proper-discovery.csv").open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fields)
        writer.writeheader()
        for candidate in ordered:
            writer.writerow(asdict(candidate))
    unmapped = [item for item in ordered if item.discovery_classification in {"unknown-feast", "unmodeled-slot", "ambiguous-owner/context", "rubrical-complex"}]
    with (output_dir / "printed-unmapped.csv").open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fields)
        writer.writeheader()
        for candidate in unmapped:
            writer.writerow(asdict(candidate))
    witnessed = {(item.canonical_owner, item.runtime_slot or item.slot) for item in candidates if item.canonical_owner and (item.runtime_slot or item.slot)}
    unwitnessed = []
    for row in inventory:
        owner = str(row.get("owner_id", row.get("owner", ""))).removeprefix("proper/")
        slot = str(row.get("slot_ref", row.get("slot", row.get("runtime_slot", ""))))
        if owner and slot and (owner, slot) not in witnessed:
            unwitnessed.append(row)
    unwitnessed.sort(key=lambda row: (str(row.get("owner_id", row.get("owner", ""))), str(row.get("slot", ""))))
    (output_dir / "runtime-unwitnessed.csv").write_text(
        "\n".join(
            ["owner_id,slot,selected_key,selected_tier,reason"]
            + [
                ",".join(csv_quote(str(row.get(key, ""))) for key in ("owner_id", "slot", "selected_key", "selected_tier", "reason"))
                for row in unwitnessed
            ]
        ) + "\n"
    )
    counts: dict[str, int] = {}
    for candidate in candidates:
        counts[candidate.discovery_classification] = counts.get(candidate.discovery_classification, 0) + 1
    (output_dir / "proper-discovery-summary.json").write_text(json.dumps({"total": len(candidates), "classes": dict(sorted(counts.items())), "runtime_unwitnessed": len(unwitnessed)}, indent=2) + "\n")


def csv_quote(value: str) -> str:
    return '"' + value.replace('"', '""') + '"' if any(char in value for char in ',"\n') else value


def load_discovery(output_dir: pathlib.Path) -> list[SourceCandidate]:
    path = output_dir / "proper-discovery.json"
    if not path.exists():
        raise FileNotFoundError(f"{path} does not exist; run discovery first")
    payload = load_json(path)
    if not isinstance(payload, dict):
        raise ValueError(f"{path}: expected an object")
    return [SourceCandidate(**item) for item in payload.get("candidates", [])]


def write_advisory_proposals(output_dir: pathlib.Path, proposal_dir: pathlib.Path) -> None:
    """Write review-only proposal artifacts; this function never writes data/."""
    candidates = load_discovery(output_dir)
    decisions = load_decisions(output_dir)
    proposal_dir.mkdir(parents=True, exist_ok=True)
    proposals = []
    diff = ["# Advisory-only diurnal proposals", "# No corpus or provenance files were changed."]
    for candidate in sorted(candidates, key=lambda item: item.candidate_id):
        if candidate.discovery_classification not in {"missing-override", "existing-different", "known-owner-unobserved"}:
            continue
        target = proper_target(candidate.canonical_owner, candidate.runtime_slot or candidate.slot)
        if not target:
            continue
        decision = decisions.get(candidate.candidate_id, {}).get("decision", "")
        accepted = decision == "accept"
        proposal = {
            "candidate_id": candidate.candidate_id,
            "target": target,
            "classification": candidate.discovery_classification,
            "decision": decision,
            "advisory": not accepted,
            "source": {"page": candidate.source_page, "printed_page": candidate.printed_page, "sha256": candidate.source_sha256},
        }
        if accepted:
            proposal["replacement_text"] = candidate.source_text
            diff.extend((f"--- a/data/texts/{target.rsplit('/', 1)[0]}.txt", f"+++ b/data/texts/{target.rsplit('/', 1)[0]}.txt", f"# Add or review [{target.rsplit('/', 1)[1]}] from {candidate.candidate_id}"))
        else:
            proposal["note"] = "Awaiting an explicit accepted source decision; content intentionally omitted."
            diff.append(f"# {candidate.candidate_id}: {target} (awaiting accepted decision)")
        proposals.append(proposal)
    (proposal_dir / "proposals.json").write_text(json.dumps({"proposals": proposals}, indent=2, ensure_ascii=False) + "\n")
    (proposal_dir / "proposals.diff").write_text("\n".join(diff) + "\n")


def choose_master(resources: pathlib.Path, pattern: str) -> pathlib.Path:
    matches = sorted(resources.glob(pattern))
    if not matches:
        raise FileNotFoundError(f"no resource matches {pattern!r} under {resources}")
    return matches[-1]


def cmd_build(args: argparse.Namespace) -> int:
    resources = pathlib.Path(args.resources)
    data_dir = pathlib.Path(args.data)
    output_dir = pathlib.Path(args.output)
    office_binary = pathlib.Path(args.office) if args.office else None

    offices = []
    for hour, pattern in MASTER_BOOKS:
        path = choose_master(resources, pattern)
        offices.extend(find_offices(path, hour))

    candidates = []
    for office in offices:
        candidates.extend(extract_candidates(office))

    for hour, canticle, pattern, targets in STANDALONE_CANTICLE_BOOKS:
        path = choose_master(resources, pattern)
        paragraphs = read_docx_paragraphs(path)
        candidates.extend(
            extract_standalone_canticles(
                path.name, hour, canticle, paragraphs, targets
            )
        )

    ferial_path = choose_master(resources, FERIAL_LAUDS_BOOK)
    candidates.extend(
        extract_ferial_lauds_antiphons(
            ferial_path.name,
            read_pdf_pages(ferial_path),
            FERIAL_LAUDS_ANTIPHON_TARGETS,
        )
    )

    corpus = load_corpus(data_dir)
    feast_names = load_feast_names(data_dir)
    queue = load_review_queue(office_binary, args.start, args.years)
    reconcile(candidates, corpus, feast_names, queue)
    assign_candidate_ids(candidates)
    write_outputs(output_dir, candidates, offices, args.batch_size)

    actionable = deduplicated_actionable(
        candidates, set(load_decisions(output_dir))
    )
    print(f"Parsed {len(offices)} offices and {len(candidates)} source candidates.")
    print(f"Prepared {len(actionable)} unique actionable corpus comparisons.")
    print(f"Review packets: {output_dir / 'README.md'}")
    return 0


def load_generated(output_dir: pathlib.Path) -> list[SourceCandidate]:
    path = output_dir / "candidates.json"
    if not path.exists():
        raise FileNotFoundError(f"{path} does not exist; run the build command first")
    payload = json.loads(path.read_text())
    return [SourceCandidate(**item) for item in payload["candidates"]]


def load_review_candidates(output_dir: pathlib.Path) -> list[SourceCandidate]:
    """Load legacy reconciliation and diurnal-discovery candidates together."""
    candidates = []
    if (output_dir / "candidates.json").exists():
        candidates.extend(load_generated(output_dir))
    if (output_dir / "proper-discovery.json").exists():
        candidates.extend(load_discovery(output_dir))
    if not candidates:
        raise FileNotFoundError(
            f"{output_dir}: no candidates.json or proper-discovery.json; run build or discover first"
        )
    by_id = {}
    for candidate in candidates:
        if candidate.candidate_id in by_id:
            raise ValueError(f"duplicate candidate ID across review artifacts: {candidate.candidate_id}")
        by_id[candidate.candidate_id] = candidate
    return list(by_id.values())


def cmd_show(args: argparse.Namespace) -> int:
    candidates = {
        candidate.candidate_id: candidate
        for candidate in load_review_candidates(pathlib.Path(args.output))
    }
    missing = [identifier for identifier in args.ids if identifier not in candidates]
    if missing:
        print("Unknown candidate ID(s): " + ", ".join(missing), file=sys.stderr)
        return 1
    print("\n".join(render_candidate(candidates[identifier]) for identifier in args.ids))
    return 0


def cmd_decide(args: argparse.Namespace) -> int:
    output_dir = pathlib.Path(args.output)
    candidates = {
        candidate.candidate_id: candidate for candidate in load_review_candidates(output_dir)
    }
    missing = [identifier for identifier in args.ids if identifier not in candidates]
    if missing:
        print("Unknown candidate ID(s): " + ", ".join(missing), file=sys.stderr)
        return 1

    decisions = load_decisions(output_dir)
    for identifier in args.ids:
        if args.decision == "pending":
            decisions.pop(identifier, None)
        else:
            decisions[identifier] = {
                "candidate_id": identifier,
                "decision": args.decision,
                "note": args.note,
            }
    write_decisions(output_dir, decisions)
    print(
        f"Recorded {args.decision!r} for {len(args.ids)} candidate(s). "
        "Re-run the relevant build or discover command to refresh reports."
    )
    return 0


def cmd_discover(args: argparse.Namespace) -> int:
    data_dir = pathlib.Path(args.data)
    output_dir = require_output_path(pathlib.Path(args.output))
    profile_path = pathlib.Path(args.profile) if args.profile else None
    inventory_path = pathlib.Path(args.inventory) if args.inventory else None
    # Validate declared external inputs before parsing any of them.  This
    # keeps the discovery contract fail-closed rather than merely noticing an
    # unsafe path after it has influenced candidate classification.
    if profile_path is not None:
        confined_dependency_path(profile_path, ROOT, "profile")
    if inventory_path is not None:
        confined_dependency_path(inventory_path, ROOT, "resolution-inventory")
    corpus = load_corpus(data_dir)
    feast_names = load_feast_names(data_dir)
    profile = load_diurnal_profile(profile_path)
    inventory = load_resolution_inventory(inventory_path)
    intake_dir = require_output_path(pathlib.Path(args.intake))
    try:
        output_dir.relative_to(intake_dir)
    except ValueError:
        pass
    else:
        raise ValueError("discovery output must not be inside the intake dependency tree")
    candidates = candidates_from_intake(intake_dir, profile, corpus, feast_names)
    classify_discovery(candidates, corpus, feast_names, inventory)
    dependency_manifest = discovery_dependency_manifest(
        intake_dir=intake_dir,
        intake_records=intake_page_records(intake_dir, profile_include_pages(profile)),
        profile_path=profile_path,
        inventory_path=inventory_path,
        data_dir=data_dir,
    )
    write_proper_discovery(
        output_dir, candidates, inventory, dependency_manifest=dependency_manifest
    )
    print(f"Classified {len(candidates)} printed diurnal witnesses.")
    print(f"Discovery reports: {output_dir / 'proper-discovery.json'}")
    return 0


_PUA_RE = re.compile(r"\ufffd|[\ue000-\uf8ff]")
_FOLIO_RE = re.compile(r"^\s*\d{1,4}\s*$")
_RUBRIC_STOP_RE = re.compile(
    r"^(collect|benedictus,?\s+tone|after the canticle|the prayers|august \d)\b",
    re.I,
)
_SYLLABLE_DASH_RE = re.compile(r"(?<=[A-Za-z])[–](?=[A-Za-z])")
_ASCII_UNDERLAY_RE = re.compile(r"(?<=[A-Za-z])-(?=[A-Za-z])")


def clean_witness_body(text: str) -> str:
    """Deterministic cleaner: drop PUA, folios, chant-code, and slot leftovers."""
    lines = []
    for line in text.splitlines():
        stripped = _PUA_RE.sub("", line).rstrip()
        stripped = re.sub(r"^([A-Za-z])\s{2,}", r"\1", stripped)
        check = stripped.strip()
        if not check:
            continue
        if _RUBRIC_STOP_RE.match(check):
            break
        if (
            _FOLIO_RE.match(check)
            or re.fullmatch(r"[ivxlcdm]+\.?", check, re.I)
            or is_artifact(check)
            or is_chant_code(check)
        ):
            continue
        lines.append(_SYLLABLE_DASH_RE.sub("", check))
    cleaned = "\n".join(lines).strip()
    if len(_ASCII_UNDERLAY_RE.findall(cleaned)) >= 4:
        cleaned = _ASCII_UNDERLAY_RE.sub("", cleaned)
    return drop_leading_latin_incipit(cleaned)


_ENGLISH_HINT_RE = re.compile(
    r"\b(the|and|of|to|thou|shall|who|that|for|from|with|this|his|her)\b", re.I
)


def drop_leading_latin_incipit(text: str) -> str:
    lines = text.splitlines()
    if len(lines) < 2:
        return text
    first = lines[0].strip()
    if 1 <= len(first.split()) <= 3 and not _ENGLISH_HINT_RE.search(first):
        return "\n".join(lines[1:]).strip()
    return text


def apply_pages_from_candidate(candidate: SourceCandidate) -> list[dict]:
    if candidate.page_slices:
        try:
            parsed = json.loads(candidate.page_slices)
        except json.JSONDecodeError:
            raise ValueError("page_slices must be valid JSON")
        if not isinstance(parsed, list) or not parsed:
            raise ValueError("page_slices must be a non-empty list")
        pages = []
        expected_column = candidate.source_column
        expected_extractor = candidate.extractor
        previous_page = 0
        for item in parsed:
            if not isinstance(item, dict):
                raise ValueError("page_slices entries must be objects")
            bbox = item.get("bbox")
            page = item.get("page")
            column = item.get("source_column")
            extractor = item.get("extractor")
            if isinstance(page, bool) or not isinstance(page, int) or page < 1 or (previous_page and page != previous_page + 1):
                raise ValueError("page_slices pages must be positive and consecutive")
            if not isinstance(item.get("printed_page"), str) or not item["printed_page"].strip():
                raise ValueError("page_slices requires printed_page")
            if not isinstance(item.get("raw_text_sha256"), str) or not re.fullmatch(r"[0-9a-fA-F]{64}", item["raw_text_sha256"]):
                raise ValueError("page_slices requires full raw_text_sha256")
            if not isinstance(item.get("offset"), str) or not item["offset"].strip() or not isinstance(extractor, str) or not extractor.strip():
                raise ValueError("page_slices requires offset and extractor")
            if expected_column:
                if not isinstance(column, str) or not re.fullmatch(r"[a-z][a-z0-9_-]*", column):
                    raise ValueError("page_slices requires a safe source_column")
            elif column:
                raise ValueError("non-column page_slices cannot declare source_column")
            if expected_column and column != expected_column or expected_extractor and extractor != expected_extractor:
                raise ValueError("page_slices disagree with candidate witness identity")
            if expected_column and (not isinstance(bbox, list) or len(bbox) != 4 or any(isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v) for v in bbox) or bbox[2] <= bbox[0] or bbox[3] <= bbox[1]):
                raise ValueError("page_slices requires a finite positive bbox")
            page_entry = {
                    "page": page,
                    "printed_page": str(item.get("printed_page") or ""),
                    "raw_text_sha256": str(item.get("raw_text_sha256") or ""),
                    "offset": str(item.get("offset") or ""),
                    "extractor": str(item.get("extractor") or candidate.extractor),
            }
            if expected_column:
                page_entry.update({"source_column": str(item["source_column"]), "bbox": bbox})
            pages.append(page_entry)
            previous_page = page
        if pages:
            if composite_slice_hashes([item["raw_text_sha256"] for item in pages]) != candidate.raw_text_sha256.lower():
                raise ValueError("candidate raw_text_sha256 disagrees with page_slices")
            return pages
        raise ValueError("page_slices must contain at least one entry")
    bbox = candidate.source_bbox
    if isinstance(bbox, str):
        try:
            bbox = json.loads(bbox)
        except json.JSONDecodeError:
            bbox = ""
    return [{
        "page": candidate.source_page,
        "printed_page": candidate.printed_page or str(candidate.source_page),
        "raw_text_sha256": candidate.raw_text_sha256,
        "offset": candidate.source_offset,
        "extractor": candidate.extractor,
        "source_column": candidate.source_column,
        "bbox": bbox,
    }]


def candidate_to_apply_packet(
    candidate: SourceCandidate,
    existing_keys: set[str] | None = None,
    corpus: dict[str, CorpusEntry] | None = None,
) -> dict | None:
    if candidate.discovery_classification not in {"missing-override", "existing-different"}:
        return None
    slot = qualify_slot(candidate.runtime_slot or candidate.slot, candidate.hour)
    target = proper_target(candidate.canonical_owner, slot)
    if not target:
        return None
    if existing_keys is not None:
        action = "replace-section" if target in existing_keys else "add-section"
    elif candidate.discovery_classification == "missing-override":
        action = "add-section"
    else:
        action = "replace-section"
    body = clean_witness_body(candidate.source_text)
    if not body:
        return None
    similarity = candidate.text_similarity
    if corpus:
        fallback = corpus.get(candidate.runtime_target) if candidate.runtime_target else None
        if fallback and fallback.text:
            similarity = round(anchored_text_similarity(body, fallback.text), 3)
            # A printed common on the feast page is not a proper override.
            if action == "add-section" and similarity >= 0.84:
                return None
        current = corpus.get(target)
        if action == "replace-section" and current and current.text:
            similarity = round(anchored_text_similarity(body, current.text), 3)
            if similarity >= 0.985:
                return None
            if "\n\n" in current.text and "\n\n" not in body:
                return None
            if current.text.lstrip().startswith("!") and not body.lstrip().startswith("!"):
                return None
    if re.search(r"THE GOSPEL CANTICLE", body, re.I):
        return None
    if re.search(r"(?:from|see|as in)\s+(?:the\s+)?(?:lauds|vespers|matins|compline)\b.*\bcommon\b.*(?:/|$)", body, re.I | re.S):
        return None
    pages = apply_pages_from_candidate(candidate)
    if not pages:
        return None
    printed = candidate.printed_page or (str(pages[0]["page"]) if pages else "")
    source = candidate.source or "monastic-diurnal"
    return {
        "candidate_id": candidate.candidate_id,
        "source_sha256": candidate.source_sha256,
        "raw_text_sha256": candidate.raw_text_sha256,
        "extractor": candidate.extractor,
        "pages": pages,
        "target_key": target,
        "action": action,
        "body": body,
        "source_comment": (
            f"{source} p. {printed} ({candidate.candidate_id}; {slot}) "
            f"— agent-proposed, not attested"
        ),
        "discovery_class": candidate.discovery_classification,
        "text_similarity": similarity,
    }


def write_apply_queue(
    output_dir: pathlib.Path,
    candidates: list[SourceCandidate],
    data_dir: pathlib.Path | None = None,
) -> pathlib.Path:
    output_dir = require_output_path(output_dir)
    existing: set[str] = set()
    corpus: dict[str, CorpusEntry] | None = None
    if data_dir is not None:
        corpus = load_corpus(data_dir)
        existing = set(corpus)
    packets = []
    skipped = 0
    for candidate in candidates:
        packet = candidate_to_apply_packet(candidate, existing or None, corpus)
        if packet is None:
            skipped += 1
            continue
        packets.append(packet)
    path = output_dir / "apply-queue.json"
    path.write_text(json.dumps({"packets": packets, "skipped": skipped}, indent=2, ensure_ascii=False) + "\n")
    return path


def cmd_apply_queue(args: argparse.Namespace) -> int:
    output_dir = require_output_path(pathlib.Path(args.output))
    candidates = load_discovery(output_dir)
    path = write_apply_queue(output_dir, candidates, pathlib.Path(args.data))
    payload = load_json(path)
    print(f"Wrote {len(payload.get('packets', []))} apply packet(s) to {path}")
    return 0


def cmd_proposals(args: argparse.Namespace) -> int:
    output_dir = pathlib.Path(args.output)
    proposal_dir = pathlib.Path(args.proposal_output)
    write_advisory_proposals(output_dir, proposal_dir)
    print(f"Advisory proposals: {proposal_dir / 'proposals.json'}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    build = subparsers.add_parser(
        "build", help="extract sources and generate scratch review packets"
    )
    build.add_argument("--resources", default="../resources")
    build.add_argument("--data", default="data")
    build.add_argument("--office", default="./office", help="office binary used for ranking")
    build.add_argument("--output", default=str(DEFAULT_OUTPUT))
    build.add_argument("--start", type=int, default=2026)
    build.add_argument("--years", type=int, default=1)
    build.add_argument("--batch-size", type=int, default=12)
    build.set_defaults(func=cmd_build)

    show = subparsers.add_parser(
        "show", help="print one or more generated candidate comparisons"
    )
    show.add_argument("ids", nargs="+")
    show.add_argument("--output", default=str(DEFAULT_OUTPUT))
    show.set_defaults(func=cmd_show)

    decide = subparsers.add_parser(
        "decide", help="record a local scratch decision for generated candidates"
    )
    decide.add_argument(
        "decision", choices=("retain", "accept", "applied", "manual", "defer", "pending")
    )
    decide.add_argument("ids", nargs="+")
    decide.add_argument("--note", default="")
    decide.add_argument("--output", default=str(DEFAULT_OUTPUT))
    decide.set_defaults(func=cmd_decide)

    discover = subparsers.add_parser(
        "discover", help="compare diurnal intake witnesses with runtime resolution inventory"
    )
    discover.add_argument("--intake", required=True, help="diurnal-intake run directory")
    discover.add_argument("--inventory", help="JSON from review resolution-inventory")
    discover.add_argument("--profile", help="edition-specific JSON heading/slot aliases")
    discover.add_argument("--data", default="data")
    discover.add_argument("--output", default=str(DEFAULT_OUTPUT))
    discover.set_defaults(func=cmd_discover)

    apply_queue = subparsers.add_parser(
        "apply-queue",
        help="build a gated apply-queue from proper-discovery.json (writable classes only)",
    )
    apply_queue.add_argument("--output", default=str(DEFAULT_OUTPUT))
    apply_queue.add_argument("--data", default="data")
    apply_queue.set_defaults(func=cmd_apply_queue)

    proposals = subparsers.add_parser(
        "proposals", help="write advisory-only proposal JSON and diff from discovery output"
    )
    proposals.add_argument("--output", default=str(DEFAULT_OUTPUT))
    proposals.add_argument("--proposal-output", default=str(DEFAULT_OUTPUT / "proposals"))
    proposals.set_defaults(func=cmd_proposals)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    try:
        for name in ("output", "proposal_output"):
            value = getattr(args, name, None)
            if value:
                setattr(args, name, str(require_output_path(pathlib.Path(value))))
        return args.func(args)
    except (
        FileNotFoundError,
        ValueError,
        subprocess.CalledProcessError,
        zipfile.BadZipFile,
    ) as exc:
        print(f"source reconciliation failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
