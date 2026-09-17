#!/usr/bin/env python3
"""Focused tests for the update-golden label workflow.

Regression coverage for PR #426: the workflow used `on: pull_request`,
whose runs are tied to the ephemeral refs/pull/N/merge commit. GitHub
cannot compute that merge while the PR is conflicted, so re-adding the
label to a conflicted PR silently started no run at all (the
pull_request_target-based UX workflow fired for the same event). The
workflow must therefore stay on `pull_request_target`, keep PR-code
execution in credential-free jobs, and merge the base branch through an
explicit refspec rather than a possibly stale tracking ref.
"""

import pathlib
import re
import shutil
import subprocess
import tempfile
import unittest

WORKFLOW = (
    pathlib.Path(__file__).resolve().parent.parent
    / ".github"
    / "workflows"
    / "update-golden.yml"
)
GOLDEN_PREFIX = "internal/e2e/testdata/golden/"


def read_workflow():
    return WORKFLOW.read_text(encoding="utf-8")


def job_section(text, name):
    """Return the raw text of one top-level job body."""
    starts = [match for match in re.finditer(r"(?m)^  (\S+):\s*$", text)]
    for index, match in enumerate(starts):
        if match.group(1) == name:
            begin = match.end()
            end = starts[index + 1].start() if index + 1 < len(starts) else len(text)
            return text[begin:end]
    raise AssertionError(f"job {name!r} not found")


def run_blocks(text):
    """Yield embedded shell scripts from `run:` steps (block and inline)."""
    lines = text.splitlines()
    index = 0
    while index < len(lines):
        match = re.match(r"^(\s*)run:\s*(\|)?\s*(.*?)\s*$", lines[index])
        if not match:
            index += 1
            continue
        indent, pipe, inline = match.groups()
        if pipe:
            base = len(indent)
            body = []
            index += 1
            while index < len(lines) and (
                lines[index].strip() == "" or len(lines[index]) - len(lines[index].lstrip()) > base
            ):
                body.append(lines[index][base + 2 :] if lines[index].strip() else "")
                index += 1
            yield "\n".join(body)
        else:
            yield inline
            index += 1


class UpdateGoldenWorkflowTest(unittest.TestCase):
    def test_label_trigger_fires_on_conflicted_prs(self):
        text = read_workflow()
        self.assertRegex(text, r"(?m)^  pull_request_target:\s*$")
        header = text.split("jobs:")[0] if "jobs:" in text else text
        self.assertIn("labeled", header)
        # A bare `pull_request:` trigger (as opposed to pull_request_target or
        # github.event.pull_request references) would silently skip conflicted
        # PRs again.
        self.assertIsNone(
            re.search(r"(?m)^\s*pull_request:\s*(#.*)?$", text),
            "update-golden must trigger on pull_request_target, not pull_request",
        )

    def test_expected_jobs_exist(self):
        text = read_workflow()
        for name in ("authorize", "generate", "commit", "report"):
            with self.subTest(job=name):
                job_section(text, name)

    def test_pr_code_never_runs_with_write_access(self):
        text = read_workflow()
        generate = job_section(text, "generate")
        self.assertIn("contents: read", generate)
        self.assertNotIn("contents: write", generate)
        self.assertIn("persist-credentials: false", generate)
        for command in ("make golden", "go test"):
            self.assertIn(command, generate)
        commit = job_section(text, "commit")
        self.assertIn("contents: write", commit)
        for command in ("make golden", "go test", "make validate", "npm "):
            self.assertNotIn(
                command, commit, f"write-access job must not execute PR code via {command!r}"
            )

    def test_merge_uses_explicit_refspec_not_tracking_ref(self):
        text = read_workflow()
        # `git fetch origin <branch>` only refreshes FETCH_HEAD, so merging
        # origin/<branch> afterwards can merge a stale commit. The workflow
        # must maintain the tracking ref with an explicit refspec first.
        self.assertIn(
            "+refs/heads/$BASE_REF:refs/remotes/origin/$BASE_REF", text
        )
        generate = job_section(text, "generate")
        self.assertIn('git merge --no-edit "origin/$BASE_REF"', generate)

    def test_only_golden_conflicts_resolve_automatically(self):
        text = read_workflow()
        for name in ("generate", "commit"):
            with self.subTest(job=name):
                section = job_section(text, name)
                self.assertIn(f"grep -v '^{GOLDEN_PREFIX}'", section)

    def test_write_job_validates_untrusted_artifact(self):
        commit = job_section(read_workflow(), "commit")
        self.assertIn("golden-candidates", commit)
        self.assertIn(GOLDEN_PREFIX, commit)
        self.assertIn("*.txt", commit)

    def test_noop_reports_honestly_instead_of_claiming_a_push(self):
        text = read_workflow()
        self.assertIn("detail=noop", text)
        self.assertIn("nothing to push", text)

    def test_embedded_shell_parses(self):
        if shutil.which("bash") is None:
            self.skipTest("bash not available")
        blocks = [
            re.sub(r"\$\{\{.*?\}\}", "__EXPR__", block) for block in run_blocks(read_workflow())
        ]
        self.assertGreater(len(blocks), 3, "expected several embedded shell scripts")
        for index, block in enumerate(blocks):
            with self.subTest(block=index):
                with tempfile.NamedTemporaryFile(
                    "w", suffix=".sh", delete=False, encoding="utf-8"
                ) as handle:
                    handle.write(block + "\n")
                    path = handle.name
                try:
                    completed = subprocess.run(
                        ["bash", "-n", path],
                        capture_output=True,
                        text=True,
                        timeout=30,
                    )
                finally:
                    pathlib.Path(path).unlink(missing_ok=True)
                self.assertEqual(
                    completed.returncode,
                    0,
                    f"shell syntax error in run block {index}:\n{completed.stderr}\n{block}",
                )


if __name__ == "__main__":
    unittest.main()
