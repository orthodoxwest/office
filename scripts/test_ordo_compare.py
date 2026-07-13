#!/usr/bin/env python3
"""Focused tests for the ordo comparison parser and normalization."""

import importlib.util
import pathlib
import unittest


SCRIPT = pathlib.Path(__file__).with_name("ordo-compare.py")
SPEC = importlib.util.spec_from_file_location("ordo_compare", SCRIPT)
ORDO_COMPARE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ORDO_COMPARE)


class CommemorationComparisonTest(unittest.TestCase):
    def test_extracts_repeated_and_unprefixed_commemorations(self):
        section = (
            'Lauds W / Ppr. / Comm. Innocents (“These are they” & Col. 202) '
            '& Comm. Titus (“Well done” 3*; Col. 454) / No Suff.'
        )
        self.assertEqual(
            ORDO_COMPARE.pdf_commemorations(section),
            ["Innocents", "Titus"],
        )

        section = (
            'Vespers V / Col. / Comm. Venantius (“O ye holy” 20*) '
            '& Pudentiana (“Come thou Bride” 4*) / No Comm. HC'
        )
        self.assertEqual(
            ORDO_COMPARE.pdf_commemorations(section),
            ["Venantius", "Pudentiana"],
        )

        # Some pdftotext rows leave the outer parenthesis unbalanced; the
        # inner page-reference close still separates the items.
        section = (
            'Vespers W / Comm. Innocents (“These are they” (221; Col. 202) '
            '& Titus (“O thou priest” 3*; Col. 454) / No Suff.'
        )
        self.assertEqual(
            ORDO_COMPARE.pdf_commemorations(section),
            ["Innocents", "Titus"],
        )

    def test_stops_before_positive_suffrage_and_trailing_rubrics(self):
        section = (
            'Lauds V / Comm. Walburga (“The kingdom” 4*) / Suff. (42f) '
            'Blessing & distribution of ashes (violet)'
        )
        self.assertEqual(ORDO_COMPARE.pdf_commemorations(section), ["Walburga"])

    def test_ignores_holy_cross_flag_not_present_in_name_column(self):
        section = 'Lauds / Comm. HC (“O Cross” 12*) / No Suff.'
        self.assertEqual(ORDO_COMPARE.pdf_commemorations(section), [])

    def test_distinguishes_explicit_none_from_unparsed(self):
        self.assertEqual(ORDO_COMPARE.pdf_commemorations("Lauds / No Comm. / No Suff."), [])
        self.assertIsNone(ORDO_COMPARE.pdf_commemorations("Lauds / Ppr. / No Suff."))
        self.assertIsNone(ORDO_COMPARE.pdf_commemorations(None))

    def test_normalizes_style_and_abbreviations(self):
        pairs = [
            ("St Sylvester, Bishop & Confessor", "St. Sylvester I, Pope"),
            ("Ss Peter & Paul, App.", "Ss. Peter and Paul, Apostles"),
            ("Sun.", "IV Sunday after Pentecost"),
            ("Fer.", "Friday after Lent III"),
            ("Oct.", "Day IV within the Octave of Easter"),
            ("Khashas", "Ss Nicholas & Habib Khasha, Martyrs"),
            ("Dorothea", "St Dorothy, Virgin & Martyr"),
            ("Alexan- der &c.", "Ss. Alexander, Eventius & Theodulus, Martyrs"),
            ("BVM", "Saturday Office of the B.V.M."),
            ("BMV", "Saturday Office of the B.V.M."),
            ("B.V.M.", "Saturday Office of the BVM"),
            ("B.M.V.", "Saturday Office of the BVM"),
        ]
        for printed, ours in pairs:
            with self.subTest(printed=printed, ours=ours):
                self.assertGreaterEqual(
                    ORDO_COMPARE.commemoration_similarity(printed, ours), 0.6
                )

        self.assertLess(
            ORDO_COMPARE.commemoration_similarity("BVM", "Annunciation of the B.V.M."),
            0.6,
        )

    def test_reports_missing_and_extra_names(self):
        missing, extra = ORDO_COMPARE.match_commemorations(
            ["Titus", "Innocents"],
            ["St. Titus, Bishop & Confessor", "St. Paul, Apostle"],
        )
        self.assertEqual(missing, ["Innocents"])
        self.assertEqual(extra, ["St. Paul, Apostle"])


if __name__ == "__main__":
    unittest.main()
