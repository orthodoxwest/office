#!/usr/bin/env python3
"""Offline unit tests for diurnal-pages.py."""

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).with_name("diurnal-pages.py")
SPEC = importlib.util.spec_from_file_location("diurnal_pages", SCRIPT)
pages = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(pages)


class LabelTests(unittest.TestCase):
    def test_detects_arabic_at_edge_not_in_body(self):
        self.assertEqual(pages.detect_printed_label("595\nTHE OFFICE\nPsalm 12\ntext"), "595")

    def test_detects_star_label(self):
        self.assertEqual(pages.detect_printed_label("COMMONS\ntext\n— 72* —"), "72*")
        self.assertIn("1*", pages.ocr_label_candidates({"text": "APPENDIX\ntext\nl*", "layout_text": ""}))

    def test_detects_and_validates_roman(self):
        self.assertEqual(pages.detect_printed_label("xxvi\nPREFACE\ntext"), "xxvi")
        self.assertIsNone(pages.canonical_label("iix"))

    def test_ambiguous_edges_are_null(self):
        self.assertIsNone(pages.detect_printed_label("12\ntext\n13"))

    def test_interpolates_only_matching_bounded_series(self):
        source = [
            {"pdf_page": 1, "printed_page": "xxiv", "inferred": False},
            {"pdf_page": 2, "printed_page": None, "inferred": False},
            {"pdf_page": 3, "printed_page": "xxvi", "inferred": False},
            {"pdf_page": 4, "printed_page": None, "inferred": False},
            {"pdf_page": 5, "printed_page": "1", "inferred": False},
        ]
        result = pages.interpolate_labels(source)
        self.assertEqual(result[1]["printed_page"], "xxv")
        self.assertTrue(result[1]["inferred"])
        self.assertIsNone(result[3]["printed_page"])

    def test_interpolates_star_pages(self):
        result = pages.interpolate_labels([
            {"pdf_page": 1, "printed_page": "7*"},
            {"pdf_page": 2, "printed_page": None},
            {"pdf_page": 3, "printed_page": "9*"},
        ])
        self.assertEqual(result[1]["printed_page"], "8*")

    def test_repairs_roman_run_and_rejects_valid_out_of_sequence_ocr(self):
        result = pages.repair_label_runs([
            {"pdf_page": 23, "printed_page": "xxv", "inferred": False, "text": "", "layout_text": ""},
            {"pdf_page": 24, "printed_page": None, "inferred": False, "text": "", "layout_text": ""},
            {"pdf_page": 25, "printed_page": "mi", "inferred": False, "text": "", "layout_text": ""},
            {"pdf_page": 26, "printed_page": None, "inferred": False, "text": "", "layout_text": ""},
            {"pdf_page": 27, "printed_page": "xxix", "inferred": False, "text": "", "layout_text": ""},
        ])
        self.assertEqual([page["printed_page"] for page in result],
                         ["xxv", "xxvi", "xxvii", "xxviii", "xxix"])
        self.assertTrue(result[2]["inferred"])

    def test_repairs_every_page_inside_star_run(self):
        result = pages.repair_label_runs([
            {"pdf_page": number, "png": f"{number}.png", "printed_page": label,
             "inferred": False, "text": "", "layout_text": ""}
            for number, label in ((714, "1*"), (715, "2*"), (716, None),
                                  (717, "73111"), (718, None), (719, "6*"))
        ])
        self.assertEqual([page["printed_page"] for page in result],
                         ["1*", "2*", "3*", "4*", "5*", "6*"])
        self.assertEqual(pages.locate_page({"pages": result}, "4*")["pdf_page"], 717)


class IndexTests(unittest.TestCase):
    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory()
        self.root = Path(self.tempdir.name)
        self.index = {
            "pages": [
                {"pdf_page": 31, "png": "output/pages/book/0031.png", "printed_page": "ii", "inferred": False,
                 "text": "Preface and calendar", "layout_text": "PREFACE  CALENDAR"},
                {"pdf_page": 624, "png": "output/pages/book/0624.png", "printed_page": "595", "inferred": True,
                 "text": "Blessed Athanasius bishop and confessor", "layout_text": "BLESSED ATHANAS1US   BISHOP"},
                {"pdf_page": 802, "png": "output/pages/book/0802.png", "printed_page": "72*", "inferred": False,
                 "text": "Common of abbots", "layout_text": "COMMON  OF  ABBOTS"},
            ]
        }

    def tearDown(self):
        self.tempdir.cleanup()

    def test_locate(self):
        self.assertEqual(pages.locate_page(self.index, "0595"), {
            "pdf_page": 624, "png": "output/pages/book/0624.png", "inferred": True,
        })
        self.assertEqual(pages.locate_page(self.index, "72*")["pdf_page"], 802)

    def test_find_tolerates_ocr_noise(self):
        found = pages.find_candidates(self.index, "blessed Athanasius bishop")
        self.assertEqual(found[0]["pdf_page"], 624)
        self.assertGreater(found[0]["score"], found[1]["score"])

    def test_cli_locate_and_find_use_synthetic_index(self):
        cache = self.root / "book"
        cache.mkdir()
        (cache / "index.json").write_text(json.dumps(self.index), encoding="utf-8")
        with mock.patch.object(pages, "PAGES_ROOT", self.root):
            with mock.patch("builtins.print") as output:
                self.assertEqual(pages.main(["locate", "book", "595"]), 0)
                self.assertEqual(json.loads(output.call_args.args[0])["pdf_page"], 624)
            with mock.patch("builtins.print") as output:
                self.assertEqual(pages.main(["find", "book", "common abbots"]), 0)
                self.assertEqual(json.loads(output.call_args.args[0])[0]["pdf_page"], 802)


if __name__ == "__main__":
    unittest.main()
