"""Atomic on-disk state for diurnal agent runs."""

from __future__ import annotations

import json
import os
import tempfile
import time
from pathlib import Path
from typing import Any


def now(clock=time.time) -> int:
    return int(clock())


def atomic_bytes(path: Path, payload: bytes, mode: int = 0o600) -> None:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, "wb") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def atomic_json(path: Path, value: Any) -> None:
    atomic_bytes(path, (json.dumps(value, indent=2, sort_keys=True, ensure_ascii=False) + "\n").encode())


def read_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def create_lease(path: Path, value: dict[str, Any]) -> bool:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        return False
    try:
        payload = (json.dumps(value, sort_keys=True) + "\n").encode()
        os.write(fd, payload)
        os.fsync(fd)
    finally:
        os.close(fd)
    return True


def active_run(base: Path) -> Path:
    pointer = base / "current.json"
    if not pointer.exists():
        raise FileNotFoundError("no planned agent run; run plan first")
    run = Path(read_json(pointer)["run"])
    if not run.is_dir():
        raise FileNotFoundError(f"agent run no longer exists: {run}")
    return run


def jobs(run: Path) -> list[dict[str, Any]]:
    return [read_json(path) for path in sorted((run / "jobs").glob("*.json"))]
