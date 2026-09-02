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

    def test_real_suffrage_label_and_flex_pairs_are_near(self):
        pairs = (
            (
                "May the blessed Mother of God, * the Virgin Mary, and all the Saints, "
                "intercede for us to the Lord.",
                "Ant. May the blessed Mother of God, the Virgin Mary, and all the Saints, "
                "intercede for us to the Lord.",
            ),
            (
                "May all the Saints * intercede for us to the Lord.",
                "May all the Saints † intercede for us to the Lord.",
            ),
        )
        for corpus, observed in pairs:
            with self.subTest(observed=observed):
                result = {"found": True, "text": observed, "confidence": "high"}
                self.assertEqual(transcribe.classify_transcription(corpus, result)[0], "near")


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
                       "Omit printed entry labels", "corpus marker ` * `", "Never infer"):
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

    def page(self, key, pdf_page):
        return next(page for page in self.resolve(key, "595") if page["pdf_page"] == pdf_page)


class FallbackResolver:
    def __init__(self, printed=None, pdf=None, found=None):
        self.printed = printed
        self.pdf = pdf
        self.found = found or []
        self.find_calls = []

    def resolve(self, key, page):
        if isinstance(self.printed, Exception):
            raise self.printed
        return self.printed

    def resolve_pdf(self, key, page):
        if isinstance(self.pdf, Exception):
            raise self.pdf
        return self.pdf

    def find(self, key, query, limit=3):
        self.find_calls.append((query, limit))
        return self.found

    def page(self, key, pdf_page):
        candidates = [self.printed, self.pdf, *self.found]
        return next(page for records in candidates if isinstance(records, list)
                    for page in records if page["pdf_page"] == pdf_page)


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
            self.assertEqual(record["first"]["text"], "Printed wording.")
            self.assertEqual(record["second"]["text"], "Printed wording")
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

    def test_process_falls_back_from_printed_label_to_pdf_page(self):
        original_corpus = transcribe.corpus_text
        original_attest = transcribe.attest
        attested = []
        try:
            transcribe.corpus_text = lambda key: "Existing corpus wording."
            transcribe.attest = lambda key, page, png: attested.append((key, page, png))
            printed = [
                {"pdf_page": 611, "printed_page": "566", "inferred": False, "png": "/cache/0611.png"},
            ]
            pdf = [
                {"pdf_page": 566, "printed_page": "521", "inferred": False, "png": "/cache/0566.png"},
            ]
            resolver = FallbackResolver(printed=printed, pdf=pdf)
            provider = FakeProvider([
                {"found": False, "text": "", "printed_page": "566", "pdf_page": 611,
                 "confidence": "high", "notes": "section absent"},
                {"found": True, "text": "Existing corpus wording.", "printed_page": "566", "pdf_page": 566,
                 "confidence": "high", "notes": "section visible"},
            ])
            with tempfile.TemporaryDirectory() as directory:
                record = transcribe.process_row(
                    {"key": "proper/st-athanasius/collect", "page": "566", "source": "monastic-diurnal"},
                    transcribe.RunOptions(Path(directory), apply=True), resolver, provider,
                    {"st-athanasius": "St. Athanasius"},
                )
            self.assertEqual(record["locate_strategy"], "pdf-page")
            self.assertEqual(record["printed_page"], "521")
            self.assertEqual(record["first"]["text"], "Existing corpus wording.")
            self.assertEqual(len(provider.calls), 2)
            self.assertEqual(attested, [("proper/st-athanasius/collect", "521", "/cache/0566.png")])
        finally:
            transcribe.corpus_text = original_corpus
            transcribe.attest = original_attest

    def test_process_falls_back_to_top_three_corpus_ocr_candidates(self):
        original_corpus = transcribe.corpus_text
        try:
            transcribe.corpus_text = lambda key: "V. First six words locate this existing corpus text."
            ocr_pages = [[
                {"pdf_page": 88, "printed_page": "42", "inferred": True, "png": "/cache/0088.png"},
            ]]
            resolver = FallbackResolver(
                printed=LookupError("printed miss"), pdf=LookupError("PDF miss"), found=ocr_pages,
            )
            provider = FakeProvider([{
                "found": True, "text": "V. First six words locate this existing corpus text.",
                "printed_page": "42", "pdf_page": 88, "confidence": "high", "notes": "visible",
            }])
            with tempfile.TemporaryDirectory() as directory:
                record = transcribe.process_row(
                    {"key": "proper/st-athanasius/collect", "page": "566", "source": "monastic-diurnal"},
                    transcribe.RunOptions(Path(directory)), resolver, provider,
                    {"st-athanasius": "St. Athanasius"},
                )
            self.assertEqual(record["locate_strategy"], "corpus-ocr")
            self.assertEqual(record["printed_page"], "42")
            self.assertEqual(resolver.find_calls[0], ("first six words locate this existing corpus text", 3))
            self.assertEqual(len(provider.calls), 1)
        finally:
            transcribe.corpus_text = original_corpus

    def test_locating_and_second_reader_share_attempt_cap(self):
        original_corpus = transcribe.corpus_text
        try:
            transcribe.corpus_text = lambda key: "Existing corpus wording."
            printed = [{"pdf_page": 10, "printed_page": "10", "inferred": False, "png": "/10.png"}]
            pdf = [{"pdf_page": 20, "printed_page": "20", "inferred": False, "png": "/20.png"}]
            found = [[{"pdf_page": number, "printed_page": str(number), "inferred": False,
                       "png": f"/{number}.png"}] for number in (30, 31, 32)]
            resolver = FallbackResolver(printed=printed, pdf=pdf, found=found)
            missing = [
                {"found": False, "text": "", "printed_page": str(number), "pdf_page": number,
                 "confidence": "high", "notes": "absent"}
                for number in (10, 20, 30, 31)
            ]
            provider = FakeProvider(missing)
            with tempfile.TemporaryDirectory() as directory:
                record = transcribe.process_row(
                    {"key": "proper/st-athanasius/collect", "page": "10", "source": "monastic-diurnal"},
                    transcribe.RunOptions(Path(directory), apply=True, max_attempts=3),
                    resolver, provider, {"st-athanasius": "St. Athanasius"},
                )
            self.assertEqual(len(provider.calls), 3)
            self.assertEqual(len(record["locate_attempts"]), 3)
            self.assertEqual(record["classification"], "not-found")
            self.assertEqual(record["decision"], "record-only")
        finally:
            transcribe.corpus_text = original_corpus


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
