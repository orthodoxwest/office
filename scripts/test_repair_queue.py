#!/usr/bin/env python3
"""Behavior and evidence boundaries of the repair queue."""

import copy
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("repair_queue", Path(__file__).with_name("repair-queue.py"))
RQ = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = RQ
SPEC.loader.exec_module(RQ)


class RepairQueueTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.data = self.root / "data"
        self.data.mkdir()
        self.book = self.root / "book.pdf"
        self.book.write_bytes(b"test source witness")
        self.source = {"path": "book.pdf", "sha256": RQ.file_digest(self.book), "locator": "p. 5"}
        self.row = {
            "owner_id": "a-feast", "canonical_owner": "a-feast", "hour": "vespers",
            "first_vespers": True, "requested_slot": "chapter", "slot_ref": "chapter",
            "resolver_hour": "vespers", "resolver_slot": "chapter", "context_id": "context-a", "part": "principal",
            "selected_ref": "commons/martyr/chapter-lauds",
            "selected_tier": "common", "reason": "common-fallback",
            "dates": ["2026-01-02", "2026-01-09"], "date": "2026-01-02", "occurrences": 4,
        }
        self.inventory = {"start_year": 2026, "years": 1, "rows": [self.row]}
        self.triduum = {"2026-04-09", "2026-04-10", "2026-04-11"}
        self.comparison = RQ.STATUS.Comparison({"vespers-color": 10}, [])
        self.actual = "commons/martyr/chapter-lauds"
        self.calls = []

    def explain(self, hour, date, form):
        self.calls.append((hour, date, form))
        resolutions = [] if self.actual is None else [{
            "owner_id": "a-feast", "first_vespers": True, "requested_slot": "chapter", "selected_ref": self.actual,
        }]
        return {"unit_key": "a-feast-1v", "resolutions": resolutions, "decisions": [{"rule": "preces", "outcome": "said"}]}

    def observations(self):
        return RQ.resolution_observations(self.inventory, self.data, self.triduum)

    def target(self, bound=True, disposition="confirmed-defect"):
        target = {
            "id": "a-repair", "year": 2026, "title": "A repair", "kind": "appointment",
            "scope": "ordinary-year", "sources": [self.source], "evidence": "Printed appointment differs.",
            "next_action": "Repair the appointed chapter.", "disposition": disposition,
        }
        if bound:
            key = next(iter(self.observations()))
            target.update(resolution_ids=[key], checks=[{"resolution_id": key, "expected_ref": "proper/a-feast/chapter"}])
        else:
            target["checks"] = [{"date": "2026-01-02", "hour": "vespers", "slot": "chapter",
                                 "owner_id": "a-feast", "first_vespers": True, "expected_ref": "proper/a-feast/chapter"}]
        item = self.item(target)
        target.update(context_hash=item["context_hash"], observation_hash=item["observation_hash"])
        return target

    def item(self, target, findings=None, rules=None):
        return RQ.target_item(target, self.observations(), findings or {}, rules or [], self.root, self.explain)

    def queue(self, targets=None, rules=None, discovery=None):
        return RQ.build_queue(2026, self.inventory, self.comparison, rules or [], targets or [],
                              self.data, self.root, self.explain, self.triduum, discovery, catalog={"a-feast"})

    def test_fallback_is_a_candidate_and_impact_counts_dates_not_frames(self):
        report = self.queue()
        item = report["entries"][0]
        self.assertEqual(item["status"], "needs-diagnosis")
        self.assertEqual(item["affected_dates"], 2)
        key = item["id"]
        self.row["selected_ref"] = "ordinary/vespers/chapter"
        self.row["selected_tier"] = "ordinary"
        self.assertEqual(self.queue()["entries"][0]["id"], key)

    def test_repair_requires_passing_checks_on_every_live_date(self):
        target = self.target()
        self.calls.clear()
        self.assertEqual(self.item(target)["status"], "ready-to-repair")
        self.assertEqual({date for _, date, _ in self.calls}, set(self.row["dates"]))
        self.row.update(selected_ref="proper/a-feast/chapter", selected_tier="proper", reason="direct")
        self.actual = "proper/a-feast/chapter"
        self.assertEqual(self.item(target)["status"], "resolved")
        # A regression to a third text needs fresh diagnosis, not an old approval.
        self.actual = "commons/virgin/chapter"
        self.row["selected_ref"] = self.actual
        self.assertEqual(self.item(target)["status"], "needs-diagnosis")

    def test_disappeared_resolution_and_changed_date_population_do_not_resolve(self):
        target = self.target()
        self.actual = "proper/a-feast/chapter"
        self.row["dates"] = ["2026-01-02"]
        self.assertEqual(self.item(target)["status"], "needs-diagnosis")
        self.inventory["rows"] = []
        item = self.item(target)
        self.assertEqual(item["status"], "needs-diagnosis")
        self.assertTrue(any("disappeared" in reason for reason in item["stale"]))

    def test_source_only_requirement_survives_an_unrendered_slot(self):
        self.inventory["rows"] = []
        self.actual = None
        target = self.target(bound=False)
        item = self.queue([target])["entries"][0]
        self.assertEqual(item["status"], "ready-to-repair")
        self.assertFalse(item["checks"][0]["passed"])
        self.actual = "proper/a-feast/chapter"
        self.assertEqual(self.queue([target])["entries"][0]["status"], "resolved")

    def test_changed_book_invalidates_even_a_passing_assertion(self):
        target = self.target(bound=False)
        self.actual = "proper/a-feast/chapter"
        self.book.write_bytes(b"new edition")
        item = self.item(target)
        self.assertEqual(item["status"], "needs-diagnosis")
        self.assertTrue(item["checks"][0]["passed"])
        self.assertEqual(item["sources"][0]["state"], "stale-or-unavailable")

    def test_empty_or_partial_checks_cannot_close_a_bound_target(self):
        target = self.target()
        target["checks"] = []
        self.assertEqual(self.item(target)["status"], "needs-diagnosis")
        target["checks"] = self.target(bound=False)["checks"]
        self.actual = "proper/a-feast/chapter"
        self.assertEqual(self.item(target)["status"], "needs-diagnosis")

    def test_supported_fallback_is_rechecked(self):
        self.actual = "proper/a-feast/chapter"
        target = self.target(bound=False, disposition="supported-fallback")
        self.assertEqual(self.item(target)["status"], "supported-fallback")
        self.actual = "ordinary/vespers/chapter"
        self.assertEqual(self.item(target)["status"], "needs-diagnosis")

    def test_same_source_in_wrong_owner_or_first_vespers_does_not_pass(self):
        check = self.target(bound=False)["checks"][0]
        self.actual = check["expected_ref"]
        self.assertTrue(RQ.evaluate_check(check, self.explain)["passed"])
        for field, value in [("owner_id", "another-feast"), ("first_vespers", False), ("slot", "hymn")]:
            with self.subTest(field=field):
                self.assertFalse(RQ.evaluate_check({**check, field: value}, self.explain)["passed"])

    def test_ordo_groups_reuse_triage_and_do_not_close_pending_rulings(self):
        target = self.target(bound=False)
        del target["disposition"]
        target["finding_ids"] = ["2026:vespers-color:01-02", "2026:magnificat-antiphon:01-02"]
        rules = [RQ.STATUS.TriageRule("2026", "*", "01-02", "suspected-reference-error", "provisional", "248", "proposed erratum")]
        self.actual = "proper/a-feast/chapter"
        self.comparison.findings = [RQ.STATUS.Finding(2026, "vespers-color", "01-02", "ours=w | reference=g"),
                                   RQ.STATUS.Finding(2026, "magnificat-antiphon", "01-02", "ours=a | reference=b")]
        RQ.STATUS.apply_triage(self.comparison.findings, rules)
        report = self.queue([target], rules)
        tracked = next(e for e in report["entries"] if e["id"] == "target:a-repair")
        self.assertEqual(tracked["status"], "suspected-reference-error")
        self.assertEqual(report["ordo"]["differences"], 2)
        self.assertEqual(len(tracked["finding_ids"]), 2)
        rules[0].category = "open-question"
        self.assertEqual(self.item(target, rules=rules)["status"], "awaiting-clergy")
        # A suspected error never becomes a confirmed reference error by confidence alone.
        rules[0].category, rules[0].confidence = "suspected-reference-error", "confirmed"
        self.assertEqual(self.item(target, rules=rules)["status"], "suspected-reference-error")

    def test_passing_check_with_remaining_ordo_defect_is_not_resolved(self):
        target = self.target(bound=False)
        del target["disposition"]
        target["finding_ids"] = ["2026:vespers-color:01-02"]
        finding = RQ.STATUS.Finding(2026, "vespers-color", "01-02", "mismatch")
        rules = [RQ.STATUS.TriageRule("2026", "vespers-color", "01-02", "engine-bug", "confirmed", "1", "defect")]
        self.actual = "proper/a-feast/chapter"
        self.assertEqual(self.item(target, {finding.finding_id: finding}, rules)["status"], "ready-to-repair")

    def test_triduum_candidates_are_separate(self):
        self.row["dates"].append("2026-04-10")
        report = self.queue()
        self.assertEqual(report["summary"]["ordinary-year"]["needs-diagnosis"], 1)
        self.assertEqual(report["summary"]["triduum"]["needs-diagnosis"], 1)
        self.assertNotEqual(report["entries"][0]["id"], report["entries"][1]["id"])

    def test_discovery_never_approves_fallbacks_or_copies_transcriptions(self):
        books = self.root / "books"
        books.mkdir()
        book = books / "Monastic Diurnal.pdf"
        book.write_bytes(b"source")
        record = {
            "feast_id": "a-feast", "source_witness": {"pdf_sha256": RQ.file_digest(book)},
            "slots": [{"slot": "chapter", "decision": "printed-false", "target_key": "proper/a-feast/chapter-first-vespers",
                       "first": {"text": "DO NOT COPY THIS TRANSCRIPTION", "printed_page": "5"},
                       "contexts": [{"hour": "vespers", "date": "2026-01-02", "key": self.actual}]}],
            "extra": [{"hour": "lauds", "section": "unmodelled hymn", "text": "DO NOT COPY THIS TRANSCRIPTION"}],
        }
        path = self.root / "results.jsonl"
        for decision in ["printed-false", "same-as-fallback", "put-and-attest", "needs-human"]:
            record["slots"][0]["decision"] = decision
            path.write_text(json.dumps(record) + "\n")
            report = self.queue(discovery=path)
            self.assertEqual({e["status"] for e in report["entries"]}, {"needs-diagnosis"})
            self.assertEqual(len(report["entries"]), 2)
            self.assertNotIn("DO NOT COPY THIS TRANSCRIPTION", json.dumps(report))
            fallback = next(e for e in report["entries"] if e["kind"] == "fallback")
            self.assertEqual(fallback["discovery"][0]["state"], "unreviewed")
        self.actual = "other/key"
        self.row["selected_ref"] = self.actual
        self.assertEqual(next(e for e in self.queue(discovery=path)["entries"] if e["kind"] == "fallback")["discovery"][0]["state"], "stale-or-unavailable")

    def test_discovery_requires_the_same_first_vespers_target(self):
        path = self.root / "results.jsonl"
        path.write_text(json.dumps({"feast_id": "a-feast", "slots": [{
            "slot": "chapter", "target_key": "proper/a-feast/chapter-vespers", "decision": "printed-false",
            "contexts": [{"hour": "vespers", "date": "2026-01-02", "key": self.actual}],
        }]}) + "\n")
        report = self.queue(discovery=path)
        self.assertEqual(report["intake"]["discovery"]["attached_slots"], 0)
        self.assertEqual(report["intake"]["discovery"]["unmatched_slots"], 1)
        self.assertTrue(any(e["id"].startswith("discovery-unmatched:") for e in report["entries"]))

    def test_discovery_source_must_match_on_the_cited_date(self):
        self.row["dates"] = ["2026-01-02"]
        self.inventory["rows"].append({**self.row, "dates": ["2026-01-09"], "selected_ref": "commons/other/chapter"})
        books = self.root / "books"
        books.mkdir()
        book = books / "Monastic Diurnal.pdf"
        book.write_bytes(b"source")
        path = self.root / "results.jsonl"
        path.write_text(json.dumps({"feast_id": "a-feast", "source_witness": {"pdf_sha256": RQ.file_digest(book)},
            "slots": [{"slot": "chapter", "target_key": "proper/a-feast/chapter-first-vespers",
                       "contexts": [{"date": "2026-01-02", "hour": "vespers", "key": "commons/other/chapter"}]}]}) + "\n")
        observation = self.queue(discovery=path)["entries"][0]
        self.assertEqual(observation["discovery"][0]["state"], "stale-or-unavailable")

    def test_empty_and_malformed_discovery_do_not_imply_coverage(self):
        path = self.root / "empty.jsonl"
        path.write_text("")
        self.assertIn("no usable records", RQ.markdown_report(self.queue(discovery=path)))
        path.write_text('{}\n')
        with self.assertRaisesRegex(ValueError, "expected a feast result record"):
            self.queue(discovery=path)
        path.write_text(json.dumps({"feast_id": "a-feast", "status": "no-pages", "slots": [], "extra": []}) + "\n")
        report = self.queue(discovery=path)
        self.assertEqual(report["intake"]["discovery"]["slotless_records"], 1)
        self.assertTrue(any(e["id"] == "discovery-incomplete:a-feast" for e in report["entries"]))
        self.assertIn("supplied no slot evidence", RQ.markdown_report(report))

    def test_inputs_are_preserved_and_changes_invalidate_snapshot(self):
        ledger = self.root / "repair-queue.json"
        ledger.write_text("source input")
        with self.assertRaisesRegex(ValueError, "overwrite an input"):
            RQ.protect_inputs(self.root, [ledger])
        before = RQ.input_snapshot(self.data, [self.book])
        (self.data / "corpus.txt").write_text("new source")
        self.assertNotEqual(RQ.input_snapshot(self.data, [self.book]), before)
        before = RQ.input_snapshot(self.data, [self.book])
        self.book.write_bytes(b"different source edition")
        self.assertNotEqual(RQ.input_snapshot(self.data, [self.book]), before)

    def test_review_link_preserves_the_checked_prayer_form(self):
        target = self.target(bound=False)
        target["checks"][0]["form"] = "priest"
        self.assertIn("vespers/2026-01-02?form=priest", RQ.csv_report([self.item(target)]))

    def test_distinct_appointment_contexts_do_not_expand_a_fallback_scope(self):
        proper = {**self.row, "context_id": "context-b", "selected_tier": "proper",
                  "selected_ref": "proper/advent/chapter", "dates": ["2026-12-08"]}
        self.inventory["rows"].append(proper)
        observations = self.observations()
        self.assertEqual(len(observations), 2)
        report = self.queue()
        self.assertEqual(len(report["entries"]), 1)
        self.assertEqual(report["entries"][0]["dates"], ["2026-01-02", "2026-01-09"])

    def test_non_catalog_owner_and_appended_office_need_mapping(self):
        self.row["owner_id"] = "unmodelled-temporal-owner"
        self.assertEqual(self.queue()["entries"][0]["kind"], "source-mapping")
        self.row["owner_id"] = "a-feast"
        self.row["part"] = "appended-office-of-the-dead"
        self.assertEqual(self.queue()["entries"][0]["kind"], "source-mapping")
        target = self.target()
        self.actual = "proper/a-feast/chapter"
        self.assertEqual(self.item(target)["status"], "needs-diagnosis")

    def test_absence_requires_context_and_cannot_pass_an_empty_hour(self):
        target = self.target(bound=False)
        check = target["checks"][0]
        del check["expected_ref"]
        check["absent"] = True
        path = self.root / "absent.json"
        path.write_text(json.dumps({"version": 1, "targets": [target]}))
        with self.assertRaisesRegex(ValueError, "companion unit_key"):
            RQ.load_ledger(path)
        target["checks"].append({"date": "2026-01-02", "hour": "vespers", "unit_key": "a-feast-1v"})
        path.write_text(json.dumps({"version": 1, "targets": [target]}))
        RQ.load_ledger(path)
        self.actual = None
        self.assertFalse(RQ.evaluate_check(check, self.explain)["passed"])

    def test_ledger_requires_owner_and_explicit_vespers_scope(self):
        for field in ["owner_id", "first_vespers"]:
            with self.subTest(field=field):
                target = self.target(bound=False)
                del target["checks"][0][field]
                path = self.root / "invalid.json"
                path.write_text(json.dumps({"version": 1, "targets": [target]}))
                with self.assertRaisesRegex(ValueError, field):
                    RQ.load_ledger(path)

    def test_triduum_comes_from_the_calendar_tabula(self):
        self.assertEqual(RQ.triduum_dates("    Easter (Pascha) Day      April 12\n", 2026), self.triduum)
        with self.assertRaisesRegex(ValueError, "Tabula"):
            RQ.triduum_dates("EASTER MONDAY", 2026)

    def test_ledger_rejects_duplicate_bindings_and_parallel_ordo_classifications(self):
        target = self.target(bound=False)
        path = self.root / "ledger.json"

        def load(targets):
            path.write_text(json.dumps({"version": 1, "targets": targets}))
            return RQ.load_ledger(path)

        self.assertEqual(len(load([target])), 1)
        target["finding_ids"] = ["2026:vespers-color:01-02"]
        with self.assertRaisesRegex(ValueError, "ordo classifications"):
            load([target])
        del target["disposition"]
        second = copy.deepcopy(target)
        second["id"] = "another-target"
        with self.assertRaisesRegex(ValueError, "bound more than once"):
            load([target, second])
        target["checks"][0]["expectd_ref"] = "typo"
        with self.assertRaisesRegex(ValueError, "unknown check fields"):
            load([target])

    def test_csv_and_summary_keep_counts_and_exact_finding_ids(self):
        self.comparison.findings = [RQ.STATUS.Finding(2026, "hours-preces", "03-16", "reverse discrepancy", category="open-question", issue="15")]
        report = self.queue()
        self.assertIn("2026:hours-preces:03-16", RQ.csv_report(report["entries"]))
        text = RQ.markdown_report(report, summary_only=True)
        self.assertIn("awaiting-clergy | 1", text)
        self.assertIn("No discovery report supplied", text)
        self.assertIn("not a whole-office correctness score", text)


if __name__ == "__main__":
    unittest.main()
