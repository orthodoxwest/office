#!/usr/bin/env python3
"""The apps' release notes, gathered from the note files pull requests add under release-notes/.

A pull request with a notable change a reader will see adds one file, release-notes/<slug>.txt,
holding a sentence or two for the stores' "What's new" (release-notes/README.md). Most pull
requests add none. A first line "Platforms: android" or "Platforms: ios" keeps a note to one app;
without it a note goes to both.

A platform's release carries every note file present at the commit being released that was not
present at the commit the platform already has out, so a note goes out once on each platform and
is left out of that platform's later releases. What a platform has out comes from its store when
the script can read it (Play's production track, the App Store's live version), otherwise from
the platform's newest tag; each promotion run tags what its store has fully out
(android/0.1.<code>, ios/<version>) as a record. A release with no new notes says DEFAULT_NOTES.

  check [--ref REF]         Every note file (working tree, or REF) parses, names known platforms
                            and fits the stores it targets.
  record --platform P --version V --build N
                            Tag build N's commit as P/V (the promotion workflows' last step, which
                            alone holds the token that can push tags).
  show --platform P         The notes P's next release would carry, as the store gets them:
       [--base REF]         since REF (default: P's newest tag reachable from --head; none, all),
       [--head REF]         up to REF (default HEAD),
       [--limit N]          in at most N characters (default: the platform's store limit).
"""

from __future__ import annotations

import argparse
import base64
import os
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
NOTES_DIR = "release-notes"
SUFFIX = ".txt"
PLATFORMS = ("android", "ios")
# Google Play's "What's new" takes 500 characters, App Store Connect's 4,000.
LIMITS = {"android": 500, "ios": 4000}
DEFAULT_NOTES = "Corrections to the texts and improvements to the app."
MORE = "And other improvements."
TAG_PREFIX = {"android": "android/", "ios": "ios/"}


def git(*args: str, check: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(["git", "-C", str(ROOT), *args], capture_output=True, text=True, check=check)


# --- One note -----------------------------------------------------------------------------


class NoteError(ValueError):
    pass


def parse(text: str) -> tuple[frozenset[str], str]:
    """A note file's platforms and its text (paragraphs kept, lines within one joined)."""
    lines = text.lstrip("\ufeff").strip().splitlines()
    platforms = frozenset(PLATFORMS)
    if lines and lines[0].lower().startswith("platforms:"):
        named = [p.strip().lower() for p in lines[0].split(":", 1)[1].split(",") if p.strip()]
        unknown = [p for p in named if p not in PLATFORMS]
        if unknown or not named:
            raise NoteError(f"Platforms must be one or more of {', '.join(PLATFORMS)}, not {lines[0]!r}")
        platforms = frozenset(named)
        lines = lines[1:]
    paragraphs, current = [], []
    for line in lines + [""]:
        if line.strip():
            current.append(line.strip())
        elif current:
            paragraphs.append(" ".join(current))
            current = []
    body = "\n\n".join(paragraphs)
    if not body:
        raise NoteError("The note has no text")
    return platforms, body


def compose(notes: list[str], limit: int) -> str:
    """The notes as one text of at most `limit` characters, newest first. Older notes that do not
    fit are dropped for MORE; with none, DEFAULT_NOTES."""
    if not notes:
        return DEFAULT_NOTES
    for keep in range(len(notes), 0, -1):
        text = "\n\n".join(notes[:keep] + ([MORE] if keep < len(notes) else []))
        if len(text) <= limit:
            return text
    return notes[0][:limit]  # the newest note alone, cut only by a --limit below Play's


# --- The notes between two commits ----------------------------------------------------------


def files_at(commit: str) -> set[str]:
    """The note files at `commit`. A commit git cannot read stops the run: taking it as having no
    notes would send every shipped note again."""
    r = git("ls-tree", "-r", "-z", "--name-only", commit, "--", NOTES_DIR + "/", check=False)
    if r.returncode != 0:
        sys.exit(f"::error::Cannot read {NOTES_DIR}/ at {commit}: {r.stderr.strip()}")
    return {p for p in r.stdout.split("\0") if p.endswith(SUFFIX)}


def diff_notes(base: str, head: str) -> tuple[set[str], set[str]]:
    """(added, modified) note files between base and head. A renamed note counts as the note it was,
    with a high similarity threshold so deleting a shipped note and adding a similar new one does
    not pair them."""
    files_at(base)  # stops on a base git cannot read
    r = git("diff", "-z", "--name-status", "--find-renames=90%", base, head,
            "--", NOTES_DIR + "/", check=False)
    if r.returncode != 0:
        sys.exit(f"::error::Cannot compare {NOTES_DIR}/ between {base} and {head}: {r.stderr.strip()}")
    tokens = r.stdout.split("\0")
    if tokens and tokens[-1] == "":
        tokens.pop()
    added, modified = set(), set()
    i = 0
    while i < len(tokens):
        status = tokens[i]
        if status[0] in ("R", "C"):
            i += 3
            continue
        path = tokens[i + 1]
        if path.endswith(SUFFIX):
            if status == "A":
                added.add(path)
            elif status == "M":
                modified.add(path)
        i += 2
    return added, modified


def added_at(path: str, head: str, modified: bool = False) -> int:
    """When head's first-parent history gained the note file (a merge's time, not its branch's),
    or when it was last changed if it gained this platform in an edit."""
    args = ["log", "--first-parent", "-1", "--no-renames", "--format=%ct"]
    if not modified:
        args.append("--diff-filter=A")
    r = git(*args, head, "--", path, check=False)
    return int(r.stdout.strip() or 0)


def read_note(head: str, path: str) -> str:
    r = subprocess.run(["git", "-C", str(ROOT), "show", f"{head}:{path}"], capture_output=True, check=True)
    try:
        return r.stdout.decode("utf-8")
    except UnicodeDecodeError as e:
        raise NoteError(f"not UTF-8 ({e})") from e


def notes_for(platform: str, base: str | None, head: str) -> list[str]:
    """The texts of the notes for `platform` added between base and head (base None: all), newest first."""
    if base is None:
        added, modified = files_at(head), set()
    else:
        added, modified = diff_notes(base, head)
    found = []
    for path in added:
        try:
            platforms, body = parse(read_note(head, path))
        except NoteError as e:
            print(f"::warning::{path} skipped: {e}", file=sys.stderr)
            continue
        if platform in platforms:
            found.append((added_at(path, head), path, body))
    for path in modified:
        try:
            platforms, body = parse(read_note(head, path))
            base_platforms, _ = parse(read_note(base, path))
        except NoteError as e:
            print(f"::warning::{path} skipped: {e}", file=sys.stderr)
            continue
        if platform in platforms and platform not in base_platforms:
            found.append((added_at(path, head, modified=True), path, body))
    return [body for _, _, body in sorted(found, key=lambda f: (-f[0], f[1]))]


def release_text(platform: str, base: str | None, head: str, limit: int | None = None) -> str:
    return compose(notes_for(platform, base, head), limit or LIMITS[platform])


# --- Commits, build numbers and tags --------------------------------------------------------


def commit_of(number: int) -> str | None:
    """master's commit whose commit count is the build number (how the Android version code and
    the iOS build number are made)."""
    chain = git("rev-list", "--first-parent", "HEAD").stdout.split()

    def count(i: int) -> int:
        return int(git("rev-list", "--count", chain[i]).stdout)

    lo, hi = 0, len(chain) - 1  # counts fall strictly from chain[0] (HEAD) back
    while lo <= hi:
        mid = (lo + hi) // 2
        c = count(mid)
        if c == number:
            return chain[mid]
        lo, hi = (mid + 1, hi) if c > number else (lo, mid - 1)
    return None


def latest_tag(platform: str, head: str = "HEAD") -> str | None:
    """The commit of the platform's newest tag in head's history, or None before its first."""
    r = git("describe", "--tags", "--abbrev=0", "--match", TAG_PREFIX[platform] + "*", head, check=False)
    if r.returncode != 0:
        return None
    return git("rev-list", "-n", "1", r.stdout.strip()).stdout.strip() or None


def has_tag(platform: str, version: str) -> bool:
    """Whether the platform's tag for `version` already exists locally."""
    name = TAG_PREFIX[platform] + version
    return git("rev-list", "-n", "1", f"refs/tags/{name}", check=False).returncode == 0


def push_args() -> list[str]:
    """git push to origin; in Actions, with the job's GITHUB_TOKEN as a header for this push alone,
    so the checkout keeps no credentials (persist-credentials: false)."""
    token, repo = os.environ.get("GITHUB_TOKEN"), os.environ.get("GITHUB_REPOSITORY")
    if not (token and repo):
        return ["push", "origin"]
    basic = base64.b64encode(f"x-access-token:{token}".encode()).decode()
    return ["-c", f"http.https://github.com/.extraheader=AUTHORIZATION: basic {basic}",
            "push", f"https://github.com/{repo}.git"]


def record(platform: str, version: str, commit: str, push: bool = True) -> None:
    """Tag `commit` as the platform's release `version`, once. A failure only warns: the tag is a
    record, and the store, not the tag, decides the next release's notes where it can be read."""
    name = TAG_PREFIX[platform] + version
    have = git("rev-list", "-n", "1", f"refs/tags/{name}", check=False)
    if have.returncode == 0:
        if have.stdout.strip() != commit:
            print(f"::warning::Tag {name} is on {have.stdout.strip()[:7]}, not {commit[:7]}; left as it is")
        return
    made = git("tag", name, commit, check=False)
    pushed = git(*push_args(), f"refs/tags/{name}", check=False) if push and made.returncode == 0 else made
    if made.returncode != 0 or pushed.returncode != 0:
        print(f"::warning::Could not tag {commit[:7]} as {name}: {(made.stderr or pushed.stderr).strip()}")
    else:
        print(f"Tagged {commit[:7]} as {name}")


# --- Check ----------------------------------------------------------------------------------


def read_worktree_note(path: str) -> str:
    try:
        return (ROOT / path).read_bytes().decode("utf-8")
    except UnicodeDecodeError as e:
        raise NoteError(f"not UTF-8 ({e})") from e


def check(head: str | None = None) -> list[str]:
    """What is wrong with the note files (working tree by default, or commit `head`), one line each."""
    if head is None:
        notes_root = ROOT / NOTES_DIR
        all_paths = sorted(
            p.relative_to(ROOT).as_posix()
            for p in notes_root.rglob("*")
            if p.is_file()
        ) if notes_root.is_dir() else []
        reader = read_worktree_note
    else:
        r = git("ls-tree", "-r", "-z", "--name-only", head, "--", NOTES_DIR + "/", check=False)
        if r.returncode != 0:
            sys.exit(f"::error::Cannot read {NOTES_DIR}/ at {head}: {r.stderr.strip()}")
        all_paths = sorted(p for p in r.stdout.split("\0") if p)
        reader = lambda path: read_note(head, path)

    problems = []
    for path in all_paths:
        if path.endswith(SUFFIX):
            try:
                platforms, body = parse(reader(path))
            except NoteError as e:
                problems.append(f"{path}: {e}")
            else:
                limit = min(LIMITS[p] for p in platforms)
                if len(body) > limit:
                    store = "Google Play" if limit == LIMITS["android"] else "the App Store"
                    problems.append(f"{path}: {len(body)} characters; a note must fit {store}'s {limit}")
        if path.count("/") != 1:
            problems.append(f"{path}: a note sits directly in {NOTES_DIR}/")
        elif not path.endswith(SUFFIX) and pathlib.PurePosixPath(path).name != "README.md":
            problems.append(f"{path}: a note file ends in {SUFFIX}")
    return problems


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="mode", required=True)
    chk = sub.add_parser("check")
    chk.add_argument("--ref", default="", help="commit to check (default: the working tree)")
    rec = sub.add_parser("record", help="tag the commit of build --build as the platform's --version")
    rec.add_argument("--platform", choices=PLATFORMS, required=True)
    rec.add_argument("--version", required=True)
    rec.add_argument("--build", type=int, required=True)
    show = sub.add_parser("show")
    show.add_argument("--platform", choices=PLATFORMS, required=True)
    show.add_argument("--base", default="")
    show.add_argument("--head", default="HEAD")
    show.add_argument("--limit", type=int, default=0)
    args = ap.parse_args()

    if args.mode == "check":
        problems = check(args.ref or None)
        for p in problems:
            print(f"::error::{p}")
        sys.exit(1 if problems else 0)
    if args.mode == "record":
        commit = commit_of(args.build)
        if commit is None:
            print(f"::warning::No master commit has build number {args.build}; nothing tagged")
        else:
            record(args.platform, args.version, commit)
        return
    base = args.base or latest_tag(args.platform, args.head)
    print(release_text(args.platform, base, args.head, args.limit or None))


if __name__ == "__main__":
    main()
