#!/usr/bin/env python3
"""Generate a clergy-facing project status report.

The report deliberately keeps four claims separate:

* known feast-proper lookup coverage (from ``office audit``),
* rendered completeness (the annual composition sweep),
* text provenance verification, and
* parity with a published archdiocesan ordo.

Ordo mismatches are classified through ``data/review/ordo-triage.csv``.  A
finding that has not been reviewed stays ``untriaged``; the script never
guesses that a calendar symptom is an engine bug or a data gap.
"""

from __future__ import annotations

import argparse
import collections
import csv
import datetime as dt
import fnmatch
import importlib.util
import json
import pathlib
import re
import subprocess
import sys
import tempfile
from dataclasses import asdict, dataclass


ROOT = pathlib.Path(__file__).resolve().parents[1]
COMPARE_SCRIPT = pathlib.Path(__file__).with_name("ordo-compare.py")
SPEC = importlib.util.spec_from_file_location("ordo_compare", COMPARE_SCRIPT)
ORDO_COMPARE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(ORDO_COMPARE)

PROPER_REFS = {
    "psalm-antiphon",
    "benedictus-antiphon",
    "magnificat-antiphon",
    "collect",
    "commemoration-antiphon",
    "commemoration-versicle",
}
CATEGORIES = {
    "translation-mismatch",
    "data-gap",
    "engine-bug",
    "open-question",
    "reference-error",
}
CATEGORY_LABELS = {
    "translation-mismatch": "Confirmed translation mismatch",
    "data-gap": "Confirmed data gap",
    "engine-bug": "Confirmed engine bug",
    "open-question": "Open question / ruling",
    "reference-error": "Known reference error",
    "untriaged": "Untriaged",
}
VESPERS_ASPECTS = {
    "magnificat-antiphon",
    "vespers-commemorations",
    "vespers-ownership",
    "vespers-color",
    "vespers-suffrage",
}
VESPERS_STRUCTURAL_ASPECTS = VESPERS_ASPECTS - {"magnificat-antiphon"}
MONTH_NAMES = [
    "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December",
]


@dataclass
class Finding:
    year: int
    aspect: str
    date: str
    detail: str
    category: str = "untriaged"
    confidence: str = ""
    issue: str = ""
    note: str = ""

    @property
    def finding_id(self) -> str:
        return f"{self.year}:{self.aspect}:{self.date}"


@dataclass
class Comparison:
    comparable: dict[str, int]
    findings: list[Finding]

    @property
    def total(self) -> int:
        return sum(self.comparable.values())

    @property
    def mismatches(self) -> int:
        return len(self.findings)


@dataclass
class ProperStatus:
    expected_slots: int
    missing_slots: int
    missing_feasts: int
    commons_fallback_feasts: int
    placeholders: int
    unresolved_rendered: int
    ordinary_fallback_candidates: int


@dataclass
class ProvenanceStatus:
    total: int
    verified: int
    needs_review: int
    source_unknown: int
    stale: int


@dataclass
class TriageRule:
    year: str
    aspect: str
    date: str
    category: str
    confidence: str
    issue: str
    note: str

    def matches(self, finding: Finding) -> bool:
        return (
            fnmatch.fnmatch(str(finding.year), self.year)
            and fnmatch.fnmatch(finding.aspect, self.aspect)
            and fnmatch.fnmatch(finding.date, self.date)
        )

    @property
    def specificity(self) -> int:
        return sum(len(value.replace("*", "").replace("?", ""))
                   for value in (self.year, self.aspect, self.date))


def run(command: list[str], *, cwd: pathlib.Path = ROOT, check: bool = True) -> str:
    result = subprocess.run(command, cwd=cwd, text=True, capture_output=True)
    if check and result.returncode:
        message = result.stderr.strip() or result.stdout.strip()
        raise RuntimeError(f"{' '.join(command)} failed: {message}")
    return result.stdout


def clean_pdf_title(raw: str) -> str:
    title = raw.splitlines()[0]
    title = ORDO_COMPARE.RANK_RE.sub(
        "", re.sub(r"^[L§†‡\s]+", "", title)
    ).strip()
    return title


def add_finding(findings: list[Finding], year: int, aspect: str,
                key: tuple[int, int], detail: str) -> None:
    findings.append(Finding(
        year=year, aspect=aspect, date=f"{key[0]:02d}-{key[1]:02d}",
        detail=detail,
    ))


def compare_ordo(year: int, pdf_path: pathlib.Path, ordo_path: pathlib.Path,
                 rubrics_path: pathlib.Path) -> Comparison:
    """Return one finding per comparable date/aspect assertion."""
    pdf = ORDO_COMPARE.pdf_days(pdf_path)
    ours_ordo = ORDO_COMPARE.our_ordo_days(ordo_path)
    ours_rubrics = ORDO_COMPARE.read_rubrics(rubrics_path)
    totals: dict[str, int] = {}
    findings: list[Finding] = []

    def compared(aspect: str) -> None:
        totals[aspect] = totals.get(aspect, 0) + 1

    # Calendar headline / winning celebration.
    for key in sorted(set(pdf) & set(ours_ordo)):
        raw = pdf[key].get("")
        if not raw:
            continue
        pdf_title = clean_pdf_title(raw)
        our_title = ours_ordo[key]["title"]
        compared("calendar")
        if (ORDO_COMPARE.similar(pdf_title, our_title) < 0.5
                and not (ORDO_COMPARE.is_ferial(pdf_title)
                         and ORDO_COMPARE.is_ferial(our_title))):
            add_finding(findings, year, "calendar", key,
                        f"ours={our_title} | reference={pdf_title}")

    # Boolean rubrics. Exact commemoration sets below supersede the less
    # informative yes/no commemoration flags.
    rubric_fields = (
        ("h_preces", "hours-preces", "Hours", r"Preces", r"No [Pp]re-?\s*ces"),
        ("l_suff", "lauds-suffrage", "Lauds", r"Suff\.", r"No Suff"),
        ("v_suff", "vespers-suffrage", "Vespers", r"Suff\.", r"No Suff"),
    )
    for field, aspect, section, yes, no in rubric_fields:
        for key in sorted(ours_rubrics):
            reference = ORDO_COMPARE.flag(pdf.get(key, {}).get(section), yes, no)
            if reference is None:
                continue
            compared(aspect)
            actual = ours_rubrics[key][field]
            if reference != actual:
                add_finding(findings, year, aspect, key,
                            f"ours={actual} | reference={reference}")

    # Commemorations are one set-valued assertion per office/date. Multiple
    # missing or extra names remain together so a single bad precedence
    # decision does not inflate the percentage according to name count.
    for field, section, aspect in (
            ("l_comms", "Lauds", "lauds-commemorations"),
            ("v_comms", "Vespers", "vespers-commemorations")):
        for key in sorted(ours_rubrics):
            reference_names = ORDO_COMPARE.pdf_commemorations(
                pdf.get(key, {}).get(section))
            if reference_names is None:
                continue
            compared(aspect)
            missing, extra = ORDO_COMPARE.match_commemorations(
                reference_names, ours_rubrics[key][field])
            if missing or extra:
                pieces = []
                if missing:
                    pieces.append("missing=" + "; ".join(missing))
                if extra:
                    pieces.append("extra=" + "; ".join(extra))
                add_finding(findings, year, aspect, key, " | ".join(pieces))

    # Gospel-canticle antiphon incipits.
    for field, section, pattern, aspect in (
            ("ben", "Lauds", r"Ben\.?\s*Ant\.?\s*[“\"]([^”\"]+)",
             "benedictus-antiphon"),
            ("mag", "Vespers", r"Mag\.?\s*Ant\.?\s*[“\"]([^”\"]+)",
             "magnificat-antiphon")):
        for key in sorted(ours_rubrics):
            match = re.search(pattern, pdf.get(key, {}).get(section, ""))
            actual = ours_rubrics[key][field]
            if not match or not actual:
                continue
            compared(aspect)
            reference = match.group(1)
            if not ORDO_COMPARE.incipit_matches(reference, actual):
                add_finding(findings, year, aspect, key,
                            f"ours={actual} | reference={reference}")

    # Liturgical colors.
    for key in sorted(set(pdf) & set(ours_ordo)):
        for section, field, aspect in (
                ("Lauds", "color", "lauds-color"),
                ("Vespers", "vcolor", "vespers-color")):
            match = re.match(section + r"\s+([WRGVB])\b",
                             pdf[key].get(section, "").strip())
            actual = ours_ordo[key][field]
            if not match or not actual:
                continue
            compared(aspect)
            reference = match.group(1).lower()
            if reference != actual:
                add_finding(findings, year, aspect, key,
                            f"ours={actual} | reference={reference}")

    # Vespers ownership. A day with a parsed Vespers section is comparable
    # even when neither side designates I/II Vespers.
    for key in sorted(set(pdf) & set(ours_ordo)):
        if "Vespers" not in pdf[key]:
            continue
        text = pdf[key].get("Vespers", "")
        reference = None
        if re.search(r"I of fol", text):
            reference = "fol"
        elif re.search(r"II of prec", text):
            reference = "prec"
        actual = ours_ordo[key].get("vespers")
        compared("vespers-ownership")
        if reference != actual:
            celebration = ours_rubrics.get(key, {}).get("cel", "")
            add_finding(findings, year, "vespers-ownership", key,
                        f"ours={actual or 'none'} | reference={reference or 'none'}"
                        f" | celebration={celebration}")

    findings.sort(key=lambda finding: (finding.date, finding.aspect))
    return Comparison(totals, findings)


def parse_audit(text: str, data_dir: pathlib.Path) -> ProperStatus:
    def header(pattern: str) -> int:
        match = re.search(pattern, text, re.MULTILINE)
        if not match:
            raise ValueError(f"audit output did not contain {pattern!r}")
        return int(match.group(1))

    missing_block = re.search(
        r"=== Missing propers:.*?\n(.*?)\n=== Commons fallback:", text, re.S)
    if not missing_block:
        raise ValueError("audit output did not contain the missing-propers block")
    missing_slots = 0
    for match in re.finditer(r"^\s+missing:\s+(.+)$", missing_block.group(1), re.M):
        missing_slots += len([part for part in match.group(1).split(",") if part.strip()])

    expected_slots = expected_proper_slots(data_dir)
    if missing_slots > expected_slots:
        raise ValueError("missing proper count exceeds audited proper universe")
    return ProperStatus(
        expected_slots=expected_slots,
        missing_slots=missing_slots,
        missing_feasts=header(r"^=== Missing propers: (\d+) feast"),
        commons_fallback_feasts=header(r"^=== Commons fallback: (\d+) feast"),
        placeholders=header(r"^=== Placeholders: (\d+) corpus"),
        unresolved_rendered=header(r"^=== Sweep \d+: unresolved texts: (\d+)"),
        ordinary_fallback_candidates=header(
            r"^=== Sweep \d+: ordinary fallbacks on Double\+ days: (\d+) slot"),
    )


def feast_ranks(data_dir: pathlib.Path) -> dict[str, str]:
    feasts: dict[str, str] = {}
    for path in sorted((data_dir / "feasts").glob("*.txt")):
        current = None
        for raw in path.read_text(errors="replace").splitlines():
            line = raw.strip()
            section = re.fullmatch(r"\[([^]]+)\]", line)
            if section:
                current = section.group(1)
                feasts.setdefault(current, "")
                continue
            rank = re.fullmatch(r"Rank\s*=\s*(.+)", line)
            if current and rank:
                feasts[current] = rank.group(1).strip()
    return feasts


def audit_suppressions(data_dir: pathlib.Path) -> dict[str, set[str]]:
    result: dict[str, set[str]] = {}
    path = data_dir / "audit-ok.txt"
    if not path.exists():
        return result
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split()
        result.setdefault(fields[0], set()).update(fields[1:] or ["*"])
    return result


def expected_proper_slots(data_dir: pathlib.Path) -> int:
    suppressions = audit_suppressions(data_dir)
    total = 0
    for feast_id, rank in feast_ranks(data_dir).items():
        if rank == "commemoration" or "*" in suppressions.get(feast_id, set()):
            continue
        total += len(PROPER_REFS - suppressions.get(feast_id, set()))
    return total


def parse_provenance(text: str) -> ProvenanceStatus:
    def count(label: str) -> int:
        match = re.search(rf"^\s*{re.escape(label)}\s+(\d+)\s*$", text, re.M)
        if not match:
            raise ValueError(f"provenance output did not contain {label!r}")
        return int(match.group(1))

    total_match = re.search(r"^=== Corpus provenance: (\d+) entries", text, re.M)
    if not total_match:
        raise ValueError("provenance output did not contain the entry total")
    return ProvenanceStatus(
        total=int(total_match.group(1)),
        verified=count("verified"),
        needs_review=count("needs-review"),
        source_unknown=count("source-unknown"),
        stale=count("stale"),
    )


def load_triage(path: pathlib.Path) -> list[TriageRule]:
    if not path.exists():
        return []
    rules = []
    with path.open(newline="") as handle:
        for row in csv.DictReader(handle):
            if not row.get("category"):
                continue
            category = row["category"].strip()
            if category not in CATEGORIES:
                raise ValueError(f"unknown ordo triage category {category!r}")
            rules.append(TriageRule(
                year=(row.get("year") or "*").strip(),
                aspect=(row.get("aspect") or "*").strip(),
                date=(row.get("date") or "*").strip(),
                category=category,
                confidence=(row.get("confidence") or "confirmed").strip(),
                issue=(row.get("issue") or "").strip(),
                note=(row.get("note") or "").strip(),
            ))
    return rules


def apply_triage(findings: list[Finding], rules: list[TriageRule]) -> None:
    for finding in findings:
        matches = [(rule.specificity, index, rule)
                   for index, rule in enumerate(rules) if rule.matches(finding)]
        if not matches:
            continue
        _, _, rule = max(matches)
        finding.category = rule.category
        finding.confidence = rule.confidence
        finding.issue = rule.issue
        finding.note = rule.note


def ruling_issues(repo: str, offline: bool) -> tuple[list[dict] | None, str]:
    if offline:
        return None, "offline mode"
    command = [
        "gh", "issue", "list", "--repo", repo, "--state", "open",
        "--label", "needs ruling", "--limit", "100",
        "--json", "number,title,url",
    ]
    try:
        output = run(command)
        return json.loads(output), ""
    except (FileNotFoundError, RuntimeError, json.JSONDecodeError) as error:
        return None, str(error)


def percent(numerator: int, denominator: int) -> float:
    return 100.0 * numerator / denominator if denominator else 100.0


def detail_values(detail: str) -> dict[str, str]:
    """Parse the report's ``key=value | key=value`` diagnostic detail."""
    values = {}
    for part in detail.split(" | "):
        key, separator, value = part.partition("=")
        if separator:
            values[key.strip()] = value.strip()
    return values


def normalized_words(text: str) -> list[str]:
    text = text.lower().replace("æ", "ae")
    text = re.sub(r"([a-z])-\s+([a-z])", r"\1\2", text)
    return re.findall(r"[a-z0-9]+", text)


def sequence_offset(needle: list[str], haystack: list[str]) -> int | None:
    if not needle or len(needle) > len(haystack):
        return None
    for index in range(len(haystack) - len(needle) + 1):
        if haystack[index:index + len(needle)] == needle:
            return index
    return None


def incipit_boundary_candidate(finding: Finding) -> bool:
    """Whether the multi-word reference incipit occurs later in our text.

    This is a diagnostic cluster, not an agreement rule: a shifted boundary
    can itself be a real translation or editorial mismatch.
    """
    values = detail_values(finding.detail)
    reference = normalized_words(values.get("reference", ""))
    actual = normalized_words(values.get("ours", ""))
    if len(reference) < 2:
        return False
    offset = sequence_offset(reference, actual)
    return offset is not None and offset > 0


def displayed_incipit(text: str, word_limit: int = 8) -> str:
    incipit = text.split("*", 1)[0].strip(" ,;:.")
    words = incipit.split()
    if len(words) > word_limit:
        return " ".join(words[:word_limit]) + "…"
    return incipit


def repeated_generated_incipits(findings: list[Finding], limit: int = 5) -> list[dict]:
    grouped: dict[str, dict] = {}
    for finding in findings:
        actual = detail_values(finding.detail).get("ours", "")
        incipit = displayed_incipit(actual)
        key = " ".join(incipit.lower().split())
        if not key:
            continue
        entry = grouped.setdefault(key, {
            "incipit": incipit, "count": 0, "dates": [],
        })
        entry["count"] += 1
        entry["dates"].append(finding.date)
    recurring = [entry for entry in grouped.values() if entry["count"] > 1]
    recurring.sort(key=lambda entry: (-entry["count"], entry["incipit"].lower()))
    return recurring[:limit]


def commemoration_direction(finding: Finding) -> str:
    values = detail_values(finding.detail)
    missing, extra = bool(values.get("missing")), bool(values.get("extra"))
    if missing and extra:
        return "both"
    if missing:
        return "missing-only"
    return "extra-only"


def ownership_direction(finding: Finding) -> str:
    values = detail_values(finding.detail)
    ours, reference = values.get("ours", "none"), values.get("reference", "none")
    if ours != "none" and reference != "none":
        return "conflicting-owners"
    if ours != "none":
        return "engine-only"
    return "reference-only"


def ownership_context(finding: Finding) -> str:
    celebration = detail_values(finding.detail).get("celebration", "").lower()
    paschal_block = (
        "holy thursday", "good friday", "holy saturday", "easter monday",
        "easter tuesday", "octave of easter", "octave of pentecost",
    )
    if any(name in celebration for name in paschal_block):
        return "triduum-easter-pentecost"
    if "octave" in celebration:
        return "other-octave"
    if celebration == "feria" or "sunday" in celebration:
        return "feria-or-sunday"
    return "feast-or-other"


def analyze_clusters(comparison: Comparison,
                     previous_comparison: Comparison | None = None) -> dict:
    """Build reproducible symptom clusters without assigning root causes."""
    findings = comparison.findings
    untriaged = [finding for finding in findings if finding.category == "untriaged"]
    by_date: dict[str, list[Finding]] = collections.defaultdict(list)
    for finding in findings:
        by_date[finding.date].append(finding)

    vespers = [finding for finding in findings if finding.aspect in VESPERS_ASPECTS]
    untriaged_vespers = [finding for finding in vespers if finding.category == "untriaged"]
    aspect_rows = {}
    for aspect in sorted(VESPERS_ASPECTS):
        rows = [finding for finding in vespers if finding.aspect == aspect]
        aspect_rows[aspect] = {
            "total": len(rows),
            "untriaged": sum(finding.category == "untriaged" for finding in rows),
            "open_question": sum(finding.category == "open-question" for finding in rows),
        }

    ownership = [finding for finding in findings if finding.aspect == "vespers-ownership"]
    ownership_directions = collections.Counter(ownership_direction(finding)
                                                for finding in ownership)
    ownership_by_category = collections.Counter(finding.category for finding in ownership)
    engine_only_contexts = collections.Counter(
        ownership_context(finding) for finding in ownership
        if ownership_direction(finding) == "engine-only"
        and finding.category == "untriaged"
    )

    commemorations = [finding for finding in untriaged_vespers
                      if finding.aspect == "vespers-commemorations"]
    comm_directions = collections.Counter(commemoration_direction(finding)
                                          for finding in commemorations)
    comm_patterns = collections.Counter()
    for finding in commemorations:
        values = detail_values(finding.detail)
        missing = values.get("missing", "")
        extra = values.get("extra", "")
        if re.search(r"\boctave\b", extra, re.I):
            comm_patterns["has_extra_octave"] += 1
        if re.search(r"\b(?:monday|tuesday|wednesday|thursday|friday|saturday) after\b",
                     extra, re.I):
            comm_patterns["has_extra_named_feria"] += 1
        if re.search(r"(?:^|; )(?:Fer\.|Ember)(?:;|$)", missing):
            comm_patterns["has_missing_feria_or_ember"] += 1
        if re.search(r"(?:^|; )(?:BVM|Sun\.|Oct\.)", missing):
            comm_patterns["has_missing_generic_temporal_or_bvm"] += 1

    overlap_aspects = [
        "magnificat-antiphon", "vespers-ownership", "vespers-color",
        "vespers-suffrage", "calendar",
    ]
    comm_overlaps = {
        aspect: sum(any(other.aspect == aspect for other in by_date[finding.date])
                    for finding in commemorations)
        for aspect in overlap_aspects
    }
    comm_isolated = sum(
        sum(other.aspect in VESPERS_ASPECTS for other in by_date[finding.date]) == 1
        for finding in commemorations
    )

    magnificats = [finding for finding in untriaged_vespers
                   if finding.aspect == "magnificat-antiphon"]
    mag_with_structure = sum(
        any(other.aspect in VESPERS_STRUCTURAL_ASPECTS
            for other in by_date[finding.date])
        for finding in magnificats
    )
    boundary_candidates = [finding for finding in magnificats
                           if incipit_boundary_candidate(finding)]

    colors = [finding for finding in findings if finding.aspect == "vespers-color"]
    color_reference = collections.Counter(
        detail_values(finding.detail).get("reference", "unknown") for finding in colors)
    color_overlaps = {
        aspect: sum(any(other.aspect == aspect for other in by_date[finding.date])
                    for finding in colors)
        for aspect in [
            "magnificat-antiphon", "vespers-commemorations",
            "vespers-ownership", "vespers-suffrage",
        ]
    }

    suffrages = [finding for finding in findings if finding.aspect == "vespers-suffrage"]
    suffrage_directions = collections.Counter()
    for finding in suffrages:
        values = detail_values(finding.detail)
        if values.get("ours") == "False" and values.get("reference") == "True":
            suffrage_directions["engine-missing"] += 1
        elif values.get("ours") == "True" and values.get("reference") == "False":
            suffrage_directions["engine-extra"] += 1

    month_counts = collections.Counter(finding.date[:2] for finding in untriaged_vespers)
    monthly_hotspots = [
        {"month": MONTH_NAMES[int(month) - 1], "count": count}
        for month, count in sorted(month_counts.items(), key=lambda item: (-item[1], item[0]))
    ]

    recurrence = None
    if previous_comparison is not None:
        previous_keys = {(finding.aspect, finding.date)
                         for finding in previous_comparison.findings}
        repeated = [finding for finding in untriaged
                    if (finding.aspect, finding.date) in previous_keys]
        recurrence = {
            "previous_year": previous_comparison.findings[0].year
            if previous_comparison.findings else None,
            "count": len(repeated),
            "by_aspect": dict(sorted(collections.Counter(
                finding.aspect for finding in repeated).items())),
        }

    canticle_antiphons = [
        finding for finding in untriaged
        if finding.aspect in {"benedictus-antiphon", "magnificat-antiphon"}
    ]
    boundary_by_aspect = collections.Counter(
        finding.aspect for finding in canticle_antiphons
        if incipit_boundary_candidate(finding)
    )

    return {
        "untriaged": {
            "total": len(untriaged),
            "unique_dates": len({finding.date for finding in untriaged}),
            "by_aspect": dict(sorted(collections.Counter(
                finding.aspect for finding in untriaged).items())),
        },
        "canticle_antiphons": {
            "total": len(canticle_antiphons),
            "by_aspect": dict(sorted(collections.Counter(
                finding.aspect for finding in canticle_antiphons).items())),
            "incipit_boundary_or_wording_candidates": sum(boundary_by_aspect.values()),
            "boundary_candidates_by_aspect": dict(sorted(boundary_by_aspect.items())),
        },
        "vespers": {
            "total": len(vespers),
            "untriaged": len(untriaged_vespers),
            "share_of_untriaged_percent": percent(len(untriaged_vespers), len(untriaged)),
            "by_aspect": aspect_rows,
            "ownership": {
                "directions": dict(sorted(ownership_directions.items())),
                "by_category": dict(sorted(ownership_by_category.items())),
                "untriaged_engine_only_contexts": dict(sorted(engine_only_contexts.items())),
            },
            "commemorations": {
                "directions": dict(sorted(comm_directions.items())),
                "patterns": dict(sorted(comm_patterns.items())),
                "overlaps": comm_overlaps,
                "isolated": comm_isolated,
            },
            "magnificat": {
                "untriaged": len(magnificats),
                "with_structural_mismatch": mag_with_structure,
                "standalone": len(magnificats) - mag_with_structure,
                "incipit_boundary_or_wording_candidates": len(boundary_candidates),
                "candidate_finding_ids": [finding.finding_id
                                          for finding in boundary_candidates],
                "repeated_generated_incipits": repeated_generated_incipits(magnificats),
            },
            "color": {
                "reference_colors": dict(sorted(color_reference.items())),
                "overlaps": color_overlaps,
            },
            "suffrage": {
                "directions": dict(sorted(suffrage_directions.items())),
            },
            "monthly_hotspots": monthly_hotspots,
        },
        "previous_year_recurrence": recurrence,
    }


def render_cluster_markdown(clusters: dict) -> list[str]:
    """Render diagnostic clusters without presenting them as adjudications."""
    untriaged = clusters["untriaged"]
    vespers = clusters["vespers"]
    recurrence = clusters.get("previous_year_recurrence")
    lines = [
        "",
        "## Untriaged diagnostic clusters",
        "",
        "These are reproducible symptoms, not automatic root-cause assignments. "
        "Only the triage ledger classifies translation, data, engine, ruling, or "
        "reference-error causes.",
        "",
        (f"- {untriaged['total']} untriaged assertions occur across "
         f"{untriaged['unique_dates']} dates."),
        (f"- {vespers['untriaged']} are Vespers-specific "
         f"({vespers['share_of_untriaged_percent']:.1f}% of the untriaged queue)."),
    ]
    if recurrence and recurrence.get("previous_year") is not None:
        lines.append(
            f"- {recurrence['count']} recur on the same date and aspect in the "
            f"{recurrence['previous_year']} reference comparison."
        )

    canticles = clusters["canticle_antiphons"]
    boundary_by_aspect = canticles["boundary_candidates_by_aspect"]
    lines.extend([
        "",
        "### Canticle-antiphon review",
        "",
        (f"- {canticles['total']} untriaged incipit differences: "
         f"{canticles['by_aspect'].get('benedictus-antiphon', 0)} Benedictus and "
         f"{canticles['by_aspect'].get('magnificat-antiphon', 0)} Magnificat."),
        (f"- {canticles['incipit_boundary_or_wording_candidates']} have a multi-word "
         f"reference incipit appearing later in the generated text "
         f"({boundary_by_aspect.get('benedictus-antiphon', 0)} Benedictus, "
         f"{boundary_by_aspect.get('magnificat-antiphon', 0)} Magnificat). They remain "
         "mismatches pending source review; this cluster does not call them comparator noise."),
    ])

    lines.extend([
        "",
        "### Vespers by aspect",
        "",
        "| Aspect | Total | Untriaged | Open question |",
        "|---|---:|---:|---:|",
    ])
    aspect_order = [
        "magnificat-antiphon", "vespers-commemorations", "vespers-ownership",
        "vespers-color", "vespers-suffrage",
    ]
    for aspect in aspect_order:
        row = vespers["by_aspect"][aspect]
        lines.append(
            f"| {aspect} | {row['total']} | {row['untriaged']} | "
            f"{row['open_question']} |"
        )

    ownership = vespers["ownership"]
    directions = ownership["directions"]
    owner_contexts = ownership["untriaged_engine_only_contexts"]
    lines.extend([
        "",
        "### Vespers ownership",
        "",
        (f"- Engine-only designation: {directions.get('engine-only', 0)}; "
         f"reference-only designation: {directions.get('reference-only', 0)}; "
         f"conflicting explicit owners: {directions.get('conflicting-owners', 0)}."),
        (f"- Confirmed open-question ownership findings: "
         f"{ownership['by_category'].get('open-question', 0)}; untriaged: "
         f"{ownership['by_category'].get('untriaged', 0)}."),
        (f"- Untriaged engine-only contexts: Triduum/Easter/Pentecost "
         f"{owner_contexts.get('triduum-easter-pentecost', 0)}, other octaves "
         f"{owner_contexts.get('other-octave', 0)}, feria/Sunday "
         f"{owner_contexts.get('feria-or-sunday', 0)}, feast/other "
         f"{owner_contexts.get('feast-or-other', 0)}."),
    ])

    commemorations = vespers["commemorations"]
    comm_directions = commemorations["directions"]
    patterns = commemorations["patterns"]
    overlaps = commemorations["overlaps"]
    lines.extend([
        "",
        "### Vespers commemorations",
        "",
        (f"- Extra-only dates: {comm_directions.get('extra-only', 0)}; "
         f"missing-only: {comm_directions.get('missing-only', 0)}; "
         f"both missing and extra: {comm_directions.get('both', 0)}."),
        (f"- {commemorations['isolated']} occur without another Vespers mismatch; "
         f"{overlaps.get('magnificat-antiphon', 0)} coincide with a Magnificat "
         f"mismatch and {overlaps.get('vespers-ownership', 0)} with ownership."),
        (f"- Pattern flags (overlapping): extra octave "
         f"{patterns.get('has_extra_octave', 0)}, extra named feria "
         f"{patterns.get('has_extra_named_feria', 0)}, missing feria/Ember "
         f"{patterns.get('has_missing_feria_or_ember', 0)}, missing generic "
         f"Sunday/octave/BVM {patterns.get('has_missing_generic_temporal_or_bvm', 0)}."),
    ])

    magnificat = vespers["magnificat"]
    lines.extend([
        "",
        "### Magnificat antiphons",
        "",
        (f"- {magnificat['with_structural_mismatch']} coincide with another "
         f"Vespers structural mismatch; {magnificat['standalone']} are standalone."),
        (f"- {magnificat['incipit_boundary_or_wording_candidates']} have a multi-word "
         "reference incipit appearing later in the generated text. These remain real "
         "wording/boundary review candidates; they are not treated as agreements."),
    ])
    recurring = magnificat["repeated_generated_incipits"]
    if recurring:
        lines.extend([
            "",
            "| Repeated generated incipit | Findings |",
            "|---|---:|",
        ])
        for entry in recurring:
            incipit = entry["incipit"].replace("|", "\\|")
            lines.append(f"| {incipit} | {entry['count']} |")

    color = vespers["color"]
    suffrage = vespers["suffrage"]["directions"]
    reference_green = color["reference_colors"].get("g", 0)
    lines.extend([
        "",
        "### Vespers color and suffrage",
        "",
        (f"- {reference_green} color findings have green in the reference. "
         f"Color findings overlap Magnificat {color['overlaps'].get('magnificat-antiphon', 0)} "
         f"times, commemorations {color['overlaps'].get('vespers-commemorations', 0)}, "
         f"and ownership {color['overlaps'].get('vespers-ownership', 0)}."),
        (f"- Suffrage is missing from the engine on "
         f"{suffrage.get('engine-missing', 0)} dates and extra on "
         f"{suffrage.get('engine-extra', 0)} dates."),
    ])
    hotspots = vespers["monthly_hotspots"][:3]
    if hotspots:
        lines.append(
            "- Highest untriaged Vespers months: "
            + ", ".join(f"{entry['month']} {entry['count']}" for entry in hotspots)
            + "."
        )
    return lines


def render_markdown(year: int, proper: ProperStatus, provenance: ProvenanceStatus,
                    comparison: Comparison, clusters: dict,
                    issues: list[dict] | None,
                    issue_warning: str, commit: str) -> str:
    matched = comparison.total - comparison.mismatches
    reference_errors = sum(
        finding.category == "reference-error" for finding in comparison.findings)
    proper_filled = proper.expected_slots - proper.missing_slots
    category_counts = {
        category: sum(f.category == category for f in comparison.findings)
        for category in (*sorted(CATEGORIES), "untriaged")
    }
    provisional = {
        category: sum(f.category == category and f.confidence == "provisional"
                      for f in comparison.findings)
        for category in CATEGORIES
    }
    text_candidates = sum(
        finding.category == "untriaged"
        and finding.aspect in {"benedictus-antiphon", "magnificat-antiphon"}
        for finding in comparison.findings
    )
    today = dt.date.today().isoformat()
    lines = [
        f"# Project status — {year}",
        "",
        f"Generated {today} from commit `{commit}`.",
        "",
        "## Executive summary",
        "",
        (f"- **Known proper-slot coverage: {percent(proper_filled, proper.expected_slots):.1f}%** "
         f"({proper_filled}/{proper.expected_slots}). There are {proper.missing_slots} known "
         f"missing slot(s) across {proper.missing_feasts} feast(s)."),
        (f"- **Rendered completeness: {percent(proper.expected_slots - proper.unresolved_rendered, proper.expected_slots):.1f}%** "
         f"({proper.unresolved_rendered} unresolved annual-sweep finding(s)); "
         f"{proper.ordinary_fallback_candidates} ordinary-fallback candidate(s) still need "
         "book-checking or an explicit acknowledgement."),
        (f"- **Text source verification: {percent(provenance.verified, provenance.total):.1f}%** "
         f"({provenance.verified}/{provenance.total} corpus entries explicitly verified; "
         f"{provenance.needs_review} need review and {provenance.source_unknown} need a source)."),
        (f"- **Strict {year} ordo parity: {percent(matched, comparison.total):.1f}%** "
         f"({matched}/{comparison.total} comparable assertions; "
         f"{comparison.mismatches} mismatches)."),
    ]
    if reference_errors:
        lines.append(
            f"- **Adjudicated ordo parity: {percent(matched + reference_errors, comparison.total):.1f}%** "
            f"when {reference_errors} confirmed reference error(s) are not charged to the engine."
        )
    if issues is None:
        lines.append(f"- **Open clergy rulings: unavailable** ({issue_warning}).")
    else:
        lines.append(f"- **Open clergy rulings: {len(issues)} GitHub issue(s).**")

    lines.extend([
        "",
        "The proper percentage is a *known-gap* measure, not a textual assurance claim. "
        "A common or seasonal fallback counts as filled when the audit considers it valid; "
        "the provenance percentage records the much stricter word-for-word source review.",
        "",
        "## Ordo difference by cause",
        "",
        "| Cause | Findings | Share of strict diff | Notes |",
        "|---|---:|---:|---|",
    ])
    ordered = [
        "translation-mismatch", "data-gap", "engine-bug", "open-question",
        "reference-error", "untriaged",
    ]
    for category in ordered:
        count = category_counts[category]
        note = ""
        if provisional.get(category):
            note = f"{provisional[category]} provisional"
        elif category == "untriaged":
            note = f"includes {text_candidates} text/selection candidate(s)"
        lines.append(
            f"| {CATEGORY_LABELS[category]} | {count} | "
            f"{percent(count, comparison.mismatches):.1f}% | {note} |"
        )

    lines.extend([
        "",
        "Each finding is one date/aspect assertion, so a date with three wrong "
        "commemoration names counts once rather than three times. Provisional classifications "
        "are useful queue estimates and should be replaced by exact triage rows after diagnosis. "
        f"The {text_candidates} untriaged canticle-antiphon incipit differences are an upper "
        "bound on translation work: selecting the wrong antiphon produces the same symptom.",
    ])
    lines.extend(render_cluster_markdown(clusters))
    lines.extend([
        "",
        "## Ordo checks",
        "",
        "| Aspect | Match | Compared | Accuracy |",
        "|---|---:|---:|---:|",
    ])
    mismatches_by_aspect = {
        aspect: sum(f.aspect == aspect for f in comparison.findings)
        for aspect in comparison.comparable
    }
    for aspect in sorted(comparison.comparable):
        total = comparison.comparable[aspect]
        good = total - mismatches_by_aspect[aspect]
        lines.append(f"| {aspect} | {good} | {total} | {percent(good, total):.1f}% |")

    lines.extend([
        "",
        "## Proper and text-review detail",
        "",
        f"- Placeholder corpus entries: {proper.placeholders}",
        f"- Feasts using unacknowledged commons fallbacks: {proper.commons_fallback_feasts}",
        f"- Stale text attestations: {provenance.stale}",
    ])
    if issues:
        lines.extend(["", "## Questions awaiting clergy rulings", ""])
        for issue in sorted(issues, key=lambda item: item["number"]):
            lines.append(f"- [#{issue['number']} — {issue['title']}]({issue['url']})")

    lines.extend([
        "",
        "## Reproduce and triage",
        "",
        f"Run `make project-status YEAR={year}`. Generated artifacts live under "
        f"`output/status/`: the Markdown report, a JSON snapshot, and every current ordo "
        f"finding as CSV. Classifications live in `data/review/ordo-triage.csv`; add the "
        f"narrowest defensible year/aspect/date rule, issue number, and note, then rerun.",
        "",
    ])
    return "\n".join(lines)


def write_findings(path: pathlib.Path, findings: list[Finding]) -> None:
    with path.open("w", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow([
            "finding_id", "year", "aspect", "date", "category", "confidence",
            "issue", "detail", "note",
        ])
        for finding in findings:
            writer.writerow([
                finding.finding_id, finding.year, finding.aspect, finding.date,
                finding.category, finding.confidence, finding.issue,
                finding.detail, finding.note,
            ])


def build_comparison(year: int, pdf: pathlib.Path, office_binary: pathlib.Path,
                     temp: pathlib.Path, prefix: str) -> Comparison:
    extracted = temp / f"{prefix}-reference.txt"
    ours = temp / f"{prefix}-ours.txt"
    rubrics = temp / f"{prefix}-rubrics.tsv"
    run(["pdftotext", "-layout", str(pdf), str(extracted)])
    ours.write_text(run([str(office_binary), "ordo", str(year)]))
    rubrics.write_text(run([str(office_binary), "rubrics", str(year)]))
    return compare_ordo(year, extracted, ours, rubrics)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--year", type=int, default=2026)
    parser.add_argument("--resources", type=pathlib.Path, default=ROOT.parent / "resources")
    parser.add_argument("--office", type=pathlib.Path, default=ROOT / "office")
    parser.add_argument("--data", type=pathlib.Path, default=ROOT / "data")
    parser.add_argument("--output", type=pathlib.Path, default=ROOT / "output" / "status")
    parser.add_argument("--triage", type=pathlib.Path,
                        default=ROOT / "data" / "review" / "ordo-triage.csv")
    parser.add_argument("--repo", default="orthodoxwest/office")
    parser.add_argument("--offline", action="store_true",
                        help="skip the optional live GitHub needs-ruling query")
    args = parser.parse_args()

    pdf = args.resources / f"{args.year}-ordo.pdf"
    if not pdf.exists():
        parser.error(f"published ordo not found: {pdf}")
    if not args.office.exists():
        parser.error(f"office binary not found: {args.office}; run make build")

    with tempfile.TemporaryDirectory(prefix="office-status-") as temp_name:
        temp = pathlib.Path(temp_name)
        comparison = build_comparison(args.year, pdf, args.office, temp, "current")
        previous_comparison = None
        previous_pdf = args.resources / f"{args.year - 1}-ordo.pdf"
        if previous_pdf.exists():
            previous_comparison = build_comparison(
                args.year - 1, previous_pdf, args.office, temp, "previous")

    audit_text = run([str(args.office), "audit", "-year", str(args.year)])
    provenance_text = run([str(args.office), "review", "provenance"])
    proper = parse_audit(audit_text, args.data)
    provenance = parse_provenance(provenance_text)
    apply_triage(comparison.findings, load_triage(args.triage))
    clusters = analyze_clusters(comparison, previous_comparison)
    issues, issue_warning = ruling_issues(args.repo, args.offline)
    commit = run(["git", "rev-parse", "--short", "HEAD"]).strip() or "unknown"

    report = render_markdown(
        args.year, proper, provenance, comparison, clusters,
        issues, issue_warning, commit)
    args.output.mkdir(parents=True, exist_ok=True)
    stem = args.output / f"project-status-{args.year}"
    (stem.with_suffix(".md")).write_text(report)
    write_findings(args.output / f"ordo-findings-{args.year}.csv", comparison.findings)
    snapshot = {
        "year": args.year,
        "generated_on": dt.date.today().isoformat(),
        "commit": commit,
        "proper": asdict(proper),
        "provenance": asdict(provenance),
        "ordo": {
            "comparable": comparison.comparable,
            "total": comparison.total,
            "mismatches": comparison.mismatches,
            "clusters": clusters,
            "findings": [dict(asdict(f), finding_id=f.finding_id)
                         for f in comparison.findings],
        },
        "ruling_issues": issues,
        "ruling_issue_warning": issue_warning,
    }
    stem.with_suffix(".json").write_text(json.dumps(snapshot, indent=2) + "\n")
    print(report)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, ValueError) as error:
        print(f"project-status: {error}", file=sys.stderr)
        raise SystemExit(1)
