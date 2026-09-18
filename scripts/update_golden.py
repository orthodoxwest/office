#!/usr/bin/env python3
"""Merge and publish golden updates. Run the workflow revision's copy, never the PR's."""

import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys

GOLDEN = Path("internal/e2e/testdata/golden")
FILENAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]*\.(txt|md|json)")


def git(*args, check=True):
    return subprocess.run(
        ["git", "-c", "core.hooksPath=/dev/null", *args],
        check=check, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
    )


def output(name, value):
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as handle:
        handle.write(f"{name}={value}\n")


def regular_files(directory):
    files = list(directory.iterdir())
    for path in files:
        if not stat.S_ISREG(path.lstat().st_mode) or not FILENAME.fullmatch(path.name):
            raise ValueError(f"Expected a flat directory of regular golden files: {path}")
    return files


def check_destination():
    # Check every ancestor: checking only golden/ misses symlinked parents.
    for path in [*reversed(GOLDEN.parents), GOLDEN]:
        if not stat.S_ISDIR(path.lstat().st_mode):
            raise ValueError(f"Golden directory must have real directory ancestors: {path}")
    return regular_files(GOLDEN)


def merge_base():
    check_destination()
    for kind in ("HEAD", "BASE"):
        ref = os.environ[f"{kind}_REF"]
        git("fetch", "origin", f"+refs/heads/{ref}:refs/remotes/origin/{ref}")
        if git("rev-parse", f"refs/remotes/origin/{ref}").stdout.strip() != os.environ[f"{kind}_SHA"]:
            raise ValueError(f"{kind.title()} branch moved; re-add the update-golden label to retry.")
    if git("rev-parse", "HEAD").stdout.strip() != os.environ["HEAD_SHA"]:
        raise ValueError("Checkout does not match the requested PR head.")
    git("config", "user.name", "github-actions[bot]")
    git("config", "user.email", "github-actions[bot]@users.noreply.github.com")
    result = git("merge", "--no-commit", "--no-edit", os.environ["BASE_SHA"], check=False)
    if result.returncode:
        conflicts = git("diff", "--name-only", "--diff-filter=U", "-z").stdout.rstrip("\0").split("\0")
        if conflicts == [""]:
            raise ValueError(f"Merge failed without file conflicts: {result.stderr}")
        unexpected = [p for p in conflicts if Path(p).parent != GOLDEN or not FILENAME.fullmatch(Path(p).name)]
        if unexpected:
            git("merge", "--abort")
            raise ValueError(f"Resolve conflicts outside generated goldens by hand: {unexpected}")
        # Restore the head's version (including deletions) solely to clear the
        # index. Generation or the complete artifact supplies the final files.
        for path in conflicts:
            if git("cat-file", "-e", f"HEAD:{path}", check=False).returncode:
                git("rm", "-f", "--", path)
            else:
                git("restore", "--source=HEAD", "--staged", "--worktree", "--", path)
    check_destination()


def publish(artifact):
    if not stat.S_ISDIR(artifact.lstat().st_mode):
        raise ValueError("Artifact must be a real directory.")
    candidates = regular_files(artifact)
    if not 1 <= len(candidates) <= 1000:
        raise ValueError("Expected between 1 and 1000 golden files.")
    for path in candidates:
        if path.stat().st_size > 8 * 1024 * 1024:
            raise ValueError(f"Golden file exceeds 8 MiB: {path}")
        if "\0" in path.read_text(encoding="utf-8"):
            raise ValueError(f"Golden file contains binary content: {path}")

    merge_base()
    # Replace the complete set, preserving deletions from regeneration.
    for path in check_destination():
        path.unlink()
    for path in candidates:
        shutil.copyfile(path, GOLDEN / path.name)
    git("add", "-f", "-A", "--", str(GOLDEN))
    if git("rev-parse", "-q", "--verify", "MERGE_HEAD", check=False).returncode == 0:
        git("commit", "--no-edit")
    elif git("diff", "--cached", "--quiet", check=False).returncode:
        git("commit", "-m", "chore: regenerate golden files")
    changed = git("rev-parse", "HEAD").stdout.strip() != os.environ["HEAD_SHA"]
    if changed:
        # The result descends from HEAD_SHA; the lease also rejects a concurrent rewind.
        ref = f"refs/heads/{os.environ['HEAD_REF']}"
        git("push", f"--force-with-lease={ref}:{os.environ['HEAD_SHA']}", "origin", f"HEAD:{ref}")
    output("pushed", str(changed).lower())


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["merge"]:
            merge_base()
        elif len(sys.argv) == 3 and sys.argv[1] == "publish":
            publish(Path(sys.argv[2]))
        else:
            raise ValueError("Usage: update_golden.py merge | publish ARTIFACT_DIR")
    except (ValueError, OSError, subprocess.CalledProcessError) as exc:
        print(getattr(exc, "stderr", None) or str(exc), file=sys.stderr)
        sys.exit(1)
