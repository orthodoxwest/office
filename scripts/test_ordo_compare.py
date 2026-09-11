#!/usr/bin/env python3
"""Focused tests for the ordo comparison parser and normalization."""

import importlib.util
import pathlib
import tempfile
import datetime
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


class ReferenceParsingTest(unittest.TestCase):
    def parse(self, text, year=None):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "reference.txt"
            path.write_text(text)
            return ORDO_COMPARE.pdf_days(path, year=year)

    def test_december_typo_and_next_year_boundary(self):
        # Synthetic offices exercise the layout without embedding a book page.
        text = "Pro Anno Domini MMXXVI\n" + "\n".join(ORDO_COMPARE.MONTHS) + "\n"
        for day in range(1, 24):
            weekday = datetime.date(2026, 12, day).strftime("%a")
            text += f"{day} {weekday} Feria\nHours Preces\nVespers / II of prec. / No Comm.\n"
        text += ("23 Thu Vigil of the Nativity of Our Lord    V1\n"
                 "Hours No Preces\nVespers / I of fol. / No Comm.\n")
        for day in range(25, 32):
            weekday = datetime.date(2026, 12, day).strftime("%a")
            text += f"{day} {weekday} Feast\nVespers / No Comm.\n"
        text += "JANUARY\n1 Fri Next year\nVespers / Comm. Somebody\nAppendix\n"
        days = self.parse(text)
        self.assertEqual(len(days), 31)
        self.assertIn("Hours Preces", days[12, 23]["Hours"])
        self.assertIn("Hours No Preces", days[12, 24]["Hours"])
        self.assertEqual(days[12, 31]["Vespers"], "Vespers / No Comm.")
        self.assertNotIn((12, 24), self.parse(text, year=2025))

    def test_default_office_excludes_optional_scope_and_page_furniture(self):
        text = ("JANUARY\n1 Thu Feria\n"
                "Lauds / Comm. HC\n    25\n\fJANUARY\nP\n"
                "Announcements Remembrances Seasonal Notes\nA table\n"
                "2 Fri Feast\nHours Preces // Solemnity: No Preces\n"
                "Vespers / Suff. // Solem-\nnity: No Suff.\n")
        days = self.parse(text)
        self.assertEqual(ORDO_COMPARE.pdf_commemorations(days[1, 1]["Lauds"]), [])
        self.assertEqual(days[1, 2]["Hours"], "Hours Preces")
        self.assertEqual(days[1, 2]["Vespers"], "Vespers / Suff.")

    def test_recovers_title_above_date(self):
        days = self.parse("JANUARY\n1 Thu A feast\nVespers / No Comm.\n"
                          "     Day II within the Octave    Sd\n2 Fri\nLauds / No Comm.\n")
        self.assertEqual(ORDO_COMPARE.clean_title(days[1, 2][""]), "Day II within the Octave")
        self.assertNotIn("Day II", days[1, 1]["Vespers"])

    def test_normalization_does_not_hide_special_feria(self):
        self.assertTrue(ORDO_COMPARE.is_ferial("L§ Feria"))
        self.assertFalse(ORDO_COMPARE.is_ferial("Friday after the Octave of Ascension"))
        self.assertEqual(ORDO_COMPARE.clean_title("Lord's Feast"), "Lord's Feast")

    def test_unclosed_quote_and_extraneous_parenthesis(self):
        for text in ['Vespers / Mag. Ant. “The king (123) / Col. Another',
                     'Vespers / Mag. Ant. (“The king” (123) / Col. Another']:
            self.assertEqual(ORDO_COMPARE.antiphon_incipit(text, "Mag"), "The king")
        self.assertTrue(ORDO_COMPARE.incipit_matches("He remem- bered", "He remembered his mercy"))
        self.assertFalse(ORDO_COMPARE.incipit_matches("Ask…a much longer fragment", "Ask now"))

    def test_slash_ampersand_commemoration_and_specific_matching(self):
        names = ORDO_COMPARE.pdf_commemorations(
            'Vespers / Comm. Oct. (“Today” 123) / & Damasus (“Well done” 4*) / No Suff.')
        self.assertEqual(names, ["Oct.", "Damasus"])
        self.assertEqual(ORDO_COMPARE.match_commemorations(
            ["Oct.", "Sun."], ["Sunday within the Nativity Octave"]), (["Oct."], []))
        self.assertEqual(ORDO_COMPARE.match_commemorations(
            ["Oct.", "Sun."], ["Sunday within the Nativity Octave", "Day IV within the Nativity Octave"]), ([], []))


if __name__ == "__main__":
    unittest.main()
