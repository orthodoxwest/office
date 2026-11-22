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
the platform's newest tag; each promotion tags what its store has out (android/0.1.<code>,
ios/<version>) as a record. A release with no new notes says DEFAULT_NOTES.

  check                     Every note file at HEAD parses, names known platforms and fits Play.
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
    lines = text.strip().splitlines()
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
    return notes[0][:limit]  # check keeps every note within Play's limit, so this is not reached


# --- The notes between two commits ----------------------------------------------------------


def files_at(commit: str) -> set[str]:
    r = git("ls-tree", "-r", "--name-only", commit, "--", NOTES_DIR + "/", check=False)
    return {p for p in r.stdout.split() if p.endswith(SUFFIX)} if r.returncode == 0 else set()


def added_at(path: str, head: str) -> int:
    """When the note file was added in head's history (commit time), for the order of the notes."""
    r = git("log", "-1", "--diff-filter=A", "--format=%ct", head, "--", path, check=False)
    return int(r.stdout.strip() or 0)


def notes_for(platform: str, base: str | None, head: str) -> list[str]:
    """The texts of the notes for `platform` present at head and not at base (base None: all), newest first."""
    new = files_at(head) - (files_at(base) if base else set())
    found = []
    for path in new:
        try:
            platforms, body = parse(git("show", f"{head}:{path}").stdout)
        except NoteError as e:
            print(f"::warning::{path} skipped: {e}", file=sys.stderr)
            continue
        if platform in platforms:
            found.append((added_at(path, head), path, body))
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
    have = git("rev-list", "-n", "1", name, check=False)
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


def check(head: str = "HEAD") -> list[str]:
    """What is wrong with the note files at head, one line each."""
    problems = []
    for path in sorted(files_at(head)):
        try:
            _, body = parse(git("show", f"{head}:{path}").stdout)
        except NoteError as e:
            problems.append(f"{path}: {e}")
            continue
        if len(body) > LIMITS["android"]:
            problems.append(f"{path}: {len(body)} characters; a note must fit Google Play's {LIMITS['android']}")
    r = git("ls-tree", "-r", "--name-only", head, "--", NOTES_DIR + "/", check=False)
    for path in r.stdout.split():
        if not path.endswith(SUFFIX) and pathlib.PurePosixPath(path).name != "README.md":
            problems.append(f"{path}: a note file ends in {SUFFIX}")
    return problems


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="mode", required=True)
    sub.add_parser("check")
    show = sub.add_parser("show")
    show.add_argument("--platform", choices=PLATFORMS, required=True)
    show.add_argument("--base", default="")
    show.add_argument("--head", default="HEAD")
    show.add_argument("--limit", type=int, default=0)
    args = ap.parse_args()

    if args.mode == "check":
        problems = check()
        for p in problems:
            print(f"::error::{p}")
        sys.exit(1 if problems else 0)
    base = args.base or latest_tag(args.platform, args.head)
    print(release_text(args.platform, base, args.head, args.limit or None))


if __name__ == "__main__":
    main()
