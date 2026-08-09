#!/usr/bin/env python3
"""Focused tests for the resumable diurnal PDF intake."""

import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("diurnal-intake.py")
SPEC = importlib.util.spec_from_file_location("intake", SCRIPT)
intake = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = intake
SPEC.loader.exec_module(intake)

RECONCILE_SCRIPT = Path(__file__).with_name("source-reconcile.py")
RECONCILE_SPEC = importlib.util.spec_from_file_location("source_reconcile_for_intake", RECONCILE_SCRIPT)
reconcile = importlib.util.module_from_spec(RECONCILE_SPEC)
sys.modules[RECONCILE_SPEC.name] = reconcile
RECONCILE_SPEC.loader.exec_module(reconcile)


class Result:
    returncode = 0
    stderr = b""


class DiurnalIntakeTest(unittest.TestCase):
    def make_registered_run(self, directory: Path, page_count: int = 2) -> tuple[Path, Path]:
        pdf = directory / "book.pdf"
        pdf.write_bytes(b"pdf")
        with patch.object(intake, "which", return_value="/bin/tool"), patch.object(intake, "pages", return_value=page_count):
            return pdf, intake.register(pdf, directory / "output")

    def test_routing_metrics(self):
        readable = "many ordinary readable prayer words " * 6
        self.assertEqual(intake.route(intake.metrics(readable)), "native")
        self.assertEqual(intake.route(intake.metrics("")), "weak-native")
        self.assertIn("nonblank_lines", intake.metrics("one\ntwo"))
        layout = ("Readable words" + " " * 200) * 20
        self.assertEqual(intake.route(intake.metrics(layout)), "native")
        noisy = (chr(0xE000) * 20 + " readable") * 20
        self.assertEqual(intake.route(intake.metrics(noisy)), "weak-native")

    def test_extract_whole_document_and_resume(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            _, run = self.make_registered_run(directory)

            def extract_command(argv, **_):
                Path(argv[-1]).write_text("first page readable text\fsecond page readable text\f")
                return Result()

            with patch.object(intake, "cmd", side_effect=extract_command):
                intake.extract(run)
            manifest = intake.load(run / "manifest.json")
            self.assertTrue(manifest["whole_document_page_count_matched"])
            self.assertEqual(manifest["pages"]["1"]["text_path"], "pages/0001/native.layout.txt")
            with patch.object(intake, "cmd") as command:
                intake.extract(run, resume=True)
            command.assert_not_called()

    def test_form_feed_mismatch_uses_per_page_fallback(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            _, run = self.make_registered_run(directory)

            def extract_command(argv, **_):
                output = Path(argv[-1])
                if "-f" in argv:
                    output.write_text(f"page {argv[argv.index('-f') + 1]} fallback text")
                else:
                    output.write_text("only one form-feed part")
                return Result()

            with patch.object(intake, "cmd", side_effect=extract_command) as command:
                intake.extract(run)
            manifest = intake.load(run / "manifest.json")
            self.assertFalse(manifest["whole_document_page_count_matched"])
            self.assertEqual(sum("-f" in call.args[0] for call in command.call_args_list), 2)

    def test_source_hash_mismatch_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            pdf, run = self.make_registered_run(directory, page_count=1)
            pdf.write_bytes(b"changed")
            with self.assertRaisesRegex(SystemExit, "no longer matches"):
                intake.extract(run)

    def test_source_key_cannot_escape_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            pdf = directory / "book.pdf"
            pdf.write_bytes(b"pdf")
            with patch.object(intake, "which", return_value="/bin/tool"):
                with self.assertRaisesRegex(SystemExit, "source key"):
                    intake.register(pdf, directory / "output", "../data")

    def test_cli_output_confinement_rejects_tracked_paths(self):
        with self.assertRaisesRegex(SystemExit, "must stay below"):
            intake.require_output_path(intake.ROOT / "data" / "texts")

    def test_missing_tesseract_marks_route_and_qa_partial(self):
        with tempfile.TemporaryDirectory() as temporary:
            run = Path(temporary)
            intake.dump(run / "manifest.json", {"run_id": "DI-x", "page_count": 1, "pages": {"1": {"page": 1, "route": "weak-native"}}})
            with patch.object(intake, "which", return_value=None):
                intake.ocr(run)
            self.assertEqual(intake.manifest(run)["pages"]["1"]["route"], "ocr-unavailable")
            self.assertEqual(intake.qa(run), 2)
            report = intake.load(run / "qa.json")
            self.assertTrue(report["partial"])
            self.assertFalse(report["complete"])

    def test_successful_ocr_selects_text_and_writes_tsv(self):
        with tempfile.TemporaryDirectory() as temporary:
            run = Path(temporary)
            page = run / "pages" / "0001"
            page.mkdir(parents=True)
            (page / "render.png").write_bytes(b"png")
            intake.dump(run / "manifest.json", {"run_id": "DI-x", "page_count": 1, "pages": {"1": {"page": 1, "route": "mixed-review", "native_path": "pages/0001/native.layout.txt", "text_path": "pages/0001/native.layout.txt"}}})

            def ocr_command(argv, **_):
                stem = Path(argv[2])
                if "tsv" in argv:
                    stem.with_suffix(".tsv").write_text("level\ttext\n")
                else:
                    stem.with_suffix(".txt").write_text("OCR prayer text")
                return Result()

            with patch.object(intake, "which", return_value="/bin/tesseract"), patch.object(intake, "cmd", side_effect=ocr_command):
                intake.ocr(run)
            record = intake.manifest(run)["pages"]["1"]
            self.assertEqual(record["route"], "ocr")
            self.assertEqual(record["text_path"], "pages/0001/ocr.txt")
            self.assertEqual(record["extractor"], "tesseract")
            self.assertEqual(record["raw_text_sha256"], intake.sha(b"OCR prayer text"))
            self.assertTrue((page / "ocr.tsv").exists())

    def test_doctor_requires_renderer_and_tesseract_for_ocr(self):
        available = {"pdfinfo": "/bin/pdfinfo", "pdftotext": "/bin/pdftotext", "tesseract": "/bin/tesseract"}
        with patch.object(intake, "which", side_effect=lambda name: available.get(name)):
            self.assertEqual(intake.doctor(ocr_required=False), 0)
            self.assertEqual(intake.doctor(ocr_required=True), 1)

    def test_ocr_unavailable_page_can_be_retried(self):
        with tempfile.TemporaryDirectory() as temporary:
            run = Path(temporary)
            page = run / "pages" / "0001"
            page.mkdir(parents=True)
            (page / "render.png").write_bytes(b"png")
            intake.dump(run / "manifest.json", {
                "run_id": "DI-x",
                "page_count": 1,
                "pages": {"1": {
                    "page": 1,
                    "route": "ocr-unavailable",
                    "ocr_error": "tesseract missing",
                }},
            })

            def ocr_command(argv, **_):
                stem = Path(argv[2])
                if "tsv" in argv:
                    stem.with_suffix(".tsv").write_text("level\ttext\n")
                else:
                    stem.with_suffix(".txt").write_text("Recovered OCR text")
                return Result()

            with patch.object(intake, "which", return_value="/bin/tesseract"), patch.object(
                intake, "cmd", side_effect=ocr_command
            ):
                intake.ocr(run)
            record = intake.manifest(run)["pages"]["1"]
            self.assertEqual(record["route"], "ocr")
            self.assertEqual(record["ocr_attempts"][0]["route"], "ocr-unavailable")

    def test_ocr_requires_rendered_page(self):
        with tempfile.TemporaryDirectory() as temporary:
            run = Path(temporary)
            intake.dump(run / "manifest.json", {"run_id": "DI-x", "page_count": 1, "pages": {"1": {"page": 1, "route": "mixed-review"}}})
            with patch.object(intake, "which", return_value="/bin/tesseract"):
                with self.assertRaisesRegex(SystemExit, "run render first"):
                    intake.ocr(run)

    def test_safe_path_and_extract_to_discover_end_to_end(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            _, run = self.make_registered_run(directory, page_count=1)

            def extract_command(argv, **_):
                Path(argv[-1]).write_text("Example Feast\nPrinted proper collect.\f")
                return Result()

            with patch.object(intake, "cmd", side_effect=extract_command):
                intake.extract(run)
            corpus = {"ordinary/lauds/collect": reconcile.CorpusEntry("ordinary/lauds/collect", "x", "collect", "Old fallback.")}
            candidates = reconcile.candidates_from_intake(run, {"heading_aliases": [{"pattern": "Example Feast", "owner": "example-feast"}], "slot_aliases": [{"pattern": "Printed proper", "slot": "collect"}]}, corpus, {"example-feast": "Example Feast"})
            reconcile.classify_discovery(candidates, corpus, {"example-feast": "Example Feast"}, [{"owner_id": "example-feast", "slot_ref": "collect", "selected_ref": "ordinary/lauds/collect", "selected_tier": "ordinary"}])
            self.assertEqual(candidates[0].discovery_classification, "missing-override")
            self.assertEqual(reconcile.intake_text(run, {"text_path": "../../outside.txt"}), "")


    def test_qa_marks_failed_ocr_partial(self):
        with tempfile.TemporaryDirectory() as temporary:
            run = Path(temporary)
            intake.dump(run / "manifest.json", {"run_id": "DI-x", "page_count": 1, "pages": {"1": {"page": 1, "route": "ocr-failed"}}})
            self.assertEqual(intake.qa(run), 2)
            self.assertTrue(intake.load(run / "qa.json")["partial"])

if __name__ == "__main__":
    unittest.main()
