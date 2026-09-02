#!/usr/bin/env python3
"""Offline unit tests for diurnal-transcribe.py."""

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("diurnal-transcribe.py")
SPEC = importlib.util.spec_from_file_location("diurnal_transcribe", SCRIPT)
transcribe = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(transcribe)


class NormalizationTests(unittest.TestCase):
    def test_normalizes_typography_sigil_and_line_hyphenation(self):
        left = "℣. Thou art œternal—\nR. Everlast-\ning."
        right = 'V. THOU ART OETERNAL\nR/ everlasting'
        self.assertEqual(transcribe.normalize_text(left), transcribe.normalize_text(right))

    def test_classification_exact_near_different_and_low(self):
        exact = {"found": True, "text": "O Lord, hear us.", "confidence": "high"}
        self.assertEqual(transcribe.classify_transcription("O Lord, hear us.", exact)[0], "exact")
        near = {"found": True, "text": "O Lord, hear us", "confidence": "high"}
        self.assertEqual(transcribe.classify_transcription("O Lord, hear us.", near)[0], "near")
        different = {"found": True, "text": "A wholly unrelated prayer.", "confidence": "high"}
        self.assertEqual(transcribe.classify_transcription("O Lord, hear us.", different)[0], "different")
        low = {"found": True, "text": "O Lord", "confidence": "low"}
        self.assertEqual(transcribe.classify_transcription("O Lord", low)[0], "low-confidence")
        missing = {"found": False, "text": "", "confidence": "medium"}
        self.assertEqual(transcribe.classify_transcription("O Lord", missing)[0], "not-found")


class PromptTests(unittest.TestCase):
    def test_prompt_names_feast_slot_pages_and_grammar(self):
        description = transcribe.describe_key(
            "proper/st-athanasius/short-responsory-vespers", Path("data"),
            {"st-athanasius": "St. Athanasius"},
        )
        prompt = transcribe.build_prompt(
            "proper/st-athanasius/short-responsory-vespers", description, "595",
            [{"printed_page": "595", "pdf_page": 624, "png": "/cache/0624.png"},
             {"printed_page": "596", "pdf_page": 625, "png": "/cache/0625.png"}],
        )
        for wanted in ("short responsory vespers for St. Athanasius", "printed page 595",
                       "PDF 624", "V. ` and `R. `", "blank line between stanzas",
                       "Never infer"):
            self.assertIn(wanted, prompt)


class FakeProvider:
    def __init__(self, answers):
        self.answers = list(answers)
        self.calls = []

    def transcribe(self, provider, model, prompt, images):
        self.calls.append((provider, model, prompt, images))
        return self.answers.pop(0), 0.01


class FakeResolver:
    def resolve(self, key, page):
        return [
            {"pdf_page": 624, "printed_page": page, "inferred": False, "png": "/cache/0624.png"},
            {"pdf_page": 625, "printed_page": "596", "inferred": False, "png": "/cache/0625.png"},
        ]


def answer(text, confidence="high"):
    return {"found": True, "text": text, "printed_page": "595", "pdf_page": 624,
            "confidence": confidence, "notes": "clearly printed"}


class ApplyDecisionTests(unittest.TestCase):
    def test_pure_apply_decisions(self):
        first = answer("O Lord, hear us.")
        self.assertEqual(transcribe.apply_decision("proper/x/collect", "near", first), "attest")
        self.assertEqual(transcribe.apply_decision("proper/x/collect", "different", first), "needs-human")
        second = answer("O Lord, hear us")
        self.assertEqual(transcribe.apply_decision("proper/x/collect", "different", first, second), "replace-and-attest")
        self.assertEqual(transcribe.apply_decision("psalms/001", "different", first, second), "needs-human")

    def test_process_dry_run_never_calls_provider(self):
        with tempfile.TemporaryDirectory() as directory:
            options = transcribe.RunOptions(Path(directory), dry_run=True)
            provider = FakeProvider([])
            record = transcribe.process_row(
                {"key": "proper/st-athanasius/collect", "page": "595", "source": "Monastic Diurnal"},
                options, FakeResolver(), provider, {"st-athanasius": "St. Athanasius"},
            )
            self.assertTrue(record["dry_run"])
            self.assertEqual(provider.calls, [])
            self.assertTrue(Path(record["prompt"]).is_file())

    def test_different_calls_second_reader_only_in_apply_mode(self):
        original_corpus = transcribe.corpus_text
        original_replace = transcribe.replace_and_attest
        replaced = []
        try:
            transcribe.corpus_text = lambda key: "Existing text."
            transcribe.replace_and_attest = lambda options, key, page, png, text: replaced.append((key, text))
            with tempfile.TemporaryDirectory() as directory:
                options = transcribe.RunOptions(Path(directory), apply=True)
                provider = FakeProvider([answer("Printed wording."), answer("Printed wording")])
                record = transcribe.process_row(
                    {"key": "proper/st-athanasius/collect", "page": "595", "source": "Monastic Diurnal"},
                    options, FakeResolver(), provider, {"st-athanasius": "St. Athanasius"},
                )
            self.assertEqual([call[0] for call in provider.calls], ["codex", "claude"])
            self.assertEqual(record["decision"], "replace-and-attest")
            self.assertEqual(replaced, [("proper/st-athanasius/collect", "Printed wording.")])
        finally:
            transcribe.corpus_text = original_corpus
            transcribe.replace_and_attest = original_replace

    def test_exact_apply_attests_without_second_reader(self):
        original_corpus = transcribe.corpus_text
        original_attest = transcribe.attest
        attested = []
        try:
            transcribe.corpus_text = lambda key: "Existing text."
            transcribe.attest = lambda key, page, png: attested.append(key)
            with tempfile.TemporaryDirectory() as directory:
                options = transcribe.RunOptions(Path(directory), apply=True)
                provider = FakeProvider([answer("Existing text.")])
                record = transcribe.process_row(
                    {"key": "proper/st-athanasius/collect", "page": "595", "source": "Monastic Diurnal"},
                    options, FakeResolver(), provider, {"st-athanasius": "St. Athanasius"},
                )
            self.assertEqual(record["classification"], "exact")
            self.assertEqual(len(provider.calls), 1)
            self.assertEqual(attested, ["proper/st-athanasius/collect"])
        finally:
            transcribe.corpus_text = original_corpus
            transcribe.attest = original_attest


class ProviderCommandTests(unittest.TestCase):
    def test_fake_executor_receives_bounded_codex_command(self):
        commands = []
        payload = answer("Text")

        def fake(command, timeout, max_bytes):
            commands.append((command, timeout, max_bytes))
            return json.dumps(payload)

        runner = transcribe.ProviderRunner(timeout=7, max_bytes=900, execute=fake)
        result, _ = runner.transcribe("codex", "test-model", "prompt", [Path("one.png"), Path("two.png")])
        self.assertEqual(result, payload)
        command = commands[0][0]
        self.assertEqual(command[:3], ["codex", "exec", "--ephemeral"])
        self.assertEqual(command.count("-i"), 2)
        self.assertIn("--sandbox", command)
        self.assertEqual(commands[0][1:], (7, 900))

    def test_fake_executor_receives_claude_schema_and_read_paths(self):
        commands = []

        def fake(command, timeout, max_bytes):
            commands.append(command)
            return json.dumps({"structured_output": answer("Text")})

        runner = transcribe.ProviderRunner(execute=fake)
        result, _ = runner.transcribe("claude", "sonnet", "prompt", [Path("page.png")])
        self.assertEqual(result["text"], "Text")
        command = commands[0]
        self.assertEqual(command[0], "claude")
        self.assertIn("--json-schema", command)
        self.assertIn("--allowedTools", command)
        self.assertIn("page.png", command[command.index("-p") + 1])


if __name__ == "__main__":
    unittest.main()
