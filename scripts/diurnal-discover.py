#!/usr/bin/env python3
"""Discover feast propers hidden behind common, seasonal, or ordinary fallbacks."""

from __future__ import annotations

import argparse
from collections import Counter
import datetime as dt
import hashlib
import importlib.util
import json
import re
import sys
from pathlib import Path
from typing import Callable, Iterable


ROOT = Path(__file__).resolve().parents[1]
DISCOVER_ROOT = ROOT / "output" / "discover"
SCHEMA = Path(__file__).with_name("diurnal-discovery-schema.json")
DEFAULT_PAGE_KEY = "monastic-diurnal"
DEFAULT_MODEL = "gpt-5.6-luna"


def import_script(name: str, filename: str):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    assert spec.loader
    spec.loader.exec_module(module)
    return module


diurnal_pages = import_script("diurnal_pages_for_discover", "diurnal-pages.py")
transcribe = import_script("diurnal_transcribe_for_discover", "diurnal-transcribe.py")
ProviderRunner = transcribe.ProviderRunner
ProviderError = transcribe.ProviderError


ALLOWED_TIERS = {"common", "seasonal", "ordinary"}
EXCLUDED_SLOTS = {"alleluia", "marian-antiphon", "pre-collect-versicle", "athanasian"}
EXCLUDED_TIERS = {"ordinary-weekday", "temporal-week"}
EXCLUDED_HOURS = {"prime", "compline"}
FIXED_FILES = ("sanctoral.txt", "commemorations.txt", "awrv.txt")


def parse_record_file(path: Path) -> dict[str, dict]:
    records: dict[str, dict] = {}
    current: dict | None = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if match := re.fullmatch(r"\[([^]]+)]", line):
            current = {"id": match.group(1), "source_file": path.name}
            records[current["id"]] = current
        elif current is not None and "=" in line:
            key, value = (part.strip() for part in line.split("=", 1))
            current[key] = value
    return records


def load_feast_catalog(data_dir: Path = ROOT / "data") -> dict[str, dict]:
    catalog: dict[str, dict] = {}
    for filename in FIXED_FILES:
        for feast_id, record in parse_record_file(data_dir / "feasts" / filename).items():
            if "Month" not in record or "Day" not in record:
                continue
            catalog[feast_id] = {
                **record, "month": int(record["Month"]), "day": int(record["Day"]),
                "kind": "fixed",
            }
    temporal = parse_record_file(data_dir / "feasts" / "temporal.txt")
    for feast_id, record in temporal.items():
        if (data_dir / "texts" / "proper" / f"{feast_id}.txt").is_file():
            catalog[feast_id] = {**record, "month": None, "day": None, "kind": "temporal"}
    return catalog


def live_sections(feast_id: str, data_dir: Path = ROOT / "data") -> dict[str, str]:
    path = data_dir / "texts" / "proper" / f"{feast_id}.txt"
    if not path.is_file():
        return {}
    sections: dict[str, list[str]] = {}
    current = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        if match := re.fullmatch(r"\[([a-z0-9][a-z0-9-]*)]", raw.strip()):
            current = match.group(1)
            sections[current] = []
            continue
        if re.fullmatch(r"#\s*\[[a-z0-9][a-z0-9-]*]", raw.strip()):
            current = None
            continue
        if current is not None and not raw.lstrip().startswith("#"):
            sections[current].append(raw)
    return {key: "\n".join(lines).strip() for key, lines in sections.items()}


def slot_is_printed(row: dict) -> bool:
    if row.get("selected_tier") in EXCLUDED_TIERS or row.get("selected_tier") not in ALLOWED_TIERS:
        return False
    slot = row.get("resolver_slot") or row.get("slot_ref") or row.get("requested_slot", "")
    if slot in EXCLUDED_SLOTS:
        return False
    # The diurnal never prints feast propers for Prime or Compline chapter/hymn/responsory.
    if row.get("hour") in EXCLUDED_HOURS and slot in {"chapter", "hymn", "short-responsory", "versicle"}:
        return False
    if slot in {
        "collect", "benedictus-antiphon", "magnificat-antiphon",
        "magnificat-antiphon-first", "commemoration-antiphon",
        "commemoration-versicle", "commemoration-collect",
    }:
        return True
    if re.fullmatch(r"psalm-antiphon-[1-5]", slot):
        return row.get("resolver_hour") in {"lauds", "vespers"}
    return bool(re.match(r"^(chapter|hymn|versicle|short-responsory)(?:-|$)", slot))


def target_section(row: dict) -> str:
    slot = row.get("resolver_slot") or row.get("slot_ref") or row.get("requested_slot", "")
    hour = row.get("resolver_hour") or row.get("hour", "")
    first = bool(row.get("first_vespers")) and hour == "vespers"
    if slot in {"collect", "benedictus-antiphon", "commemoration-collect"}:
        return slot
    if slot.startswith("magnificat-antiphon"):
        return "magnificat-antiphon-first" if first or slot.endswith("-first") else "magnificat-antiphon"
    if slot.startswith("commemoration-antiphon") or slot.startswith("commemoration-versicle"):
        base = slot.split("-lauds", 1)[0].split("-vespers", 1)[0]
        return f"{base}-{hour}" if hour in {"lauds", "vespers"} else base
    if re.fullmatch(r"psalm-antiphon-[1-5]", slot):
        if hour == "vespers":
            return f"{slot}-first-vespers" if first else f"{slot}-vespers"
        return slot
    base = re.sub(r"-(?:lauds|prime|terce|sext|none|vespers|compline)$", "", slot)
    if first:
        return f"{base}-first-vespers"
    return f"{base}-{hour}" if hour else base


def representative_url(row: dict) -> str:
    date = row.get("date", "")
    hour = row.get("hour", "")
    return f"https://office.fly.dev/{hour}/{date}" if date and hour else ""


def build_dossiers(inventory: dict, catalog: dict[str, dict],
                   data_dir: Path = ROOT / "data") -> list[dict]:
    grouped: dict[str, list[dict]] = {}
    for row in inventory.get("rows", []):
        feast_id = row.get("owner_id", "")
        if feast_id not in catalog or not slot_is_printed(row):
            continue
        grouped.setdefault(feast_id, []).append(row)
    dossiers = []
    for feast_id, rows in sorted(grouped.items()):
        feast = catalog[feast_id]
        combined: dict[str, dict] = {}
        for row in rows:
            section = target_section(row)
            if section not in combined:
                combined[section] = {
                    "hour": row.get("hour", ""), "hours": [row.get("hour", "")],
                    "slot": row.get("slot_ref") or row.get("requested_slot", ""),
                    "target_section": section,
                    "target_key": f"proper/{feast_id}/{section}",
                    "current_tier": row.get("selected_tier", ""),
                    "current_key": row.get("selected_ref", ""),
                    "current_tiers": [row.get("selected_tier", "")],
                    "current_keys": [row.get("selected_ref", "")],
                    "contexts": [{
                        "hour": row.get("hour", ""), "tier": row.get("selected_tier", ""),
                        "key": row.get("selected_ref", ""), "date": row.get("date", ""),
                    }],
                    "date": row.get("date", ""), "representative_url": representative_url(row),
                }
            else:
                item = combined[section]
                if row.get("hour", "") not in item["hours"]:
                    item["hours"].append(row.get("hour", ""))
                if row.get("selected_tier", "") not in item["current_tiers"]:
                    item["current_tiers"].append(row.get("selected_tier", ""))
                if row.get("selected_ref", "") not in item["current_keys"]:
                    item["current_keys"].append(row.get("selected_ref", ""))
                item["contexts"].append({
                    "hour": row.get("hour", ""), "tier": row.get("selected_tier", ""),
                    "key": row.get("selected_ref", ""), "date": row.get("date", ""),
                })
        fallbacks = list(combined.values())
        for index, fallback in enumerate(fallbacks, 1):
            fallback["id"] = f"slot-{index}"
            fallback["hours"] = sorted(filter(None, fallback["hours"]))
        dossiers.append({
            "feast_id": feast_id, "name": feast.get("Name", feast_id),
            "proper_name": feast.get("ProperName", ""), "month": feast.get("month"),
            "day": feast.get("day"), "rank": feast.get("Rank", ""),
            "category": feast.get("Category", ""), "kind": feast["kind"],
            "fallbacks": fallbacks, "live_sections": live_sections(feast_id, data_dir),
        })
    return dossiers


def parse_inventory(path: Path | None) -> dict:
    if path:
        return json.loads(path.read_text(encoding="utf-8"))
    output = transcribe.run_office([
        "review", "resolution-inventory", "-start", "2026", "-years", "1",
        "-fallback-only", "-json",
    ])
    return json.loads(output)


def page_record(page: dict) -> dict:
    png = Path(page["png"])
    if not png.is_absolute():
        png = ROOT / png
    return {
        "pdf_page": page["pdf_page"], "printed_page": page.get("printed_page"),
        "inferred": bool(page.get("inferred", False)), "png": str(png.resolve()),
        "ocr_route": "pdftotext+pdftotext-layout",
        "ocr_text_sha256": hashlib.sha256(
            (page.get("text", "") + "\0" + page.get("layout_text", "")).encode("utf-8")
        ).hexdigest(),
    }


class FeastPageLocator:
    def __init__(self, page_key: str = DEFAULT_PAGE_KEY):
        self.page_key = page_key
        self.index = diurnal_pages.load_index(page_key)

    def locate(self, dossier: dict) -> dict:
        names = [
            dossier.get("name", ""), dossier.get("proper_name", ""),
            transcribe.humanize(dossier.get("feast_id", "")),
        ]
        if dossier.get("kind") == "temporal":
            alias = {
                "holy-monday": "Monday in Holy Week",
                "holy-tuesday": "Tuesday in Holy Week",
                "holy-wednesday": "Wednesday in Holy Week",
                "holy-thursday": "Thursday in Holy Week",
                "christ-the-king": "Christ the King",
                "pentecost": "Whitsunday",
            }.get(dossier.get("feast_id", ""))
            if alias:
                names.append(alias)
        if dossier.get("kind") == "fixed":
            attempts = [diurnal_pages.locate_feast_pages(
                self.index, dossier["month"], dossier["day"], name,
            ) for name in names if name]
        else:
            attempts = [diurnal_pages.locate_named_pages(self.index, name) for name in names if name]
        found = [attempt for attempt in attempts if attempt.get("pages")]
        if not found:
            return {
                "pages": [], "locate_confidence": "none", "locate_status": "no-pages",
                "source_witness": {
                    "page_key": self.page_key, "pdf_sha256": self.index.get("pdf_sha256", ""),
                    "locator_route": "ocr-running-head-date+fuzzy-title"
                    if dossier.get("kind") == "fixed" else "ocr-fuzzy-title",
                },
            }
        best = max(found, key=lambda item: (item.get("name_score", 0), len(item["pages"])))
        pages = [page_record(page) for page in best["pages"]]
        return {
            "pages": pages,
            "locate_confidence": best["locate_confidence"], "locate_status": "matched",
            "locate_name_score": best.get("name_score", 0),
            "source_witness": {
                "page_key": self.page_key, "pdf_sha256": self.index.get("pdf_sha256", ""),
                "locator_route": "ocr-running-head-date+fuzzy-title"
                if dossier.get("kind") == "fixed" else "ocr-fuzzy-title",
                "pages": [{
                    key: page[key] for key in (
                        "pdf_page", "printed_page", "inferred", "ocr_route", "ocr_text_sha256",
                    )
                } for page in pages],
            },
        }


def packet_hash(dossier: dict) -> str:
    payload = {key: value for key, value in dossier.items() if key != "packet_sha256"}
    encoded = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(encoded.encode("utf-8")).hexdigest()


def build_prompt(dossier: dict) -> str:
    pages = ", ".join(
        f"printed {page.get('printed_page') or '?'} / PDF {page['pdf_page']} ({page['png']})"
        for page in dossier.get("pages", [])
    )

    def context_summary(item: dict) -> str:
        contexts = item.get("contexts") or [{
            "tier": item["current_tier"], "key": item["current_key"],
        }]
        return ", ".join(f"{context['tier']} `{context['key']}`" for context in contexts)

    requests = "\n".join(
        f"- {item['id']}: hour(s) {', '.join(item['hours'])}; slot {item['slot']}; "
        f"current fallback(s) {context_summary(item)}; "
        f"proposed corpus key `{item['target_key']}`"
        for item in dossier["fallbacks"]
    )
    return f"""You are a literal reader of the attached printed Monastic Diurnal pages for {dossier['name']}.
Inspect all attached page images (at most eight): {pages}

For every requested id below, decide whether these pages print feast-specific proper text for that exact slot:
{requests}

Return one result per requested id. A cross-reference such as "Com. from p. 561" or "all from the Common of Confessors p. 47*" is printed=false, with the exact cross-reference summarized in note. Printed means that the Diurnal prints the actual proper text for this slot on these pages for this feast. Do not treat a common printed elsewhere, a nearby feast, a rubric, or a cross-reference as proper text.

For printed=true, transcribe the text literally, report the printed page, and use confidence high, medium, or low. For printed=false, return text="", printed_page="", and explain the cross-reference or absence in note. Also list in extra every other feast-specific proper section visibly printed for this feast but absent from the request list; extra rows are discovery notes only and will not be applied.

{transcribe.CORPUS_GRAMMAR}

Never infer, reconstruct, modernize, or borrow wording from memory or another edition. Return exactly one JSON object matching the supplied schema with keys slots, extra, and notes."""


def find_discovery_object(value) -> dict | None:
    if isinstance(value, dict):
        if {"slots", "extra", "notes"} <= value.keys():
            return value
        for child in value.values():
            if found := find_discovery_object(child):
                return found
    elif isinstance(value, list):
        for child in reversed(value):
            if found := find_discovery_object(child):
                return found
    elif isinstance(value, str):
        try:
            return find_discovery_object(json.loads(value))
        except json.JSONDecodeError:
            return None
    return None


def parse_discovery_output(output: str) -> dict:
    candidates = []
    try:
        candidates.append(json.loads(output))
    except json.JSONDecodeError:
        for line in output.splitlines():
            try:
                candidates.append(json.loads(line))
            except json.JSONDecodeError:
                pass
    result = next((found for value in reversed(candidates)
                   if (found := find_discovery_object(value))), None)
    if result is None or not isinstance(result["slots"], list) or not isinstance(result["extra"], list):
        raise ProviderError("provider output contained no discovery JSON")
    for slot in result["slots"]:
        required = {"id", "printed", "text", "printed_page", "confidence", "note"}
        if not isinstance(slot, dict) or not required <= slot.keys():
            raise ProviderError("provider returned an invalid discovery slot")
        expected = {"id": str, "printed": bool, "text": str, "printed_page": str, "note": str}
        if any(not isinstance(slot[key], kind) for key, kind in expected.items()):
            raise ProviderError("provider returned invalid discovery slot field types")
        if slot["confidence"] not in {"high", "medium", "low"}:
            raise ProviderError("provider returned invalid discovery confidence")
    # Extra sections are advisory (recorded, never written): normalize rather than fail.
    normalized_extra = []
    for extra in result["extra"]:
        if not isinstance(extra, dict):
            continue
        item = {
            "section": str(extra.get("section") or extra.get("slot") or extra.get("id") or ""),
            "hour": str(extra.get("hour") or ""),
            "text": str(extra.get("text") or ""),
            "printed_page": str(extra.get("printed_page") or ""),
            "confidence": extra.get("confidence") if extra.get("confidence") in {"high", "medium", "low"} else "low",
            "note": str(extra.get("note") or extra.get("notes") or ""),
        }
        if item["section"] or item["text"]:
            normalized_extra.append(item)
    result["extra"] = normalized_extra
    return result


def resolved_corpus_text(key: str, proper_name: str = "", seen: set[str] | None = None) -> str:
    seen = seen or set()
    if key in seen:
        raise RuntimeError(f"corpus @use cycle at {key}")
    seen.add(key)
    text = transcribe.corpus_text(key)
    if match := re.fullmatch(r"@use\s+([^\s]+)", text.strip()):
        text = resolved_corpus_text(match.group(1), proper_name, seen)
    return text.replace("N.", proper_name) if proper_name else text


def gate_decision(primary: dict, fallback_text: str | list[str], secondary: dict | None = None,
                  applying: bool = False, target_key: str = "") -> tuple[str, float]:
    if not primary.get("printed"):
        return "printed-false", 0.0
    if primary.get("confidence") == "low" or not str(primary.get("text", "")).strip():
        return "needs-human", 0.0
    if transcribe.looks_like_incipit(target_key, str(primary["text"])):
        return "incipit-crossref", 0.0
    fallbacks = fallback_text if isinstance(fallback_text, list) else [fallback_text]
    fallback_score = max(
        (transcribe.similarity(str(primary["text"]), text, target_key) for text in fallbacks),
        default=0.0,
    )
    if fallback_score >= 0.9:
        return "same-as-fallback", fallback_score
    if not applying:
        return "printed-proper", fallback_score
    if (secondary is None or not secondary.get("found") or secondary.get("confidence") == "low"
            or not str(secondary.get("text", "")).strip()):
        return "needs-human", fallback_score
    agreement = transcribe.similarity(str(primary["text"]), str(secondary["text"]), target_key)
    return ("put-and-attest" if agreement >= 0.985 else "needs-human"), agreement


def exact_page(pages: list[dict], printed_page: str) -> dict | None:
    canonical = diurnal_pages.canonical_label(printed_page)
    if canonical is None:
        return None
    return next((page for page in pages
                 if diurnal_pages.canonical_label(str(page.get("printed_page", ""))) == canonical), None)


def secondary_prompt(dossier: dict, request: dict, primary: dict, page: dict) -> str:
    description = f"the {request['slot']} at {', '.join(request['hours'])} for {dossier['name']}"
    prompt = transcribe.build_prompt(
        request["target_key"], description, primary["printed_page"], [page], paths_for_read=True,
    )
    return prompt + (
        "\n\nFor this verification, found=true only when the named feast's actual proper text is "
        "printed on this page. A cross-reference or text belonging to a common/nearby feast is "
        "found=false."
    )


class CorpusApplier:
    def __init__(self, run_dir: Path):
        self.options = transcribe.RunOptions(run_dir, apply=True)
        self.scaffolded: set[str] = set()

    def __call__(self, dossier: dict, request: dict, primary: dict, page: dict) -> None:
        feast_id = dossier["feast_id"]
        if feast_id not in self.scaffolded:
            args = ["scaffold", "propers", "-feast", feast_id]
            if dossier.get("rank") == "commemoration":
                args.append("-include-commemorations")
            transcribe.run_office(args)
            self.scaffolded.add(feast_id)
        transcribe.replace_and_attest(
            self.options, request["target_key"], primary["printed_page"], page["png"], primary["text"],
        )


def process_dossier(dossier: dict, runner: ProviderRunner, *, apply: bool = False, provider: str = "codex",
                    corpus_get: Callable[[str, str], str] = resolved_corpus_text,
                    apply_text: Callable[[dict, dict, dict, dict], None] | None = None) -> dict:
    record = {key: dossier[key] for key in (
        "feast_id", "name", "month", "day", "rank", "category", "kind",
        "locate_confidence", "locate_status", "pages", "source_witness", "packet_sha256",
    ) if key in dossier}
    record["extra"] = []
    record["slots"] = []
    if not dossier.get("pages"):
        record["status"] = "no-pages"
        return record
    prompt = build_prompt(dossier)
    try:
        answer, seconds = runner.read_json(
            provider, transcribe.PROVIDER_MODELS.get(provider, DEFAULT_MODEL), prompt, [Path(page["png"]) for page in dossier["pages"]],
            SCHEMA, parse_discovery_output,
        )
    except (OSError, RuntimeError, ProviderError) as exc:
        record.update(status="needs-human", error=str(exc))
        return record
    by_id = {item.get("id"): item for item in answer["slots"]}
    expected_ids = {item["id"] for item in dossier["fallbacks"]}
    if set(by_id) != expected_ids or len(by_id) != len(answer["slots"]):
        record.update(status="needs-human", error="reader did not return each requested id exactly once")
        record["extra"] = answer.get("extra", [])
        return record
    record["extra"] = answer.get("extra", [])
    record["reader_notes"] = answer.get("notes", "")
    record["timing_seconds"] = {"first": round(seconds, 3)}
    for request in dossier["fallbacks"]:
        primary = by_id[request["id"]]
        slot_record = {
            **request, "first": primary, "second": None,
            "first_text_sha256": hashlib.sha256(str(primary.get("text", "")).encode("utf-8")).hexdigest(),
        }
        if not primary.get("printed"):
            slot_record.update(decision="printed-false", score=0.0)
            record["slots"].append(slot_record)
            continue
        try:
            fallback = [
                corpus_get(key, dossier.get("proper_name", ""))
                for key in request.get("current_keys", [request["current_key"]])
            ]
            slot_record["fallback_text_sha256"] = [
                hashlib.sha256(text.encode("utf-8")).hexdigest() for text in fallback
            ]
        except (OSError, RuntimeError) as exc:
            slot_record.update(decision="needs-human", error=f"fallback read failed: {exc}", score=0.0)
            record["slots"].append(slot_record)
            continue
        decision, score = gate_decision(
            primary, fallback, applying=False, target_key=request["target_key"],
        )
        page = exact_page(dossier["pages"], str(primary.get("printed_page", "")))
        if page is None and decision not in {"same-as-fallback", "printed-false", "incipit-crossref"}:
            decision = "needs-human"
            slot_record["error"] = "reader printed page is not one of the located pages"
        secondary = None
        if apply and decision == "printed-proper" and page is not None:
            try:
                secondary, second_seconds = runner.transcribe(
                    "claude", "sonnet", secondary_prompt(dossier, request, primary, page),
                    [Path(page["png"])],
                )
                decision, score = gate_decision(
                    primary, fallback, secondary, applying=True, target_key=request["target_key"],
                )
                slot_record["second_timing_seconds"] = round(second_seconds, 3)
                slot_record["second_text_sha256"] = hashlib.sha256(
                    str(secondary.get("text", "")).encode("utf-8")
                ).hexdigest()
            except (OSError, RuntimeError, ProviderError) as exc:
                decision = "needs-human"
                slot_record["error"] = f"second reader failed: {exc}"
        slot_record.update(decision=decision, score=round(score, 6), second=secondary)
        if apply and decision == "put-and-attest" and apply_text is not None and page is not None:
            try:
                apply_text(dossier, request, primary, page)
            except (OSError, RuntimeError) as exc:
                slot_record.update(decision="needs-human", apply_error=str(exc))
        record["slots"].append(slot_record)
    decisions = Counter(slot["decision"] for slot in record["slots"])
    record["status"] = "needs-human" if decisions["needs-human"] else "processed"
    return record


def write_jsonl(path: Path, record: dict) -> None:
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n")


def default_run_id() -> str:
    return dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def parse_ids(values: Iterable[str]) -> set[str]:
    return {item.strip() for value in values for item in value.split(",") if item.strip()}


def filter_dossiers(dossiers: list[dict], feast_ids: set[str], month: int | None,
                    limit: int | None) -> list[dict]:
    selected = [dossier for dossier in dossiers if not feast_ids or dossier["feast_id"] in feast_ids]
    if feast_ids:
        missing = feast_ids - {dossier["feast_id"] for dossier in selected}
        if missing:
            raise ValueError("feasts absent from selected fallback inventory: " + ", ".join(sorted(missing)))
    if month is not None:
        selected = [dossier for dossier in selected if dossier.get("month") == month or (
            dossier.get("month") is None and any(
                len(item.get("date", "").split("-")) == 3
                and item["date"].split("-")[1] == f"{month:02d}"
                for item in dossier["fallbacks"]
            )
        )]
    return selected[:limit] if limit is not None else selected


def run_command(args: argparse.Namespace) -> int:
    if args.apply and args.dry_run:
        raise ValueError("--apply and --dry-run are mutually exclusive")
    if args.month is not None and not 1 <= args.month <= 12:
        raise ValueError("--month must be 1 through 12")
    if args.limit is not None and args.limit < 1:
        raise ValueError("--limit must be positive")
    run_dir = DISCOVER_ROOT / (args.run_id or default_run_id())
    run_dir.mkdir(parents=True, exist_ok=False)
    dossiers = filter_dossiers(
        build_dossiers(parse_inventory(args.inventory), load_feast_catalog()),
        parse_ids(args.feasts), args.month, args.limit,
    )
    locator = FeastPageLocator(args.page_key)
    runner = ProviderRunner(args.timeout, args.max_output_bytes)
    applier = CorpusApplier(run_dir) if args.apply else None
    dossier_path = run_dir / "dossiers.jsonl"
    prompt_path = run_dir / "prompts.jsonl"
    results_path = run_dir / "results.jsonl"
    for dossier in dossiers:
        dossier.update(locator.locate(dossier))
        dossier["packet_sha256"] = packet_hash(dossier)
        write_jsonl(dossier_path, dossier)
        prompt_record = {
            "feast_id": dossier["feast_id"], "locate_status": dossier["locate_status"],
            "prompt": build_prompt(dossier) if dossier.get("pages") else None,
            "pages": dossier.get("pages", []),
            "source_witness": dossier.get("source_witness", {}),
            "packet_sha256": dossier["packet_sha256"],
        }
        write_jsonl(prompt_path, prompt_record)
        if not args.dry_run:
            write_jsonl(results_path, process_dossier(
                dossier, runner, provider=args.provider, apply=args.apply, apply_text=applier,
            ))
    print(json.dumps({
        "run": str(run_dir), "feasts": len(dossiers),
        "dry_run": args.dry_run, "apply": args.apply,
    }))
    return 0


def resolve_run_path(value: str) -> Path:
    candidate = Path(value)
    return candidate if candidate.is_dir() else DISCOVER_ROOT / value


def render_report(records: list[dict], run_name: str) -> str:
    slots = [slot for record in records for slot in record.get("slots", [])]
    extras = [(record, item) for record in records for item in record.get("extra", [])]
    decisions = Counter(slot.get("decision", "unknown") for slot in slots)
    dossier_needs = [
        record for record in records
        if record.get("status") == "needs-human" and not record.get("slots")
    ]
    lines = [f"# Diurnal discovery report: {run_name}", "", "## Summary", "",
             f"- Feasts processed: {len(records)}",
             f"- Slots added and attested: {decisions['put-and-attest']}",
             f"- Printed false: {decisions['printed-false']}",
             f"- Extra unmodelled sections: {len(extras)}",
             f"- Needs human: {decisions['needs-human'] + len(dossier_needs)}",
             f"- Same as fallback: {decisions['same-as-fallback']}",
             f"- Incipit cross-references: {decisions['incipit-crossref']}"]

    def section(title: str, entries: list[str]) -> None:
        lines.extend(["", f"## {title}", ""])
        lines.extend(entries or ["None."])

    section("Printed false", [
        f"- `{slot['target_key']}` — {slot.get('first', {}).get('note', '')}"
        for slot in slots if slot.get("decision") == "printed-false"
    ])
    section("Extra unmodelled sections", [
        f"- `{record['feast_id']}` / {item.get('hour') or '?'} / {item.get('section') or '?'} — {item.get('note', '')}"
        for record, item in extras
    ])
    needs_entries = [
        f"- [{slot['target_key']}]({slot['representative_url']}) — {slot.get('error', 'reader disagreement or low confidence')}"
        if slot.get("representative_url") else
        f"- `{slot['target_key']}` — {slot.get('error', 'reader disagreement or low confidence')}"
        for slot in slots if slot.get("decision") == "needs-human"
    ]
    needs_entries.extend(
        f"- `{record['feast_id']}` — {record.get('error', 'dossier reader failed')}"
        for record in dossier_needs
    )
    section("Needs human", needs_entries)
    section("Same as fallback", [
        f"- `{slot['target_key']}` — matches "
        f"{', '.join(f'`{key}`' for key in slot.get('current_keys', [slot['current_key']]))} "
        f"({slot.get('score', 0):.3f})"
        for slot in slots if slot.get("decision") == "same-as-fallback"
    ])
    no_pages = [record for record in records if record.get("status") == "no-pages"]
    section("No pages", [f"- `{record['feast_id']}` — {record['name']}" for record in no_pages])
    return "\n".join(lines) + "\n"


def report_command(value: str) -> int:
    run_dir = resolve_run_path(value)
    path = run_dir / "results.jsonl"
    if not path.is_file():
        raise FileNotFoundError(path)
    records = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line]
    print(render_report(records, run_dir.name), end="")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    run = subparsers.add_parser("run", help="build and optionally read discovery dossiers")
    run.add_argument("--inventory", type=Path, help="fixture inventory JSON instead of invoking office")
    run.add_argument("--page-key", default=DEFAULT_PAGE_KEY)
    run.add_argument("--feasts", action="append", default=[], metavar="ID[,ID]")
    run.add_argument("--month", type=int)
    run.add_argument("--limit", type=int)
    run.add_argument("--run-id")
    run.add_argument("--provider", choices=("codex", "claude", "grok", "muse"), default="codex")
    run.add_argument("--dry-run", action="store_true")
    run.add_argument("--apply", action="store_true")
    run.add_argument("--timeout", type=int, default=transcribe.DEFAULT_TIMEOUT)
    run.add_argument("--max-output-bytes", type=int, default=transcribe.MAX_OUTPUT_BYTES)
    report = subparsers.add_parser("report", help="print PR-body markdown for a completed run")
    report.add_argument("run")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return report_command(args.run) if args.command == "report" else run_command(args)
    except (OSError, ValueError, RuntimeError, ProviderError, json.JSONDecodeError) as exc:
        print(f"diurnal-discover: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
