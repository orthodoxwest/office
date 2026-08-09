"""Read-only provider CLI adapters."""

from __future__ import annotations

import json
import os
import selectors
import signal
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

from .model import RESULT_SCHEMA, validate_result


@dataclass(frozen=True)
class Execution:
    code: int
    stdout: str
    stderr: str
    timed_out: bool = False
    output_limited: bool = False


def argv(provider: str, policy: dict[str, Any], repo: Path, schema_file: Path, prompt: str) -> list[str]:
    settings = policy["providers"][provider]
    schema_json = json.dumps(RESULT_SCHEMA, separators=(",", ":"))
    if provider == "codex":
        return [
            "codex", "exec", "--ephemeral", "--json", "--output-schema", str(schema_file),
            "--color", "never", "--sandbox", "read-only", "-C", str(repo),
            "-m", settings["model"], prompt,
        ]
    if provider == "grok":
        return [
            "grok", "--single", prompt, "--output-format", "json", "--json-schema", schema_json,
            "--permission-mode", "plan", "--cwd", str(repo), "--no-memory", "--no-subagents",
            "--disable-web-search", "--max-turns", str(settings.get("max_turns", 2)),
            "--model", settings["model"],
        ]
    if provider == "claude":
        return [
            "claude", "-p", prompt, "--output-format", "json", "--json-schema", schema_json,
            "--permission-mode", "plan", "--no-session-persistence", "--max-budget-usd",
            str(settings.get("max_budget_usd", 0.5)), "--model", settings["model"], "--tools", "",
        ]
    raise ValueError(f"unknown provider: {provider}")


def execute(
    provider: str,
    policy: dict[str, Any],
    repo: Path,
    schema_file: Path,
    prompt: str,
    runner: Callable[..., subprocess.CompletedProcess] = subprocess.run,
) -> Execution:
    command = argv(provider, policy, repo, schema_file, prompt)
    if runner is subprocess.run:
        return run_capped(
            command,
            repo,
            policy.get("wall_timeout_seconds", 720),
            policy.get("max_output_bytes", 131072),
        )
    try:
        result = runner(
            command,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            env=os.environ.copy(),
            cwd=repo,
            check=False,
            timeout=policy.get("wall_timeout_seconds", 720),
        )
        return Execution(result.returncode, result.stdout, result.stderr)
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout.decode(errors="replace") if isinstance(exc.stdout, bytes) else (exc.stdout or "")
        stderr = exc.stderr.decode(errors="replace") if isinstance(exc.stderr, bytes) else (exc.stderr or "")
        return Execution(124, stdout, stderr, timed_out=True)


def _stop_process(process: subprocess.Popen) -> None:
    try:
        os.killpg(process.pid, signal.SIGTERM)
        process.wait(timeout=1)
    except (ProcessLookupError, subprocess.TimeoutExpired):
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


def run_capped(command: list[str], repo: Path, timeout: int, maximum: int) -> Execution:
    """Run a provider while bounding stdout+stderr memory before completion."""
    process = subprocess.Popen(
        command,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=os.environ.copy(),
        cwd=repo,
        start_new_session=True,
    )
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ, "stdout")
    selector.register(process.stderr, selectors.EVENT_READ, "stderr")
    buffers = {"stdout": bytearray(), "stderr": bytearray()}
    deadline = time.monotonic() + timeout
    timed_out = False
    limited = False
    stopped = False
    while selector.get_map():
        remaining = deadline - time.monotonic()
        if remaining <= 0 and not stopped:
            timed_out = True
            stopped = True
            _stop_process(process)
        events = selector.select(timeout=max(0, min(0.2, remaining)) if not stopped else 0.05)
        for key, _ in events:
            chunk = os.read(key.fd, 8192)
            if not chunk:
                selector.unregister(key.fileobj)
                continue
            current_size = len(buffers["stdout"]) + len(buffers["stderr"])
            room = max(0, maximum - current_size)
            buffers[key.data].extend(chunk[:room])
            if len(chunk) > room and not stopped:
                limited = True
                stopped = True
                _stop_process(process)
    selector.close()
    if process.stdout is not None:
        process.stdout.close()
    if process.stderr is not None:
        process.stderr.close()
    process.wait()
    if timed_out:
        code = 124
    elif limited:
        code = 125
    else:
        code = process.returncode
    return Execution(
        code,
        buffers["stdout"].decode(errors="replace"),
        buffers["stderr"].decode(errors="replace"),
        timed_out=timed_out,
        output_limited=limited,
    )


def _structured_values(value: Any):
    if isinstance(value, dict):
        yield value
        for key in ("structured_output", "proposal", "result", "output", "content", "message", "item", "text"):
            if key in value:
                yield from _structured_values(value[key])
    elif isinstance(value, list):
        for item in reversed(value):
            yield from _structured_values(item)
    elif isinstance(value, str):
        try:
            parsed = json.loads(value)
        except json.JSONDecodeError:
            return
        yield from _structured_values(parsed)


def parse_output(provider: str, output: str) -> dict[str, Any]:
    documents: list[Any] = []
    try:
        documents.append(json.loads(output))
    except json.JSONDecodeError:
        for line in output.splitlines():
            try:
                documents.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    for document in reversed(documents):
        for candidate in _structured_values(document):
            try:
                return validate_result(candidate)
            except ValueError:
                continue
    raise ValueError(f"{provider} did not emit a valid structured proposal")


def classify_failure(execution: Execution) -> tuple[str, bool]:
    text = f"{execution.stderr}\n{execution.stdout}".lower()
    if any(token in text for token in ("not logged in", "unauthorized", "authentication", "credential", "keychain")):
        return "execution-context-auth-unavailable", False
    if any(token in text for token in ("unknown option", "unrecognized option", "invalid config", "model not found")):
        return "provider-config-error", False
    if execution.timed_out:
        return "provider-timeout", True
    if execution.output_limited:
        return "provider-output-too-large", True
    return "provider-exit", True
