#!/usr/bin/env python3
"""Offline unit tests for diurnal-transcribe.py."""

import importlib.util
import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


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

    def test_pilot2_collect_conclusion_cue_is_near(self):
        corpus = (
            "O Lord, we beseech thee, hear the prayers which we offer thee on the solemnity "
            "of blessed Athanasius, thy Bishop and Confessor: and by the interceding merits "
            "of him who worthily attained to serve thee, absolve us from all our sins."
        )
        observed = (
            "O LORD, we beseech thee, hear the prayers which we offer thee on the solemnity "
            "of blessed Athanasius, thy Bishop and Confessor: and by the interceding merits "
            "of him who worthily attained to serve thee, absolve us from all our sins. Through."
        )
        result = {"found": True, "text": observed, "confidence": "high"}
        classification, score = transcribe.classify_transcription(
            corpus, result, "proper/st-athanasius/collect",
        )
        self.assertEqual(classification, "near")
        self.assertEqual(score, 1.0)

    def test_collect_conclusion_entries_are_not_stripped(self):
        key = "shared/formulas/collect-conclusion-through"
        self.assertEqual(transcribe.normalize_text("Through.", key), "through")


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
                       "Omit printed entry labels", "corpus marker ` * `", "Never infer",
                       "must stop before its conclusion cue"):
            self.assertIn(wanted, prompt)

    def test_canticle_descriptions_include_biblical_references(self):
        self.assertEqual(
            transcribe.describe_key("canticles/habakkuk-3", Path("data")),
            "Canticle of Habakkuk (Hab. 3)",
        )
        self.assertIn("1 Sam. 2", transcribe.describe_key("canticles/hannah", Path("data")))


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


class MappingResolver:
    def __init__(self, printed, pdf=None):
        self.printed = printed
        self.pdf = pdf or {}
        self.resolve_calls = []

    def resolve(self, key, page):
        self.resolve_calls.append(page)
        value = self.printed[page]
        if isinstance(value, Exception):
            raise value
        return value

    def resolve_pdf(self, key, page):
        value = self.pdf.get(page, LookupError("PDF miss"))
        if isinstance(value, Exception):
            raise value
        return value

    def find(self, key, query, limit=3):
        return []

    def page(self, key, pdf_page):
        candidates = [*self.printed.values(), *self.pdf.values()]
        return next(page for records in candidates if isinstance(records, list)
                    for page in records if page["pdf_page"] == pdf_page)


def answer(text, confidence="high"):
    return {"found": True, "text": text, "printed_page": "595", "pdf_page": 624,
            "confidence": confidence, "notes": "clearly printed"}


class ApplyDecisionTests(unittest.TestCase):
    def test_high_similarity_word_disagreements_never_attest_or_replace(self):
        # Synthetic context for the July 29 / p. 575 disagreement in #424.
        context = "Grant thy servants grace and mercy throughout all the days of their lives. " * 5
        first = answer(context + "May they thereafter attain unto peace.")
        for ending in ("May they thereat attain to peace.",
                       "May they thereafter attain to peace.",
                       "May they not thereafter attain unto peace."):
            with self.subTest(ending=ending), tempfile.TemporaryDirectory() as directory:
                corpus = context + ending
                classification, score = transcribe.classify_transcription(corpus, first, "proper/x/collect")
                self.assertGreaterEqual(score, 0.985)
                self.assertEqual(classification, "different")
                runner = FakeProvider([first, answer(corpus)])
                with patch.object(transcribe, "corpus_text", return_value=corpus), \
                     patch.object(transcribe, "attest") as attest, \
                     patch.object(transcribe, "replace_and_attest") as replace:
                    result = transcribe.process_row(
                        {"key": "proper/x/collect", "page": "595", "source": "Monastic Diurnal"},
                        transcribe.RunOptions(Path(directory), apply=True), FakeResolver(), runner, {},
                    )
                self.assertEqual(result["decision"], "needs-human")
                self.assertEqual(len(runner.calls), 2)
                self.assertEqual(runner.calls[1][0:2], ("claude", "sonnet"))
                attest.assert_not_called()
                replace.assert_not_called()

    def test_pure_apply_decisions(self):
        first = answer("O Lord, hear us.")
        self.assertEqual(transcribe.apply_decision("proper/x/collect", "near", first), "attest")
        self.assertEqual(transcribe.apply_decision("proper/x/collect", "different", first), "needs-human")
        second = answer("O Lord, hear us")
        corpus = "O Lord, hear us today."
        self.assertEqual(
            transcribe.apply_decision(
                "proper/x/collect", "different", first, second, corpus_text=corpus,
            ),
            "replace-and-attest",
        )
        self.assertEqual(
            transcribe.apply_decision(
                "psalms/001", "different", first, second, corpus_text=corpus,
            ),
            "needs-human",
        )

    def test_replace_requires_first_reader_similarity_to_corpus(self):
        corpus = "abcdefghij"
        first = answer("abcdeXXXXX")
        second = answer("abcdeXXXXX")
        self.assertLess(transcribe.similarity(first["text"], corpus), 0.6)
        self.assertEqual(
            transcribe.apply_decision(
                "proper/x/collect", "different", first, second, corpus_text=corpus,
            ),
            "needs-human",
        )

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

    def test_source_unknown_starts_with_ocr_strategies(self):
        original_corpus = transcribe.corpus_text
        try:
            transcribe.corpus_text = lambda key: "O Lord, hear the opening words of this canticle."
            found = [[{"pdf_page": 40, "printed_page": "12", "inferred": False,
                       "png": "/cache/0040.png"}]]
            resolver = FallbackResolver(found=found)
            provider = FakeProvider([{
                "found": True, "text": "O Lord, hear the opening words of this canticle.",
                "printed_page": "12", "pdf_page": 40, "confidence": "high", "notes": "visible",
            }])
            with tempfile.TemporaryDirectory() as directory:
                record = transcribe.process_row(
                    {"key": "canticles/habakkuk-3", "page": "", "source": "",
                     "status": "source-unknown"},
                    transcribe.RunOptions(Path(directory)), resolver, provider, {},
                )
            self.assertEqual(record["locate_strategy"], "corpus-ocr")
            self.assertEqual(record["classification"], "exact")
            self.assertEqual(len(resolver.find_calls), 1)
            self.assertIn("o lord hear the opening", resolver.find_calls[0][0])
        finally:
            transcribe.corpus_text = original_corpus

    def test_status_filter_defaults_to_unknown_and_needs_review(self):
        rows = [
            {"key": "proper/a/collect", "status": "source-unknown"},
            {"key": "proper/b/collect", "status": "needs-review", "source": "diurnal", "page": "2"},
            {"key": "proper/c/collect", "status": "verified", "source": "diurnal", "page": "3"},
        ]
        self.assertEqual(
            [row["key"] for row in transcribe.selected_rows(rows, False, set())],
            ["proper/a/collect", "proper/b/collect"],
        )
        self.assertEqual(
            [row["key"] for row in transcribe.selected_rows(
                rows, False, set(), {"source-unknown"},
            )],
            ["proper/a/collect"],
        )

    def test_different_calls_second_reader_only_in_apply_mode(self):
        original_corpus = transcribe.corpus_text
        original_replace = transcribe.replace_and_attest
        replaced = []
        try:
            transcribe.corpus_text = lambda key: "Printed wording in corpus."
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

    def test_wrong_page_continues_to_next_locate_strategy(self):
        original_corpus = transcribe.corpus_text
        corpus = "Defend us, we beseech thee, O Lord, from all perils of mind and body."
        wrong = "O God, grant rest to the departed brethren and benefactors of our Congregation."
        try:
            transcribe.corpus_text = lambda key: corpus
            printed = {
                "29": [{"pdf_page": 75, "printed_page": "29", "inferred": False,
                        "png": "/cache/0075.png"}],
                "42": [{"pdf_page": 88, "printed_page": "42", "inferred": False,
                        "png": "/cache/0088.png"},
                       {"pdf_page": 89, "printed_page": "43", "inferred": False,
                        "png": "/cache/0089.png"}],
                "xxxi": [{"pdf_page": 29, "printed_page": "xxxi", "inferred": False,
                           "png": "/cache/0029.png"}],
            }
            resolver = MappingResolver(printed)
            provider = FakeProvider([
                {"found": True, "text": wrong, "printed_page": "29", "pdf_page": 75,
                 "confidence": "high", "notes": "a different collect is visible"},
                {"found": True, "text": corpus, "printed_page": "42", "pdf_page": 88,
                 "confidence": "high", "notes": "requested collect is visible"},
            ])
            with tempfile.TemporaryDirectory() as directory:
                record = transcribe.process_row(
                    {"key": "ordinary/shared/suffrage-collect", "page": "29",
                     "source": "Monastic Diurnal"},
                    transcribe.RunOptions(Path(directory)), resolver, provider, {},
                )
            self.assertEqual(record["classification"], "exact")
            self.assertEqual(record["locate_strategy"], "source-page")
            self.assertEqual(record["locate_attempts"][0]["classification"], "wrong-page")
            self.assertLess(record["locate_attempts"][0]["similarity"], 0.5)
            self.assertEqual(resolver.resolve_calls[:2], ["29", "42"])
            self.assertEqual(
                provider.calls[1][3], [Path("/cache/0088.png"), Path("/cache/0089.png")],
            )
        finally:
            transcribe.corpus_text = original_corpus

    def test_exhausted_wrong_page_is_not_found_record_only(self):
        original_corpus = transcribe.corpus_text
        try:
            transcribe.corpus_text = lambda key: "abcdefghij"
            resolver = MappingResolver({
                "10": [{"pdf_page": 10, "printed_page": "10", "inferred": False,
                        "png": "/cache/0010.png"}],
                "566": [{"pdf_page": 566, "printed_page": "520", "inferred": False,
                         "png": "/cache/0566.png"}],
            })
            provider = FakeProvider([{
                "found": True, "text": "XXXXXXXXXX", "printed_page": "10", "pdf_page": 10,
                "confidence": "high", "notes": "unrelated text",
            }])
            with tempfile.TemporaryDirectory() as directory:
                record = transcribe.process_row(
                    {"key": "proper/st-athanasius/collect", "page": "10",
                     "source": "Monastic Diurnal"},
                    transcribe.RunOptions(Path(directory), apply=True, max_attempts=1),
                    resolver, provider, {"st-athanasius": "St. Athanasius"},
                )
            self.assertEqual(record["classification"], "not-found")
            self.assertEqual(record["decision"], "record-only")
            self.assertEqual(record["locate_attempts"][0]["classification"], "wrong-page")
            self.assertEqual(len(provider.calls), 1)
        finally:
            transcribe.corpus_text = original_corpus

    def test_source_comment_pages_are_tried_with_range_continuation(self):
        self.assertEqual(
            transcribe.corpus_source_pages("ordinary/shared/suffrage-collect"),
            ["42", "xxxi"],
        )

    def test_source_comment_parser_accepts_hyphen_and_en_dash_ranges(self):
        with tempfile.TemporaryDirectory() as directory:
            data = Path(directory)
            path = data / "texts" / "ordinary" / "example.txt"
            path.parent.mkdir(parents=True)
            path.write_text(
                "[collect]\n"
                "# SOURCE: Monastic Diurnal.pdf, printed pp. 12-13 (PDF pp. 58-59)\n"
                "# SOURCE: monastic diurnal p. 20; pp. 30–31\n"
                "Prayer.\n",
                encoding="utf-8",
            )
            self.assertEqual(
                transcribe.corpus_source_pages("ordinary/example/collect", data),
                ["12", "20", "30"],
            )

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


class ReviewedReplacementTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.run = self.root / "output" / "transcribe" / "held"
        self.run.mkdir(parents=True)
        self.cache = self.root / "output" / "pages" / "monastic-diurnal"
        self.cache.mkdir(parents=True)
        self.pdf = self.root / "diurnal.pdf"
        self.pdf.write_bytes(b"PDF witness")
        (self.root / "office").write_bytes(b"compiled engine")
        self.feasts = self.root / "data" / "feasts"
        self.feasts.mkdir(parents=True)
        (self.feasts / "fixed.txt").write_text("[example]\nName = Example Feast\nDate = 1-1\n")
        self.text_file = self.root / "data" / "texts" / "proper" / "example.txt"
        self.text_file.parent.mkdir(parents=True)
        self.text_file.write_text("[collect]\n# SOURCE: diurnal p. 594\nAn unrelated text requiring replacement.\n")
        self.index = {"version": 1, "key": "monastic-diurnal", "source_pdf": str(self.pdf),
                      "pdf_sha256": transcribe.diurnal_pages.sha256_file(self.pdf),
                      "dpi": 150, "page_count": 625}
        (self.cache / "manifest.json").write_text(json.dumps(self.index))
        self.index["pages"] = []
        for number, label in ((624, "595"), (625, "596")):
            png = self.cache / f"{number:04d}.png"
            png.write_bytes(f"image of {label}".encode())
            self.index["pages"].append({"pdf_page": number, "printed_page": label,
                                        "png": str(png), "inferred": False})
        self.save_index()
        for name, value in (("ROOT", self.root), ("TRANSCRIBE_ROOT", self.run.parent)):
            patcher = patch.object(transcribe, name, value)
            patcher.start()
            self.addCleanup(patcher.stop)
        self.key = "proper/example/collect"
        self.old = self.current = "An unrelated text requiring replacement."
        self.reading = answer("Almighty and everlasting God, mercifully grant us thy peace.")
        self.assertLess(transcribe.similarity(self.old, self.reading["text"], self.key), 0.5)
        self.calls = []
        patcher = patch.object(transcribe, "_run_office_unlocked", side_effect=self.office)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.write_held()

    def save_index(self):
        (self.cache / "index.json").write_text(json.dumps(self.index))

    def write_held(self):
        result = {"key": self.key, "decision": "record-only", "classification": "not-found",
                  "first": self.reading, "corpus_text": self.old,
                  "pages": transcribe.PageResolver().resolve("monastic-diurnal", "595")}
        (self.run / "results.jsonl").unlink(missing_ok=True)
        transcribe.write_jsonl(self.run / "results.jsonl", result)

    def office(self, args, **kwargs):
        # Every call, including both writes, must hold the ingestion lock.
        import fcntl
        with (self.root / "output" / ".office-write.lock").open("a") as lock:
            with self.assertRaises(BlockingIOError):
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        self.calls.append(args)
        if args[:2] == ["corpus", "show"]:
            return self.current + "\n"
        if args[:2] == ["corpus", "put"]:
            self.current = Path(args[args.index("--file") + 1]).read_text().rstrip("\n")
        return ""

    def command(self, argv):
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return transcribe.main(argv)

    def prepare(self):
        code = self.command(["prepare-replacement", str(self.run), "--key", self.key,
                             "--context", "Example Feast collect, page heading and current Ordo appointment checked."])
        self.assertEqual(code, 0)
        path = self.run / "replacements" / f"{transcribe.safe_key(self.key)}.json"
        return path, json.loads(path.read_text())

    def apply(self, path, packet, runner):
        self.calls.clear()
        with patch.object(transcribe, "ProviderRunner", return_value=runner):
            return self.command(["apply-replacement", str(path), "--packet-sha256", packet["packet_sha256"],
                                 "--reviewer", "test-reviewer", "--review-note",
                                 "Inspected the heading, slot boundaries, candidate and output body against the page.",
                                 "--run-id", "recovery"])

    def assert_no_write(self):
        self.assertFalse(any(c[:2] in (["corpus", "put"], ["review", "attest"]) for c in self.calls))

    def test_reviewed_unrelated_replacement_uses_two_blind_readers_and_locked_writes(self):
        path, packet = self.prepare()
        runner = FakeProvider([self.reading, {**self.reading, "text": self.reading["text"].upper()}])
        self.assertEqual(self.apply(path, packet, runner), 0)
        self.assertEqual([c[:2] for c in runner.calls], [("codex", "gpt-5.6-luna"), ("claude", "sonnet")])
        for call in runner.calls:
            self.assertNotIn(self.reading["text"], call[2])
            self.assertEqual(call[3], [self.cache / "0624.png", self.cache / "0625.png"])
        writes = [c for c in self.calls if c[:2] != ["corpus", "show"]]
        self.assertEqual([c[:2] for c in writes], [["corpus", "put"], ["review", "attest"]])
        self.assertEqual(self.current, packet["body"].rstrip("\n"))
        self.assertEqual(writes[1][writes[1].index("--page") + 1], "595")
        result = json.loads((self.run.parent / "recovery" / "results.jsonl").read_text())
        self.assertEqual(result["replacement_packet_sha256"], packet["packet_sha256"])
        self.assertEqual(result["reviewer"], "test-reviewer")
        # The successful write invalidates the saved corpus fingerprint.
        runner.calls.clear()
        self.assertEqual(self.apply(path, packet, runner), 1)
        self.assertEqual(runner.calls, [])
        self.assert_no_write()

    def test_changed_review_inputs_hold_before_readers(self):
        path, packet = self.prepare()
        changes = {
            "corpus": lambda: setattr(self, "current", "A newer correction"),
            "PDF": lambda: self.pdf.write_bytes(b"another PDF"),
            "image": lambda: (self.cache / "0624.png").write_bytes(b"different image"),
            "index": lambda: (self.cache / "index.json").write_text(json.dumps({**self.index, "dpi": 300})),
            "manifest": lambda: (self.cache / "manifest.json").write_text("{}"),
            "feast": lambda: (self.feasts / "fixed.txt").write_text("changed feast context"),
            "old source citation": lambda: self.text_file.write_text(self.text_file.read_text().replace("594", "593")),
            "engine": lambda: (self.root / "office").write_bytes(b"new engine"),
            "context": lambda: path.write_text(json.dumps({**packet, "appointment_context": "new mapping"})),
        }
        for name, mutate in changes.items():
            with self.subTest(name=name):
                files = {p: p.read_bytes() for p in self.root.rglob("*") if p.is_file()}
                mutate()
                runner = FakeProvider([])
                self.assertEqual(self.apply(path, packet, runner), 1)
                self.assertEqual(runner.calls, [])
                self.assert_no_write()
                self.current = self.old
                for p, body in files.items():
                    p.write_bytes(body)

    def test_rehashed_packet_cannot_reuse_old_review_digest(self):
        path, packet = self.prepare()
        changed = {**packet, "appointment_context": "different feast or neighboring slot"}
        changed["packet_sha256"] = transcribe.replacement_packet_hash(changed)
        path.write_text(json.dumps(changed))
        runner = FakeProvider([])
        self.assertEqual(self.apply(path, packet, runner), 1)
        self.assertEqual(runner.calls, [])
        self.assert_no_write()

    def test_reread_disagreements_and_neighboring_text_remain_held(self):
        _, packet = self.prepare()
        variants = [
            {**self.reading, "text": self.reading["text"].replace("peace", "grace")},
            {**self.reading, "confidence": "low"},
            {**self.reading, "found": False},
            {**self.reading, "pdf_page": 625, "printed_page": "596"},
            {**self.reading, "printed_page": "594"},
            {**self.reading, "text": self.reading["text"] + " Through."},
        ]
        for other in variants:
            for first, second in ((self.reading, other), (other, self.reading), (other, other)):
                with self.subTest(first=first, second=second):
                    self.assertEqual(transcribe.reviewed_replacement_decision(packet, first, second), "needs-human")
        # Two readers agreeing on different text cannot supersede the reviewed candidate.
        runner = FakeProvider([variants[0], variants[0]])
        path = self.run / "replacements" / f"{transcribe.safe_key(self.key)}.json"
        self.assertEqual(self.apply(path, packet, runner), 1)
        self.assert_no_write()

    def test_corpus_change_during_reading_holds_under_write_lock(self):
        path, packet = self.prepare()
        test = self

        class RacingReader(FakeProvider):
            def transcribe(self, *args):
                result = super().transcribe(*args)
                test.current = "Concurrent correction"
                return result

        runner = RacingReader([self.reading, self.reading])
        self.assertEqual(self.apply(path, packet, runner), 1)
        self.assertEqual(len(runner.calls), 2)
        self.assert_no_write()

    def test_prepare_rejects_disputed_page_unreadable_text_and_collect_cues(self):
        for fields in ({"printed_page": "594"}, {"pdf_page": 623}, {"confidence": "low"},
                       {"text": self.reading["text"] + " Through."},
                       {"text": "GOD, mercifully grant thy servants an everlasting inheritance in heaven."}):
            with self.subTest(fields=fields):
                original = self.reading
                self.reading = {**original, **fields}
                self.write_held()
                self.assertEqual(self.command(["prepare-replacement", str(self.run), "--key", self.key,
                                               "--context", "reviewed"]), 1)
                self.assert_no_write()
                self.reading = original

    def test_reviewed_chapter_body_includes_the_fixed_response(self):
        self.key = "proper/example/chapter-lauds"
        self.reading = answer("!Rom 13:11\nBrethren: it is high time to awake out of sleep.")
        self.write_held()
        path, packet = self.prepare()
        self.assertTrue(packet["body"].endswith("R. Thanks be to God.\n"))
        runner = FakeProvider([self.reading, self.reading])
        self.assertEqual(self.apply(path, packet, runner), 0)
        self.assertEqual(self.current, packet["body"].rstrip("\n"))

    def test_second_reader_failure_records_first_without_writing(self):
        path, packet = self.prepare()

        class FailedSecondReader(FakeProvider):
            def transcribe(self, *args):
                if self.calls:
                    raise transcribe.ProviderError("bounded reader failed")
                return super().transcribe(*args)

        runner = FailedSecondReader([self.reading])
        self.assertEqual(self.apply(path, packet, runner), 1)
        self.assert_no_write()
        result = json.loads((self.run.parent / "recovery" / "results.jsonl").read_text())
        self.assertEqual(result["first"], self.reading)
        self.assertEqual(result["error"], "bounded reader failed")

    def test_psalter_replacements_cannot_enter_recovery(self):
        for key in ("psalms/001", "canticles/magnificat"):
            self.assertEqual(self.command(["prepare-replacement", str(self.run), "--key", key,
                                           "--context", "reviewed"]), 1)
        _, packet = self.prepare()
        for key in ("psalms/001", "canticles/magnificat"):
            self.assertEqual(transcribe.reviewed_replacement_decision({**packet, "key": key},
                                                                     self.reading, self.reading), "needs-human")

    def test_cli_requires_signoff_and_independent_provider(self):
        path, packet = self.prepare()
        argv = ["apply-replacement", str(path), "--reviewer", "reviewer", "--review-note", "reviewed",
                "--packet-sha256", packet["packet_sha256"]]
        for flag in ("--reviewer", "--review-note", "--packet-sha256"):
            incomplete = argv.copy()
            index = incomplete.index(flag)
            del incomplete[index:index + 2]
            with self.subTest(flag=flag), self.assertRaises(SystemExit):
                self.command(incomplete)
        with self.assertRaises(SystemExit):
            self.command(argv + ["--provider", "claude"])


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
        inline_schema = command[command.index("--json-schema") + 1]
        self.assertEqual(json.loads(inline_schema), json.loads(transcribe.SCHEMA.read_text()))
        self.assertNotEqual(inline_schema, str(transcribe.SCHEMA))
        self.assertIn("--allowedTools", command)
        self.assertIn("page.png", command[command.index("-p") + 1])


if __name__ == "__main__":
    unittest.main()
