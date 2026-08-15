#!/usr/bin/env python3
"""Focused tests for the disposable source reconciliation workflow."""

import argparse
import importlib.util
import json
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

    def test_diurnal_discovery_marks_different_fallback_as_missing_override(self):
        corpus = {
            "ordinary/lauds/collect": SOURCE_RECONCILE.CorpusEntry(
                "ordinary/lauds/collect", "ordinary/lauds.txt", "collect", "Old fallback."
            )
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="diurnal.pdf", source_page=42, hour="lauds",
            office_title="Example Feast", office_variant="", slot="collect",
            latin_incipit="", source_text="Printed proper collect.",
            canonical_owner="example-feast", source_sha256="a" * 64,
            raw_text_sha256="b" * 64,
        )
        SOURCE_RECONCILE.classify_discovery(
            [candidate], corpus, {"example-feast": "Example Feast"},
            [{"owner_id": "example-feast", "hour": "lauds", "slot": "collect",
              "selected_key": "ordinary/lauds/collect", "selected_tier": "ordinary"}],
        )
        self.assertEqual(candidate.discovery_classification, "missing-override")
        self.assertEqual(candidate.runtime_target, "ordinary/lauds/collect")

    def test_diurnal_discovery_recognizes_hour_specific_direct_proper(self):
        corpus = {
            "proper/example-feast/collect-lauds": SOURCE_RECONCILE.CorpusEntry(
                "proper/example-feast/collect-lauds",
                "proper/example-feast.txt",
                "collect-lauds",
                "Existing direct collect.",
            )
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="diurnal.pdf",
            source_page=42,
            hour="lauds",
            office_title="Example Feast",
            office_variant="",
            slot="collect",
            latin_incipit="",
            source_text="Existing direct collect.",
            canonical_owner="example-feast",
        )
        SOURCE_RECONCILE.classify_discovery(
            [candidate],
            corpus,
            {"example-feast": "Example Feast"},
            [{
                "owner_id": "example-feast",
                "hour": "lauds",
                "slot_ref": "collect",
                "selected_ref": "proper/example-feast/collect-lauds",
                "selected_tier": "proper",
                "direct_existing": ["proper/example-feast/collect-lauds"],
            }],
        )
        self.assertEqual(candidate.discovery_classification, "verify-existing")

    def test_diurnal_discovery_does_not_cross_office_hours(self):
        corpus = {
            "proper/example-feast/chapter-vespers": SOURCE_RECONCILE.CorpusEntry(
                "proper/example-feast/chapter-vespers",
                "proper/example-feast.txt",
                "chapter-vespers",
                "Vespers chapter.",
            ),
            "ordinary/lauds/chapter": SOURCE_RECONCILE.CorpusEntry(
                "ordinary/lauds/chapter",
                "ordinary/lauds.txt",
                "chapter",
                "Lauds fallback.",
            ),
        }
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="diurnal.pdf",
            source_page=42,
            hour="lauds",
            office_title="Example Feast",
            office_variant="",
            slot="chapter",
            latin_incipit="",
            source_text="Printed Lauds chapter.",
            canonical_owner="example-feast",
        )
        SOURCE_RECONCILE.classify_discovery(
            [candidate],
            corpus,
            {"example-feast": "Example Feast"},
            [
                {
                    "owner_id": "example-feast",
                    "hour": "vespers",
                    "slot_ref": "chapter",
                    "selected_ref": "proper/example-feast/chapter-vespers",
                    "selected_tier": "proper",
                    "direct_existing": ["proper/example-feast/chapter-vespers"],
                },
                {
                    "owner_id": "example-feast",
                    "hour": "lauds",
                    "slot_ref": "chapter",
                    "selected_ref": "ordinary/lauds/chapter",
                    "selected_tier": "ordinary",
                    "direct_existing": [],
                },
            ],
        )
        self.assertEqual(candidate.runtime_target, "ordinary/lauds/chapter")
        self.assertEqual(candidate.resolution_tier, "ordinary")
        self.assertEqual(candidate.discovery_classification, "missing-override")
        self.assertEqual(candidate.runtime_slot, "chapter-lauds")

    def test_diurnal_discovery_keeps_known_unobserved_owner_source_first(self):
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="diurnal.pdf", source_page=7, hour="lauds",
            office_title="Example Feast", office_variant="", slot="hymn-lauds",
            latin_incipit="", source_text="A printed hymn.", canonical_owner="example-feast",
        )
        SOURCE_RECONCILE.classify_discovery(
            [candidate], {}, {"example-feast": "Example Feast"}, []
        )
        self.assertEqual(candidate.discovery_classification, "known-owner-unobserved")

    def test_diurnal_discovery_outputs_and_proposals_are_advisory(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = pathlib.Path(tmp) / "output"
            proposal_output = output / "proposals"
            candidate = SOURCE_RECONCILE.SourceCandidate(
                source="diurnal.pdf", source_page=7, hour="lauds",
                office_title="Example Feast", office_variant="", slot="collect",
                latin_incipit="", source_text="Reviewed printed collect.",
                canonical_owner="example-feast", source_sha256="a" * 64,
                raw_text_sha256="b" * 64, discovery_classification="missing-override",
            )
            candidate.candidate_id = SOURCE_RECONCILE.diurnal_candidate_id(candidate)
            SOURCE_RECONCILE.write_proper_discovery(output, [candidate], [])
            SOURCE_RECONCILE.write_decisions(output, {
                candidate.candidate_id: {
                    "candidate_id": candidate.candidate_id, "decision": "accept", "note": "accepted source witness"
                }
            })
            SOURCE_RECONCILE.write_advisory_proposals(output, proposal_output)
            proposal = SOURCE_RECONCILE.load_json(proposal_output / "proposals.json")["proposals"][0]
            self.assertEqual(proposal["target"], "proper/example-feast/collect")
            self.assertFalse(proposal["advisory"])
            self.assertEqual(proposal["replacement_text"], "Reviewed printed collect.")
            self.assertFalse((pathlib.Path(tmp) / "data").exists())

    def test_decide_accepts_diurnal_discovery_candidate(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = pathlib.Path(tmp)
            candidate = SOURCE_RECONCILE.SourceCandidate(
                source="diurnal.pdf",
                source_page=7,
                hour="lauds",
                office_title="Example Feast",
                office_variant="",
                slot="collect",
                latin_incipit="",
                source_text="Reviewed collect.",
                canonical_owner="example-feast",
                raw_text_sha256="b" * 64,
            )
            candidate.candidate_id = SOURCE_RECONCILE.diurnal_candidate_id(candidate)
            SOURCE_RECONCILE.write_proper_discovery(output, [candidate], [])
            args = argparse.Namespace(
                output=str(output),
                ids=[candidate.candidate_id],
                decision="accept",
                note="printed page reviewed",
            )
            self.assertEqual(SOURCE_RECONCILE.cmd_decide(args), 0)
            decisions = SOURCE_RECONCILE.load_decisions(output)
            self.assertEqual(decisions[candidate.candidate_id]["decision"], "accept")

    def test_intake_page_text_path_discovers_missing_override(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            page = intake / "pages" / "0001"
            page.mkdir(parents=True)
            text = "Example Feast\nPrinted proper collect."
            (page / "native.layout.txt").write_text(text)
            (page / "page.json").write_text(__import__("json").dumps({
                "page": 1, "source_key": "synthetic", "source_sha256": "a" * 64,
                "native_path": "pages/0001/native.layout.txt", "text_path": "pages/0001/native.layout.txt",
                "raw_text_sha256": "b" * 64, "slot": "collect", "hour": "lauds",
            }))
            (intake / "manifest.json").write_text(__import__("json").dumps({"pages": {"1": {
                "page": 1, "source_key": "synthetic", "source_sha256": "a" * 64,
                "native_path": "pages/0001/native.layout.txt", "text_path": "pages/0001/native.layout.txt",
                "raw_text_sha256": "b" * 64, "slot": "collect", "hour": "lauds",
            }}}))
            corpus = {"ordinary/lauds/collect": SOURCE_RECONCILE.CorpusEntry("ordinary/lauds/collect", "x", "collect", "Old fallback.")}
            candidates = SOURCE_RECONCILE.candidates_from_intake(intake, {"heading_aliases": [{"pattern": "Example Feast", "owner": "example-feast"}]}, corpus, {"example-feast": "Example Feast"})
            SOURCE_RECONCILE.classify_discovery(candidates, corpus, {"example-feast": "Example Feast"}, [{"owner_id": "example-feast", "hour": "lauds", "slot_ref": "collect", "selected_ref": "ordinary/lauds/collect", "selected_tier": "ordinary", "direct_candidates": ["proper/example-feast/collect"]}])
            self.assertEqual(len(candidates), 1)
            self.assertEqual(candidates[0].source_text, text)
            self.assertEqual(candidates[0].discovery_classification, "missing-override")

    def test_intake_segments_multiple_slots_on_one_page(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            page = intake / "pages" / "0001"
            page.mkdir(parents=True)
            text = (
                "Example Feast\n"
                "THE CHAPTER\n"
                "First printed body.\n"
                "THE PRAYERS\n"
                "Second printed body.\n"
            )
            (page / "native.layout.txt").write_text(text)
            (page / "page.json").write_text(__import__("json").dumps({
                "page": 1, "source_key": "synthetic", "source_sha256": "a" * 64,
                "text_path": "pages/0001/native.layout.txt", "raw_text_sha256": "b" * 64,
            }))
            profile = {
                "heading_aliases": [{"pattern": "^Example Feast$", "owner": "example-feast"}],
                "slot_aliases": [
                    {"pattern": "^THE CHAPTER$", "slot": "chapter-lauds", "hour": "lauds"},
                    {"pattern": "^THE PRAYERS$", "slot": "collect", "hour": "lauds"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            self.assertEqual(len(candidates), 2)
            self.assertEqual([candidate.slot for candidate in candidates], ["chapter-lauds", "collect"])
            self.assertEqual([candidate.source_text for candidate in candidates], ["First printed body.", "Second printed body."])
            self.assertNotEqual(candidates[0].candidate_id, candidates[1].candidate_id)
            self.assertNotEqual(candidates[0].raw_text_sha256, candidates[1].raw_text_sha256)
            self.assertEqual(candidates[0].canonical_owner, "example-feast")
            self.assertEqual(candidates[1].hour, "lauds")

    def test_new_office_heading_terminates_previous_slot_body(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            page = intake / "pages" / "0001"
            page.mkdir(parents=True)
            text = (
                "First Feast\nTHE CHAPTER\nFirst body.\n"
                "Second Feast\nTHE PRAYERS\nSecond body.\n"
            )
            (page / "native.layout.txt").write_text(text)
            (page / "page.json").write_text(__import__("json").dumps({
                "page": 1,
                "source_key": "synthetic",
                "source_sha256": "a" * 64,
                "text_path": "pages/0001/native.layout.txt",
                "raw_text_sha256": "b" * 64,
            }))
            profile = {
                "heading_aliases": [
                    {"pattern": "^First Feast$", "owner": "first-feast"},
                    {"pattern": "^Second Feast$", "owner": "second-feast"},
                ],
                "slot_aliases": [
                    {"pattern": "^THE CHAPTER$", "slot": "chapter-lauds"},
                    {"pattern": "^THE PRAYERS$", "slot": "collect"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            self.assertEqual([item.source_text for item in candidates], ["First body.", "Second body."])
            self.assertEqual(
                [item.canonical_owner for item in candidates],
                ["first-feast", "second-feast"],
            )

    def test_intake_carries_unambiguous_owner_across_page_boundary(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            for number, text in ((1, "Example Feast\nTHE CHAPTER\nFirst body."), (2, "THE PRAYERS\nContinued body.")):
                page = intake / "pages" / f"{number:04d}"
                page.mkdir(parents=True)
                (page / "native.layout.txt").write_text(text)
                (page / "page.json").write_text(__import__("json").dumps({
                    "page": number, "source_key": "synthetic", "source_sha256": "a" * 64,
                    "text_path": f"pages/{number:04d}/native.layout.txt", "raw_text_sha256": str(number) * 64,
                }))
            profile = {
                "heading_aliases": [{"pattern": "^Example Feast$", "owner": "example-feast", "hour": "lauds", "variant": "first"}],
                "slot_aliases": [
                    {"pattern": "^THE CHAPTER$", "slot": "chapter-lauds"},
                    {"pattern": "^THE PRAYERS$", "slot": "collect"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            self.assertEqual(len(candidates), 2)
            continued = next(candidate for candidate in candidates if candidate.source_page == 2)
            self.assertEqual(continued.canonical_owner, "example-feast")
            self.assertEqual(continued.hour, "lauds")
            self.assertEqual(continued.office_variant, "first")
            self.assertEqual(continued.source_text, "Continued body.")

    def test_intake_continuation_page_does_not_clear_carried_owner(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            pages = (
                (1, "Example Feast\nIntroductory matter."),
                (2, "Continuation text with no mapped boundary."),
                (3, "THE CHAPTER\nContinued chapter."),
            )
            for number, text in pages:
                page = intake / "pages" / f"{number:04d}"
                page.mkdir(parents=True)
                (page / "native.layout.txt").write_text(text)
                (page / "page.json").write_text(__import__("json").dumps({
                    "page": number,
                    "source_key": "synthetic",
                    "source_sha256": "a" * 64,
                    "text_path": f"pages/{number:04d}/native.layout.txt",
                    "raw_text_sha256": str(number) * 64,
                }))
            profile = {
                "default_hour": "lauds",
                "heading_aliases": [
                    {"pattern": "^Example Feast$", "owner": "example-feast"},
                ],
                "slot_aliases": [
                    {"pattern": "^THE CHAPTER$", "slot": "chapter"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(
                intake, profile, {}, {}
            )
            continued = next(
                candidate for candidate in candidates if candidate.source_page == 3
            )
            self.assertEqual(continued.canonical_owner, "example-feast")
            self.assertEqual(continued.slot, "chapter")

    def test_intake_carries_final_heading_from_multi_office_page(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            pages = (
                (1, "First Feast\nIntroductory matter.\nSecond Feast\nMore introductory matter."),
                (2, "THE CHAPTER\nSecond feast chapter."),
            )
            for number, text in pages:
                page = intake / "pages" / f"{number:04d}"
                page.mkdir(parents=True)
                (page / "native.layout.txt").write_text(text)
                (page / "page.json").write_text(__import__("json").dumps({
                    "page": number,
                    "source_key": "synthetic",
                    "source_sha256": "a" * 64,
                    "text_path": f"pages/{number:04d}/native.layout.txt",
                    "raw_text_sha256": str(number) * 64,
                }))
            profile = {
                "default_hour": "lauds",
                "heading_aliases": [
                    {"pattern": "^First Feast$", "owner": "first-feast"},
                    {"pattern": "^Second Feast$", "owner": "second-feast"},
                ],
                "slot_aliases": [
                    {"pattern": "^THE CHAPTER$", "slot": "chapter"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(
                intake, profile, {}, {}
            )
            by_page = {candidate.source_page: candidate for candidate in candidates}
            self.assertEqual(by_page[1].canonical_owner, "")
            self.assertEqual(by_page[1].mapping_confidence, "ambiguous")
            self.assertEqual(by_page[2].canonical_owner, "second-feast")
            self.assertEqual(by_page[2].slot, "chapter")
            self.assertEqual(by_page[2].source_text, "Second feast chapter.")

    def _write_intake_pages(self, intake, pages, source_sha256="a" * 64, extractor="pdftotext-layout"):
        for number, text in pages:
            page = intake / "pages" / f"{number:04d}"
            page.mkdir(parents=True, exist_ok=True)
            (page / "native.layout.txt").write_text(text)
            (page / "page.json").write_text(__import__("json").dumps({
                "page": number,
                "source_key": "synthetic",
                "source_sha256": source_sha256,
                "text_path": f"pages/{number:04d}/native.layout.txt",
                "raw_text_sha256": str(number) * 64,
                "extractor": extractor,
            }))

    def test_intake_joins_hymn_across_consecutive_pages(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            self._write_intake_pages(intake, (
                (80, "Example Feast\nTHE HYMN\nFirst stanza of the hymn."),
                (81, "Second stanza of the hymn.\nTHE VERSICLE\nV. They declared the work of God.\nR. And wisely considered."),
            ))
            profile = {
                "default_hour": "lauds",
                "heading_aliases": [{"pattern": "^Example Feast$", "owner": "example-feast"}],
                "slot_aliases": [
                    {"pattern": "^THE HYMN$", "slot": "hymn-lauds"},
                    {"pattern": "^THE VERSICLE$", "slot": "versicle-lauds"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            hymns = [item for item in candidates if item.slot == "hymn-lauds"]
            versicles = [item for item in candidates if item.slot == "versicle-lauds"]
            self.assertEqual(len(hymns), 1)
            self.assertEqual(hymns[0].source_page, 80)
            self.assertEqual(hymns[0].source_page_last, 81)
            self.assertIn("First stanza", hymns[0].source_text)
            self.assertIn("Second stanza", hymns[0].source_text)
            self.assertNotIn("They declared", hymns[0].source_text)
            self.assertEqual(len(versicles), 1)
            self.assertEqual(versicles[0].source_page, 81)
            self.assertIn("They declared", versicles[0].source_text)
            self.assertTrue(hymns[0].candidate_id.startswith("DI-"))
            self.assertIn("-P80-P81-", hymns[0].candidate_id)

    def test_intake_does_not_join_gapped_or_mixed_route_pages(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            self._write_intake_pages(intake, (
                (80, "Example Feast\nTHE HYMN\nFirst stanza."),
                (82, "Orphan continuation."),
            ))
            profile = {
                "heading_aliases": [{"pattern": "^Example Feast$", "owner": "example-feast"}],
                "slot_aliases": [{"pattern": "^THE HYMN$", "slot": "hymn-lauds"}],
            }
            gapped = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            hymns = [item for item in gapped if item.slot == "hymn-lauds"]
            self.assertEqual(len(hymns), 1)
            self.assertEqual(hymns[0].source_page, 80)
            self.assertIn(hymns[0].source_page_last, (0, 80))
            self.assertNotIn("Orphan", hymns[0].source_text)

        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            self._write_intake_pages(intake, ((80, "Example Feast\nTHE HYMN\nFirst stanza."),))
            self._write_intake_pages(
                intake, ((81, "Second stanza."),), extractor="tesseract"
            )
            profile = {
                "heading_aliases": [{"pattern": "^Example Feast$", "owner": "example-feast"}],
                "slot_aliases": [{"pattern": "^THE HYMN$", "slot": "hymn-lauds"}],
            }
            mixed = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            hymns = [item for item in mixed if item.slot == "hymn-lauds"]
            self.assertEqual(len(hymns), 1)
            self.assertNotIn("Second stanza", hymns[0].source_text)

    def test_intake_ambiguous_heading_clears_carried_owner(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            pages = (
                (1, "First Feast\nTHE CHAPTER\nFirst body."),
                (2, "AMBIGUOUS HEADING\nTHE PRAYERS\nUnknown body."),
                (3, "THE HYMN\nStill unknown."),
            )
            for number, text in pages:
                page = intake / "pages" / f"{number:04d}"
                page.mkdir(parents=True)
                (page / "native.layout.txt").write_text(text)
                (page / "page.json").write_text(__import__("json").dumps({
                    "page": number, "source_key": "synthetic", "source_sha256": "a" * 64,
                    "text_path": f"pages/{number:04d}/native.layout.txt", "raw_text_sha256": str(number) * 64,
                }))
            profile = {
                "heading_aliases": [
                    {"pattern": "^First Feast$", "owner": "first-feast"},
                    {"pattern": "^AMBIGUOUS HEADING$", "owner": "second-feast"},
                    {"pattern": "^AMBIGUOUS HEADING$", "owner": "third-feast"},
                ],
                "slot_aliases": [
                    {"pattern": "^THE CHAPTER$", "slot": "chapter-lauds"},
                    {"pattern": "^THE PRAYERS$", "slot": "collect"},
                    {"pattern": "^THE HYMN$", "slot": "hymn-lauds"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            by_page = {candidate.source_page: candidate for candidate in candidates}
            self.assertEqual(by_page[1].canonical_owner, "first-feast")
            self.assertEqual(by_page[2].canonical_owner, "")
            self.assertEqual(by_page[2].mapping_confidence, "ambiguous")
            self.assertEqual(by_page[3].canonical_owner, "")

    def test_clean_witness_body_drops_pua_and_folios(self):
        raw = "123\nBehold a great Confessor.\n\uf000noise\n456"
        self.assertEqual(SOURCE_RECONCILE.clean_witness_body(raw), "Behold a great Confessor.\nnoise")

    def test_clean_witness_body_drops_chant_code_lines(self):
        raw = (
            "vi.\n"
            "Bvz fcv vfcv vz gcv z Ghvcz yg.,c{cvgcv vFgv z gcvGhcv\n"
            "T      hou hast crowned him * with glory and honour, O Lord.\n"
        )
        self.assertEqual(
            SOURCE_RECONCILE.clean_witness_body(raw),
            "Thou hast crowned him * with glory and honour, O Lord.",
        )

    def test_apply_packet_skips_common_reprint(self):
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="sanctoral-lauds-2025", source_page=98, hour="lauds",
            office_title="Saints Philip and James", office_variant="", slot="chapter",
            latin_incipit="",
            source_text="Then shall the righteous man stand in great boldness before the face of such as have afflicted him, and made no account of his labours.\nR. Thanks be to God.",
            canonical_owner="ss-philip-james", source_sha256="a" * 64,
            raw_text_sha256="b" * 64, extractor="pdftotext-layout",
            discovery_classification="missing-override",
            runtime_target="commons/apostle-paschal/chapter-lauds",
            text_similarity=0.4,
        )
        corpus = {
            "commons/apostle-paschal/chapter-lauds": SOURCE_RECONCILE.CorpusEntry(
                "commons/apostle-paschal/chapter-lauds",
                "commons/apostle-paschal.txt",
                "chapter-lauds",
                "Then shall the righteous man stand in great boldness before the face of such as have afflicted him, and made no account of his labours.\nR. Thanks be to God.",
            )
        }
        self.assertIsNone(SOURCE_RECONCILE.candidate_to_apply_packet(candidate, set(), corpus))

    def test_candidate_to_apply_packet_skips_unwritable_classes(self):
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="diurnal.pdf", source_page=7, hour="lauds",
            office_title="Example Feast", office_variant="", slot="collect",
            latin_incipit="", source_text="O God, who didst raise up thy servant.",
            canonical_owner="example-feast", source_sha256="a" * 64,
            raw_text_sha256="b" * 64, extractor="pdftotext-layout",
            discovery_classification="rubrical-complex",
        )
        self.assertIsNone(SOURCE_RECONCILE.candidate_to_apply_packet(candidate))
        candidate.discovery_classification = "missing-override"
        packet = SOURCE_RECONCILE.candidate_to_apply_packet(candidate)
        self.assertEqual(packet["action"], "add-section")
        self.assertEqual(packet["target_key"], "proper/example-feast/collect")
        self.assertIn("agent-proposed, not attested", packet["source_comment"])

    def test_qualify_slot_adds_hour_suffix(self):
        self.assertEqual(SOURCE_RECONCILE.qualify_slot("hymn", "lauds"), "hymn-lauds")
        self.assertEqual(SOURCE_RECONCILE.qualify_slot("hymn-lauds", "lauds"), "hymn-lauds")
        self.assertEqual(SOURCE_RECONCILE.qualify_slot("collect", "lauds"), "collect")
        self.assertEqual(SOURCE_RECONCILE.qualify_slot("chapter", "vespers"), "chapter-vespers")

    def test_apply_packet_uses_hour_qualified_target_and_existing_keys(self):
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="sanctoral-lauds-2025", source_page=80, hour="lauds",
            office_title="Father Benedict", office_variant="", slot="hymn",
            latin_incipit="", source_text="Gem of the highest.",
            canonical_owner="st-benedict", source_sha256="a" * 64,
            raw_text_sha256="b" * 64, extractor="pdftotext-layout",
            discovery_classification="missing-override",
        )
        added = SOURCE_RECONCILE.candidate_to_apply_packet(candidate, set())
        self.assertEqual(added["target_key"], "proper/st-benedict/hymn-lauds")
        self.assertEqual(added["action"], "add-section")
        replaced = SOURCE_RECONCILE.candidate_to_apply_packet(
            candidate, {"proper/st-benedict/hymn-lauds"}
        )
        self.assertEqual(replaced["action"], "replace-section")

    def test_legacy_sr_candidate_ids_remain_unchanged(self):
        candidate = SOURCE_RECONCILE.SourceCandidate(
            source="legacy.docx", source_page=7, hour="lauds",
            office_title="Legacy Office", office_variant="", slot="collect",
            latin_incipit="", source_text="A legacy source witness.",
        )
        SOURCE_RECONCILE.assign_candidate_ids([candidate])
        self.assertEqual(candidate.candidate_id, "SR-0001-d28755be")

    def test_discovery_output_records_declared_dependency_hashes(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = pathlib.Path(tmp)
            profile = directory / "profile.json"
            profile.write_text('{"default_hour": "lauds"}')
            output = directory / "output"
            SOURCE_RECONCILE.write_proper_discovery(
                output,
                [],
                [],
                {
                    "profile": {
                        "path": str(profile),
                        "sha256": __import__("hashlib").sha256(profile.read_bytes()).hexdigest(),
                    }
                },
            )
            payload = SOURCE_RECONCILE.load_json(output / "proper-discovery.json")
            self.assertEqual(payload["schema_version"], 1)
            self.assertNotIn("dependency_manifest", payload)
            self.assertEqual(payload["dependencies"]["profile"]["path"], str(profile))

    def test_discovery_dependency_manifest_is_complete_and_deterministic(self):
        with tempfile.TemporaryDirectory(dir=SOURCE_RECONCILE.ROOT / "output") as repo_tmp:
            intake = pathlib.Path(repo_tmp) / "intake"
            page = intake / "pages" / "0001"
            page.mkdir(parents=True)
            text = page / "native.layout.txt"
            text.write_text("Copyrighted printed witness must not be copied into the manifest.")
            page_record = {
                "page": 1,
                "source_sha256": "a" * 64,
                "raw_text_sha256": "b" * 64,
                "text_path": "pages/0001/native.layout.txt",
            }
            (page / "page.json").write_text(json.dumps(page_record))
            (intake / "manifest.json").write_text(json.dumps({"pages": {"1": page_record}}))
            repo_tmp = pathlib.Path(repo_tmp)
            profile = repo_tmp / "profile.json"
            inventory = repo_tmp / "inventory.json"
            profile.write_text('{"default_hour":"lauds"}')
            inventory.write_text('{"rows":[]}')

            first = SOURCE_RECONCILE.discovery_dependency_manifest(
                intake_dir=intake,
                intake_records=SOURCE_RECONCILE.intake_page_records(intake),
                profile_path=profile,
                inventory_path=inventory,
                data_dir=SOURCE_RECONCILE.ROOT / "data",
            )
            second = SOURCE_RECONCILE.discovery_dependency_manifest(
                intake_dir=intake,
                intake_records=SOURCE_RECONCILE.intake_page_records(intake),
                profile_path=profile,
                inventory_path=inventory,
                data_dir=SOURCE_RECONCILE.ROOT / "data",
            )
            self.assertEqual(first, second)
            output = pathlib.Path(repo_tmp) / "discovery-output"
            SOURCE_RECONCILE.write_proper_discovery(
                output, [], [], dependency_manifest=first
            )
            written = SOURCE_RECONCILE.load_json(output / "proper-discovery.json")
            self.assertEqual(written["dependency_manifest"], first)
            entries = first["entries"]
            self.assertEqual(first["schema_version"], 1)
            self.assertEqual(first["roots"]["repository"], ".")
            self.assertTrue(first["roots"]["intake"].startswith("output/"))
            self.assertEqual(entries, sorted(entries, key=lambda item: (item["root"], item["role"], item["kind"], item["path"])))
            self.assertTrue(any(item["role"] == "profile" for item in entries))
            self.assertTrue(any(item["role"] == "resolution-inventory" for item in entries))
            self.assertTrue(any(item["role"] == "intake-artifact" and item["path"] == "manifest.json" for item in entries))
            self.assertTrue(any(item["role"] == "intake-artifact" and item["path"] == "pages/0001/native.layout.txt" for item in entries))
            self.assertTrue(any(item["role"] == "corpus-text" for item in entries))
            self.assertTrue(any(item["role"] == "feast-metadata" for item in entries))
            self.assertTrue(any(item["role"] == "intake-tree" and item["kind"] == "tree" for item in entries))
            self.assertTrue(any(item["role"] == "corpus-tree" and item["kind"] == "tree" for item in entries))
            for entry in entries:
                self.assertFalse(pathlib.PurePosixPath(entry["path"]).is_absolute())
                self.assertRegex(entry["sha256"], r"^[0-9a-f]{64}$")
            self.assertNotIn("Copyrighted printed witness", json.dumps(first))

            (intake / "new-artifact.txt").write_text("later input")
            changed = SOURCE_RECONCILE.discovery_dependency_manifest(
                intake_dir=intake,
                intake_records=SOURCE_RECONCILE.intake_page_records(intake),
                profile_path=profile,
                inventory_path=inventory,
                data_dir=SOURCE_RECONCILE.ROOT / "data",
            )
            tree_digest = lambda payload: next(
                item["sha256"] for item in payload["entries"]
                if item["role"] == "intake-tree"
            )
            self.assertNotEqual(tree_digest(first), tree_digest(changed))

    def test_discovery_dependency_manifest_detects_profile_and_inventory_changes(self):
        with tempfile.TemporaryDirectory(dir=SOURCE_RECONCILE.ROOT / "output") as repo_tmp:
            intake = pathlib.Path(repo_tmp) / "intake"
            intake.mkdir()
            (intake / "manifest.json").write_text('{"pages":[]}')
            repo_tmp = pathlib.Path(repo_tmp)
            profile = repo_tmp / "profile.json"
            inventory = repo_tmp / "inventory.json"
            profile.write_text('{"version":1}')
            inventory.write_text('{"rows":[]}')
            def build():
                return SOURCE_RECONCILE.discovery_dependency_manifest(
                    intake_dir=intake, intake_records=[], profile_path=profile,
                    inventory_path=inventory, data_dir=SOURCE_RECONCILE.ROOT / "data",
                )
            before = build()
            profile.write_text('{"version":2}')
            after_profile = build()
            inventory.write_text('{"rows":[{"owner_id":"example"}]}')
            after_inventory = build()
            def digest(payload, role):
                return next(item["sha256"] for item in payload["entries"] if item["role"] == role)
            self.assertNotEqual(digest(before, "profile"), digest(after_profile, "profile"))
            self.assertNotEqual(digest(after_profile, "resolution-inventory"), digest(after_inventory, "resolution-inventory"))

    def test_discovery_dependency_manifest_rejects_outside_root_artifacts(self):
        with tempfile.TemporaryDirectory(dir=SOURCE_RECONCILE.ROOT / "output") as tmp, tempfile.TemporaryDirectory() as foreign:
            intake = pathlib.Path(tmp) / "intake"
            intake.mkdir()
            (intake / "manifest.json").write_text('{"pages":[]}')
            foreign_profile = pathlib.Path(foreign) / "profile.json"
            foreign_profile.write_text('{}')
            with self.assertRaisesRegex(ValueError, "outside its allowed root"):
                SOURCE_RECONCILE.discovery_dependency_manifest(
                    intake_dir=intake, intake_records=[], profile_path=foreign_profile,
                    inventory_path=None, data_dir=SOURCE_RECONCILE.ROOT / "data",
                )
            outside = pathlib.Path(foreign) / "witness.txt"
            outside.write_text("outside")
            with self.assertRaisesRegex(ValueError, "outside its allowed root"):
                SOURCE_RECONCILE.declared_intake_artifact_paths(
                    intake, [{"text_path": "../witness.txt"}]
                )

    def test_legacy_intake_dependency_manifest_is_deterministic(self):
        with tempfile.TemporaryDirectory(dir=SOURCE_RECONCILE.ROOT / "output") as tmp:
            intake = pathlib.Path(tmp) / "intake"
            for number in (2, 1):
                page = intake / "pages" / f"{number:04d}"
                page.mkdir(parents=True)
                (page / "page.json").write_text(json.dumps({
                    "page": number, "source_sha256": "a" * 64,
                    "raw_text_sha256": str(number) * 64,
                }))
            records = SOURCE_RECONCILE.intake_page_records(intake)
            first = SOURCE_RECONCILE.discovery_dependency_manifest(
                intake_dir=intake, intake_records=records, profile_path=None,
                inventory_path=None, data_dir=SOURCE_RECONCILE.ROOT / "data",
            )
            second = SOURCE_RECONCILE.discovery_dependency_manifest(
                intake_dir=intake, intake_records=list(reversed(records)), profile_path=None,
                inventory_path=None, data_dir=SOURCE_RECONCILE.ROOT / "data",
            )
            self.assertEqual(first, second)
            intake_paths = [
                item["path"] for item in first["entries"]
                if item["root"] == "intake" and item["kind"] == "file"
            ]
            self.assertEqual(intake_paths, ["pages/0001/page.json", "pages/0002/page.json"])

    def test_profile_page_alias_uses_python_capture_expansion(self):
        self.assertEqual(
            SOURCE_RECONCILE.profile_page_alias(
                42,
                [{"pattern": "^([0-9]+)$", "printed_page": "\\1"}],
            ),
            "42",
        )

    def test_manifest_rejects_foreign_page_artifact(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            page = intake / "pages" / "0001"
            page.mkdir(parents=True)
            declared = {
                "page": 1,
                "id": "DI-aaaaaaaaaaaa-P0001-bbbbbbbbbbbb",
                "source_sha256": "a" * 64,
                "raw_text_sha256": "b" * 64,
            }
            (intake / "manifest.json").write_text(__import__("json").dumps({
                "run_id": "DI-aaaaaaaaaaaa",
                "source_sha256": "a" * 64,
                "pages": {"1": declared},
            }))
            foreign = dict(declared, source_sha256="c" * 64)
            (page / "page.json").write_text(__import__("json").dumps(foreign))
            with self.assertRaisesRegex(ValueError, "foreign source document"):
                SOURCE_RECONCILE.intake_page_records(intake)

    def test_source_reconcile_output_is_confined(self):
        with self.assertRaisesRegex(ValueError, "must stay below"):
            SOURCE_RECONCILE.require_output_path(SOURCE_RECONCILE.ROOT / "data" / "texts")

    def test_source_document_change_clears_carried_heading_context(self):
        with tempfile.TemporaryDirectory() as tmp:
            intake = pathlib.Path(tmp) / "intake"
            pages = (
                (1, "a" * 64, "First Feast\nTHE CHAPTER\nFirst body."),
                (2, "c" * 64, "THE PRAYERS\nForeign body."),
            )
            for number, source_hash, text in pages:
                page = intake / "pages" / f"{number:04d}"
                page.mkdir(parents=True)
                (page / "native.layout.txt").write_text(text)
                (page / "page.json").write_text(__import__("json").dumps({
                    "page": number,
                    "source_sha256": source_hash,
                    "text_path": f"pages/{number:04d}/native.layout.txt",
                    "raw_text_sha256": str(number) * 64,
                }))
            profile = {
                "heading_aliases": [{"pattern": "^First Feast$", "owner": "first-feast"}],
                "slot_aliases": [
                    {"pattern": "^THE CHAPTER$", "slot": "chapter-lauds"},
                    {"pattern": "^THE PRAYERS$", "slot": "collect"},
                ],
            }
            candidates = SOURCE_RECONCILE.candidates_from_intake(intake, profile, {}, {})
            by_page = {candidate.source_page: candidate for candidate in candidates}
            self.assertEqual(by_page[1].canonical_owner, "first-feast")
            self.assertEqual(by_page[2].canonical_owner, "")

if __name__ == "__main__":
    unittest.main()
