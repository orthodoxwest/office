#!/usr/bin/env python3
"""Focused tests for the disposable source reconciliation workflow."""

import importlib.util
import pathlib
import sys
import tempfile
import unittest


SCRIPT = pathlib.Path(__file__).with_name("source-reconcile.py")
SPEC = importlib.util.spec_from_file_location("source_reconcile", SCRIPT)
SOURCE_RECONCILE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = SOURCE_RECONCILE
SPEC.loader.exec_module(SOURCE_RECONCILE)


Paragraph = SOURCE_RECONCILE.Paragraph
OfficeSection = SOURCE_RECONCILE.OfficeSection


class SourceReconcileTest(unittest.TestCase):
    def test_chant_code_detection_preserves_english(self):
        self.assertTrue(
            SOURCE_RECONCILE.is_chant_code(
                "BvvzGhcvvijcvzygcvhjhcg,cvg,c}ccccccccccccccvv"
            )
        )
        self.assertTrue(
            SOURCE_RECONCILE.is_chant_code("BzFgvzzvGkzokzjvvvvzok")
        )
        self.assertFalse(
            SOURCE_RECONCILE.is_chant_code(
                "Be joyful, O daughter of Sion, and exceeding glad."
            )
        )

    def test_extracts_and_maps_standalone_canticle_underlay(self):
        paragraphs = [
            Paragraph(1, "THE GOSPEL CANTICLE: MAGNIFICAT"),
            Paragraph(1, "vii.1 Antiphon on Magnificat. Suscepit Deus"),
            Paragraph(1, "vii.1"),
            Paragraph(1, "BvvzGhcvvijcvzygcvhjhcg,cvg,c}ccccccccccccccvv"),
            Paragraph(1, "G od hath hol–pen † his ser–vant Is–ra–el,"),
            Paragraph(1, "as he pro–mised to A–bra–ham and to his seed."),
            Paragraph(2, "THE GOSPEL CANTICLE: MAGNIFICAT"),
        ]
        candidates = SOURCE_RECONCILE.extract_standalone_canticles(
            "magnificat.docx",
            "vespers",
            "Magnificat",
            paragraphs,
            (
                (
                    "Saturdays Throughout the Year",
                    "ordinary/vespers/magnificat-antiphon-saturday",
                ),
            ),
        )
        self.assertEqual(len(candidates), 1)
        candidate = candidates[0]
        self.assertEqual(
            candidate.corpus_key, "ordinary/vespers/magnificat-antiphon-saturday"
        )
        self.assertEqual(candidate.latin_incipit, "Suscepit Deus")
        self.assertEqual(
            candidate.source_text,
            "God hath holpen * his servant Israel, as he promised to Abraham and to his seed.",
        )

        corpus = {
            candidate.corpus_key: SOURCE_RECONCILE.CorpusEntry(
                candidate.corpus_key,
                "ordinary/vespers.txt",
                "magnificat-antiphon-saturday",
                "God hath holpen * his servant Israel.",
            )
        }
        SOURCE_RECONCILE.reconcile([candidate], corpus, {}, {})
        self.assertEqual(candidate.current_text, corpus[candidate.corpus_key].text)
        self.assertEqual(candidate.title_similarity, 1.0)

    def test_extracts_fixed_ferial_lauds_antiphons(self):
        pages = {
            7: "Antiphon 1: Have mercy † upon me, O God.\nPsalm 50\n"
            "Antiphon 1: Have mercy † upon me, O God.\n",
            8: "Antiphon 2: Consider † my meditation, O Lord.\n",
        }
        candidates = SOURCE_RECONCILE.extract_ferial_lauds_antiphons(
            "ferial-lauds.pdf",
            pages,
            (("Monday", 1, 7), ("Monday", 2, 8)),
        )
        self.assertEqual(len(candidates), 2)
        self.assertEqual(
            candidates[0].corpus_key,
            "ordinary/lauds/psalm-antiphon-1-monday",
        )
        self.assertEqual(candidates[0].source_page, 7)
        self.assertEqual(candidates[0].source_text, "Have mercy * upon me, O God.")
        self.assertEqual(
            candidates[1].source_text, "Consider * my meditation, O Lord."
        )

    def test_rejects_ambiguous_ferial_lauds_page(self):
        with self.assertRaisesRegex(ValueError, "one distinct Antiphon 1 form"):
            SOURCE_RECONCILE.extract_ferial_lauds_antiphons(
                "ferial-lauds.pdf",
                {
                    7: "Antiphon 1: First form.\n"
                    "Antiphon 1: Different form.\n"
                },
                (("Monday", 1, 7),),
            )

    def test_title_and_variant_come_from_office_prelude(self):
        history = [
            Paragraph(39, "december 25"),
            Paragraph(40, "The Nativity of Our Lord"),
            Paragraph(40, "AT I VESPERS"),
        ]
        title, variant = SOURCE_RECONCILE.infer_title_and_variant(history, "vespers")
        self.assertEqual(title, "The Nativity of Our Lord")
        self.assertEqual(variant, "first")

        title, variant = SOURCE_RECONCILE.infer_title_and_variant(
            [Paragraph(48, "December 25 – II Vespers & within the Octave of Christmas")],
            "vespers",
        )
        self.assertIn("II Vespers", title)
        self.assertEqual(variant, "second")

        title, _ = SOURCE_RECONCILE.infer_title_and_variant(
            [
                Paragraph(550, "september 15"),
                Paragraph(551, "The Seven Sorrows of the"),
                Paragraph(551, "Blessed Virgin Mary"),
            ],
            "lauds",
        )
        self.assertEqual(title, "The Seven Sorrows of the Blessed Virgin Mary")

        title, _ = SOURCE_RECONCILE.infer_title_and_variant(
            [
                Paragraph(590, "Common of Many Martyrs"),
                Paragraph(590, "out of Paschaltide"),
                Paragraph(590, "AT I VESPERS"),
            ],
            "vespers",
        )
        self.assertEqual(title, "Common of Many Martyrs out of Paschaltide")

    def test_extracts_structured_slots_from_an_office(self):
        paragraphs = [
            Paragraph(1, "Our Father. Hail Mary. O God, make speed. p. 3"),
            Paragraph(1, "THE PSALMS"),
            Paragraph(1, "Antiphon 1. In illa die"),
            Paragraph(1, "BvvzGhcvvijcvzygcvhjhcg,cvg,c}ccccccccccccccvv"),
            Paragraph(1, "In that day † the mountains shall drop down new wine."),
            Paragraph(1, "Psalm 144. Confiteantur tibi"),
            Paragraph(2, "THE CHAPTER"),
            Paragraph(2, "Romans 13:11"),
            Paragraph(2, "Brethren: It is high time to awake out of sleep."),
            Paragraph(2, "R. Thanks be to God."),
            Paragraph(2, "THE SHORT RESPONSORY"),
            Paragraph(2, "R. Shew us thy mercy, O Lord."),
            Paragraph(2, "THE HYMN"),
            Paragraph(2, "Conditor alme siderum"),
            Paragraph(2, "Creator of the stars of night."),
            Paragraph(2, "THE VERSICLE"),
            Paragraph(2, "V. Drop down, ye heavens, from above."),
            Paragraph(2, "THE GOSPEL CANTICLE: MAGNIFICAT"),
            Paragraph(2, "Antiphon on Magnificat. Ecce nomen"),
            Paragraph(2, "Behold, the Name of the Lord † cometh from afar."),
            Paragraph(2, "Magnificat, tone 1.2"),
            Paragraph(3, "THE PRAYERS"),
            Paragraph(3, "Let us pray. Collect"),
            Paragraph(3, "Stir up thy might, we beseech thee, O Lord."),
            Paragraph(3, "R. Amen."),
        ]
        office = OfficeSection(
            source="vespers.docx",
            hour="vespers",
            title="Saturday before Advent I",
            variant="first",
            start_page=1,
            end_page=3,
            paragraphs=paragraphs,
        )
        candidates = SOURCE_RECONCILE.extract_candidates(office)
        by_slot = {candidate.slot: candidate for candidate in candidates}
        self.assertIn("psalm-antiphon-1", by_slot)
        self.assertIn("chapter-first-vespers", by_slot)
        self.assertIn("hymn-first-vespers", by_slot)
        self.assertIn("magnificat-antiphon-first", by_slot)
        self.assertIn("collect", by_slot)
        self.assertIn(
            "mountains shall drop", by_slot["psalm-antiphon-1"].source_text
        )

    def test_comparison_ignores_pointing_and_typography(self):
        source = "Brethrên: † the household of God; * Jesus Christ."
        current = "Brethren: the household of God: Jesus Christ."
        self.assertGreater(SOURCE_RECONCILE.text_similarity(source, current), 0.98)

    def test_long_repetitive_text_keeps_sequence_anchors(self):
        stanza = "All praise to God the Father and the Son and Holy Ghost. "
        current = stanza * 20
        midpoint = len(current) // 2
        source = current[:midpoint] + "Page header " + current[midpoint:]
        self.assertGreater(
            SOURCE_RECONCILE.anchored_text_similarity(source, current), 0.98
        )

    def test_load_corpus_resolves_use_alias_text(self):
        with tempfile.TemporaryDirectory() as tmp:
            texts = pathlib.Path(tmp) / "texts"
            (texts / "proper").mkdir(parents=True)
            (texts / "commons").mkdir()
            (texts / "commons" / "apostle.txt").write_text(
                "[hymn-lauds]\nLet heaven's exultant praises ring.\n"
            )
            (texts / "proper" / "st-andrew.txt").write_text(
                "[hymn-vespers]\n@use commons/apostle/hymn-lauds\n"
            )

            corpus = SOURCE_RECONCILE.load_corpus(pathlib.Path(tmp))
            self.assertEqual(
                corpus["proper/st-andrew/hymn-vespers"].text,
                "Let heaven's exultant praises ring.",
            )

    def test_slot_compatibility_allows_plain_ordinary_fallback(self):
        self.assertEqual(
            SOURCE_RECONCILE.slot_compatibility("chapter-lauds", "chapter"), 0.90
        )
        self.assertEqual(
            SOURCE_RECONCILE.slot_compatibility(
                "psalm-antiphon-1", "psalm-antiphon"
            ),
            0.96,
        )
        self.assertEqual(
            SOURCE_RECONCILE.slot_compatibility("hymn-lauds", "collect"), 0.0
        )
        self.assertEqual(
            SOURCE_RECONCILE.slot_compatibility(
                "versicle-first-vespers", "versicle-vespers"
            ),
            0.90,
        )

    def test_title_owner_turns_absent_proper_slot_into_gap(self):
        corpus = {
            "proper/advent-sunday-1/collect": SOURCE_RECONCILE.CorpusEntry(
                "proper/advent-sunday-1/collect",
                "proper/advent-sunday-1.txt",
                "collect",
                "Stir up thy power, O Lord.",
            ),
            "ordinary/vespers/short-responsory": SOURCE_RECONCILE.CorpusEntry(
                "ordinary/vespers/short-responsory",
                "ordinary/vespers.txt",
                "short-responsory",
                "How great are thy works, O Lord.",
            ),
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="vespers.docx",
            source_page=8,
            hour="vespers",
            office_title="Saturday before the I Sunday in Advent",
            office_variant="first",
            slot="short-responsory-vespers",
            latin_incipit="",
            source_text="Shew us thy mercy, O Lord.",
        )
        SOURCE_RECONCILE.reconcile(
            [candidate],
            corpus,
            {"advent-sunday-1": "I Sunday of Advent"},
            {},
        )
        self.assertEqual(
            candidate.corpus_key,
            "proper/advent-sunday-1/short-responsory-vespers",
        )
        self.assertEqual(candidate.confidence, "missing")

    def test_owner_matching_distinguishes_numbered_and_ambiguous_titles(self):
        corpus = {
            f"proper/advent-sunday-{number}/collect": SOURCE_RECONCILE.CorpusEntry(
                f"proper/advent-sunday-{number}/collect",
                f"proper/advent-sunday-{number}.txt",
                "collect",
                "Collect",
            )
            for number in range(1, 5)
        }
        names = {
            "annunciation": "Annunciation of the Blessed Virgin Mary",
            "assumption-bvm": "Assumption of the Blessed Virgin Mary",
            "seven-sorrows-bvm": "Seven Sorrows of the B.V.M",
            "solemnity-st-joseph": "Solemnity of St. Joseph, Spouse of the Blessed Virgin Mary",
            "st-joseph": "St. Joseph, Spouse of the Blessed Virgin Mary",
        }
        self.assertEqual(
            SOURCE_RECONCILE.infer_owner(
                "Saturday before the III Sunday in Advent", corpus, names
            )[0],
            "proper/advent-sunday-3",
        )
        self.assertEqual(
            SOURCE_RECONCILE.infer_owner("Blessed Virgin Mary", corpus, names)[0],
            "",
        )
        self.assertEqual(
            SOURCE_RECONCILE.infer_owner(
                "The Solemnity of Saint Joseph", corpus, names
            )[0],
            "proper/solemnity-st-joseph",
        )
        self.assertEqual(
            SOURCE_RECONCILE.infer_owner(
                "The Seven Sorrows of the Blessed Virgin Mary", corpus, names
            )[0],
            "proper/seven-sorrows-bvm",
        )
        names["all-saints"] = "All Saints"
        self.assertEqual(
            SOURCE_RECONCILE.infer_owner(
                "November 1 – II Vespers for The Feast of All Saints",
                corpus,
                names,
            )[0],
            "proper/all-saints",
        )
        self.assertEqual(
            SOURCE_RECONCILE.infer_owner(
                "Bishop, Confessor, & Doctor", corpus, names
            )[0],
            "",
        )

    def test_owner_aliases_duplicate_commemoration_to_principal_feast(self):
        corpus = {
            "proper/conversion-st-paul/collect": SOURCE_RECONCILE.CorpusEntry(
                "proper/conversion-st-paul/collect",
                "proper/conversion-st-paul.txt",
                "collect",
                "O God, who hast taught the whole world.",
            )
        }
        names = {
            "conversion-st-paul": "Conversion of St. Paul",
            "comm-extra-01-25-the-conversion-of-st-paul-the-apostle": (
                "The Conversion of St. Paul The Apostle"
            ),
        }
        self.assertEqual(
            SOURCE_RECONCILE.infer_owner(
                "The Conversion of Saint Paul, Apostle", corpus, names
            )[0],
            "proper/conversion-st-paul",
        )

    def test_absent_advent_proper_uses_seasonal_fallback(self):
        corpus = {
            "proper/advent-sunday-1/collect": SOURCE_RECONCILE.CorpusEntry(
                "proper/advent-sunday-1/collect",
                "proper/advent-sunday-1.txt",
                "collect",
                "Stir up thy power, O Lord.",
            ),
            "seasonal/advent/hymn-lauds": SOURCE_RECONCILE.CorpusEntry(
                "seasonal/advent/hymn-lauds",
                "seasonal/advent.txt",
                "hymn-lauds",
                "A thrilling voice by Jordan rings.",
            ),
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="lauds.docx",
            source_page=13,
            hour="lauds",
            office_title="The First Sunday in Advent",
            office_variant="",
            slot="hymn-lauds",
            latin_incipit="Vox clara ecce intonat",
            source_text="A thrilling voice by Jordan rings.",
        )
        SOURCE_RECONCILE.reconcile([candidate], corpus, {}, {})
        self.assertEqual(candidate.corpus_key, "seasonal/advent/hymn-lauds")
        self.assertEqual(candidate.confidence, "exact")

    def test_advent_first_vespers_matches_date_specific_o_antiphon(self):
        corpus = {
            "proper/advent-sunday-4/collect": SOURCE_RECONCILE.CorpusEntry(
                "proper/advent-sunday-4/collect",
                "proper/advent-sunday-4.txt",
                "collect",
                "Raise up thy power, O Lord.",
            ),
            "seasonal/advent/magnificat-antiphon-december-17": (
                SOURCE_RECONCILE.CorpusEntry(
                    "seasonal/advent/magnificat-antiphon-december-17",
                    "seasonal/advent.txt",
                    "magnificat-antiphon-december-17",
                    "O Wisdom, which camest out of the mouth of the Most High.",
                )
            ),
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="vespers.docx",
            source_page=34,
            hour="vespers",
            office_title="Saturday before the IV Sunday in Advent",
            office_variant="first",
            slot="magnificat-antiphon-first",
            latin_incipit="O Sapientia",
            source_text="O Wisdom, which camest out of the mouth of the Most High.",
        )
        SOURCE_RECONCILE.reconcile([candidate], corpus, {}, {})
        self.assertEqual(
            candidate.corpus_key,
            "seasonal/advent/magnificat-antiphon-december-17",
        )
        self.assertEqual(candidate.confidence, "exact")

    def test_ash_wednesday_uses_weekday_ordinary_fallback(self):
        corpus = {
            "proper/ash-wednesday/collect": SOURCE_RECONCILE.CorpusEntry(
                "proper/ash-wednesday/collect",
                "proper/ash-wednesday.txt",
                "collect",
                "Grant us, O Lord, to begin with holy fasting.",
            ),
            "ordinary/lauds/psalm-antiphon-1-wednesday": (
                SOURCE_RECONCILE.CorpusEntry(
                    "ordinary/lauds/psalm-antiphon-1-wednesday",
                    "ordinary/lauds.txt",
                    "psalm-antiphon-1-wednesday",
                    "Wash me throughly, O Lord, from my wickedness.",
                )
            ),
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="lauds.docx",
            source_page=179,
            hour="lauds",
            office_title="Ash Wednesday",
            office_variant="",
            slot="psalm-antiphon-1",
            latin_incipit="",
            source_text="Wash me throughly, O Lord, from my wickedness.",
        )
        SOURCE_RECONCILE.reconcile([candidate], corpus, {}, {})
        self.assertEqual(
            candidate.corpus_key,
            "ordinary/lauds/psalm-antiphon-1-wednesday",
        )
        self.assertEqual(candidate.confidence, "exact")

    def test_different_first_vespers_text_becomes_an_override_gap(self):
        corpus = {
            "commons/martyr/versicle-vespers": SOURCE_RECONCILE.CorpusEntry(
                "commons/martyr/versicle-vespers",
                "commons/martyr.txt",
                "versicle-vespers",
                "V. The righteous shall flourish like a palm tree.",
            )
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="vespers.docx",
            source_page=10,
            hour="vespers",
            office_title="Common of One Martyr out of Paschaltide",
            office_variant="first",
            slot="versicle-first-vespers",
            latin_incipit="",
            source_text="V. Thou hast crowned him with glory and honour, O Lord.",
        )
        second = SOURCE_RECONCILE.SourceCandidate(
            source="vespers.docx",
            source_page=20,
            hour="vespers",
            office_title="II Vespers for Common of One Martyr out of Paschaltide",
            office_variant="second",
            slot="versicle-vespers",
            latin_incipit="",
            source_text="V. The righteous shall flourish like a palm tree.",
        )
        SOURCE_RECONCILE.reconcile([candidate, second], corpus, {}, {})
        self.assertEqual(
            candidate.corpus_key, "commons/martyr/versicle-first-vespers"
        )
        self.assertEqual(candidate.confidence, "missing")

    def test_matching_first_vespers_text_uses_generic_fallback(self):
        hymn = " ".join(
            ["Let heaven and earth their joyful praises sing."] * 20
        )
        corpus = {
            "proper/st-andrew/hymn-vespers": SOURCE_RECONCILE.CorpusEntry(
                "proper/st-andrew/hymn-vespers",
                "proper/st-andrew.txt",
                "hymn-vespers",
                hymn,
            )
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="vespers.docx",
            source_page=230,
            hour="vespers",
            office_title="Saint Andrew, Apostle",
            office_variant="first",
            slot="hymn-first-vespers",
            latin_incipit="Exsultet caelum laudibus",
            source_text=hymn + " November 29 - I Vespers for Saint Andrew",
        )
        second = SOURCE_RECONCILE.SourceCandidate(
            source="vespers.docx",
            source_page=240,
            hour="vespers",
            office_title="II Vespers for Saint Andrew, Apostle",
            office_variant="second",
            slot="hymn-vespers",
            latin_incipit="Deus, tuorum militum",
            source_text="A genuinely different later hymn.",
        )
        SOURCE_RECONCILE.reconcile([candidate, second], corpus, {}, {})
        self.assertEqual(candidate.corpus_key, "proper/st-andrew/hymn-vespers")
        self.assertNotEqual(candidate.confidence, "missing")

    def test_saturday_ordinary_does_not_replace_all_week_fallback(self):
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="vespers.docx",
            source_page=214,
            hour="vespers",
            office_title="Saturdays Throughout the Year",
            office_variant="",
            slot="short-responsory-vespers",
            latin_incipit="",
            source_text="Great is our Lord, and great is his power.",
        )
        SOURCE_RECONCILE.reconcile([candidate], {}, {}, {})
        self.assertEqual(
            candidate.corpus_key, "ordinary/vespers/short-responsory-saturday"
        )
        self.assertEqual(candidate.confidence, "missing")


if __name__ == "__main__":
    unittest.main()
