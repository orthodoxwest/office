#!/usr/bin/env python3
"""Exercise the golden updater against disposable local Git repositories."""

import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest
from unittest.mock import patch

HELPER = Path(__file__).with_name("update_golden.py").resolve()
GOLDEN = Path("internal/e2e/testdata/golden")


def authorize(stale_base, current_base, permission="write"):
    workflow = HELPER.parent.parent / ".github/workflows/update-golden.yml"
    script = re.search(r"(?m)^          script: \|\n((?:            .*\n|\n)+)", workflow.read_text())[1]
    # Execute the workflow's JavaScript, including its API lookup and outputs.
    harness = """
        const input = JSON.parse(require('fs').readFileSync(0, 'utf8'));
        const result = { outputs: {}, errors: [], refs: [] };
        const context = { actor: 'maintainer', repo: {owner: 'test', repo: 'office'},
            payload: {pull_request: {base: {ref: 'master', sha: input.stale_base}}} };
        const github = { rest: {
            repos: { getCollaboratorPermissionLevel: async () => ({data: {permission: input.permission}}) },
            git: { getRef: async args => {
                result.refs.push(args.ref);
                return {data: {object: {sha: input.current_base}}};
            } },
        } };
        const core = {setOutput: (k,v) => result.outputs[k] = v, setFailed: e => result.errors.push(e)};
        const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
        new AsyncFunction('github', 'context', 'core', input.script)(github, context, core)
            .then(() => process.stdout.write(JSON.stringify(result)));
    """
    result = subprocess.run(
        ["node", "-e", harness], check=True, text=True, capture_output=True,
        input=json.dumps(dict(script=textwrap.dedent(script), stale_base=stale_base,
                             current_base=current_base, permission=permission)),
    )
    return json.loads(result.stdout)


class GoldenUpdateTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.remote = self.root / "remote.git"
        self.source = self.root / "source"
        self.env = dict(os.environ, GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
        self.git(self.root, "init", "--bare", str(self.remote))
        self.git(self.root, "init", "-b", "master", str(self.source))
        self.git(self.source, "config", "user.name", "Test")
        self.git(self.source, "config", "user.email", "test@example.com")
        self.git(self.source, "remote", "add", "origin", str(self.remote))
        self.change({str(GOLDEN / "a.txt"): "original\n", str(GOLDEN / "b.txt"): "keep\n", "data.txt": "original\n"})
        self.git(self.source, "branch", "feature")
        self.git(self.source, "push", "origin", "master", "feature")
        self.artifact = self.root / "artifact"
        self.artifact.mkdir()
        (self.artifact / "a.txt").write_text("original\n")
        (self.artifact / "b.txt").write_text("keep\n")
        self.work = self.checkout("work")
        self.refresh_event()

    def git(self, cwd, *args):
        return subprocess.run(
            ["git", "-c", "core.hooksPath=/dev/null", *args], cwd=cwd,
            env=self.env, check=True, capture_output=True, text=True,
        ).stdout.strip()

    def change(self, files):
        for name, text in files.items():
            path = self.source / name
            if text is None:
                path.unlink()
            else:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(text)
        self.git(self.source, "add", "-A")
        self.git(self.source, "commit", "-m", "fixture change")

    def advance(self, branch, files):
        self.git(self.source, "checkout", branch)
        self.change(files)
        self.git(self.source, "push", "origin", branch)

    def checkout(self, name):
        work = self.root / name
        self.git(self.root, "clone", "--branch", "feature", str(self.remote), str(work))
        return work

    def refresh_event(self):
        self.env.update(
            HEAD_REF="feature", BASE_REF="master",
            HEAD_SHA=self.git(self.source, "rev-parse", "feature"),
            BASE_SHA=self.git(self.source, "rev-parse", "master"),
            GITHUB_OUTPUT=str(self.root / "outputs"),
        )
        self.git(self.work, "fetch", "origin")
        self.git(self.work, "checkout", "--detach", self.env["HEAD_SHA"])

    def run_helper(self, command="publish", work=None, success=True):
        args = [sys.executable, str(HELPER), command]
        if command == "publish":
            args.append(str(self.artifact))
        result = subprocess.run(args, cwd=work or self.work, env=self.env, capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, success, result.stderr)
        return result

    def remote_head(self):
        return self.git(self.remote, "rev-parse", "feature")

    def test_noop_does_not_push(self):
        self.run_helper()
        self.assertEqual(self.remote_head(), self.env["HEAD_SHA"])
        self.assertEqual((self.root / "outputs").read_text(), "pushed=false\n")

    def test_artifact_replaces_files_including_deletions(self):
        (self.artifact / "a.txt").unlink()
        (self.artifact / "new.json").write_text('{"new": true}\n')
        self.run_helper()
        files = self.git(self.remote, "ls-tree", "--name-only", "feature", f"{GOLDEN}/").splitlines()
        self.assertEqual(set(files), {str(GOLDEN / "b.txt"), str(GOLDEN / "new.json")})
        self.assertEqual((self.root / "outputs").read_text(), "pushed=true\n")
        self.assertEqual(self.git(self.remote, "show", "feature:data.txt"), "original")

    def check_merge_roundtrip(self):
        self.refresh_event()
        self.run_helper("merge")
        (self.work / GOLDEN / "a.txt").write_text("regenerated\n")
        (self.artifact / "a.txt").write_text("regenerated\n")
        self.git(self.work, "add", "-A")
        tested_tree = self.git(self.work, "write-tree")
        self.run_helper(work=self.checkout("publisher"))
        self.assertEqual(self.git(self.remote, "rev-parse", "feature^{tree}"), tested_tree)
        self.git(self.remote, "merge-base", "--is-ancestor", self.env["BASE_SHA"], "feature")
        self.git(self.remote, "merge-base", "--is-ancestor", self.env["HEAD_SHA"], "feature")

    def test_golden_conflict_roundtrip_preserves_merged_tree(self):
        self.advance("master", {str(GOLDEN / "a.txt"): "base\n", "base.txt": "base\n"})
        self.advance("feature", {str(GOLDEN / "a.txt"): "head\n", "head.txt": "head\n"})
        self.check_merge_roundtrip()

    def test_clean_merge_roundtrip_preserves_merged_tree(self):
        self.advance("master", {"base.txt": "base\n"})
        self.advance("feature", {"head.txt": "head\n"})
        self.check_merge_roundtrip()

    def test_fast_forward_is_pushed_without_golden_changes(self):
        self.advance("master", {"base.txt": "base\n"})
        self.refresh_event()
        self.run_helper()
        self.assertEqual(self.remote_head(), self.env["BASE_SHA"])
        self.assertEqual((self.root / "outputs").read_text(), "pushed=true\n")

    def test_stale_pr_event_pins_and_merges_current_base(self):
        stale_base = self.env["BASE_SHA"]
        self.advance("master", {str(GOLDEN / "a.txt"): "base\n", "base.txt": "base\n"})
        self.advance("feature", {str(GOLDEN / "a.txt"): "head\n"})
        self.refresh_event()
        result = authorize(stale_base, self.env["BASE_SHA"])
        self.assertEqual(result["refs"], ["heads/master"])
        self.assertEqual(result["errors"], [])
        self.assertNotEqual(result["outputs"]["base_sha"], stale_base)
        self.env["BASE_SHA"] = result["outputs"]["base_sha"]
        (self.artifact / "a.txt").write_text("regenerated\n")
        self.run_helper()
        self.git(self.remote, "merge-base", "--is-ancestor", self.env["BASE_SHA"], "feature")
        self.assertEqual(self.git(self.remote, "show", "feature:base.txt"), "base")

    def test_unauthorized_actor_cannot_pin_base(self):
        result = authorize("stale", "current", permission="read")
        self.assertTrue(result["errors"])
        self.assertEqual(result["refs"], [])
        self.assertEqual(result["outputs"], {})

    def test_non_golden_conflicts_abort_without_push(self):
        self.advance("master", {"data.txt": "base\n"})
        self.advance("feature", {"data.txt": "head\n"})
        self.refresh_event()
        result = self.run_helper(success=False)
        self.assertIn("Resolve conflicts outside generated goldens", result.stderr)
        self.assertEqual(self.remote_head(), self.env["HEAD_SHA"])
        self.assertFalse((self.work / ".git/MERGE_HEAD").exists())

    def test_golden_modify_delete_conflict(self):
        self.advance("master", {str(GOLDEN / "a.txt"): "base\n"})
        self.advance("feature", {str(GOLDEN / "a.txt"): None})
        self.refresh_event()
        (self.artifact / "a.txt").unlink()
        self.run_helper()
        self.assertFalse((self.work / GOLDEN / "a.txt").exists())
        self.git(self.remote, "merge-base", "--is-ancestor", self.env["BASE_SHA"], "feature")

    def test_moved_branch_is_rejected(self):
        for branch in ("master", "feature"):
            with self.subTest(branch=branch):
                self.refresh_event()
                self.advance(branch, {f"{branch}.txt": "advance\n"})
                before = self.remote_head()
                self.assertIn("branch moved", self.run_helper(success=False).stderr)
                self.assertEqual(self.remote_head(), before)

    def test_lease_rejects_head_rewound_after_fetch(self):
        ancestor = self.env["HEAD_SHA"]
        self.advance("feature", {"head.txt": "advance\n"})
        self.refresh_event()
        (self.artifact / "a.txt").write_text("regenerated\n")
        spec = importlib.util.spec_from_file_location("updater", HELPER)
        updater = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(updater)
        real_git = updater.git

        def rewind_before_push(*args, **kwargs):
            if args[0] == "push":
                self.git(self.remote, "update-ref", "refs/heads/feature", ancestor)
            return real_git(*args, **kwargs)

        previous = Path.cwd()
        try:
            os.chdir(self.work)
            with patch.dict(os.environ, self.env), patch.object(updater, "git", rewind_before_push):
                with self.assertRaises(subprocess.CalledProcessError):
                    updater.publish(self.artifact)
        finally:
            os.chdir(previous)
        self.assertEqual(self.remote_head(), ancestor)

    def test_invalid_artifacts_are_rejected_before_merging(self):
        for kind in ("empty", "nested", "symlink", "extension", "large", "binary"):
            with self.subTest(kind=kind):
                shutil.rmtree(self.artifact)
                self.artifact.mkdir()
                path = self.artifact / "a.txt"
                if kind == "nested":
                    path.mkdir()
                elif kind == "symlink":
                    path.symlink_to(self.source / "data.txt")
                elif kind == "extension":
                    (self.artifact / "script.sh").write_text("invalid")
                elif kind == "large":
                    with path.open("wb") as handle:
                        handle.truncate(8 * 1024 * 1024 + 1)
                elif kind == "binary":
                    path.write_bytes(b"\0")
                self.run_helper(success=False)
                self.assertEqual(self.remote_head(), self.env["HEAD_SHA"])

    def test_destination_symlinks_are_rejected_without_writing_outside(self):
        for name in ("internal", str(GOLDEN), str(GOLDEN / "a.txt")):
            with self.subTest(path=name):
                work = self.checkout("symlink-" + name.replace("/", "-"))
                target = work / name
                outside = self.root / ("outside-" + name.replace("/", "-"))
                target.rename(outside)
                target.symlink_to(outside)
                self.run_helper(work=work, success=False)
                original = outside / "e2e/testdata/golden/a.txt" if name == "internal" else outside / "a.txt" if name == str(GOLDEN) else outside
                self.assertEqual(original.read_text(), "original\n")

    def test_symlink_introduced_by_base_merge_is_rejected(self):
        self.git(self.source, "checkout", "master")
        path = self.source / GOLDEN / "a.txt"
        path.unlink()
        path.symlink_to("../../../../data.txt")
        self.git(self.source, "add", "-A")
        self.git(self.source, "commit", "-m", "symlink in base")
        self.git(self.source, "push", "origin", "master")
        self.refresh_event()
        self.run_helper(success=False)
        self.assertEqual((self.work / "data.txt").read_text(), "original\n")
        self.assertEqual(self.remote_head(), self.env["HEAD_SHA"])


if __name__ == "__main__":
    unittest.main()
