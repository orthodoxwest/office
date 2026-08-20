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
import math
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
KNOWN_PROVIDERS = {"codex", "grok", "claude"}
ROUTING_ROLES = {"primary", "ambiguous_replica", "adjudicator"}
LEASE_CUSHION_SECONDS = 30
IDENTIFIER_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]{0,159}")


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def tree_sha256(path: Path) -> str:
    """Hash a directory's relative file names and contents."""
    root = path.resolve()
    if not root.is_dir():
        raise ValueError(f"dependency is not a directory: {path}")
    digest = hashlib.sha256()
    for child in sorted(item for item in root.rglob("*") if item.is_file()):
        resolved = child.resolve()
        try:
            relative = resolved.relative_to(root)
        except ValueError as exc:
            raise ValueError(f"dependency tree escapes its root: {child}") from exc
        digest.update(relative.as_posix().encode())
        digest.update(b"\0")
        digest.update(bytes.fromhex(file_sha256(resolved)))
    return digest.hexdigest()


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
    required = {
        "schema_version", "lease_seconds", "max_attempts", "max_prompt_bytes",
        "max_output_bytes", "wall_timeout_seconds", "providers", "routing",
    }
    if not isinstance(policy, dict) or set(policy) != required:
        raise ValueError(f"{path}: invalid agent policy")
    policy_schema = policy.get("schema_version")
    if isinstance(policy_schema, bool) or not isinstance(policy_schema, int) or policy_schema != 1:
        raise ValueError("agent policy schema_version must be 1")
    attempts = policy["max_attempts"]
    if isinstance(attempts, bool) or not isinstance(attempts, int) or not 1 <= attempts <= 5:
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
    # The lease must outlast a provider invocation and leave time to persist its
    # bounded result and release the lease.  This prevents a second runner from
    # acquiring the same job while the first is still writing durable state.
    if policy["lease_seconds"] < policy["wall_timeout_seconds"] + LEASE_CUSHION_SECONDS:
        raise ValueError(
            "lease_seconds must be wall_timeout_seconds plus at least "
            f"{LEASE_CUSHION_SECONDS} seconds for result persistence"
        )
    if not isinstance(policy["providers"], dict) or set(policy["providers"]) != KNOWN_PROVIDERS:
        raise ValueError("policy providers must contain exactly codex, grok, and claude")
    for name, settings in policy["providers"].items():
        expected = {
            "codex": {"enabled", "model", "reasoning_effort"},
            "grok": {"enabled", "model", "max_turns"},
            "claude": {"enabled", "model", "max_budget_usd"},
        }[name]
        if not isinstance(settings, dict) or set(settings) != expected:
            raise ValueError(f"{name}: policy settings must contain exactly {', '.join(sorted(expected))}")
        if not isinstance(settings, dict) or not isinstance(settings.get("enabled"), bool):
            raise ValueError(f"{name}: enabled must be boolean")
        if not isinstance(settings.get("model"), str) or not settings["model"].strip():
            raise ValueError(f"{name}: model must be a non-empty string")
        if name == "codex":
            effort = settings.get("reasoning_effort")
            if not isinstance(effort, str) or effort not in {
                "low", "medium", "high", "xhigh", "max", "ultra"
            }:
                raise ValueError("codex: reasoning_effort must be a supported level")
        if name == "grok":
            value = settings.get("max_turns")
            if isinstance(value, bool) or not isinstance(value, int) or not 1 <= value <= 10:
                raise ValueError("grok: max_turns must be an integer from 1 to 10")
        if name == "claude":
            value = settings.get("max_budget_usd")
            if isinstance(value, bool) or not isinstance(value, (int, float)) or not 0.01 <= value <= 10:
                raise ValueError("claude: max_budget_usd must be a number from 0.01 to 10")
    if not isinstance(policy["routing"], dict) or set(policy["routing"]) != ROUTING_ROLES:
        raise ValueError("policy routing must contain exactly primary, ambiguous_replica, and adjudicator")
    for name, provider in policy["routing"].items():
        if not isinstance(provider, str) or provider not in KNOWN_PROVIDERS:
            raise ValueError(f"policy routing {name} must name a known provider")
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
    for name in ("canonical_text_sha256", "text_sha256"):
        value = candidate.get(name)
        if value is not None and (not isinstance(value, str) or not re.fullmatch(r"[0-9a-fA-F]{64}", value)):
            raise ValueError(f"candidate witness {name} must be a full SHA-256 when present")
    column = candidate.get("source_column", "")
    if column:
        if not isinstance(column, str) or not re.fullmatch(r"[a-z][a-z0-9_-]*", column):
            raise ValueError("candidate source_column is not a safe column label")
        bbox = candidate.get("source_bbox", candidate.get("bbox"))
        if isinstance(bbox, str):
            try: bbox = json.loads(bbox)
            except json.JSONDecodeError: bbox = None
        if not isinstance(bbox, list) or len(bbox) != 4 or any(isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v) for v in bbox):
            raise ValueError("column candidate requires a finite four-coordinate bbox")
        if bbox[2] <= bbox[0] or bbox[3] <= bbox[1]:
            raise ValueError("column candidate bbox must have positive dimensions")
    slices = candidate.get("page_slices")
    if slices:
        if isinstance(slices, str):
            try: slices = json.loads(slices)
            except json.JSONDecodeError: slices = None
        if not isinstance(slices, list) or not slices:
            raise ValueError("candidate page_slices must be a non-empty list")
        columns = set()
        for index, item in enumerate(slices):
            if not isinstance(item, dict) or not isinstance(item.get("page"), int) or item["page"] < 1:
                raise ValueError(f"candidate page_slices[{index}] has invalid page")
            if not isinstance(item.get("printed_page"), str) or not item["printed_page"]:
                raise ValueError(f"candidate page_slices[{index}] requires printed_page")
            if not isinstance(item.get("raw_text_sha256"), str) or not re.fullmatch(r"[0-9a-fA-F]{64}", item["raw_text_sha256"]):
                raise ValueError(f"candidate page_slices[{index}] requires full slice hash")
            if not isinstance(item.get("extractor"), str) or not item["extractor"]:
                raise ValueError(f"candidate page_slices[{index}] requires route")
            if not isinstance(item.get("offset"), str) or not item["offset"]:
                raise ValueError(f"candidate page_slices[{index}] requires offset")
            if column:
                slice_column = item.get("source_column")
                if not isinstance(slice_column, str) or slice_column != column:
                    raise ValueError(f"candidate page_slices[{index}] disagrees with source_column")
                box = item.get("bbox")
                if not isinstance(box, list) or len(box) != 4 or any(isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v) for v in box) or box[2] <= box[0] or box[3] <= box[1]:
                    raise ValueError(f"candidate page_slices[{index}] requires valid bbox")
                columns.add(slice_column)
        if column and len(columns) != 1:
            raise ValueError("candidate page_slices mix source columns")


def validate_identifier(value: Any, label: str) -> str:
    """Reject traversal and ambiguous names before they are used in paths."""
    if not isinstance(value, str) or not IDENTIFIER_RE.fullmatch(value):
        raise ValueError(f"{label} must contain only letters, numbers, dot, underscore, and hyphen")
    return value


def confined_child(base: Path, child: str, label: str, containment_root: Path | None = None) -> Path:
    """Resolve a generated child and reject existing symlink escapes."""
    validate_identifier(child, label)
    root = (containment_root or base).resolve()
    path = (base / child).resolve()
    try:
        path.relative_to(root)
    except ValueError as exc:
        raise ValueError(f"{label} must stay below {root}") from exc
    return path


def prompt_for(candidate: dict[str, Any]) -> str:
    instructions = PROMPT_FILE.read_text(encoding="utf-8")
    return f"{instructions.rstrip()}\n\nCandidate packet:\n{json.dumps(candidate, indent=2, sort_keys=True, ensure_ascii=False)}\n"


def manifest_dependencies(
    manifest: dict[str, Any], repo: Path
) -> dict[str, str]:
    """Resolve and verify the portable discovery dependency manifest."""
    manifest_schema = manifest.get("schema_version")
    if isinstance(manifest_schema, bool) or not isinstance(manifest_schema, int) or manifest_schema != 1:
        raise ValueError("dependency manifest schema_version must be 1")
    roots = manifest.get("roots")
    if not isinstance(roots, dict) or set(roots) != {"repository", "intake"}:
        raise ValueError("dependency manifest requires repository and intake roots")
    if roots["repository"] != "." or not isinstance(roots["intake"], str):
        raise ValueError("dependency manifest roots are malformed")
    intake_locator = Path(roots["intake"])
    if intake_locator.is_absolute() or ".." in intake_locator.parts:
        raise ValueError("dependency intake root must be repository-relative")
    repository = repo.resolve()
    intake = (repository / intake_locator).resolve()
    try:
        intake.relative_to((repository / "output").resolve())
    except ValueError as exc:
        raise ValueError("dependency intake root must stay below repository output") from exc
    if not intake.is_dir():
        raise FileNotFoundError(f"dependency intake root no longer exists: {intake}")
    resolved_roots = {"repository": repository, "intake": intake}

    entries = manifest.get("entries")
    if not isinstance(entries, list) or not entries:
        raise ValueError("dependency manifest entries must be a non-empty list")
    hashes: dict[str, str] = {}
    tree_roles: dict[str, list[tuple[str, str]]] = {}
    for record in entries:
        required = {"kind", "role", "root", "path", "sha256"}
        if not isinstance(record, dict) or set(record) != required:
            raise ValueError("dependency manifest entry is malformed")
        kind, role, root_name = record["kind"], record["role"], record["root"]
        path_value, declared_hash = record["path"], record["sha256"]
        if kind not in {"file", "tree"} or root_name not in resolved_roots:
            raise ValueError("dependency manifest entry has an unknown kind or root")
        if (
            not isinstance(role, str)
            or not re.fullmatch(r"[a-z][a-z0-9-]{0,63}", role)
            or not isinstance(path_value, str)
            or not path_value
        ):
            raise ValueError("dependency manifest entry role and path are required")
        if not isinstance(declared_hash, str) or not re.fullmatch(r"[0-9a-f]{64}", declared_hash):
            raise ValueError("dependency manifest entry requires a full SHA-256")
        relative = Path(path_value)
        if relative.is_absolute() or ".." in relative.parts or (path_value == "." and kind != "tree"):
            raise ValueError("dependency manifest entry path is unsafe")
        base = resolved_roots[root_name]
        path = (base / relative).resolve()
        try:
            path.relative_to(base)
        except ValueError as exc:
            raise ValueError("dependency manifest entry escapes its root") from exc
        if kind == "file" and not path.is_file():
            raise FileNotFoundError(f"dependency file no longer exists: {path}")
        if kind == "tree" and not path.is_dir():
            raise FileNotFoundError(f"dependency tree no longer exists: {path}")
        actual_hash = file_sha256(path) if kind == "file" else tree_sha256(path)
        if actual_hash != declared_hash:
            raise ValueError(f"dependency {role} hash does not match its manifest")
        key = f"{kind}:{root_name}:{relative.as_posix()}"
        if key in hashes:
            raise ValueError(f"duplicate dependency manifest entry: {key}")
        hashes[key] = actual_hash
        if kind == "tree":
            tree_roles.setdefault(role, []).append((root_name, relative.as_posix()))
    required_trees = {"intake-tree", "corpus-tree", "feast-metadata-tree"}
    if any(len(tree_roles.get(role, [])) != 1 for role in required_trees):
        raise ValueError(
            "dependency manifest requires exactly one intake, corpus, and feast-metadata tree"
        )
    if tree_roles["intake-tree"] != [("intake", ".")]:
        raise ValueError("dependency intake tree must describe the complete intake root")
    corpus_root, corpus_path = tree_roles["corpus-tree"][0]
    feast_root, feast_path = tree_roles["feast-metadata-tree"][0]
    if corpus_root != "repository" or Path(corpus_path).name != "texts":
        raise ValueError("dependency corpus tree must be a repository texts directory")
    if feast_root != "repository" or Path(feast_path).name != "feasts":
        raise ValueError("dependency feast-metadata tree must be a repository feasts directory")
    return hashes


def declared_dependencies(input_path: Path, repo: Path) -> dict[str, str]:
    payload = store.read_json(input_path)
    if isinstance(payload, dict) and "dependency_manifest" in payload:
        manifest = payload["dependency_manifest"]
        if not isinstance(manifest, dict):
            raise ValueError("dependency_manifest must be an object")
        return manifest_dependencies(manifest, repo)
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
        try:
            path.resolve().relative_to(repo.resolve())
        except ValueError as exc:
            raise ValueError(f"dependency {name}: path must stay below the repository") from exc
        if not path.is_file():
            raise FileNotFoundError(f"dependency {name} no longer exists: {path}")
        actual_hash = file_sha256(path)
        declared_hash = record.get("sha256")
        if not isinstance(declared_hash, str) or not re.fullmatch(r"[0-9a-f]{64}", declared_hash):
            raise ValueError(f"dependency {name}: a full SHA-256 is required")
        if actual_hash != declared_hash:
            raise ValueError(f"dependency {name} hash does not match its manifest")
        hashes[name] = actual_hash
    return hashes


def snapshot(input_path: Path, policy_path: Path, repo: Path) -> dict[str, Any]:
    values: dict[str, Any] = {
        "input_sha256": file_sha256(input_path),
        "policy_sha256": file_sha256(policy_path),
        "prompt_sha256": file_sha256(PROMPT_FILE),
        "schema_sha256": file_sha256(SCHEMA_FILE),
        "git_head": git_head(repo),
        "git_worktree_sha256": git_worktree_digest(repo),
        "dependencies": declared_dependencies(input_path, repo),
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
    if getattr(args, "run", None):
        run = args.run.resolve()
    else:
        pointer = base / "current.json"
        if not pointer.is_file():
            raise FileNotFoundError("no planned agent run; run plan first")
        value = store.read_json(pointer)
        if not isinstance(value, dict) or not isinstance(value.get("run"), str):
            raise ValueError("active agent run pointer is malformed")
        run = Path(value["run"]).resolve()
    try:
        relative = run.relative_to((base / "runs").resolve())
    except ValueError as exc:
        raise ValueError(f"agent run must stay below {base / 'runs'}") from exc
    if len(relative.parts) != 1:
        raise ValueError("agent run must be a direct run directory")
    validate_identifier(relative.name, "run ID")
    if not (run / "run.json").is_file():
        raise FileNotFoundError(f"not an agent run: {run}")
    return run


def validated_run_record(
    run: Path, args: argparse.Namespace
) -> dict[str, Any]:
    """Validate persisted run paths before any dependent read or provider call."""
    record = store.read_json(run / "run.json")
    required = {"schema_version", "run_id", "input_path", "policy_path", "repo", "snapshot"}
    if not isinstance(record, dict) or not required <= set(record):
        raise ValueError("agent run record is malformed")
    run_schema = record.get("schema_version")
    if isinstance(run_schema, bool) or not isinstance(run_schema, int) or run_schema != 1:
        raise ValueError("agent run record schema_version must be 1")
    if record.get("run_id") != run.name:
        raise ValueError("agent run record ID does not match its directory")
    expected_repo = args.repo.resolve()
    repo_value = record.get("repo")
    if not isinstance(repo_value, str) or Path(repo_value).resolve() != expected_repo:
        raise ValueError("agent run repository does not match the requested repository")
    for name in ("input_path", "policy_path"):
        value = record.get(name)
        if not isinstance(value, str):
            raise ValueError(f"agent run {name} must be a path string")
        path = Path(value).resolve()
        try:
            path.relative_to(expected_repo)
        except ValueError as exc:
            raise ValueError(f"agent run {name} must stay below the repository") from exc
        if not path.is_file():
            raise FileNotFoundError(f"agent run {name} no longer exists: {path}")
    if not isinstance(record.get("snapshot"), dict):
        raise ValueError("agent run snapshot must be an object")
    return record


def job_path(run: Path, job_id: str) -> Path:
    validate_identifier(job_id, "job ID")
    return confined_child(run / "jobs", f"{job_id}.json", "job file", run)


def lease_path(run: Path, job_id: str) -> Path:
    validate_identifier(job_id, "job ID")
    return confined_child(run / "leases", f"{job_id}.json", "lease file", run)


def confined_state_directory(
    run: Path, name: str, label: str, *, required: bool = False
) -> Path | None:
    """Return an existing run-state directory without following symlinks."""
    requested = run / name
    if not requested.exists() and not requested.is_symlink():
        if required:
            raise FileNotFoundError(f"agent {label} directory does not exist")
        return None
    if requested.is_symlink():
        raise ValueError(f"agent {label} directory must not be a symlink")
    resolved = requested.resolve()
    try:
        resolved.relative_to(run.resolve())
    except ValueError as exc:
        raise ValueError(f"agent {label} directory must stay below the run") from exc
    if not resolved.is_dir():
        raise ValueError(f"agent {label} path is not a directory")
    return resolved


def load_jobs(run: Path) -> list[dict[str, Any]]:
    directory = confined_state_directory(run, "jobs", "jobs", required=True)
    jobs = []
    for entry in sorted(directory.iterdir()):
        if entry.suffix != ".json":
            continue
        if entry.is_symlink():
            raise ValueError("agent job file must not be a symlink")
        validate_identifier(entry.stem, "job ID")
        resolved = entry.resolve()
        try:
            relative = resolved.relative_to(directory)
        except ValueError as exc:
            raise ValueError("agent job file must stay below the jobs directory") from exc
        if len(relative.parts) != 1 or not resolved.is_file():
            raise ValueError("agent job entry is not a direct file")
        job = store.read_json(resolved)
        if not isinstance(job, dict) or job.get("job_id") != entry.stem:
            raise ValueError("agent job ID does not match its file")
        jobs.append(job)
    return jobs


def attempt_directory(run: Path, job_id: str, provider: str, attempt: int) -> Path:
    validate_identifier(job_id, "job ID")
    if provider not in KNOWN_PROVIDERS:
        raise ValueError("result provider is unknown")
    if isinstance(attempt, bool) or not isinstance(attempt, int) or attempt < 1:
        raise ValueError("result attempt must be positive")
    return confined_child(
        run / "results" / job_id, f"{provider}-{attempt}", "result directory", run,
    )


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


def confined_repo_input(path: Path, repo: Path, label: str) -> Path:
    resolved = path.resolve()
    try:
        resolved.relative_to(repo.resolve())
    except ValueError as exc:
        raise ValueError(f"{label} must stay below the repository") from exc
    if not resolved.is_file():
        raise FileNotFoundError(f"{label} does not exist: {resolved}")
    return resolved


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
    repo = args.repo.resolve()
    input_path = confined_repo_input(args.input, repo, "agent input")
    policy_path = confined_repo_input(args.policy, repo, "agent policy")
    policy = load_policy(policy_path)
    candidates = load_candidates(input_path)
    for candidate in candidates:
        validate_witness(candidate)
    stamp = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    run_id = validate_identifier(args.run_id or f"dar-{stamp}-{uuid.uuid4().hex[:8]}", "run ID")
    output = confined_output(args.output, repo)
    # Validate every configuration and candidate before creating the run
    # directory.  A malformed policy must never leave a lease or partial run.
    planned_candidates = []
    job_ids = set()
    for candidate in candidates:
        identifier = candidate_identifier(candidate)
        identifier_on_disk = model.safe_job_id(identifier, candidate)
        validate_identifier(identifier_on_disk, "job ID")
        if identifier_on_disk in job_ids:
            raise ValueError(f"duplicate candidate packet: {identifier}")
        job_ids.add(identifier_on_disk)
        prompt = prompt_for(candidate)
        if len(prompt.encode()) > policy["max_prompt_bytes"]:
            raise ValueError(f"{identifier}: prompt exceeds policy limit")
        planned_candidates.append((
            candidate,
            identifier,
            identifier_on_disk,
            immutable_witness(candidate),
            selected_providers(candidate, policy),
        ))
    input_snapshot = snapshot(input_path, policy_path, repo)
    run = confined_child(output / "runs", run_id, "run ID", output)
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
        "snapshot": input_snapshot,
        "status": "planned",
        "jobs_total": len(candidates),
    }
    store.atomic_json(run / "run.json", run_record)
    planned = []
    for candidate, identifier, identifier_on_disk, witness, provider_names in planned_candidates:
        job = {
            "schema_version": 1,
            "job_id": identifier_on_disk,
            "candidate_id": identifier,
            "candidate_sha256": model.digest(candidate),
            # This is derived locally while planning and never taken from a
            # provider proposal.  Results retain this exact object.
            "witness": witness,
            "candidate": candidate,
            "providers": provider_names,
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
    directory: Path,
    job: dict[str, Any],
    provider: str,
    attempt: int,
    execution: providers.Execution,
    result: dict[str, Any],
    maximum: int,
) -> None:
    stdout, stdout_truncated = bounded_output(redact_environment(execution.stdout), maximum)
    stderr, stderr_truncated = bounded_output(redact_environment(execution.stderr), maximum)
    store.atomic_bytes(directory / "native.stdout.txt", stdout.encode())
    store.atomic_bytes(directory / "native.stderr.txt", stderr.encode())
    result["native_truncated"] = stdout_truncated or stderr_truncated
    store.atomic_json(directory / "result.json", result)


def witness_from_candidate(candidate: dict[str, Any]) -> dict[str, Any]:
    """Make the bounded printed-witness identity retained with every result.

    It deliberately excludes candidate prose and model output.  Optional
    fields are retained verbatim when the intake supplied them, so a reviewer
    can locate the exact document/page/folio and extraction route without
    merging witnesses.
    """
    validate_witness(candidate)
    fields = (
        "source", "source_sha256", "source_page", "printed_page", "printed_folio",
        "source_column", "source_bbox", "source_offset", "page_slices", "extractor", "ocr_route",
        "extraction_confidence", "raw_text_sha256", "canonical_text_sha256", "text_sha256",
    )
    witness = {name: candidate[name] for name in fields if candidate.get(name) not in (None, "")}
    # A canonical serialization prevents a provider or later in-process change
    # to the candidate dictionary from changing the durable identity.
    return json.loads(model.canonical_json(witness))


def immutable_witness(candidate: dict[str, Any]) -> dict[str, Any]:
    witness = witness_from_candidate(candidate)
    return {"identity": witness, "sha256": model.digest(witness)}


def validate_job_identity(job: dict[str, Any]) -> None:
    """Validate persisted, potentially hand-edited state before path writes."""
    validate_identifier(job.get("job_id"), "job ID")
    candidate_sha = job.get("candidate_sha256")
    if not isinstance(candidate_sha, str) or not re.fullmatch(r"[0-9a-f]{64}", candidate_sha):
        raise ValueError("job candidate_sha256 must be a full SHA-256")
    adjudication = job.get("adjudication")
    if adjudication:
        if not isinstance(adjudication, dict):
            raise ValueError("job adjudication metadata must be an object")
        packet_sha = adjudication.get("packet_sha256")
        if not isinstance(packet_sha, str) or not re.fullmatch(r"[0-9a-f]{64}", packet_sha):
            raise ValueError("adjudication job requires packet_sha256")
        if model.digest(job.get("candidate")) != packet_sha:
            raise ValueError("adjudication packet_sha256 does not match its candidate")
        if adjudication.get("original_candidate_sha256") != candidate_sha:
            raise ValueError("adjudication original candidate hash does not match its job")
    elif model.digest(job.get("candidate")) != candidate_sha:
        raise ValueError("job candidate_sha256 does not match its candidate")
    witness = job.get("witness")
    if not isinstance(witness, dict) or set(witness) != {"identity", "sha256"}:
        raise ValueError("job requires an immutable witness object")
    if not isinstance(witness["identity"], dict) or not isinstance(witness["sha256"], str):
        raise ValueError("job immutable witness is malformed")
    if model.digest(witness["identity"]) != witness["sha256"]:
        raise ValueError("job immutable witness hash does not match")
    # Validate the actual required source identity, not merely its hash.
    validate_witness(witness["identity"])
    if adjudication and job.get("candidate", {}).get("witness") != witness["identity"]:
        raise ValueError("adjudication packet witness does not match its job")
    providers_for_job = job.get("providers")
    if not isinstance(providers_for_job, list) or not providers_for_job:
        raise ValueError("job providers must be a non-empty list")
    for provider_name in providers_for_job:
        if not isinstance(provider_name, str) or provider_name not in KNOWN_PROVIDERS:
            raise ValueError("job contains an unknown provider")


def execute_job(run: Path, job: dict[str, Any], run_record: dict[str, Any], policy: dict[str, Any]) -> str:
    # Do this before consulting or creating a lease: persisted state is input,
    # not trusted path material.
    validate_job_identity(job)
    if run_is_stale(run_record):
        job["status"] = "stale"
        store.atomic_json(job_path(run, job["job_id"]), job)
        return "stale"
    provider = next_provider(job, policy)
    if provider is None:
        job["status"] = "complete" if set(job["completed_providers"]) == set(job["providers"]) else "terminal-failed"
        store.atomic_json(job_path(run, job["job_id"]), job)
        return job["status"]
    if provider not in KNOWN_PROVIDERS or provider not in job["providers"]:
        raise ValueError("selected provider is not valid for this job")
    attempt = job["provider_attempts"].get(provider, 0) + 1
    result_directory = attempt_directory(run, job["job_id"], provider, attempt)
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
        "witness": job["witness"],
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
        result_directory, job, provider, attempt, execution, result,
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
    run_record = validated_run_record(run, args)
    policy = load_policy(Path(run_record["policy_path"]))
    if not args.execute:
        ready = [job["job_id"] for job in load_jobs(run) if job["status"] in {"queued", "transient-failed"}]
        print(json.dumps({"run": str(run), "dry_run": True, "ready": ready}, indent=2))
        return 0
    outcomes = {}
    for job in load_jobs(run):
        if job["status"] in {"complete", "stale", "terminal-failed"}:
            continue
        outcomes[job["job_id"]] = execute_job(run, job, run_record, policy)
    print(json.dumps({"run": str(run), "outcomes": outcomes}, indent=2))
    return 0


def cmd_status(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    run_record = validated_run_record(run, args)
    counts: dict[str, int] = {}
    for job in load_jobs(run):
        counts[job["status"]] = counts.get(job["status"], 0) + 1
    print(json.dumps({"run": str(run), "stale": run_is_stale(run_record), "jobs": counts}, indent=2))
    return 0


def cmd_reap(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    reaped = 0
    directory = confined_state_directory(run, "leases", "leases")
    for entry in sorted(directory.iterdir()) if directory is not None else []:
        if entry.suffix != ".json":
            continue
        if entry.is_symlink():
            raise ValueError("agent lease file must not be a symlink")
        validate_identifier(entry.stem, "job ID")
        path = entry.resolve()
        try:
            relative = path.relative_to(directory)
        except ValueError as exc:
            raise ValueError("agent lease file must stay below the leases directory") from exc
        if len(relative.parts) != 1 or not path.is_file():
            raise ValueError("agent lease entry is not a direct file")
        if store.read_json(path).get("expires_at", 0) <= store.now():
            path.unlink()
            reaped += 1
    print(json.dumps({"run": str(run), "reaped": reaped}, indent=2))
    return 0


def successful_results(run: Path) -> list[dict[str, Any]]:
    results = []
    root = confined_state_directory(run, "results", "results")
    for job_dir in sorted(root.iterdir()) if root is not None else []:
        if job_dir.is_symlink():
            raise ValueError("agent result job directory must not be a symlink")
        validate_identifier(job_dir.name, "job ID")
        resolved_job = job_dir.resolve()
        try:
            resolved_job.relative_to(root)
        except ValueError as exc:
            raise ValueError("agent result job directory must stay below results") from exc
        if not resolved_job.is_dir():
            raise ValueError("agent result job entry is not a directory")
        for attempt_dir in sorted(resolved_job.iterdir()):
            if attempt_dir.is_symlink():
                raise ValueError("agent result attempt directory must not be a symlink")
            match = re.fullmatch(r"(codex|grok|claude)-([1-9][0-9]*)", attempt_dir.name)
            if not match:
                raise ValueError("agent result attempt directory has an invalid name")
            resolved_attempt = attempt_dir.resolve()
            try:
                resolved_attempt.relative_to(resolved_job)
            except ValueError as exc:
                raise ValueError("agent result attempt must stay below its job") from exc
            path = resolved_attempt / "result.json"
            if path.is_symlink():
                raise ValueError("agent result file must not be a symlink")
            if not path.is_file():
                continue
            result = store.read_json(path)
            if result.get("status") == "complete":
                candidate_sha = result.get("candidate_sha256")
                witness = result.get("witness")
                if not isinstance(candidate_sha, str) or not re.fullmatch(r"[0-9a-f]{64}", candidate_sha):
                    raise ValueError(f"{path}: result is missing candidate_sha256")
                if not isinstance(witness, dict) or set(witness) != {"identity", "sha256"}:
                    raise ValueError(f"{path}: result is missing immutable witness")
                if model.digest(witness.get("identity")) != witness.get("sha256"):
                    raise ValueError(f"{path}: immutable witness hash does not match")
                validate_witness(witness["identity"])
                results.append(result)
    return results


def validate_result_bindings(run: Path, results: list[dict[str, Any]]) -> None:
    """Require collection entries to retain the exact planned identity.

    A structurally valid witness is insufficient: a hand-edited result must
    not be able to attach one candidate's proposal to another candidate's
    printed witness.
    """
    jobs_by_id = {str(job.get("job_id", "")): job for job in load_jobs(run)}
    for result in results:
        job = jobs_by_id.get(str(result.get("job_id", "")))
        if job is None:
            raise ValueError("result does not belong to a planned job")
        validate_job_identity(job)
        for field in ("candidate_id", "candidate_sha256", "witness"):
            if result.get(field) != job.get(field):
                raise ValueError(f"result {field} does not match its planned job")


def cmd_collect(args: argparse.Namespace) -> int:
    run = resolve_run(args)
    run_record = validated_run_record(run, args)
    if run_is_stale(run_record) and not args.include_stale:
        raise RuntimeError("agent run is stale; re-plan or pass --include-stale for display only")
    results = sorted(
        successful_results(run),
        key=lambda item: (item["candidate_id"], item["provider"], item["attempt"]),
    )
    validate_result_bindings(run, results)
    grouped: dict[str, set[str]] = {}
    for result in results:
        grouped.setdefault(result["candidate_id"], set()).add(result["proposal"]["verdict"])
    disagreements = sorted(candidate for candidate, verdicts in grouped.items() if len(verdicts) > 1)
    collection = {
        "schema_version": 1,
        "run_id": run_record["run_id"],
        "stale": run_is_stale(run_record),
        "results": results,
        # Collection entries retain each result's locally-derived witness and
        # candidate hash; no provider-controlled field is used as identity.
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
    validate_result_bindings(run, results)

    # Ignore adjudication jobs when the command is re-run.  Their existence
    # must not make a completed replica collection look incomplete.
    replicas = [job for job in load_jobs(run) if not job.get("adjudication")]
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
    run_record = validated_run_record(run, args)
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
        for job in load_jobs(run)
        if not job.get("adjudication")
    }
    existing = {
        str(job.get("adjudication", {}).get("original_candidate_id", "")): job
        for job in load_jobs(run)
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
            "witness": original["witness"],
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
                "packet_sha256": model.digest(packet),
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
    matches = [job for job in load_jobs(run) if job["job_id"] == args.job or job["candidate_id"] == args.job]
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
