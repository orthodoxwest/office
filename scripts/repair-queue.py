#!/usr/bin/env python3
"""Generate a read-only repair queue from live resolutions, ordo triage and sources.

Reader output is evidence to investigate, never an approval, attestation, or
instruction to write the corpus. Tracked expectations are executable checks
of specific appointments, not whole-office signoffs.
"""

from __future__ import annotations

import argparse
from collections import Counter
import csv
from dataclasses import asdict
import datetime as dt
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]


def import_script(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


STATUS = import_script("status_for_repairs", "project-status.py")
DISCOVERY = import_script("discovery_for_repairs", "diurnal-discover.py")
HOURS = {"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}
STATES = ["ready-to-repair", "needs-diagnosis", "awaiting-clergy",
          "suspected-reference-error", "reference-error", "supported-fallback", "resolved"]
DISPOSITIONS = {"confirmed-defect": "ready-to-repair", "supported-fallback": "supported-fallback",
                "needs-diagnosis": "needs-diagnosis", "awaiting-clergy": "awaiting-clergy"}
NEXT_ACTIONS = {
    "ready-to-repair": "Repair the source-backed defect and verify its appointment checks.",
    "needs-diagnosis": "Read the cited source and establish the expected appointment.",
    "awaiting-clergy": "Obtain the outstanding ruling before changing the appointment.",
    "suspected-reference-error": "Confirm the proposed printed erratum; retain the existing default.",
    "reference-error": "Retain the documented reference-error classification.",
    "supported-fallback": "Retain the source-supported fallback; recheck if its evidence changes.",
    "resolved": "Retain the executable appointment checks to detect recurrence.",
}


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def file_digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def input_snapshot(data, paths):
    files = sorted(p for p in data.rglob("*") if p.is_file())
    return {"data_sha256": digest([(str(p.relative_to(data)), file_digest(p)) for p in files]),
            "files": {str(p): file_digest(p) if p.is_file() else "" for p in sorted(set(paths))}}


def protect_inputs(output, paths):
    artifacts = {output / name for name in ["resolution.json", "ordo.txt", "rubrics.tsv", "reference.txt",
                                           "repair-queue.csv", "repair-queue.json", "repair-queue.md"]}
    if {p.resolve() for p in artifacts} & {p.resolve() for p in paths}:
        raise ValueError("output artifacts would overwrite an input; choose another output directory")


def run(command, cwd=ROOT):
    result = subprocess.run([str(x) for x in command], cwd=cwd, text=True, capture_output=True)
    if result.returncode:
        raise ValueError(f"{command[0]} failed: {result.stderr.strip() or result.stdout.strip()}")
    return result.stdout


def scope_for(dates, triduum):
    return "triduum" if set(dates) & set(triduum) else "ordinary-year"


def triduum_dates(ordo_text, year):
    match = re.search(r"^\s*Easter \(Pascha\) Day\s+([A-Za-z]+)\s+(\d{1,2})\s*$", ordo_text, re.M)
    if not match or match[1] not in STATUS.ORDO_COMPARE.MONTH_NAMES:
        raise ValueError("cannot identify Easter in the generated Tabula for Triduum scope")
    easter = dt.date(year, STATUS.ORDO_COMPARE.MONTH_NAMES[match[1]], int(match[2]))
    return {(easter - dt.timedelta(days=n)).isoformat() for n in [1, 2, 3]}


def finding_state(finding):
    category, confidence = finding.category, finding.confidence
    if category == "open-question":
        return "awaiting-clergy"
    if category == "suspected-reference-error":
        return category
    if category == "reference-error":
        return category if confidence == "confirmed" else "suspected-reference-error"
    if category in {"data-gap", "engine-bug", "translation-mismatch"} and confidence == "confirmed":
        return "ready-to-repair"
    return "needs-diagnosis"


def combined_state(states):
    # A grouped target cannot hide an unresolved part behind a resolved one.
    for state in ["awaiting-clergy", "needs-diagnosis", "suspected-reference-error",
                  "ready-to-repair", "reference-error"]:
        if state in states:
            return state
    return "needs-diagnosis"


def source_file_hash(data, ref):
    # Include corpus content changes in a decision's observation binding.
    parts = ref.split("/")
    if len(parts) < 2 or any(p in {".", ".."} for p in parts):
        return ""
    path = data / "texts" / parts[0] / (parts[1] + ".txt")
    return file_digest(path) if path.is_file() else ""


def resolution_observations(inventory, data, triduum):
    """Stable owner/hour/slot IDs; a source change changes evidence, not identity."""
    observations = {}
    for row in inventory["rows"]:
        if not row.get("dates") or not row.get("context_id"):
            raise ValueError("resolution inventory lacks dated appointment contexts; rebuild the office binary")
        for scope in ["ordinary-year", "triduum"]:
            dates = sorted(d for d in row["dates"] if scope_for([d], triduum) == scope)
            if not dates:
                continue
            slot = row["requested_slot"]
            first = bool(row.get("first_vespers"))
            key = f"resolution:{inventory['start_year']}:{row['owner_id']}:{row['hour']}:{'1v' if first else 'day'}:{slot}:{scope}:{row['context_id']}"
            observation = observations.setdefault(key, {
                "id": key, "kind": "fallback", "scope": scope, "owner_id": row["owner_id"],
                "hour": row["hour"], "slot": slot, "first_vespers": first,
                "part": row.get("part", "principal"), "appointment_context": row["context_id"],
                "resolver_hour": row["resolver_hour"], "resolver_slot": row["resolver_slot"],
                "title": f"{row['owner_id']} / {row['hour']} / {slot}",
                "candidate": False, "contexts": [], "discovery": [],
            })
            observation["candidate"] |= (row["selected_tier"] == "not-found" or DISCOVERY.slot_is_printed(row))
            observation["contexts"].append({
                "dates": dates, "selected_ref": row["selected_ref"], "selected_tier": row["selected_tier"],
                "reason": row["reason"], "canonical_owner": row.get("canonical_owner", ""),
                "proper_ids": row.get("proper_ids", []),
                "source_hash": source_file_hash(data, row["selected_ref"]),
            })
    for observation in observations.values():
        observation["contexts"].sort(key=lambda c: json.dumps(c, sort_keys=True))
        observation["dates"] = sorted({d for c in observation["contexts"] for d in c["dates"]})
    return observations


def validate_sources(sources, resources):
    evidence = []
    for source in sources:
        path = (resources / source["path"]).resolve()
        if not path.is_relative_to(resources.resolve()):
            raise ValueError("source paths must stay within the external resources directory")
        actual = file_digest(path) if path.is_file() else ""
        evidence.append({**source, "current_sha256": actual,
                         "state": "current" if actual == source["sha256"] else "stale-or-unavailable"})
    return evidence


def load_ledger(path):
    ledger = json.loads(path.read_text())
    if ledger.get("version") != 1 or not isinstance(ledger.get("targets"), list):
        raise ValueError("repair ledger must have version 1 and a targets list")
    ids, bindings = set(), set()
    allowed = {"id", "year", "title", "kind", "scope", "evidence", "sources", "next_action",
               "issue", "finding_ids", "resolution_ids", "disposition", "observation_hash",
               "context_hash", "checks"}
    for target in ledger["targets"]:
        unknown = set(target) - allowed
        if unknown:
            raise ValueError(f"unknown repair-target fields: {sorted(unknown)}")
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", target.get("id", "")) or target["id"] in ids:
            raise ValueError("target IDs must be unique lowercase names")
        ids.add(target["id"])
        for field in ["title", "kind", "evidence", "next_action"]:
            if not target.get(field):
                raise ValueError(f"{target['id']}: {field} is required")
        if not isinstance(target.get("year"), int) or not 1900 <= target["year"] <= 2199:
            raise ValueError(f"{target['id']}: year must be 1900–2199")
        if target.get("scope") not in {"ordinary-year", "triduum"}:
            raise ValueError(f"{target['id']}: explicit ordinary-year or triduum scope required")
        if target.get("finding_ids") and "disposition" in target:
            raise ValueError(f"{target['id']}: ordo classifications come from ordo-triage.csv")
        if not target.get("finding_ids") and target.get("disposition") not in DISPOSITIONS:
            raise ValueError(f"{target['id']}: invalid or missing disposition")
        for key in target.get("finding_ids", []) + target.get("resolution_ids", []):
            if key in bindings:
                raise ValueError(f"observation bound more than once: {key}")
            bindings.add(key)
        if not target.get("sources"):
            raise ValueError(f"{target['id']}: source citations are required")
        for source in target["sources"]:
            if set(source) != {"path", "sha256", "locator"} or not source["locator"] or not re.fullmatch(r"[a-f0-9]{64}", source["sha256"]):
                raise ValueError(f"{target['id']}: sources need path, SHA-256 and printed locator")
        for check in target.get("checks", []):
            check_fields = {"date", "hour", "form", "slot", "owner_id", "first_vespers",
                            "expected_ref", "absent", "rule", "outcome", "unit_key", "resolution_id"}
            if set(check) - check_fields:
                raise ValueError(f"{target['id']}: unknown check fields")
            kinds = sum(["expected_ref" in check or "absent" in check, "rule" in check, "unit_key" in check])
            if kinds != 1:
                raise ValueError(f"{target['id']}: check needs exactly one source, decision, or office assertion")
            for field in ["expected_ref", "unit_key", "rule", "outcome"]:
                if field in check and (not isinstance(check[field], str) or not check[field].strip()):
                    raise ValueError(f"{target['id']}: {field} must be a nonempty string")
            if "first_vespers" in check and not isinstance(check["first_vespers"], bool):
                raise ValueError(f"{target['id']}: first_vespers must be boolean")
            if "resolution_id" in check:
                if check["resolution_id"] not in target.get("resolution_ids", []) or "date" in check or "hour" in check:
                    raise ValueError(f"{target['id']}: resolution check must bind a listed resolution ID")
            else:
                date = dt.date.fromisoformat(check.get("date", ""))
                if date.year != target["year"] or check.get("hour") not in HOURS:
                    raise ValueError(f"{target['id']}: check date/hour outside target scope")
            if check.get("form", "private") not in {"private", "deacon", "priest"}:
                raise ValueError(f"{target['id']}: invalid prayer form")
            if ("expected_ref" in check or "absent" in check) and "resolution_id" not in check and not check.get("slot"):
                raise ValueError(f"{target['id']}: source assertions require a slot")
            if ("expected_ref" in check or "absent" in check) and "resolution_id" not in check:
                if not check.get("owner_id"):
                    raise ValueError(f"{target['id']}: source assertions require an explicit owner_id")
                if check["hour"] == "vespers" and not isinstance(check.get("first_vespers"), bool):
                    raise ValueError(f"{target['id']}: Vespers source assertions require explicit first_vespers")
            if "absent" in check and (check["absent"] is not True or "expected_ref" in check):
                raise ValueError(f"{target['id']}: absent must be true and cannot name an expected ref")
            if check.get("absent"):
                if "resolution_id" in check or not any(
                    "unit_key" in guard and all(guard.get(k, "private" if k == "form" else None) == check.get(k, "private" if k == "form" else None)
                                                for k in ["date", "hour", "form"])
                    for guard in target["checks"]
                ):
                    raise ValueError(f"{target['id']}: an absence check needs an explicit date and a companion unit_key check")
            if "rule" in check and not check.get("outcome"):
                raise ValueError(f"{target['id']}: decision checks need an outcome")
    return ledger["targets"]


def evaluate_check(check, explain):
    hour = explain(check["hour"], check["date"], check.get("form", "private"))
    if "unit_key" in check:
        actual = hour["unit_key"]
        passed = actual == check["unit_key"]
    elif "rule" in check:
        actual = sorted({d["outcome"] for d in hour["decisions"] if d["rule"] == check["rule"]})
        passed = actual == [check["outcome"]]
    else:
        selected = [r for r in hour["resolutions"]
                    if r["requested_slot"] == check["slot"]
                    and r.get("part", "principal") == "principal"
                    and ("owner_id" not in check or r.get("owner_id") == check["owner_id"])
                    and ("first_vespers" not in check or bool(r.get("first_vespers")) == check["first_vespers"])]
        actual = sorted({r["selected_ref"] for r in selected})
        passed = (bool(hour["resolutions"]) and not actual) if check.get("absent") else actual == [check["expected_ref"]]
    return {**check, "actual": actual, "passed": passed}


def expand_checks(target, observations):
    checks, missing = [], []
    for check in target.get("checks", []):
        if "resolution_id" not in check:
            checks.append(check)
            continue
        observation = observations.get(check["resolution_id"])
        if observation is None:
            missing.append(check["resolution_id"])
            continue
        for date in observation["dates"]:
            checks.append({**check, "date": date, "hour": observation["hour"],
                           "slot": observation["slot"], "owner_id": observation["owner_id"],
                           "first_vespers": observation["first_vespers"]})
    return checks, missing


def target_item(target, observations, findings, rules, resources, explain):
    bound = [observations[key] for key in target.get("resolution_ids", []) if key in observations]
    raw = [findings[key] for key in target.get("finding_ids", []) if key in findings]
    classified = []
    for key in target.get("finding_ids", []):
        match = re.fullmatch(r"(\d{4}):([a-z-]+):(\d{2}-\d{2})", key)
        if not match or int(match[1]) != target["year"]:
            raise ValueError(f"invalid finding ID for {target['id']}: {key}")
        finding = STATUS.Finding(int(match[1]), match[2], match[3], "")
        STATUS.apply_triage([finding], rules)
        classified.append(finding)
    state = combined_state([finding_state(f) for f in classified]) if classified else DISPOSITIONS[target["disposition"]]
    checks, missing = expand_checks(target, observations)
    results = [evaluate_check(c, explain) for c in checks]
    evidence = validate_sources(target["sources"], resources)
    # Context identity excludes selected text so a proper repair can satisfy
    # the checks, but an altered calendar population cannot silently close it.
    contexts = [{k: o[k] for k in ["id", "owner_id", "hour", "slot", "first_vespers", "dates"]} for o in bound]
    context_hash = digest(sorted(contexts, key=lambda x: x["id"]))
    observation_hash = digest({"resolutions": [{k: v for k, v in o.items() if k != "discovery"} for o in bound],
                               "findings": [(f.finding_id, f.detail) for f in raw],
                               "checks": results})
    stale = []
    if any(s["state"] != "current" for s in evidence):
        stale.append("Source changed or unavailable; review the citation again.")
    if len(bound) != len(target.get("resolution_ids", [])) or missing:
        stale.append("A bound resolution disappeared; absence is not proof of repair.")
    if bound and target.get("context_hash") != context_hash:
        stale.append("Calendar/slot context is unbound or changed; review affected dates.")
    if any(o["part"] != "principal" for o in bound):
        stale.append("The appended office requires an independent owner mapping before these checks can certify it.")
    all_pass = bool(results) and all(r["passed"] for r in results) and not missing
    if state in {"ready-to-repair", "supported-fallback"}:
        covered = {c.get("resolution_id") for c in checks}
        if set(target.get("resolution_ids", [])) - covered:
            stale.append("Every bound resolution needs a check covering its live dates.")
        if not results:
            stale.append("Add executable appointment checks before declaring a repair ready.")
        elif all_pass:
            if state == "ready-to-repair" and not raw:
                state = "resolved"
        elif target.get("observation_hash") != observation_hash:
            stale.append("Observed behavior is unbound or changed; diagnose before proceeding.")
        if state == "supported-fallback" and not all_pass:
            stale.append("The supported-fallback assertion no longer passes.")
    if stale:
        state = "needs-diagnosis"
    dates = sorted({c["date"] for c in checks} | {d for o in bound for d in o["dates"]}
                   | {f"{f.year}-{f.date}" for f in raw})
    return {
        "id": "target:" + target["id"], "title": target["title"], "kind": target["kind"],
        "scope": target["scope"], "status": state, "next_action": " ".join(stale) or target["next_action"],
        "evidence": target["evidence"], "sources": evidence,
        "finding_ids": target.get("finding_ids", []), "current_finding_ids": [f.finding_id for f in raw],
        "resolution_ids": target.get("resolution_ids", []), "observations": bound,
        "dates": dates, "affected_dates": len(dates), "checks": results, "stale": stale,
        "issues": sorted({f.issue for f in classified if f.issue} | ({str(target["issue"])} if target.get("issue") else set())),
        "context_hash": context_hash, "observation_hash": observation_hash,
    }


def discovery_evidence(path, observations, resources):
    """Attach only metadata; reader conclusions never change queue dispositions."""
    extras = []
    counts = {"records": 0, "attached_slots": 0, "unmatched_slots": 0, "extra_sections": 0, "slotless_records": 0}
    if path is None:
        return extras, counts
    artifact_hash = file_digest(path)
    book = resources / "books" / "Monastic Diurnal.pdf"
    book_hash = file_digest(book) if book.is_file() else ""
    for line_number, line in enumerate(path.read_text().splitlines(), 1):
        if not line.strip():
            continue
        record = json.loads(line)
        if not isinstance(record, dict) or not isinstance(record.get("feast_id"), str) or not record["feast_id"]:
            raise ValueError(f"discovery line {line_number}: expected a feast result record")
        if not isinstance(record.get("slots", []), list) or not isinstance(record.get("extra", []), list):
            raise ValueError(f"discovery line {line_number}: slots and extra must be lists")
        counts["records"] += 1
        witness = record.get("source_witness", {}).get("pdf_sha256", "")
        if not record.get("slots") and not record.get("extra"):
            counts["slotless_records"] += 1
            extras.append({
                "id": "discovery-incomplete:" + record["feast_id"], "kind": "source-discovery", "status": "needs-diagnosis",
                "title": record["feast_id"] + " / incomplete discovery",
                "scope": "triduum" if record["feast_id"] in {"holy-thursday", "good-friday", "holy-saturday"} else "ordinary-year",
                "next_action": "Locate and inspect the printed appointment; this discovery record supplied no slot evidence.",
                "artifact": str(path), "artifact_sha256": artifact_hash, "line": line_number,
                "source_state": "unreviewed" if witness and witness == book_hash else "stale-or-unavailable",
                "dates": [], "affected_dates": 0, "finding_ids": [], "issues": [],
            })
        for slot in record.get("slots", []):
            if not isinstance(slot, dict) or not slot.get("target_key"):
                raise ValueError(f"discovery line {line_number}: slot lacks a target_key")
            attached = False
            for observation in observations.values():
                if observation["owner_id"] != record.get("feast_id") or observation["slot"] != slot.get("slot"):
                    continue
                expected_key = f"proper/{observation['owner_id']}/{DISCOVERY.target_section({**observation, 'slot_ref': observation['slot']})}"
                if slot["target_key"] != expected_key or observation["part"] != "principal":
                    continue
                matching = [c for c in slot.get("contexts", [])
                            if c.get("hour") == observation["hour"] and c.get("date") in observation["dates"]]
                if not matching:
                    continue
                live_selections = {(date, c["selected_ref"]) for c in observation["contexts"] for date in c["dates"]}
                state = "unreviewed"
                if not witness or witness != book_hash or any((c["date"], c.get("key")) not in live_selections for c in matching):
                    state = "stale-or-unavailable"
                observation["discovery"].append({
                    "artifact": str(path), "artifact_sha256": artifact_hash, "line": line_number,
                    "target_key": slot.get("target_key", ""), "decision": slot.get("decision", "unknown"),
                    "printed_page": slot.get("first", {}).get("printed_page", ""),
                    "state": state,
                })
                attached = True
                counts["attached_slots"] += 1
            if not attached:
                counts["unmatched_slots"] += 1
                key = digest([record["feast_id"], slot["target_key"]])[:16]
                extras.append({
                    "id": "discovery-unmatched:" + key, "kind": "source-discovery", "status": "needs-diagnosis",
                    "title": slot["target_key"], "scope": "triduum" if record["feast_id"] in {"holy-thursday", "good-friday", "holy-saturday"} else "ordinary-year",
                    "next_action": "Reconcile the discovery slot with the current owner, hour and I/II Vespers context; no live matching resolution was found.",
                    "artifact": str(path), "artifact_sha256": artifact_hash, "line": line_number,
                    "dates": [], "affected_dates": 0, "finding_ids": [], "issues": [],
                })
        for extra in record.get("extra", []):
            if not isinstance(extra, dict) or not extra.get("section"):
                raise ValueError(f"discovery line {line_number}: extra section lacks a name")
            counts["extra_sections"] += 1
            # The engine may never emit this section: retain it independently.
            key = digest([record.get("feast_id"), extra.get("hour"), extra.get("section")])[:16]
            extras.append({
                "id": "discovery-extra:" + key, "kind": "source-discovery", "status": "needs-diagnosis",
                "title": f"{record.get('feast_id')} / {extra.get('hour', '?')} / {extra.get('section', '?')}",
                "scope": "triduum" if record.get("feast_id") in {"holy-thursday", "good-friday", "holy-saturday"} else "ordinary-year",
                "next_action": "Read the cited discovery pages; establish whether this unmodelled section is appointed.",
                "artifact": str(path), "artifact_sha256": artifact_hash, "line": line_number,
                "source_state": "unreviewed" if witness and witness == book_hash else "stale-or-unavailable",
                "dates": [], "affected_dates": 0, "finding_ids": [], "issues": [],
            })
    unique = {}
    for extra in extras:
        item = unique.setdefault(extra["id"], {**extra, "evidence_lines": []})
        item["evidence_lines"].append(extra["line"])
    return list(unique.values()), counts


def build_queue(year, inventory, comparison, rules, targets, data, resources, explain, triduum, discovery=None, catalog=None):
    observations = resolution_observations(inventory, data, triduum)
    extras, discovery_counts = discovery_evidence(discovery, observations, resources)
    if catalog is None:
        catalog = DISCOVERY.load_feast_catalog(data)
    findings = {f.finding_id: f for f in comparison.findings}
    if len(findings) != len(comparison.findings):
        raise ValueError("duplicate ordo finding IDs")
    entries, used_findings, used_resolutions = [], set(), set()
    for target in targets:
        if target["year"] != year:
            continue
        item = target_item(target, observations, findings, rules, resources, explain)
        scopes = {scope_for([date], triduum) for date in item["dates"]}
        if scopes and scopes != {target["scope"]}:
            raise ValueError(f"{target['id']}: target scope disagrees with its dates; split ordinary-year and Triduum work")
        entries.append(item)
        used_findings.update(target.get("finding_ids", []))
        used_resolutions.update(target.get("resolution_ids", []))
    for key, observation in observations.items():
        if not observation["candidate"] or key in used_resolutions:
            continue
        eligible = observation["owner_id"] in catalog and observation["part"] == "principal"
        entries.append({
            **observation, "kind": "fallback" if eligible else "source-mapping", "status": "needs-diagnosis",
            "next_action": "Check the printed appointment using the discovery workflow." if eligible else "Establish the source and owner mapping; this context is outside automatic proper discovery.",
            "observations": [observation], "resolution_ids": [key], "finding_ids": [],
            "affected_dates": len(observation["dates"]), "issues": [],
        })
    for key, finding in findings.items():
        if key in used_findings:
            continue
        state, date = finding_state(finding), f"{year}-{finding.date}"
        entries.append({
            "id": "ordo:" + key, "kind": "ordo", "scope": scope_for([date], triduum),
            "status": state, "title": f"{date} / {finding.aspect}",
            "next_action": NEXT_ACTIONS[state], "evidence": finding.note,
            "finding_ids": [key], "dates": [date], "affected_dates": 1,
            "issues": [finding.issue] if finding.issue else [], "finding": asdict(finding),
        })
    entries.extend(extras)
    entries.sort(key=lambda e: (STATES.index(e["status"]), -e["affected_dates"], e["id"]))
    return {
        "version": 1, "year": year, "calendar": "default", "entries": entries,
        "summary": {scope: dict(Counter(e["status"] for e in entries if e["scope"] == scope))
                    for scope in ["ordinary-year", "triduum"]},
        "ordo": {"compared": comparison.total, "differences": comparison.mismatches,
                 "categories": dict(Counter(f.category for f in comparison.findings))},
        "intake": {"dynamic_resolution_groups": len(observations),
                   "fallback_candidates": sum(o["candidate"] for o in observations.values()),
                   "discovery_supplied": discovery is not None,
                   "discovery": discovery_counts,
                   "source_mapping_candidates": sum(e["kind"] == "source-mapping" for e in entries),
                   "out_of_year_targets": [t["id"] for t in targets if t["year"] != year]},
    }


def csv_report(entries):
    output = io.StringIO()
    fields = ["id", "kind", "scope", "status", "title", "observed", "expected", "source_citations", "review_url", "next_action", "affected_dates",
              "dates", "finding_ids", "resolution_ids", "issues", "evidence", "context_hash", "observation_hash"]
    writer = csv.DictWriter(output, fieldnames=fields)
    writer.writeheader()
    for entry in entries:
        selected = sorted({c["selected_ref"] for o in entry.get("observations", []) for c in o["contexts"]})
        expected = [f"{c['date']} {c['hour']}: " + (c.get("expected_ref") or ("absent " + c["slot"] if c.get("absent") else c.get("unit_key") or f"{c.get('rule')}={c.get('outcome')}")) for c in entry.get("checks", [])]
        citations = [f"{s['path']} ({s['locator']}; {s['state']})" for s in entry.get("sources", [])]
        hour = entry.get("hour") or next((c["hour"] for c in entry.get("checks", [])), "")
        if not hour and entry.get("finding"):
            aspect = entry["finding"]["aspect"]
            hour = "prime" if aspect == "hours-preces" else "lauds" if aspect.startswith(("lauds", "benedictus")) else "vespers" if aspect != "calendar" else ""
        date = next(iter(entry["dates"]), "")
        representative = next(iter(entry.get("checks", [])), {})
        if representative:
            date, hour = representative["date"], representative["hour"]
        form = representative.get("form", "private")
        values = {**entry, "observed": entry.get("finding", {}).get("detail") or ";".join(selected),
                  "expected": ";".join(expected), "source_citations": ";".join(citations),
                  "review_url": f"https://office.fly.dev/{hour}/{date}?form={form}" if hour and date else ""}
        writer.writerow({key: ";".join(values[key]) if isinstance(values.get(key), list) else values.get(key, "") for key in fields})
    return output.getvalue()


def markdown_report(report, summary_only=False):
    lines = [f"# Repair queue — {report['year']}", "",
             "Counts are concrete targets and diagnosis candidates, not a whole-office correctness score.", "",
             "| State | Ordinary year | Triduum |", "|---|---:|---:|"]
    for state in STATES:
        lines.append(f"| {state} | {report['summary']['ordinary-year'].get(state, 0)} | {report['summary']['triduum'].get(state, 0)} |")
    lines += ["", f"Ordo: {report['ordo']['differences']} differences / {report['ordo']['compared']} comparable assertions.",
              "Linked findings retain their exact IDs; grouping targets does not change strict parity.",
              "", "Fallback candidates reuse the discovery slot filter, plus unresolved texts. "
              "Direct/inherited propers and legitimate weekday/shared selections are not automatically repair targets. "
              "Source requirements can also describe slots absent from the engine. "
              "Discovery reader outcomes require review and do not authorize corpus changes."]
    if not report["intake"]["discovery_supplied"]:
        lines += ["", "No discovery report supplied; unmodelled printed sections have not been inventoried by this run."]
    else:
        counts = report["intake"]["discovery"]
        lines += ["", f"Discovery: {counts['records']} record(s), {counts['attached_slots']} attached slot context(s), "
                  f"{counts['unmatched_slots']} unmatched slot(s), {counts['extra_sections']} extra section(s). "
                  "This is metadata from the supplied report, not complete discovery coverage."]
        if counts["slotless_records"]:
            lines.append(f"{counts['slotless_records']} record(s) supplied no slot evidence and remain incomplete discovery targets.")
        if not sum(counts.values()):
            lines.append("The supplied discovery file contains no usable records.")
    lines += ["", f"{report['intake']['source_mapping_candidates']} candidate(s) need source/owner mapping before automatic discovery is applicable."]
    if not summary_only:
        for state in STATES:
            entries = [e for e in report["entries"] if e["status"] == state]
            if not entries:
                continue
            lines += ["", f"## {state}", ""]
            for entry in entries:
                lines.append(f"- `{entry['id']}` — {entry['title']} ({entry['affected_dates']} affected date(s)). {entry['next_action']}")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--year", type=int, default=dt.date.today().year)
    parser.add_argument("--office", type=Path, default=ROOT / "office")
    parser.add_argument("--resources", type=Path, default=ROOT.parent / "resources")
    parser.add_argument("--ledger", type=Path, default=ROOT / "data/review/repair-targets.json")
    parser.add_argument("--discovery", type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--scope", choices=["all", "ordinary-year", "triduum"], default="all")
    parser.add_argument("--format", choices=["csv", "json", "markdown", "summary"], default="csv")
    args = parser.parse_args()
    if not 1900 <= args.year <= 2199:
        parser.error("year must be 1900–2199")
    output = (args.output or ROOT / "output/repair-queue" / str(args.year)).resolve()
    if not output.is_relative_to((ROOT / "output").resolve()):
        parser.error("generated repair artifacts must stay beneath this checkout's ignored output/")
    output.mkdir(parents=True, exist_ok=True)
    args.office = args.office.resolve()
    args.resources = args.resources.resolve()
    pdf = args.resources / f"{args.year}-ordo.pdf"
    if not pdf.is_file():
        parser.error(f"current comparison source unavailable: {pdf}")
    data = ROOT / "data"
    targets = load_ledger(args.ledger)
    inputs = [args.office, pdf, args.ledger, data / "review/ordo-triage.csv"]
    inputs += [(args.resources / source["path"]).resolve() for target in targets for source in target["sources"]]
    inputs += [ROOT / "scripts" / name for name in ["repair-queue.py", "project-status.py", "ordo-compare.py", "diurnal-discover.py"]]
    if args.discovery:
        inputs += [args.discovery.resolve(), args.resources / "books/Monastic Diurnal.pdf"]
    protect_inputs(output, inputs)
    snapshot = input_snapshot(data, inputs)
    # Capture fresh inputs with the same binary; never ingest a saved status CSV.
    inventory = json.loads(run([args.office, "review", "resolution-inventory", "-start", args.year, "-years", "1", "-json"]))
    (output / "resolution.json").write_text(json.dumps(inventory, indent=2) + "\n")
    ordo, rubrics, reference = (output / name for name in ["ordo.txt", "rubrics.tsv", "reference.txt"])
    ordo.write_text(run([args.office, "ordo", args.year]))
    rubrics.write_text(run([args.office, "rubrics", args.year]))
    run(["pdftotext", "-layout", pdf, reference])
    comparison = STATUS.compare_ordo(args.year, reference, ordo, rubrics)
    rules = STATUS.load_triage(data / "review/ordo-triage.csv")
    STATUS.apply_triage(comparison.findings, rules)
    triduum = triduum_dates(ordo.read_text(), args.year)
    cache = {}

    def explain(hour, date, form):
        key = (hour, date, form)
        if key not in cache:
            cache[key] = json.loads(run([args.office, "review", "explain", hour, date, "--form", form]))
        return cache[key]

    report = build_queue(args.year, inventory, comparison, rules, targets,
                         data, args.resources, explain, triduum, args.discovery)
    if input_snapshot(data, inputs) != snapshot:
        raise ValueError("inputs changed during the sweep; rerun before using this report")
    report["inputs"] = {"binary_sha256": file_digest(args.office), "ordo_pdf_sha256": file_digest(pdf),
                        "data_sha256": snapshot["data_sha256"], "input_snapshot_sha256": digest(snapshot),
                        "triage_sha256": file_digest(data / "review/ordo-triage.csv"),
                        "ledger_sha256": file_digest(args.ledger), "ordo_sha256": file_digest(ordo),
                        "rubrics_sha256": file_digest(rubrics), "resolution_sha256": file_digest(output / "resolution.json")}
    if args.scope != "all":
        report["entries"] = [e for e in report["entries"] if e["scope"] == args.scope]
    report["view_scope"] = args.scope
    csv_text, markdown = csv_report(report["entries"]), markdown_report(report)
    json_text = json.dumps(report, indent=2) + "\n"
    (output / "repair-queue.csv").write_text(csv_text)
    (output / "repair-queue.json").write_text(json_text)
    (output / "repair-queue.md").write_text(markdown)
    print({"csv": csv_text, "json": json_text, "markdown": markdown,
           "summary": markdown_report(report, summary_only=True)}[args.format], end="")
    print(f"Repair artifacts: {output}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (ValueError, OSError, KeyError, TypeError) as error:
        print(f"repair-queue: {error}", file=sys.stderr)
        raise SystemExit(1)
