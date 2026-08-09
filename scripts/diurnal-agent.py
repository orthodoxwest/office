#!/usr/bin/env python3
"""Plan, run, and collect read-only diurnal review agents.

All durable state is written below output/. Provider results are advisory:
this command never edits the text corpus, decisions, attestations, or review
ledgers.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
import uuid
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).parent))

from diurnal_agent import model, providers, store


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT = ROOT / "output" / "source-reconcile" / "agent"
DEFAULT_POLICY = ROOT / "scripts" / "diurnal-agent-policy.json"
SCHEMA_FILE = ROOT / "scripts" / "diurnal_agent" / "result.schema.json"
PROMPT_FILE = ROOT / "scripts" / "diurnal_agent" / "reconcile.md"
AMBIGUOUS_CLASSES = {"ambiguous-owner/context", "ambiguous", "rubrical-complex"}


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def git_head(repo: Path) -> str:
    result = subprocess.run(
        ["git", "-C", str(repo), "rev-parse", "HEAD"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        check=False,
    )
    return result.stdout.strip() if result.returncode == 0 else "unknown"


def git_worktree_digest(repo: Path) -> str:
    """Hash all tracked and unignored untracked files without persisting names."""
    result = subprocess.run(
        ["git", "-C", str(repo), "ls-files", "-co", "--exclude-standard", "-z"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if result.returncode != 0:
        return "unknown"
    digest = hashlib.sha256()
    for encoded in sorted(item for item in result.stdout.split(b"\0") if item):
        relative = Path(os.fsdecode(encoded))
        path = (repo / relative).resolve()
        try:
            path.relative_to(repo.resolve())
        except ValueError:
            return "unknown"
        if not path.is_file():
            continue
        digest.update(encoded)
        digest.update(b"\0")
        digest.update(hashlib.sha256(path.read_bytes()).digest())
    return digest.hexdigest()


def load_policy(path: Path) -> dict[str, Any]:
    policy = store.read_json(path)
    required = {"schema_version", "max_attempts", "providers", "routing"}
    if not isinstance(policy, dict) or not required <= set(policy):
        raise ValueError(f"{path}: invalid agent policy")
    if not 1 <= policy["max_attempts"] <= 5:
        raise ValueError("max_attempts must be between one and five")
    numeric = {
        "lease_seconds": (60, 86400),
        "wall_timeout_seconds": (1, 3600),
        "max_prompt_bytes": (1024, 1048576),
        "max_output_bytes": (1024, 1048576),
    }
    for name, (minimum, maximum) in numeric.items():
        value = policy.get(name)
        if isinstance(value, bool) or not isinstance(value, int) or not minimum <= value <= maximum:
            raise ValueError(f"{name} must be an integer from {minimum} to {maximum}")
    if policy["lease_seconds"] < policy["wall_timeout_seconds"] + 30:
        raise ValueError("lease_seconds must exceed wall_timeout_seconds by at least 30 seconds")
    known = {"codex", "grok", "claude"}
    if set(policy["providers"]) - known:
        raise ValueError("policy contains an unknown provider")
    for name, settings in policy["providers"].items():
        if not isinstance(settings, dict) or not isinstance(settings.get("enabled"), bool):
            raise ValueError(f"{name}: enabled must be boolean")
        if not isinstance(settings.get("model"), str) or not settings["model"].strip():
            raise ValueError(f"{name}: model must be a non-empty string")
        if name == "codex" and settings.get("reasoning_effort") not in {
            "low", "medium", "high", "xhigh", "max", "ultra"
        }:
            raise ValueError("codex: reasoning_effort must be a supported level")
    for name, provider in policy["routing"].items():
        if name not in {"primary", "ambiguous_replica", "adjudicator"} or provider not in known:
            raise ValueError("policy routing contains an unknown role or provider")
    return policy


def load_candidates(path: Path) -> list[dict[str, Any]]:
    payload = store.read_json(path)
    if isinstance(payload, dict):
        payload = payload.get("candidates", payload.get("findings", []))
    if not isinstance(payload, list) or any(not isinstance(item, dict) for item in payload):
        raise ValueError(f"{path}: expected a candidates list")
    return payload


def candidate_identifier(candidate: dict[str, Any]) -> str:
    return str(candidate.get("candidate_id", candidate.get("id", f"DA-{model.digest(candidate)[:16]}")))


def validate_witness(candidate: dict[str, Any]) -> None:
    """Require the immutable printed-source identity needed for safe review."""
    for name in ("source", "extractor"):
        if not isinstance(candidate.get(name), str) or not candidate[name].strip():
            raise ValueError(f"candidate witness requires {name}")
    for name in ("source_sha256", "raw_text_sha256"):
        value = candidate.get(name)
        if not isinstance(value, str) or not re.fullmatch(r"[0-9a-fA-F]{64}", value):
            raise ValueError(f"candidate witness requires a full {name}")
    page = candidate.get("source_page")
    if isinstance(page, bool) or not isinstance(page, int) or page < 1:
        raise ValueError("candidate witness requires a positive source_page")


def prompt_for(candidate: dict[str, Any]) -> str:
    instructions = PROMPT_FILE.read_text(encoding="utf-8")
    return f"{instructions.rstrip()}\n\nCandidate packet:\n{json.dumps(candidate, indent=2, sort_keys=True, ensure_ascii=False)}\n"


def declared_dependencies(input_path: Path) -> dict[str, str]:
    payload = store.read_json(input_path)
    declared = payload.get("dependencies", {}) if isinstance(payload, dict) else {}
    if not isinstance(declared, dict):
        raise ValueError("input dependencies must be an object")
    hashes = {}
    for name, record in declared.items():
        if not isinstance(name, str) or not isinstance(record, dict):
            raise ValueError("invalid input dependency record")
        path_value = record.get("path")
        if not isinstance(path_value, str) or not path_value:
            raise ValueError(f"dependency {name}: path is required")
        path = Path(path_value)
        if not path.is_absolute():
            path = (input_path.parent / path).resolve()
        if not path.is_file():
            raise FileNotFoundError(f"dependency {name} no longer exists: {path}")
        hashes[name] = file_sha256(path)
    return hashes


def snapshot(input_path: Path, policy_path: Path, repo: Path) -> dict[str, Any]:
    values: dict[str, Any] = {
        "input_sha256": file_sha256(input_path),
        "policy_sha256": file_sha256(policy_path),
        "prompt_sha256": file_sha256(PROMPT_FILE),
        "schema_sha256": file_sha256(SCHEMA_FILE),
        "git_head": git_head(repo),
        "git_worktree_sha256": git_worktree_digest(repo),
        "dependencies": declared_dependencies(input_path),
    }
    return values


def selected_providers(candidate: dict[str, Any], policy: dict[str, Any]) -> list[str]:
    routing = policy["routing"]
    names = [routing["primary"]]
    classification = candidate.get("discovery_classification", candidate.get("classification", ""))
    if classification in AMBIGUOUS_CLASSES:
        names.append(routing["ambiguous_replica"])
    selected = []
    for name in names:
        if name not in selected and policy["providers"].get(name, {}).get("enabled"):
            selected.append(name)
    if not selected:
        raise ValueError("policy has no enabled provider for this candidate")
    return selected


def confined_output(output: Path, repo: Path) -> Path:
    resolved = output.resolve()
    allowed = (repo.resolve() / "output").resolve()
    try:
        resolved.relative_to(allowed)
    except ValueError as exc:
        raise ValueError(f"agent output must stay below {allowed}") from exc
    return resolved


def resolve_run(args: argparse.Namespace) -> Path:
    base = confined_output(args.output, args.repo)
    run = args.run.resolve() if getattr(args, "run", None) else store.active_run(base)
    try:
        run.relative_to(base / "runs")
    except ValueError as exc:
        raise ValueError(f"agent run must stay below {base / 'runs'}") from exc
    if not (run / "run.json").is_file():
        raise FileNotFoundError(f"not an agent run: {run}")
    return run


def job_path(run: Path, job_id: str) -> Path:
    return run / "jobs" / f"{job_id}.json"


def lease_path(run: Path, job_id: str) -> Path:
    return run / "leases" / f"{job_id}.json"


def current_snapshot(run_record: dict[str, Any]) -> dict[str, Any]:
    return snapshot(
        Path(run_record["input_path"]),
        Path(run_record["policy_path"]),
        Path(run_record["repo"]),
    )


def run_is_stale(run_record: dict[str, Any]) -> bool:
    try:
        return current_snapshot(run_record) != run_record["snapshot"]
    except (FileNotFoundError, ValueError):
        return True


def cmd_doctor(args: argparse.Namespace) -> int:
    report: dict[str, Any] = {"schema_version": 1, "providers": {}}
    for name in ("codex", "grok", "claude"):
        executable = shutil.which(name)
        entry: dict[str, Any] = {"available": bool(executable), "version": None}
        if executable:
            result = subprocess.run(
                [executable, "--version"],
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                check=False,
                timeout=10,
            )
            entry["version"] = result.stdout.strip()[:300]
        report["providers"][name] = entry
    report["schema_file"] = SCHEMA_FILE.is_file()
    if args.json:
        print(json.dumps(report, indent=2))
    else:
        for name, entry in report["providers"].items():
            version = entry["version"] or "unavailable"
            print(f"{name}: {version}")
    return 0 if report["schema_file"] and any(item["available"] for item in report["providers"].values()) else 1


def cmd_plan(args: argparse.Namespace) -> int:
    input_path = args.input.resolve()
    policy_path = args.policy.resolve()
    repo = args.repo.resolve()
    policy = load_policy(policy_path)
    candidates = load_candidates(input_path)
    for candidate in candidates:
        validate_witness(candidate)
    stamp = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    run_id = args.run_id or f"dar-{stamp}-{uuid.uuid4().hex[:8]}"
    if not all(char.isalnum() or char in "._-" for char in run_id):
        raise ValueError("run ID may contain only letters, numbers, dot, underscore, and hyphen")
    output = confined_output(args.output, repo)
    run = output / "runs" / run_id
    if run.exists():
        raise FileExistsError(f"agent run already exists: {run}")
    run.mkdir(parents=True, mode=0o700)
    run_record = {
        "schema_version": 1,
        "run_id": run_id,
        "created_at": store.now(),
        "input_path": str(input_path),
        "policy_path": str(policy_path),
        "repo": str(repo),
        "snapshot": snapshot(input_path, policy_path, repo),
        "status": "planned",
        "jobs_total": len(candidates),
    }
    store.atomic_json(run / "run.json", run_record)
    planned = []
    job_ids = set()
    for candidate in candidates:
        identifier = candidate_identifier(candidate)
        identifier_on_disk = model.safe_job_id(identifier, candidate)
        if identifier_on_disk in job_ids:
            raise ValueError(f"duplicate candidate packet: {identifier}")
        job_ids.add(identifier_on_disk)
        prompt = prompt_for(candidate)
        if len(prompt.encode()) > policy.get("max_prompt_bytes", 24576):
            raise ValueError(f"{identifier}: prompt exceeds policy limit")
        job = {
            "schema_version": 1,
            "job_id": identifier_on_disk,
            "candidate_id": identifier,
            "candidate_sha256": model.digest(candidate),
            "candidate": candidate,
            "providers": selected_providers(candidate, policy),
            "completed_providers": [],
            "provider_attempts": {},
            "result_ids": [],
            "failures": [],
            "status": "queued",
            "created_at": store.now(),
        }
        store.atomic_json(job_path(run, identifier_on_disk), job)
        planned.append({"job_id": identifier_on_disk, "candidate_id": identifier, "providers": job["providers"]})
    store.atomic_json(output / "current.json", {"schema_version": 1, "run": str(run)})
    print(json.dumps({"run": str(run), "planned": len(planned), "jobs": planned}, indent=2))
    return 0


def next_provider(job: dict[str, Any], policy: dict[str, Any]) -> str | None:
    for name in job["providers"]:
        attempts = job["provider_attempts"].get(name, 0)
        if name not in job["completed_providers"] and attempts < policy["max_attempts"]:
            return name
    return None


def redact_environment(text: str) -> str:
    redacted = text
    for value in os.environ.values():
        if len(value) >= 12 and value in redacted:
            redacted = redacted.replace(value, "[REDACTED_ENV]")
    return redacted


def bounded_output(text: str, maximum: int) -> tuple[str, bool]:
    payload = text.encode(errors="replace")
    if len(payload) <= maximum:
        return text, False
    return payload[:maximum].decode(errors="replace"), True


def persist_attempt(
    run: Path,
    job: dict[str, Any],
    provider: str,
    attempt: int,
    execution: providers.Execution,
    result: dict[str, Any],
    maximum: int,
) -> None:
    directory = run / "results" / job["job_id"] / f"{provider}-{attempt}"
    stdout, stdout_truncated = bounded_output(redact_environment(execution.stdout), maximum)
    stderr, stderr_truncated = bounded_output(redact_environment(execution.stderr), maximum)
    store.atomic_bytes(directory / "native.stdout.txt", stdout.encode())
    store.atomic_bytes(directory / "native.stderr.txt", stderr.encode())
    result["native_truncated"] = stdout_truncated or stderr_truncated
    store.atomic_json(directory / "result.json", result)


def witness_from_candidate(candidate: dict[str, Any]) -> dict[str, Any]:
    fields = (
        "source",
        "source_sha256",
        "source_page",
        "printed_page",
        "source_bbox",
        "source_offset",
        "extractor",
        "extraction_confidence",
        "raw_text_sha256",
    )
    return {name: candidate.get(name) for name in fields if candidate.get(name) not in (None, "")}


def execute_job(run: Path, job: dict[str, Any], run_record: dict[str, Any], policy: dict[str, Any]) -> str:
    if run_is_stale(run_record):
        job["status"] = "stale"
        store.atomic_json(job_path(run, job["job_id"]), job)
        return "stale"
    provider = next_provider(job, policy)
    if provider is None:
        job["status"] = "complete" if set(job["completed_providers"]) == set(job["providers"]) else "terminal-failed"
        store.atomic_json(job_path(run, job["job_id"]), job)
        return job["status"]
    lease = lease_path(run, job["job_id"])
    if lease.exists():
        current = store.read_json(lease)
        if current.get("expires_at", 0) > store.now():
            return "leased"
        return "expired-lease"
    token = uuid.uuid4().hex
    lease_record = {
        "schema_version": 1,
        "job_id": job["job_id"],
        "token": token,
        "provider": provider,
        "acquired_at": store.now(),
        "expires_at": store.now() + policy.get("lease_seconds", 900),
    }
    if not store.create_lease(lease, lease_record):
        return "leased"
    attempt = job["provider_attempts"].get(provider, 0) + 1
    job["provider_attempts"][provider] = attempt
    job["status"] = "running"
    store.atomic_json(job_path(run, job["job_id"]), job)
    proposal: dict[str, Any] | None = None
    execution = providers.Execution(1, "", "provider was not invoked")
    error = ""
    retryable = False
    try:
        prompt = prompt_for(job["candidate"])
        execution = providers.execute(provider, policy, Path(run_record["repo"]), SCHEMA_FILE, prompt)
        if execution.code != 0:
            error, retryable = providers.classify_failure(execution)
        elif len(execution.stdout.encode(errors="replace")) > policy.get("max_output_bytes", 131072):
            error, retryable = "provider-output-too-large", True
        else:
            try:
                proposal = providers.parse_output(provider, execution.stdout)
            except ValueError as exc:
                error, retryable = str(exc), True
    except OSError as exc:
        error, retryable = f"provider-start-failed: {exc}", True
    result_id = f"{job['job_id']}-{provider}-{attempt}"
    result = {
        "schema_version": 1,
        "result_id": result_id,
        "job_id": job["job_id"],
        "candidate_id": job["candidate_id"],
        "candidate_sha256": job["candidate_sha256"],
        "witness": witness_from_candidate(job["candidate"]),
        "provider": provider,
        "attempt": attempt,
        "input_snapshot": run_record["snapshot"],
        "exit_code": execution.code,
        "timed_out": execution.timed_out,
        "output_limited": execution.output_limited,
        "status": "complete" if proposal is not None else "failed",
        "proposal": proposal,
        "error": error or None,
        "retryable": retryable,
        "finished_at": store.now(),
    }
    persist_attempt(
        run, job, provider, attempt, execution, result,
        policy.get("max_output_bytes", 131072),
    )
    if proposal is not None:
        job["completed_providers"].append(provider)
        job["result_ids"].append(result_id)
        job["status"] = "complete" if set(job["completed_providers"]) == set(job["providers"]) else "queued"
    else:
        job["failures"].append({"provider": provider, "attempt": attempt, "error": error, "retryable": retryable})
        if not retryable:
            job["provider_attempts"][provider] = policy["max_attempts"]
        job["status"] = "queued" if next_provider(job, policy) else "terminal-failed"
    current_lease = store.read_json(lease) if lease.exists() else {}
    if current_lease.get("token") == token:
        lease.unlink()
    store.atomic_json(job_path(run, job["job_id"]), job)
    return result["status"] if proposal is None else job["status"]


def cmd_run(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    run_record = store.read_json(run / "run.json")
    policy = load_policy(Path(run_record["policy_path"]))
    if not args.execute:
        ready = [job["job_id"] for job in store.jobs(run) if job["status"] in {"queued", "transient-failed"}]
        print(json.dumps({"run": str(run), "dry_run": True, "ready": ready}, indent=2))
        return 0
    outcomes = {}
    for job in store.jobs(run):
        if job["status"] in {"complete", "stale", "terminal-failed"}:
            continue
        outcomes[job["job_id"]] = execute_job(run, job, run_record, policy)
    print(json.dumps({"run": str(run), "outcomes": outcomes}, indent=2))
    return 0


def cmd_status(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    counts: dict[str, int] = {}
    for job in store.jobs(run):
        counts[job["status"]] = counts.get(job["status"], 0) + 1
    print(json.dumps({"run": str(run), "stale": run_is_stale(store.read_json(run / "run.json")), "jobs": counts}, indent=2))
    return 0


def cmd_reap(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    reaped = 0
    for path in sorted((run / "leases").glob("*.json")):
        if store.read_json(path).get("expires_at", 0) <= store.now():
            path.unlink()
            reaped += 1
    print(json.dumps({"run": str(run), "reaped": reaped}, indent=2))
    return 0


def successful_results(run: Path) -> list[dict[str, Any]]:
    results = []
    for path in sorted((run / "results").glob("*/*/result.json")):
        result = store.read_json(path)
        if result.get("status") == "complete":
            results.append(result)
    return results


def cmd_collect(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    run_record = store.read_json(run / "run.json")
    if run_is_stale(run_record) and not args.include_stale:
        raise RuntimeError("agent run is stale; re-plan or pass --include-stale for display only")
    results = sorted(
        successful_results(run),
        key=lambda item: (item["candidate_id"], item["provider"], item["attempt"]),
    )
    grouped: dict[str, set[str]] = {}
    for result in results:
        grouped.setdefault(result["candidate_id"], set()).add(result["proposal"]["verdict"])
    disagreements = sorted(candidate for candidate, verdicts in grouped.items() if len(verdicts) > 1)
    collection = {
        "schema_version": 1,
        "run_id": run_record["run_id"],
        "stale": run_is_stale(run_record),
        "results": results,
        "replica_disagreements": disagreements,
        "created_at": store.now(),
    }
    store.atomic_json(run / "collection.json", collection)
    print(json.dumps({"run": str(run), "collected": len(results), "replica_disagreements": disagreements}, indent=2))
    return 0


def complete_replica_collection(run: Path, run_record: dict[str, Any]) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    """Load a complete, current replica collection suitable for adjudication.

    Collections are immutable review snapshots.  Planning an adjudicator from
    a partial or stale snapshot could mix evidence from different source
    witnesses, so this is deliberately stricter than ``collect --include-stale``.
    """
    path = run / "collection.json"
    if not path.is_file():
        raise FileNotFoundError("no collection found; run collect after all replica jobs complete")
    collection = store.read_json(path)
    if not isinstance(collection, dict) or collection.get("run_id") != run_record.get("run_id"):
        raise ValueError("collection does not belong to this run")
    if collection.get("stale") or run_is_stale(run_record):
        raise RuntimeError("collection is stale; re-plan, run, and collect again")
    results = collection.get("results")
    if not isinstance(results, list) or any(not isinstance(item, dict) for item in results):
        raise ValueError("collection is incomplete: results must be a list")

    # Ignore adjudication jobs when the command is re-run.  Their existence
    # must not make a completed replica collection look incomplete.
    replicas = [job for job in store.jobs(run) if not job.get("adjudication")]
    if any(job.get("status") != "complete" for job in replicas):
        raise RuntimeError("collection is incomplete: all replica jobs must complete before adjudication")
    expected_ids = {
        result_id
        for job in replicas
        for result_id in job.get("result_ids", [])
    }
    actual_ids = {str(result.get("result_id", "")) for result in results}
    if len(actual_ids) != len(results) or not expected_ids or not expected_ids <= actual_ids:
        raise RuntimeError("collection is incomplete: it does not contain every successful replica result")
    replica_results = [result for result in results if str(result.get("result_id", "")) in expected_ids]
    if any(
        result.get("status") != "complete"
        or not isinstance(result.get("proposal"), dict)
        or not str(result.get("provider", ""))
        for result in replica_results
    ):
        raise RuntimeError("collection is incomplete: successful replica proposals are required")

    calculated: dict[str, set[str]] = {}
    for result in replica_results:
        calculated.setdefault(str(result.get("candidate_id", "")), set()).add(
            str(result["proposal"].get("verdict", ""))
        )
    disagreements = sorted(candidate for candidate, verdicts in calculated.items() if len(verdicts) > 1)
    declared = collection.get("replica_disagreements")
    if not isinstance(declared, list) or sorted(str(item) for item in declared) != disagreements:
        raise ValueError("collection disagreement summary does not match its replica results")
    return collection, replica_results


def adjudication_packet(candidate: dict[str, Any], results: list[dict[str, Any]]) -> dict[str, Any]:
    """Build a frozen Claude packet from one original witness and its replicas."""
    replicas = []
    for result in sorted(results, key=lambda item: (str(item["provider"]), str(item["result_id"]))):
        proposal = result["proposal"]
        replicas.append({
            "provider": result["provider"],
            "result_id": result["result_id"],
            "result_sha256": model.digest(result),
            "proposal_sha256": model.digest(proposal),
            "proposal": proposal,
        })
    witness = witness_from_candidate(candidate)
    return {
        # Retain this identity verbatim; it is the only printed witness Claude
        # is allowed to adjudicate, and contains no corpus-mutation instruction.
        "candidate_id": candidate_identifier(candidate),
        "candidate_sha256": model.digest(candidate),
        # Repeat the witness identity at top level so execute_job preserves it
        # verbatim in Claude's durable result record as well.
        **witness,
        "witness": witness,
        "original_candidate": candidate,
        "replica_proposals": replicas,
        "adjudication": "Compare the advisory replica proposals only; return an advisory JSON verdict.",
    }


def cmd_adjudicate(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    run_record = store.read_json(run / "run.json")
    collection, results = complete_replica_collection(run, run_record)
    disagreements = [str(item) for item in collection["replica_disagreements"]]
    if not disagreements:
        print(json.dumps({"run": str(run), "planned": 0, "reason": "no replica disagreements"}, indent=2))
        return 0
    policy = load_policy(Path(run_record["policy_path"]))
    if policy["routing"].get("adjudicator") != "claude" or not policy["providers"].get("claude", {}).get("enabled"):
        raise ValueError("Claude adjudication is disabled by policy; enable providers.claude and route adjudicator to claude")

    originals = {
        job["candidate_id"]: job
        for job in store.jobs(run)
        if not job.get("adjudication")
    }
    existing = {
        str(job.get("adjudication", {}).get("original_candidate_id", "")): job
        for job in store.jobs(run)
        if isinstance(job.get("adjudication"), dict)
    }
    planned = []
    for candidate_id in disagreements:
        original = originals.get(candidate_id)
        replica_results = [result for result in results if str(result.get("candidate_id", "")) == candidate_id]
        if original is None or len(replica_results) < 2:
            raise RuntimeError(f"collection is incomplete for disagreement {candidate_id!r}")
        providers_seen = {str(result["provider"]) for result in replica_results}
        if len(providers_seen) < 2:
            raise RuntimeError(f"collection is incomplete for disagreement {candidate_id!r}: independent replicas are required")
        # The same candidate is deliberately not re-planned on a second run of
        # this command, even if an earlier Claude attempt subsequently fails.
        if candidate_id in existing:
            continue
        packet = adjudication_packet(original["candidate"], replica_results)
        packet_bytes = json.dumps(packet, sort_keys=True, ensure_ascii=False).encode()
        if len(packet_bytes) > policy.get("max_prompt_bytes", 24576):
            raise ValueError(f"{candidate_id}: adjudication packet exceeds policy prompt limit")
        job_id = model.safe_job_id(f"adjudication-{candidate_id}", packet)
        if job_path(run, job_id).exists():
            # A hand-edited or interrupted state must never be overwritten.
            raise FileExistsError(f"adjudication job path already exists: {job_id}")
        job = {
            "schema_version": 1,
            "job_id": job_id,
            "candidate_id": candidate_id,
            "candidate_sha256": original["candidate_sha256"],
            "candidate": packet,
            "providers": ["claude"],
            "completed_providers": [],
            "provider_attempts": {},
            "result_ids": [],
            "failures": [],
            "status": "queued",
            "created_at": store.now(),
            "adjudication": {
                "original_candidate_id": candidate_id,
                "original_candidate_sha256": original["candidate_sha256"],
                "collection_sha256": model.digest(collection),
                "replica_result_ids": [item["result_id"] for item in packet["replica_proposals"]],
                "replica_proposal_sha256": [item["proposal_sha256"] for item in packet["replica_proposals"]],
            },
        }
        store.atomic_json(job_path(run, job_id), job)
        planned.append({"job_id": job_id, "candidate_id": candidate_id, "providers": ["claude"]})
    print(json.dumps({"run": str(run), "planned": len(planned), "jobs": planned}, indent=2))
    return 0


def cmd_show(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    matches = [job for job in store.jobs(run) if job["job_id"] == args.job or job["candidate_id"] == args.job]
    if len(matches) != 1:
        raise ValueError(f"expected one job matching {args.job!r}, found {len(matches)}")
    print(json.dumps(matches[0], indent=2, ensure_ascii=False))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--repo", type=Path, default=ROOT)
    parser.add_argument("--policy", type=Path, default=DEFAULT_POLICY)
    parser.add_argument("--run", type=Path, help="specific run directory; defaults to the current run")
    commands = parser.add_subparsers(dest="command", required=True)
    doctor = commands.add_parser("doctor")
    doctor.add_argument("--json", action="store_true")
    plan = commands.add_parser("plan")
    plan.add_argument("input", type=Path)
    plan.add_argument("--run-id")
    run = commands.add_parser("run")
    run.add_argument("--execute", action="store_true")
    commands.add_parser("status")
    commands.add_parser("reap")
    collect = commands.add_parser("collect")
    collect.add_argument("--include-stale", action="store_true")
    commands.add_parser("adjudicate", help="plan Claude-only advisory jobs for completed replica disagreements")
    show = commands.add_parser("show")
    show.add_argument("job")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    handlers = {
        "doctor": cmd_doctor,
        "plan": cmd_plan,
        "run": cmd_run,
        "status": cmd_status,
        "reap": cmd_reap,
        "collect": cmd_collect,
        "adjudicate": cmd_adjudicate,
        "show": cmd_show,
    }
    try:
        return handlers[args.command](args)
    except (FileNotFoundError, ValueError, RuntimeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
