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

    def test_feast_pages_uses_date_run_and_title_confirmation(self):
        index = {"pages": [
            {"pdf_page": 10, "png": "10.png", "printed_page": "565", "inferred": False,
             "text": "July 15, 16\nOTHER FEAST", "layout_text": "July 15, 16"},
            {"pdf_page": 11, "png": "11.png", "printed_page": "566", "inferred": False,
             "text": "566  July 16, 17, 18, 19\n(JULY 17)\nST. OSMUND",
             "layout_text": "566  July 16, 17, 18, 19\nTRANSLATION OF ST. OSMUND"},
            {"pdf_page": 12, "png": "12.png", "printed_page": "567", "inferred": False,
             "text": "July 16, 17, 18, 19\ncontinued text", "layout_text": "July 16, 17, 18, 19"},
            {"pdf_page": 13, "png": "13.png", "printed_page": "568", "inferred": False,
             "text": "July 18, 19, 20\nnext feast", "layout_text": "July 18, 19, 20"},
        ]}
        found = pages.locate_feast_pages(index, 7, 17, "Translation of St. Osmund")
        self.assertEqual([page["pdf_page"] for page in found["pages"]], [11, 12])
        self.assertEqual(found["locate_confidence"], "high")

    def test_cli_feast_pages(self):
        index = {"pages": [{
            "pdf_page": 11, "png": "11.png", "printed_page": "566", "inferred": False,
            "text": "(JULY 17)\nTRANSLATION OF ST. OSMUND", "layout_text": "July 17",
        }]}
        cache = self.root / "book"
        cache.mkdir(exist_ok=True)
        (cache / "index.json").write_text(json.dumps(index), encoding="utf-8")
        with mock.patch.object(pages, "PAGES_ROOT", self.root):
            with mock.patch("builtins.print") as output:
                self.assertEqual(pages.main([
                    "feast-pages", "7", "17", "Translation of St. Osmund", "--key", "book",
                ]), 0)
                self.assertEqual(json.loads(output.call_args.args[0])["pages"][0]["pdf_page"], 11)

    def test_temporal_name_locator_ignores_roman_table_of_contents(self):
        index = {"pages": [
            {"pdf_page": 8, "png": "8.png", "printed_page": "x", "inferred": False,
             "text": "CORPUS CHRISTI ........ 401", "layout_text": "TABLE OF CONTENTS"},
            {"pdf_page": 447, "png": "447.png", "printed_page": "401", "inferred": False,
             "text": "THE FEAST OF CORPUS CHRISTI", "layout_text": "CORPUS CHRISTI"},
        ]}
        found = pages.locate_named_pages(index, "Corpus Christi")
        self.assertEqual(found["pages"][0]["pdf_page"], 447)

    def test_temporal_name_locator_rejects_scattered_ordinary_rubric_words(self):
        index = {"pages": [
            {"pdf_page": 49, "png": "49.png", "printed_page": "3", "inferred": False,
             "text": "Monday at Prime\nTo God the Holy Paraclete", "layout_text": "Monday at Prime"},
            {"pdf_page": 350, "png": "350.png", "printed_page": "304", "inferred": False,
             "text": "MONDAY IN HOLY WEEK", "layout_text": "MONDAY IN HOLY WEEK"},
        ]}
        found = pages.locate_named_pages(index, "Monday in Holy Week")
        self.assertEqual(found["pages"][0]["pdf_page"], 350)


if __name__ == "__main__":
    unittest.main()
