#!/usr/bin/env python3
"""Focused tests for project-status parsing and triage."""

import csv
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
    def repair_row(self):
        return dict(id="chapter-example", scope="ordinary-year", problem="Chapter appointment",
                    examples="2026-01-02, Lauds, private", expected="The cited proper chapter",
                    source="Fixture Diurnal, p. 1", finding_ids="2026:calendar:01-02",
                    issue="", next_action="Confirm the appointment with clergy.")

    def test_backlog_keeps_open_examples_without_current_findings(self):
        rows = [self.repair_row()]
        comparison = PROJECT_STATUS.Comparison({"calendar": 10}, [])
        markdown = PROJECT_STATUS.render_markdown(
            2027, PROJECT_STATUS.ProperStatus(0, 0, 0, 0, 0, 0),
            PROJECT_STATUS.ProvenanceStatus(0, 0, 0, 0, 0), comparison,
            PROJECT_STATUS.analyze_clusters(comparison, None), [], "", "test", rows)
        self.assertIn("chapter-example", markdown)
        self.assertIn("Confirm the appointment with clergy", markdown)
        self.assertIn("2026:calendar:01-02", markdown)
        self.assertEqual(comparison.mismatches, 0)
        self.assertIn("Strict 2027 ordo parity: 100.0%", markdown)

    def test_backlog_separates_triduum_and_does_not_retain_completed_rows(self):
        row = self.repair_row()
        triduum = {**row, "id": "triduum-example", "scope": "triduum", "problem": "Triduum example"}
        markdown = "\n".join(PROJECT_STATUS.render_repair_backlog([row, triduum]))
        ordinary, separate = markdown.split("### Triduum")
        self.assertIn(row["id"], ordinary)
        self.assertNotIn("triduum-example", ordinary)
        self.assertIn("triduum-example", separate)
        markdown = "\n".join(PROJECT_STATUS.render_repair_backlog([]))
        self.assertNotIn(row["id"], markdown)
        self.assertIn("does not establish completeness", markdown)

    def test_backlog_rejects_ambiguous_or_incomplete_rows(self):
        row = self.repair_row()
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "backlog.csv"
            for rows in [[], [row], [row, row], [{**row, "scope": "all"}], [{**row, "source": ""}]]:
                with self.subTest(rows=rows):
                    with path.open("w", newline="") as handle:
                        writer = csv.DictWriter(handle, fieldnames=list(row))
                        writer.writeheader()
                        writer.writerows(rows)
                    if rows in ([], [row]):
                        self.assertEqual(PROJECT_STATUS.load_repair_backlog(path), rows)
                    else:
                        with self.assertRaises(ValueError):
                            PROJECT_STATUS.load_repair_backlog(path)

    def test_only_explicit_vespers_ownership_is_compared(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            reference, ordo, rubrics = (root / name for name in ("ref.txt", "ours.txt", "rubrics.tsv"))
            reference.write_text("JANUARY\n1 Thu Feast\nVespers W / No Comm.\n"
                                 "2 Fri Feast\nVespers W / I of fol. / No Comm.\n")
            ordo.write_text("JANUARY\n1  Thu  Feast [d] w\nVespers w · II prec.\n"
                            "2  Fri  Feast [d] w\nVespers w · II prec.\n")
            rubrics.write_text("date\n")
            comparison = PROJECT_STATUS.compare_ordo(2026, reference, ordo, rubrics)
            self.assertEqual(comparison.comparable["vespers-ownership"], 1)
            findings = [f for f in comparison.findings if f.aspect == "vespers-ownership"]
            self.assertEqual([f.date for f in findings], ["01-02"])

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
        # A finding no rule matches is never guessed into a category.
        unmatched = PROJECT_STATUS.Finding(2026, "calendar", "01-01", "detail")
        PROJECT_STATUS.apply_triage([unmatched], rules[1:])
        self.assertEqual(unmatched.category, "untriaged")

    def test_suspected_errata_receive_no_adjudicated_parity_credit(self):
        findings = [
            PROJECT_STATUS.Finding(2026, "calendar", "01-01", "detail"),
            PROJECT_STATUS.Finding(
                2026, "calendar", "01-02", "detail",
                category="suspected-reference-error", confidence="provisional"),
            PROJECT_STATUS.Finding(
                2026, "calendar", "01-03", "detail",
                category="reference-error", confidence="provisional"),
        ]

        def render():
            comparison = PROJECT_STATUS.Comparison({"calendar": 10}, findings)
            return PROJECT_STATUS.render_markdown(
                2026, PROJECT_STATUS.ProperStatus(0, 0, 0, 0, 0, 0),
                PROJECT_STATUS.ProvenanceStatus(0, 0, 0, 0, 0), comparison,
                PROJECT_STATUS.analyze_clusters(comparison, None), [], "", "test")

        markdown = render()
        self.assertIn("Suspected reference error | 1", markdown)
        self.assertNotIn("Adjudicated ordo parity", markdown)
        findings.append(PROJECT_STATUS.Finding(
            2026, "calendar", "01-04", "detail",
            category="reference-error", confidence="confirmed"))
        markdown = render()
        self.assertIn("Strict 2026 ordo parity: 60.0%", markdown)
        self.assertIn("Adjudicated ordo parity: 70.0%", markdown)
        self.assertIn("when 1 confirmed reference error(s)", markdown)
        self.assertIn("1 confirmed; 1 unconfirmed", markdown)

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

if __name__ == "__main__":
    unittest.main()
