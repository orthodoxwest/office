#!/usr/bin/env python3
"""Build disposable, page-aware review packets from local office books.

The extracted source text is deliberately written only below ``output/``
(which is gitignored). The checked-in artifact is this parser, not a copy of
the source books.

Typical use::

    make review-sources
    scripts/source-reconcile.py show SR-0001-01234567

The generated packets are suggestions, never automatic corpus edits or human
attestations. They are intended for a maintainer and Codex to review together
before changing ``data/``.
"""

from __future__ import annotations

import argparse
import csv
import difflib
import functools
import hashlib
import json
import pathlib
import re
import subprocess
import sys
import unicodedata
import zipfile
import xml.etree.ElementTree as ET
from dataclasses import asdict, dataclass


W_NS = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
DEFAULT_OUTPUT = pathlib.Path("output/source-reconcile")
DECISION_FIELDS = ("candidate_id", "decision", "note")

MASTER_BOOKS = (
    ("lauds", "00. Lauds for Sundays & Major Feasts*.docx"),
    ("vespers", "00. Vespers for Saturdays & Major Feasts*.docx"),
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
    (r"bishop,\s*confessor,\s*(?:&|and)\s*doctor", "commons/confessor-doctor"),
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
    if re.fullmatch(r"(?:[ivx]+|T\.P\.)\.?\d*", text, re.I):
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
    return entries


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
        return f"proper/{identifier}", score
    return "", score


def source_review_flags(candidate: SourceCandidate) -> list[str]:
    """Identify source blocks that need modeling rather than direct copying."""
    text = candidate.source_text
    flags = []
    if len(text) > 1800:
        flags.append("long extraction")
    if "¶" in text or re.search(
        r"\b(?:in|out of) paschaltide\b|\bP\.\s*T\.|\bT\.\s*P\.|"
        r"^(?:for (?:a|the) (?:doctor|confessor|bishop|patron)|if\b)|"
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
            text_similarity(candidate.source_text, other.source_text)
            for other in later
        ) >= 0.84:
            candidate.slot = generic

    for candidate in candidates:
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
                score = text_similarity(candidate.source_text, entry.text)
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
                source_score = text_similarity(candidate.source_text, entry.text)
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


def cmd_show(args: argparse.Namespace) -> int:
    candidates = {
        candidate.candidate_id: candidate
        for candidate in load_generated(pathlib.Path(args.output))
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
        candidate.candidate_id: candidate for candidate in load_generated(output_dir)
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
        "Re-run `make review-sources` to refresh the batches."
    )
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
        "decision", choices=("retain", "applied", "manual", "defer", "pending")
    )
    decide.add_argument("ids", nargs="+")
    decide.add_argument("--note", default="")
    decide.add_argument("--output", default=str(DEFAULT_OUTPUT))
    decide.set_defaults(func=cmd_decide)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    try:
        return args.func(args)
    except (FileNotFoundError, subprocess.CalledProcessError, zipfile.BadZipFile) as exc:
        print(f"source reconciliation failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
