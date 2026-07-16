#!/usr/bin/env python3
"""Focused tests for project-status parsing and triage."""

import importlib.util
import pathlib
import sys
import tempfile
import unittest


SCRIPT = pathlib.Path(__file__).with_name("project-status.py")
SPEC = importlib.util.spec_from_file_location("project_status", SCRIPT)
PROJECT_STATUS = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = PROJECT_STATUS
SPEC.loader.exec_module(PROJECT_STATUS)


class ProjectStatusTest(unittest.TestCase):
    def test_exact_triage_rule_wins_over_wildcard(self):
        finding = PROJECT_STATUS.Finding(2026, "calendar", "07-16", "detail")
        rules = [
            PROJECT_STATUS.TriageRule(
                "2026", "*", "*", "data-gap", "provisional", "", "broad"),
            PROJECT_STATUS.TriageRule(
                "2026", "calendar", "07-16", "open-question", "confirmed", "11", "exact"),
        ]
        PROJECT_STATUS.apply_triage([finding], rules)
        self.assertEqual(finding.category, "open-question")
        self.assertEqual(finding.issue, "11")
        self.assertEqual(finding.note, "exact")

    def test_unmatched_finding_stays_untriaged(self):
        finding = PROJECT_STATUS.Finding(2026, "calendar", "01-01", "detail")
        PROJECT_STATUS.apply_triage([finding], [])
        self.assertEqual(finding.category, "untriaged")

    def test_expected_proper_slots_honors_suppressions(self):
        with tempfile.TemporaryDirectory() as name:
            data = pathlib.Path(name)
            (data / "feasts").mkdir()
            (data / "feasts" / "test.txt").write_text(
                "[one]\nRank = double\n\n"
                "[two]\nRank = commemoration\n\n"
                "[three]\nRank = double\n"
            )
            (data / "audit-ok.txt").write_text(
                "one collect\n"
                "three *\n"
            )
            self.assertEqual(PROJECT_STATUS.expected_proper_slots(data), 5)

    def test_parses_audit_and_provenance_summaries(self):
        with tempfile.TemporaryDirectory() as name:
            data = pathlib.Path(name)
            (data / "feasts").mkdir()
            (data / "feasts" / "test.txt").write_text("[one]\nRank = double\n")
            audit = """=== Placeholders: 0 corpus entries ===
=== Missing propers: 1 feast(s) ===
  [base]
  [d] One (one)
    missing: collect, magnificat-antiphon

=== Commons fallback: 3 feast(s) ===
=== Sweep 2026: unresolved texts: 0 ===
=== Sweep 2026: ordinary fallbacks on Double+ days: 7 slot(s) ===
"""
            status = PROJECT_STATUS.parse_audit(audit, data)
            self.assertEqual(status.expected_slots, 6)
            self.assertEqual(status.missing_slots, 2)
            self.assertEqual(status.ordinary_fallback_candidates, 7)

        provenance = """=== Corpus provenance: 20 entries ===
  verified           3
  needs-review      12
  source-unknown     5
  page-located       4
  stale              0
"""
        status = PROJECT_STATUS.parse_provenance(provenance)
        self.assertEqual(status.total, 20)
        self.assertEqual(status.verified, 3)
        self.assertEqual(status.source_unknown, 5)

    def test_percent_handles_empty_denominator(self):
        self.assertEqual(PROJECT_STATUS.percent(0, 0), 100.0)
        self.assertEqual(PROJECT_STATUS.percent(1, 4), 25.0)

    def test_cluster_analysis_reports_symptoms_without_assigning_causes(self):
        findings = [
            PROJECT_STATUS.Finding(
                2026, "magnificat-antiphon", "01-01",
                "ours=O Lord, give light * to them | reference=Give light"),
            PROJECT_STATUS.Finding(
                2026, "magnificat-antiphon", "01-02",
                "ours=O Lord, give light * to them | reference=Another text"),
            PROJECT_STATUS.Finding(
                2026, "vespers-commemorations", "01-01",
                "extra=Day II within the Octave"),
            PROJECT_STATUS.Finding(
                2026, "vespers-commemorations", "01-03",
                "missing=Fer."),
            PROJECT_STATUS.Finding(
                2026, "vespers-ownership", "01-01",
                "ours=prec | reference=none | celebration=Holy Thursday"),
            PROJECT_STATUS.Finding(
                2026, "vespers-ownership", "01-04",
                "ours=none | reference=fol", category="open-question"),
            PROJECT_STATUS.Finding(
                2026, "vespers-color", "01-01",
                "ours=w | reference=g"),
            PROJECT_STATUS.Finding(
                2026, "vespers-suffrage", "01-05",
                "ours=False | reference=True"),
            PROJECT_STATUS.Finding(
                2026, "calendar", "01-06", "ours=A | reference=B"),
        ]
        previous = PROJECT_STATUS.Comparison({}, [
            PROJECT_STATUS.Finding(
                2025, "magnificat-antiphon", "01-01", "ours=X | reference=Y"),
        ])
        clusters = PROJECT_STATUS.analyze_clusters(
            PROJECT_STATUS.Comparison({}, findings), previous)

        self.assertEqual(clusters["untriaged"]["total"], 8)
        self.assertEqual(clusters["vespers"]["total"], 8)
        self.assertEqual(clusters["vespers"]["untriaged"], 7)
        self.assertEqual(
            clusters["vespers"]["ownership"]["directions"],
            {"engine-only": 1, "reference-only": 1},
        )
        self.assertEqual(
            clusters["vespers"]["ownership"]
            ["untriaged_engine_only_contexts"],
            {"triduum-easter-pentecost": 1},
        )
        self.assertEqual(
            clusters["vespers"]["commemorations"]["directions"],
            {"extra-only": 1, "missing-only": 1},
        )
        self.assertEqual(
            clusters["vespers"]["magnificat"]
            ["incipit_boundary_or_wording_candidates"], 1)
        self.assertEqual(
            clusters["canticle_antiphons"]
            ["incipit_boundary_or_wording_candidates"], 1)
        self.assertEqual(
            clusters["vespers"]["magnificat"]
            ["repeated_generated_incipits"][0]["count"], 2)
        self.assertEqual(clusters["previous_year_recurrence"]["count"], 1)

        markdown = "\n".join(PROJECT_STATUS.render_cluster_markdown(clusters))
        self.assertIn("symptoms, not automatic root-cause assignments", markdown)
        self.assertIn("wording/boundary review candidates", markdown)


class IncipitMatchTest(unittest.TestCase):
    OC = PROJECT_STATUS.ORDO_COMPARE

    def test_leading_o_interjection_is_ignored(self):
        self.assertTrue(self.OC.incipit_matches(
            "King of glory",
            "O King of glory, * thou Lord of Sabaoth, who triumphing to-day"))
        self.assertTrue(self.OC.incipit_matches(
            "O King of glory", "King of glory, thou Lord of Sabaoth"))

    def test_leading_o_on_both_sides_still_compared(self):
        self.assertTrue(self.OC.incipit_matches(
            "O Teacher right excellent",
            "O Teacher right excellent, * O light of Holy Church"))
        self.assertFalse(self.OC.incipit_matches(
            "O right excellent Teacher",
            "O Teacher right excellent, * O light of Holy Church"))

    def test_s_z_spelling_variants_fold(self):
        self.assertTrue(self.OC.incipit_matches(
            "When Elizabeth",
            "When Elisabeth * heard the salutation of Mary"))

    def test_different_antiphons_still_mismatch(self):
        self.assertFalse(self.OC.incipit_matches(
            "Come, Bride of Christ",
            "All generations shall call me blessed"))


if __name__ == "__main__":
    unittest.main()
