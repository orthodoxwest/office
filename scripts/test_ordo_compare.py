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

    def test_extracts_appointments_without_comm_prefix(self):
        # Printed appointments: 2026 ordo pp. 32, 37, 56, 61. In particular,
        # the HC flag must not hide the preceding named commemoration.
        for name, quotation, ending in [
            ("Polycarp", "He that hateth", "Suff. (42f)"),
            ("Martyrs", "For theirs", "Suff. (145f)"),
            ("Soter & Caius", "Daughters of Jerusalem", "Comm. HC (43)"),
            ("Boniface", "O thou Priest", "No Comm. HC"),
        ]:
            with self.subTest(name=name):
                section = f'Lauds W / Col. (371) / {name} (\u201c{quotation}\u201d 2*; Col. suppl.) / {ending}'
                self.assertEqual(ORDO_COMPARE.pdf_commemorations(section), [name])

    def test_does_not_infer_commemoration_from_an_ordinary_slot(self):
        for section in [
            'Lauds / Ben. Ant. \u201cThe Lord\u201d / Col. (371)',
            'Vespers / Ant. (\u201cThe Lord\u201d 2*; Col. suppl.)',
            'Lauds / Polycarp (32)',
            'Lauds / Polycarp (\u201cHe that hateth\u201d 1*)',
        ]:
            with self.subTest(section=section):
                self.assertIsNone(ORDO_COMPARE.pdf_commemorations(section))

    def test_extracts_separate_slash_delimited_commemorations(self):
        # May 9, 2026: the second name is both unprefixed and abbreviated.
        section = ('Vespers / Comm. Gregory (\u201cO Teacher\u201d 35*; Col. 37*) '
                   '/ Gordian &c. (\u201cLight perpetual\u201d 2*; Col. 526)/ Comm. HC (146f)')
        self.assertEqual(ORDO_COMPARE.pdf_commemorations(section), ["Gregory", "Gordian &c."])
        self.assertEqual(
            ORDO_COMPARE.pdf_commemorations(section.replace('/ Gordian', '/ Comm. Gordian')),
            ["Gregory", "Gordian &c."],
        )

    def test_redundant_comm_prefix_does_not_duplicate_a_name(self):
        self.assertEqual(
            ORDO_COMPARE.pdf_commemorations('Lauds / Comm. Comm. Walburga (\u201cThe kingdom\u201d 4*) / Suff.'),
            ["Walburga"],
        )

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
            ("St Savior", "Dedication of the Basilica of St Saviour"),
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

    def test_bvm_shorthand_uses_the_printed_feast_context(self):
        section = 'Vespers / Comm. BVM ("O ever blessed"; Col. 665) / No Suff.'
        title = "The Presentation of the B.V.M.                 Gd\nSt Gelasius M"
        self.assertEqual(ORDO_COMPARE.pdf_commemorations(section, title),
                         ["The Presentation of the B.V.M."])
        for title in ("", "Holy Guardian Angels                 Gd", "St Mary Magdalene",
                      "Solemnity of St Joseph, Spouse of the Blessed Virgin Mary",
                      "St Anne, Mother of the B.V.M."):
            self.assertEqual(ORDO_COMPARE.pdf_commemorations(section, title), ["BVM"])
        names = ORDO_COMPARE.pdf_commemorations(section, "The Most Holy Name of Mary")
        self.assertEqual(ORDO_COMPARE.match_commemorations(names, ["Presentation of the B.V.M."]),
                         (["The Most Holy Name of Mary"], ["Presentation of the B.V.M."]))


class ReferenceParsingTest(unittest.TestCase):
    def test_octave_weekday_names_preserve_day_and_season(self):
        for number, weekday in zip(("II", "III", "IV", "V", "VI", "VII"),
                                   ("Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday")):
            for season in ("Easter", "Pentecost"):
                numbered = f"Day {number} within the Octave of {season}"
                named = f"{weekday} in Easter Week" if season == "Easter" else f"Ember {weekday} in Pentecost"
                self.assertTrue(ORDO_COMPARE.calendar_titles_match(numbered, named))
                self.assertTrue(ORDO_COMPARE.calendar_titles_match(numbered, f"{weekday} within the Octave of {season}"))
                self.assertFalse(ORDO_COMPARE.calendar_titles_match(numbered, "Feria"))
        self.assertTrue(ORDO_COMPARE.calendar_titles_match("EASTER MONDAY", "Monday in Easter Week"))
        self.assertTrue(ORDO_COMPARE.calendar_titles_match("EASTER TUESDAY", "Tuesday in Easter Week"))
        self.assertTrue(ORDO_COMPARE.calendar_titles_match(
            "Day VII within the Octave of Easter", "Saturday before Low Sunday (Sabbato in Albis)"))
        self.assertFalse(ORDO_COMPARE.calendar_titles_match(
            "Day IV within the Octave of Easter", "Friday in Easter Week"))
        self.assertFalse(ORDO_COMPARE.calendar_titles_match(
            "Day IV within the Octave of Easter", "Ember Wednesday in Pentecost"))

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
