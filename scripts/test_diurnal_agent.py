#!/usr/bin/env python3
"""Tests for the durable, advisory-only diurnal agent runner."""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("diurnal-agent.py")
SPEC = importlib.util.spec_from_file_location("diurnal_agent_cli", SCRIPT)
agent = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = agent
SPEC.loader.exec_module(agent)


def proposal(verdict: str = "match") -> dict:
    return {
        "verdict": verdict,
        "confidence": 0.8,
        "rationale": "The bounded witnesses agree.",
        "target_key": None,
        "evidence": [{"claim": "Texts agree", "location": "PDF 42 / proper/example/collect"}],
        "review_questions": [],
    }


class DiurnalAgentTest(unittest.TestCase):
    def make_plan(self, directory: Path, classification: str = "verify-existing") -> tuple[Path, Path, Path]:
        directory.mkdir(parents=True, exist_ok=True)
        input_path = directory / "candidates.json"
        input_path.write_text(json.dumps({
            "candidates": [{
                "candidate_id": "DI/example/42",
                "discovery_classification": classification,
                "source_text": "Bounded witness",
                "source": "diurnal",
                "source_sha256": "a" * 64,
                "source_page": 42,
                "extractor": "pdftotext-layout",
                "raw_text_sha256": "b" * 64,
            }]
        }))
        policy_path = directory / "policy.json"
        policy_path.write_bytes(agent.DEFAULT_POLICY.read_bytes())
        output = directory / "output"
        args = argparse.Namespace(
            input=input_path,
            policy=policy_path,
            repo=directory,
            output=output,
            run_id="test-run",
        )
        with patch.object(agent, "git_head", return_value="test-head"):
            self.assertEqual(agent.cmd_plan(args), 0)
        return agent.store.active_run(output), input_path, policy_path

    def collect_replica_disagreement(
        self, directory: Path, verdicts: tuple[str, str] = ("match", "missing-override")
    ) -> tuple[Path, Path]:
        run, _, policy_path = self.make_plan(directory, "ambiguous-owner/context")
        run_record = agent.store.read_json(run / "run.json")
        policy = agent.load_policy(policy_path)
        executions = [
            agent.providers.Execution(0, json.dumps(proposal(verdict)), "")
            for verdict in verdicts
        ]
        with patch.object(agent, "run_is_stale", return_value=False), patch.object(
            agent.providers, "execute", side_effect=executions
        ):
            self.assertEqual(agent.execute_job(run, agent.store.jobs(run)[0], run_record, policy), "queued")
            self.assertEqual(agent.execute_job(run, agent.store.jobs(run)[0], run_record, policy), "complete")
            self.assertEqual(agent.cmd_collect(argparse.Namespace(
                run=run, output=directory / "output", repo=directory, include_stale=False,
            )), 0)
        return run, policy_path

    @staticmethod
    def enable_claude(policy_path: Path) -> None:
        policy = json.loads(policy_path.read_text())
        policy["providers"]["claude"]["enabled"] = True
        policy_path.write_text(json.dumps(policy))

    def test_schema_file_matches_model_and_validation_is_strict(self):
        self.assertEqual(json.loads(agent.SCHEMA_FILE.read_text()), agent.model.RESULT_SCHEMA)

        def assert_openai_strict_schema(node):
            if not isinstance(node, dict):
                return
            if node.get("type") == "object":
                self.assertEqual(
                    set(node.get("required", [])),
                    set(node.get("properties", {})),
                )
            for child in node.get("properties", {}).values():
                assert_openai_strict_schema(child)
            assert_openai_strict_schema(node.get("items"))

        assert_openai_strict_schema(agent.model.RESULT_SCHEMA)
        self.assertEqual(agent.model.validate_result(proposal())["verdict"], "match")
        unsafe = proposal()
        unsafe["rationale"] = "Run git apply now"
        with self.assertRaisesRegex(ValueError, "mutation"):
            agent.model.validate_result(unsafe)
        missing_evidence = proposal()
        missing_evidence.pop("evidence")
        with self.assertRaises(ValueError):
            agent.model.validate_result(missing_evidence)
        missing_optional = proposal()
        missing_optional.pop("target_key")
        with self.assertRaises(ValueError):
            agent.model.validate_result(missing_optional)

    def test_provider_argv_uses_correct_schema_forms_and_read_only_flags(self):
        policy = agent.load_policy(agent.DEFAULT_POLICY)
        repo, schema = Path("/repo"), Path("/schema.json")
        codex = agent.providers.argv("codex", policy, repo, schema, "prompt")
        grok = agent.providers.argv("grok", policy, repo, schema, "prompt")
        claude_policy = json.loads(json.dumps(policy))
        claude_policy["providers"]["claude"]["enabled"] = True
        claude = agent.providers.argv("claude", claude_policy, repo, schema, "prompt")
        self.assertEqual(codex[codex.index("--output-schema") + 1], str(schema))
        self.assertIn('model_reasoning_effort="medium"', codex)
        self.assertEqual(json.loads(grok[grok.index("--json-schema") + 1]), agent.model.RESULT_SCHEMA)
        self.assertEqual(json.loads(claude[claude.index("--json-schema") + 1]), agent.model.RESULT_SCHEMA)
        self.assertIn("read-only", codex)
        self.assertIn("--no-subagents", grok)
        self.assertIn("plan", claude)
        for command in (codex, grok, claude):
            self.assertNotIn("--always-approve", command)
            self.assertNotIn("--bare", command)
            self.assertNotIn("--dangerously-skip-permissions", command)

    def test_provider_parsers_accept_realistic_wrappers(self):
        payload = json.dumps(proposal())
        codex = "\n".join([
            json.dumps({"type": "thread.started"}),
            json.dumps({"type": "item.completed", "item": {"type": "agent_message", "text": payload}}),
        ])
        claude = json.dumps({"result": "complete", "structured_output": proposal("missing-override")})
        grok = json.dumps({"output": proposal("ambiguous")})
        self.assertEqual(agent.providers.parse_output("codex", codex)["verdict"], "match")
        self.assertEqual(agent.providers.parse_output("claude", claude)["verdict"], "missing-override")
        self.assertEqual(agent.providers.parse_output("grok", grok)["verdict"], "ambiguous")

    def test_execute_inherits_auth_environment_without_logging_it(self):
        policy = agent.load_policy(agent.DEFAULT_POLICY)
        seen = {}

        class Result:
            returncode = 0
            stdout = json.dumps(proposal())
            stderr = ""

        def fake_runner(argv, **kwargs):
            seen.update(kwargs)
            return Result()

        with patch.dict(os.environ, {"DIURNAL_AUTH_SENTINEL": "secret-auth-value"}, clear=False):
            execution = agent.providers.execute(
                "grok", policy, Path("/repo"), Path("/schema"), "prompt", runner=fake_runner
            )
        self.assertEqual(execution.code, 0)
        self.assertEqual(seen["env"]["DIURNAL_AUTH_SENTINEL"], "secret-auth-value")

    def test_real_subprocess_output_is_bounded_before_completion(self):
        execution = agent.providers.run_capped(
            [
                sys.executable,
                "-c",
                "import sys; sys.stdout.write('x' * 200000); sys.stdout.flush()",
            ],
            Path.cwd(),
            timeout=10,
            maximum=1024,
        )
        self.assertTrue(execution.output_limited)
        self.assertEqual(execution.code, 125)
        self.assertLessEqual(len(execution.stdout.encode()) + len(execution.stderr.encode()), 1024)

    def test_plan_makes_safe_job_ids_and_routes_ambiguous_replicas(self):
        with tempfile.TemporaryDirectory() as temporary:
            run, _, _ = self.make_plan(Path(temporary), "ambiguous-owner/context")
            job = agent.store.jobs(run)[0]
            self.assertNotIn("/", job["job_id"])
            self.assertEqual(job["providers"], ["codex", "grok"])
            self.assertTrue((run / "run.json").exists())

    def test_atomic_lease_allows_only_one_owner_and_reap_expires_it(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            lease = base / "lease.json"
            self.assertTrue(agent.store.create_lease(lease, {"token": "one", "expires_at": 1}))
            self.assertFalse(agent.store.create_lease(lease, {"token": "two", "expires_at": 2}))
            run, _, _ = self.make_plan(base / "plan")
            planned_lease = agent.lease_path(run, agent.store.jobs(run)[0]["job_id"])
            agent.store.atomic_json(planned_lease, {"expires_at": 0, "token": "expired"})
            args = argparse.Namespace(run=run, output=base / "plan" / "output", repo=base / "plan")
            self.assertEqual(agent.cmd_reap(args), 0)
            self.assertFalse(planned_lease.exists())

            (run / "leases").rmdir()
            outside = base / "outside-leases"
            outside.mkdir()
            external_lease = outside / "external.json"
            external_lease.write_text('{"expires_at": 0}')
            (run / "leases").symlink_to(outside, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "leases directory must not be a symlink"):
                agent.cmd_reap(args)
            self.assertTrue(external_lease.is_file())

    def test_collection_rejects_external_results_symlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, _, _ = self.make_plan(directory)
            outside = directory / "outside-results"
            outside.mkdir()
            sentinel = outside / "sentinel.json"
            sentinel.write_text('{"private": true}')
            (run / "results").symlink_to(outside, target_is_directory=True)
            args = argparse.Namespace(
                run=run, output=directory / "output", repo=directory,
                include_stale=False,
            )
            with patch.object(agent, "run_is_stale", return_value=False):
                with self.assertRaisesRegex(ValueError, "results directory must not be a symlink"):
                    agent.cmd_collect(args)
            self.assertEqual(sentinel.read_text(), '{"private": true}')

    def test_replica_results_run_separately_and_disagreement_is_collected(self):
        with tempfile.TemporaryDirectory() as temporary:
            run, _, policy_path = self.make_plan(Path(temporary), "ambiguous-owner/context")
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            executions = [
                agent.providers.Execution(0, json.dumps(proposal("match")), ""),
                agent.providers.Execution(0, json.dumps(proposal("missing-override")), ""),
            ]
            with patch.object(agent, "run_is_stale", return_value=False), patch.object(
                agent.providers, "execute", side_effect=executions
            ):
                first = agent.store.jobs(run)[0]
                self.assertEqual(agent.execute_job(run, first, run_record, policy), "queued")
                second = agent.store.jobs(run)[0]
                self.assertEqual(agent.execute_job(run, second, run_record, policy), "complete")
                args = argparse.Namespace(
                    run=run,
                    output=Path(temporary) / "output",
                    repo=Path(temporary),
                    include_stale=False,
                )
                self.assertEqual(agent.cmd_collect(args), 0)
            collection = agent.store.read_json(run / "collection.json")
            self.assertEqual(collection["replica_disagreements"], ["DI/example/42"])
            self.assertEqual(len(collection["results"]), 2)

    def test_auth_failure_is_non_retryable(self):
        with tempfile.TemporaryDirectory() as temporary:
            run, _, policy_path = self.make_plan(Path(temporary))
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            failure = agent.providers.Execution(1, "", "Not logged in: keychain unavailable")
            with patch.object(agent, "run_is_stale", return_value=False), patch.object(
                agent.providers, "execute", return_value=failure
            ):
                agent.execute_job(run, agent.store.jobs(run)[0], run_record, policy)
            job = agent.store.jobs(run)[0]
            self.assertEqual(job["status"], "terminal-failed")
            self.assertEqual(job["provider_attempts"]["codex"], policy["max_attempts"])
            self.assertEqual(job["failures"][0]["error"], "execution-context-auth-unavailable")

    def test_transient_failure_retries_only_to_policy_cap(self):
        with tempfile.TemporaryDirectory() as temporary:
            run, _, policy_path = self.make_plan(Path(temporary))
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            failure = agent.providers.Execution(1, "", "temporary provider outage")
            with patch.object(agent, "run_is_stale", return_value=False), patch.object(
                agent.providers, "execute", return_value=failure
            ):
                agent.execute_job(run, agent.store.jobs(run)[0], run_record, policy)
                self.assertEqual(agent.store.jobs(run)[0]["status"], "queued")
                agent.execute_job(run, agent.store.jobs(run)[0], run_record, policy)
            job = agent.store.jobs(run)[0]
            self.assertEqual(job["status"], "terminal-failed")
            self.assertEqual(job["provider_attempts"]["codex"], 2)

    def test_changed_input_marks_job_stale_without_provider_call(self):
        with tempfile.TemporaryDirectory() as temporary:
            run, input_path, policy_path = self.make_plan(Path(temporary))
            input_path.write_text('{"candidates": []}')
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            with patch.object(agent.providers, "execute") as execute:
                self.assertEqual(
                    agent.execute_job(run, agent.store.jobs(run)[0], run_record, policy),
                    "stale",
                )
            execute.assert_not_called()

    def test_runner_never_changes_decision_or_provenance_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            decisions = directory / "decisions.csv"
            provenance = directory / "provenance.csv"
            decisions.write_text("candidate_id,decision,note\n")
            provenance.write_text("key,status\n")
            before = (decisions.read_bytes(), provenance.read_bytes())
            self.make_plan(directory / "plan")
            self.assertEqual(before, (decisions.read_bytes(), provenance.read_bytes()))

    def test_output_and_run_must_stay_below_repo_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            directory.mkdir(exist_ok=True)
            with self.assertRaisesRegex(ValueError, "must stay below"):
                agent.confined_output(directory / "data" / "texts", directory)
            safe = directory / "output" / "source-reconcile" / "agent"
            self.assertEqual(agent.confined_output(safe, directory), safe.resolve())
            args = argparse.Namespace(
                run=directory / "data" / "review",
                output=safe,
                repo=directory,
            )
            with self.assertRaisesRegex(ValueError, "agent run must stay"):
                agent.resolve_run(args)

    def test_output_symlink_escape_is_rejected_before_any_write(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            outside = directory / "outside"
            outside.mkdir()
            (directory / "output").mkdir()
            (directory / "output" / "escape").symlink_to(outside, target_is_directory=True)
            input_path = directory / "candidates.json"
            input_path.write_text(json.dumps({"candidates": [{
                "candidate_id": "safe", "source": "diurnal", "source_sha256": "a" * 64,
                "source_page": 1, "extractor": "native", "raw_text_sha256": "b" * 64,
            }]}))
            policy_path = directory / "policy.json"
            policy_path.write_bytes(agent.DEFAULT_POLICY.read_bytes())
            with self.assertRaisesRegex(ValueError, "must stay below"):
                agent.cmd_plan(argparse.Namespace(
                    input=input_path, policy=policy_path, repo=directory,
                    output=directory / "output" / "escape" / "agent", run_id="safe-run",
                ))
            self.assertEqual(list(outside.iterdir()), [])

    def test_invalid_policy_fails_before_plan_or_lease_writes(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            input_path = directory / "candidates.json"
            input_path.write_text(json.dumps({"candidates": [{
                "candidate_id": "safe", "source": "diurnal", "source_sha256": "a" * 64,
                "source_page": 1, "extractor": "native", "raw_text_sha256": "b" * 64,
            }]}))
            policy_path = directory / "policy.json"
            policy = json.loads(agent.DEFAULT_POLICY.read_text())
            policy["providers"]["grok"]["max_turns"] = 0
            policy_path.write_text(json.dumps(policy))
            output = directory / "output" / "agents"
            with self.assertRaisesRegex(ValueError, "max_turns"):
                agent.cmd_plan(argparse.Namespace(
                    input=input_path, policy=policy_path, repo=directory, output=output, run_id="safe-run",
                ))
            self.assertFalse(output.exists())
            policy = json.loads(agent.DEFAULT_POLICY.read_text())
            policy["max_attempts"] = "2"
            policy_path.write_text(json.dumps(policy))
            with self.assertRaisesRegex(ValueError, "max_attempts"):
                agent.load_policy(policy_path)
            policy = json.loads(agent.DEFAULT_POLICY.read_text())
            policy["schema_version"] = True
            policy_path.write_text(json.dumps(policy))
            with self.assertRaisesRegex(ValueError, "schema_version"):
                agent.load_policy(policy_path)
            policy["schema_version"] = 1.0
            policy_path.write_text(json.dumps(policy))
            with self.assertRaisesRegex(ValueError, "schema_version"):
                agent.load_policy(policy_path)
            policy = json.loads(agent.DEFAULT_POLICY.read_text())
            policy["providers"]["codex"]["reasoning_effort"] = []
            policy_path.write_text(json.dumps(policy))
            with self.assertRaisesRegex(ValueError, "reasoning_effort"):
                agent.load_policy(policy_path)

    def test_persisted_traversal_job_is_rejected_before_lease_write(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, _, policy_path = self.make_plan(directory)
            job_file = next((run / "jobs").glob("*.json"))
            job = agent.store.read_json(job_file)
            job["job_id"] = "../outside"
            agent.store.atomic_json(job_file, job)
            with self.assertRaisesRegex(ValueError, "job ID"):
                agent.cmd_run(argparse.Namespace(
                    run=run, output=directory / "output", repo=directory, execute=True,
                ))
            self.assertFalse((run / "leases").exists())
            self.assertFalse((directory / "outside").exists())

    def test_persisted_run_paths_are_rejected_before_external_read_or_provider(self):
        with tempfile.TemporaryDirectory() as temporary, tempfile.TemporaryDirectory() as foreign:
            repo = Path(temporary) / "repo"
            run, _, _ = self.make_plan(repo)
            run_path = run / "run.json"
            original = agent.store.read_json(run_path)
            foreign_policy = Path(foreign) / "policy.json"
            foreign_policy.write_bytes(agent.DEFAULT_POLICY.read_bytes())
            args = argparse.Namespace(
                run=run, output=repo / "output", repo=repo, execute=True,
            )

            tampered = json.loads(json.dumps(original))
            tampered["policy_path"] = str(foreign_policy)
            agent.store.atomic_json(run_path, tampered)
            with patch.object(agent.providers, "execute") as execute:
                with self.assertRaisesRegex(ValueError, "policy_path must stay"):
                    agent.cmd_run(args)
            execute.assert_not_called()
            self.assertFalse((run / "leases").exists())

            tampered = json.loads(json.dumps(original))
            tampered["repo"] = str(Path(foreign))
            agent.store.atomic_json(run_path, tampered)
            with patch.object(agent.providers, "execute") as execute:
                with self.assertRaisesRegex(ValueError, "repository does not match"):
                    agent.cmd_run(args)
            execute.assert_not_called()
            self.assertFalse((run / "leases").exists())

    def test_job_directory_symlink_escape_is_rejected_before_result_write(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, _, policy_path = self.make_plan(directory)
            job = agent.store.jobs(run)[0]
            job_file = next((run / "jobs").glob("*.json"))
            job_file.unlink()
            (run / "jobs").rmdir()
            outside = directory / "outside"
            outside.mkdir()
            (run / "jobs").symlink_to(outside, target_is_directory=True)
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            with patch.object(agent, "run_is_stale", return_value=True):
                with self.assertRaisesRegex(ValueError, "job file must stay below"):
                    agent.execute_job(run, job, run_record, policy)
            self.assertEqual(list(outside.iterdir()), [])

    def test_collection_retains_local_witness_identity(self):
        candidate = {
            "source": "diurnal",
            "source_sha256": "a" * 64,
            "source_page": 42,
            "printed_page": "37",
            "extractor": "ocr",
            "raw_text_sha256": "b" * 64,
            "source_column": "left",
            "source_bbox": [0, 0, 10, 10],
        }
        witness = agent.witness_from_candidate(candidate)
        self.assertEqual(witness["source_page"], 42)
        self.assertEqual(witness["printed_page"], "37")
        self.assertEqual(witness["raw_text_sha256"], "b" * 64)
        self.assertEqual(witness["source_column"], "left")

    def test_collection_binds_structured_page_slices(self):
        candidate = {"source": "diurnal", "source_sha256": "a" * 64, "source_page": 1, "extractor": "column", "raw_text_sha256": "b" * 64, "source_column": "left", "source_bbox": [0, 0, 10, 10], "page_slices": [{"page": 1, "printed_page": "12", "raw_text_sha256": "c" * 64, "extractor": "column", "source_column": "left", "bbox": [0, 0, 10, 10], "offset": "1:4"}]}
        witness = agent.witness_from_candidate(candidate)
        self.assertEqual(witness["page_slices"][0]["printed_page"], "12")

    def test_legacy_page_slices_without_columns_remain_valid(self):
        candidate = {"source": "diurnal", "source_sha256": "a" * 64, "source_page": 1, "extractor": "native", "raw_text_sha256": "b" * 64, "page_slices": [{"page": 1, "printed_page": "12", "raw_text_sha256": "c" * 64, "extractor": "native", "offset": "1:4"}]}
        witness = agent.witness_from_candidate(candidate)
        self.assertNotIn("source_column", witness)

    def test_result_and_collection_keep_immutable_local_witness(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, _, policy_path = self.make_plan(directory)
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            original = agent.store.jobs(run)[0]
            with patch.object(agent, "run_is_stale", return_value=False), patch.object(
                agent.providers, "execute", return_value=agent.providers.Execution(0, json.dumps(proposal()), "")
            ):
                self.assertEqual(agent.execute_job(run, original, run_record, policy), "complete")
                self.assertEqual(agent.cmd_collect(argparse.Namespace(
                    run=run, output=directory / "output", repo=directory, include_stale=False,
                )), 0)
            result = agent.successful_results(run)[0]
            self.assertEqual(result["candidate_sha256"], original["candidate_sha256"])
            self.assertEqual(result["witness"], original["witness"])
            self.assertEqual(result["witness"]["sha256"], agent.model.digest(result["witness"]["identity"]))
            collection = agent.store.read_json(run / "collection.json")
            self.assertEqual(collection["results"][0]["witness"], original["witness"])
            result_path = next((run / "results").glob("*/*/result.json"))
            result["candidate_sha256"] = "c" * 64
            agent.store.atomic_json(result_path, result)
            with patch.object(agent, "run_is_stale", return_value=False):
                with self.assertRaisesRegex(ValueError, "does not match its planned job"):
                    agent.cmd_collect(argparse.Namespace(
                        run=run, output=directory / "output", repo=directory, include_stale=False,
                    ))

    def test_plan_rejects_candidate_without_immutable_witness(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            input_path = directory / "candidates.json"
            input_path.write_text(json.dumps({"candidates": [{"candidate_id": "DI-incomplete"}]}))
            policy_path = directory / "policy.json"
            policy_path.write_bytes(agent.DEFAULT_POLICY.read_bytes())
            with self.assertRaisesRegex(ValueError, "candidate witness requires source"):
                agent.cmd_plan(argparse.Namespace(
                    input=input_path,
                    policy=policy_path,
                    repo=directory,
                    output=directory / "output",
                    run_id="incomplete",
                ))

    def test_worktree_digest_changes_with_uncommitted_file_content(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            tracked = directory / "tracked.txt"
            tracked.write_text("before")
            listing = type("Result", (), {"returncode": 0, "stdout": b"tracked.txt\0"})()
            with patch.object(agent.subprocess, "run", return_value=listing):
                before = agent.git_worktree_digest(directory)
                tracked.write_text("after")
                after = agent.git_worktree_digest(directory)
            self.assertNotEqual(before, after)

    def test_declared_dependency_change_invalidates_snapshot(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            dependency = directory / "profile.json"
            dependency.write_text('{"version": 1}')
            candidates = directory / "candidates.json"
            candidates.write_text(json.dumps({
                "dependencies": {
                    "profile": {"path": str(dependency), "sha256": agent.file_sha256(dependency)}
                },
                "candidates": [],
            }))
            policy = directory / "policy.json"
            policy.write_bytes(agent.DEFAULT_POLICY.read_bytes())
            original_hash = agent.file_sha256(dependency)
            with patch.object(agent, "git_head", return_value="head"):
                before = agent.snapshot(candidates, policy, directory)
                dependency.write_text('{"version": 2}')
                with self.assertRaisesRegex(ValueError, "hash does not match"):
                    agent.snapshot(candidates, policy, directory)
            self.assertEqual(before["dependencies"]["profile"], original_hash)

    def test_manifest_tree_change_marks_job_stale_without_provider_call(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            intake = directory / "output" / "intake"
            intake.mkdir(parents=True)
            (intake / "manifest.json").write_text('{"pages": []}')
            texts = directory / "data" / "texts"
            feasts = directory / "data" / "feasts"
            texts.mkdir(parents=True)
            feasts.mkdir(parents=True)
            discovery = directory / "output" / "discovery" / "proper-discovery.json"
            discovery.parent.mkdir(parents=True)
            discovery.write_text(json.dumps({
                "schema_version": 2,
                "dependency_manifest": {
                    "schema_version": 1,
                    "roots": {"repository": ".", "intake": "output/intake"},
                    "entries": [
                        {
                            "kind": "tree", "role": "intake-tree",
                            "root": "intake", "path": ".",
                            "sha256": agent.tree_sha256(intake),
                        },
                        {
                            "kind": "tree", "role": "corpus-tree",
                            "root": "repository", "path": "data/texts",
                            "sha256": agent.tree_sha256(texts),
                        },
                        {
                            "kind": "tree", "role": "feast-metadata-tree",
                            "root": "repository", "path": "data/feasts",
                            "sha256": agent.tree_sha256(feasts),
                        },
                    ],
                },
                "candidates": [{
                    "candidate_id": "DI/example/42",
                    "discovery_classification": "verify-existing",
                    "source_text": "Bounded witness",
                    "source": "diurnal",
                    "source_sha256": "a" * 64,
                    "source_page": 42,
                    "extractor": "pdftotext-layout",
                    "raw_text_sha256": "b" * 64,
                }],
            }))
            policy_path = directory / "policy.json"
            policy_path.write_bytes(agent.DEFAULT_POLICY.read_bytes())
            args = argparse.Namespace(
                input=discovery,
                policy=policy_path,
                repo=directory,
                output=directory / "output" / "agent",
                run_id="manifest-stale",
            )
            with patch.object(agent, "git_head", return_value="head"):
                self.assertEqual(agent.cmd_plan(args), 0)
            run = agent.store.active_run(args.output)
            (intake / "later-page.json").write_text('{}')
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            with patch.object(agent.providers, "execute") as execute:
                self.assertEqual(
                    agent.execute_job(
                        run, agent.store.jobs(run)[0], run_record, policy
                    ),
                    "stale",
                )
            execute.assert_not_called()

    def test_unsafe_manifest_root_is_rejected_before_run_write(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            outside = directory / "outside"
            outside.mkdir()
            discovery = directory / "candidates.json"
            discovery.write_text(json.dumps({
                "dependency_manifest": {
                    "schema_version": 1,
                    "roots": {"repository": ".", "intake": "../outside"},
                    "entries": [{
                        "kind": "tree", "role": "intake-tree",
                        "root": "intake", "path": ".",
                        "sha256": agent.tree_sha256(outside),
                    }],
                },
                "candidates": [{
                    "candidate_id": "DI/example/42", "source": "diurnal",
                    "source_sha256": "a" * 64, "source_page": 42,
                    "extractor": "native", "raw_text_sha256": "b" * 64,
                }],
            }))
            policy_path = directory / "policy.json"
            policy_path.write_bytes(agent.DEFAULT_POLICY.read_bytes())
            output = directory / "output" / "agent"
            with self.assertRaisesRegex(ValueError, "repository-relative"):
                agent.cmd_plan(argparse.Namespace(
                    input=discovery, policy=policy_path, repo=directory,
                    output=output, run_id="unsafe-manifest",
                ))
            self.assertFalse((output / "runs").exists())

            intake = directory / "output" / "intake"
            intake.mkdir(parents=True)
            payload = json.loads(discovery.read_text())
            payload["dependency_manifest"]["roots"]["intake"] = "output/intake"
            payload["dependency_manifest"]["entries"][0]["sha256"] = agent.tree_sha256(intake)
            discovery.write_text(json.dumps(payload))
            with self.assertRaisesRegex(ValueError, "exactly one intake, corpus"):
                agent.cmd_plan(argparse.Namespace(
                    input=discovery, policy=policy_path, repo=directory,
                    output=output, run_id="incomplete-manifest",
                ))
            self.assertFalse((output / "runs").exists())

    def test_adjudicate_does_not_plan_without_replica_disagreement(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, policy_path = self.collect_replica_disagreement(directory, ("match", "match"))
            self.enable_claude(policy_path)
            with patch.object(agent, "run_is_stale", return_value=False):
                self.assertEqual(agent.cmd_adjudicate(argparse.Namespace(
                    run=run, output=directory / "output", repo=directory,
                )), 0)
            self.assertEqual(len(agent.store.jobs(run)), 1)

    def test_adjudicate_plans_one_claude_job_with_frozen_replica_packet(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, policy_path = self.collect_replica_disagreement(directory)
            self.enable_claude(policy_path)
            original = agent.store.jobs(run)[0]
            with patch.object(agent, "run_is_stale", return_value=False):
                self.assertEqual(agent.cmd_adjudicate(argparse.Namespace(
                    run=run, output=directory / "output", repo=directory,
                )), 0)
            jobs = agent.store.jobs(run)
            self.assertEqual(len(jobs), 2)
            adjudication = next(job for job in jobs if job.get("adjudication"))
            self.assertEqual(adjudication["providers"], ["claude"])
            self.assertEqual(adjudication["candidate_id"], original["candidate_id"])
            self.assertEqual(adjudication["candidate_sha256"], original["candidate_sha256"])
            packet = adjudication["candidate"]
            self.assertEqual(
                adjudication["adjudication"]["packet_sha256"],
                agent.model.digest(packet),
            )
            self.assertEqual(packet["original_candidate"], original["candidate"])
            self.assertEqual(packet["witness"], agent.witness_from_candidate(original["candidate"]))
            self.assertEqual(len(packet["replica_proposals"]), 2)
            for replica in packet["replica_proposals"]:
                self.assertTrue(replica["result_id"])
                self.assertEqual(replica["proposal_sha256"], agent.model.digest(replica["proposal"]))
                self.assertTrue(replica["result_sha256"])

            tampered = json.loads(json.dumps(adjudication))
            tampered["candidate"]["adjudication"] = "altered after planning"
            run_record = agent.store.read_json(run / "run.json")
            policy = agent.load_policy(policy_path)
            with patch.object(agent, "run_is_stale", return_value=False), patch.object(
                agent.providers, "execute"
            ) as execute:
                with self.assertRaisesRegex(ValueError, "packet_sha256"):
                    agent.execute_job(run, tampered, run_record, policy)
            execute.assert_not_called()

    def test_adjudicate_is_idempotent_for_an_existing_disagreement_job(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, policy_path = self.collect_replica_disagreement(directory)
            self.enable_claude(policy_path)
            args = argparse.Namespace(run=run, output=directory / "output", repo=directory)
            with patch.object(agent, "run_is_stale", return_value=False):
                self.assertEqual(agent.cmd_adjudicate(args), 0)
                self.assertEqual(agent.cmd_adjudicate(args), 0)
            self.assertEqual(len([job for job in agent.store.jobs(run) if job.get("adjudication")]), 1)

    def test_adjudicate_rejects_disabled_claude_for_disagreement(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, _ = self.collect_replica_disagreement(directory)
            with patch.object(agent, "run_is_stale", return_value=False):
                with self.assertRaisesRegex(ValueError, "Claude adjudication is disabled"):
                    agent.cmd_adjudicate(argparse.Namespace(
                        run=run, output=directory / "output", repo=directory,
                    ))

    def test_adjudicate_rejects_stale_or_incomplete_collections(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, _ = self.collect_replica_disagreement(directory)
            args = argparse.Namespace(run=run, output=directory / "output", repo=directory)
            with patch.object(agent, "run_is_stale", return_value=True):
                with self.assertRaisesRegex(RuntimeError, "collection is stale"):
                    agent.cmd_adjudicate(args)

        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            run, _, _ = self.make_plan(directory, "ambiguous-owner/context")
            agent.store.atomic_json(run / "collection.json", {
                "run_id": "test-run", "stale": False, "results": [], "replica_disagreements": [],
            })
            with patch.object(agent, "run_is_stale", return_value=False):
                with self.assertRaisesRegex(RuntimeError, "collection is incomplete"):
                    agent.cmd_adjudicate(argparse.Namespace(
                        run=run, output=directory / "output", repo=directory,
                    ))


if __name__ == "__main__":
    unittest.main()
