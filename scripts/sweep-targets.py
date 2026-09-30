#!/usr/bin/env python3
"""Delete Cargo target/ directories in idle checkouts of this repository.

Covers the main checkout and every linked worktree that `git worktree list`
reports. A checkout is idle when nothing in its target/, its working tree or
its HEAD reflog (commits, checkouts, resets) has changed within --hours. Clean
rebuilds are cheap, so idle build output is not worth its disk space.
"""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parent.parent
# Working-tree directories that are not source activity.
SKIP_DIRS = {".git", "target", "node_modules", "__pycache__"}


def worktrees(root):
    out = subprocess.run(["git", "-C", str(root), "worktree", "list", "--porcelain"],
                         check=True, capture_output=True, text=True).stdout
    return [Path(line.removeprefix("worktree ")) for line in out.splitlines()
            if line.startswith("worktree ")]


def touched_since(path, cutoff, max_depth=None, skip_nested=False):
    """True when any file or directory under path has an mtime after cutoff."""
    base = len(path.parts)
    for dirpath, dirnames, filenames in os.walk(path):
        current = Path(dirpath)
        if skip_nested:
            # A nested worktree reports its own activity.
            dirnames[:] = [d for d in dirnames
                           if d not in SKIP_DIRS and not (current / d / ".git").exists()]
        if max_depth is not None and len(current.parts) - base >= max_depth:
            dirnames.clear()
        for name in [*filenames, *dirnames, "."]:
            try:
                if (current / name).lstat().st_mtime > cutoff:
                    return True
            except FileNotFoundError:
                pass
    return False


def git_touched_since(worktree, cutoff):
    out = subprocess.run(["git", "-C", str(worktree), "rev-parse", "--absolute-git-dir"],
                         capture_output=True, text=True)
    if out.returncode != 0:
        return False
    # Entry timestamps, not file mtimes: gc and branch renames rewrite reflogs.
    try:
        last = (Path(out.stdout.strip()) / "logs/HEAD").read_text().splitlines()[-1]
    except (FileNotFoundError, IndexError):
        return False
    return int(last.split("\t", 1)[0].split()[-2]) > cutoff


def is_active(worktree, cutoff):
    return (touched_since(worktree / "target", cutoff, max_depth=3)
            or git_touched_since(worktree, cutoff)
            or touched_since(worktree, cutoff, skip_nested=True))


def size(path):
    total = 0
    for dirpath, _, filenames in os.walk(path):
        for name in filenames:
            try:
                total += (Path(dirpath) / name).lstat().st_size
            except FileNotFoundError:
                pass
    return total


def sweep(root, hours, dry_run=False, log=print):
    cutoff = time.time() - hours * 3600
    freed = 0
    for worktree in worktrees(root):
        target = worktree / "target"
        if not target.is_dir():
            continue
        if is_active(worktree, cutoff):
            log(f"keep   {worktree}")
            continue
        bytes_ = size(target)
        freed += bytes_
        log(f"{'would sweep' if dry_run else 'sweep'}  {worktree} ({bytes_ / 2**30:.1f} GiB)")
        if not dry_run:
            shutil.rmtree(target)
    log(f"{'would free' if dry_run else 'freed'} {freed / 2**30:.1f} GiB")
    return freed


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--hours", type=float, default=72,
                        help="idle time before a target/ is swept (default 72)")
    parser.add_argument("--dry-run", action="store_true", help="report without deleting")
    args = parser.parse_args(argv)
    sweep(ROOT, args.hours, args.dry_run)


if __name__ == "__main__":
    sys.exit(main())
