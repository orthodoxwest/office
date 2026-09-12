#!/usr/bin/env python3
"""Offline unit tests for diurnal-discover.py."""

import importlib.util
import copy
import hashlib
import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("diurnal-discover.py")
SPEC = importlib.util.spec_from_file_location("diurnal_discover", SCRIPT)
discover = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(discover)


def inventory_row(**updates):
    row = {
        "owner_id": "st-example", "hour": "lauds", "first_vespers": False,
        "slot_ref": "collect", "resolver_hour": "lauds", "resolver_slot": "collect",
        "selected_ref": "commons/confessor/collect", "selected_tier": "common",
        "date": "2026-07-17",
    }
    row.update(updates)
    return row


def dossier():
    return {
        "feast_id": "st-example", "name": "St. Example", "proper_name": "Example",
        "month": 7, "day": 17, "rank": "double", "category": "confessor", "kind": "fixed",
        "locate_confidence": "high", "locate_status": "matched",
        "pages": [{"pdf_page": 11, "printed_page": "566", "png": "/cache/0011.png"}],
        "fallbacks": [{
            "id": "slot-1", "hour": "lauds", "hours": ["lauds"], "slot": "collect",
            "target_section": "collect", "target_key": "proper/st-example/collect",
            "current_tier": "common", "current_key": "commons/confessor/collect",
            "date": "2026-07-17", "representative_url": "https://office.fly.dev/lauds/2026-07-17",
        }],
    }


def primary(text, printed=True, confidence="high"):
    return {"id": "slot-1", "printed": printed, "text": text if printed else "",
            "printed_page": "566" if printed else "", "confidence": confidence, "note": "visible"}


class FakeRunner:
    def __init__(self, first, second=None):
        self.first = first
        self.second = second
        self.primary_calls = []
        self.secondary_calls = []

    def read_json(self, provider, model, prompt, images, schema, parser):
        self.primary_calls.append((provider, model, prompt, images, schema))
        return {"slots": [self.first], "extra": [], "notes": ""}, 0.01

    def transcribe(self, provider, model, prompt, images):
        self.secondary_calls.append((provider, model, prompt, images))
        return self.second, 0.01


class SelectionTests(unittest.TestCase):
    def test_builds_fixed_and_temporal_dossiers_and_filters_noise(self):
        catalog = {
            "st-example": {"Name": "St. Example", "ProperName": "Example", "Rank": "double",
                           "Category": "confessor", "month": 7, "day": 17, "kind": "fixed"},
            "easter-sunday": {"Name": "Easter Sunday", "Rank": "double-1st-class",
                              "Category": "lord", "month": None, "day": None, "kind": "temporal"},
        }
        inventory = {"rows": [
            inventory_row(),
            inventory_row(slot_ref="alleluia", resolver_slot="alleluia"),
            inventory_row(slot_ref="hymn", resolver_slot="hymn", selected_tier="ordinary-weekday"),
            inventory_row(owner_id="unknown-feast"),
            inventory_row(owner_id="easter-sunday", hour="terce", resolver_hour="terce",
                          slot_ref="short-responsory", resolver_slot="short-responsory",
                          selected_ref="seasonal/easter/short-responsory-terce", selected_tier="seasonal"),
        ]}
        with tempfile.TemporaryDirectory() as directory:
            fixture = Path(directory) / "inventory.json"
            fixture.write_text(json.dumps(inventory), encoding="utf-8")
            found = discover.build_dossiers(discover.parse_inventory(fixture), catalog, Path(directory))
        self.assertEqual([item["feast_id"] for item in found], ["easter-sunday", "st-example"])
        self.assertEqual(found[0]["fallbacks"][0]["target_section"], "short-responsory-terce")
        self.assertEqual(found[1]["fallbacks"][0]["target_key"], "proper/st-example/collect")

    def test_exact_section_names_for_first_vespers_and_commemorations(self):
        self.assertEqual(discover.target_section(inventory_row(
            hour="vespers", resolver_hour="vespers", resolver_slot="chapter", first_vespers=True,
        )), "chapter-first-vespers")
        self.assertEqual(discover.target_section(inventory_row(
            hour="vespers", resolver_hour="vespers", resolver_slot="psalm-antiphon-2",
        )), "psalm-antiphon-2-vespers")
        self.assertEqual(discover.target_section(inventory_row(
            hour="lauds", resolver_hour="lauds", resolver_slot="commemoration-antiphon",
        )), "commemoration-antiphon-lauds")


class PromptTests(unittest.TestCase):
    def test_prompt_defines_printed_cross_references_and_extra(self):
        prompt_text = discover.build_prompt(dossier())
        for wanted in ("slot-1", "proper/st-example/collect", "printed=false",
                       "all from the Common", "extra", "stop before its conclusion cue",
                       "own heading", "neighboring feast's Common reference"):
            self.assertIn(wanted, prompt_text)
        temporal = {**dossier(), "month": None, "day": None}
        self.assertNotIn("None/None", discover.build_prompt(temporal))


class GateTests(unittest.TestCase):
    def test_uncertain_absence_is_held_for_review(self):
        runner = FakeRunner(primary("", printed=False, confidence="low"))
        result = discover.process_dossier(dossier(), runner, apply=True,
                                          apply_text=lambda *a: self.fail("uncertain absence must not write"))
        self.assertEqual(result["status"], "needs-human")
        self.assertEqual(result["slots"][0]["decision"], "needs-human")
        self.assertEqual(runner.secondary_calls, [])

    def test_agreement_puts_and_attests_through_applier(self):
        text = "Grant, we beseech thee, a singular grace unto thy servants."
        second = {"found": True, "text": text, "printed_page": "566", "pdf_page": 11,
                  "confidence": "high", "notes": "visible"}
        runner = FakeRunner(primary(text), second)
        applied = []
        result = discover.process_dossier(
            dossier(), runner, apply=True,
            corpus_get=lambda key, name: "The common collect has unrelated wording.",
            apply_text=lambda *args: applied.append(args[1]["target_key"]),
        )
        self.assertEqual(result["slots"][0]["decision"], "put-and-attest")
        self.assertEqual(applied, ["proper/st-example/collect"])
        self.assertEqual(runner.secondary_calls[0][0:2], ("claude", "sonnet"))

    def test_reader_disagreement_needs_human(self):
        first_text = "Grant, we beseech thee, a singular grace unto thy servants."
        second = {"found": True, "text": "Bestow a wholly different mercy upon us.",
                  "printed_page": "566", "pdf_page": 11, "confidence": "high", "notes": "visible"}
        result = discover.process_dossier(
            dossier(), FakeRunner(primary(first_text), second), apply=True,
            corpus_get=lambda key, name: "The common collect has unrelated wording.",
            apply_text=lambda *args: self.fail("must not apply disagreement"),
        )
        self.assertEqual(result["slots"][0]["decision"], "needs-human")

    def test_same_as_fallback_skips_second_reader_and_attestation(self):
        text = "The common collect has the same wording."
        runner = FakeRunner(primary(text))
        result = discover.process_dossier(
            dossier(), runner, apply=True, corpus_get=lambda key, name: text,
            apply_text=lambda *args: self.fail("must not attest a common printed in full"),
        )
        self.assertEqual(result["slots"][0]["decision"], "same-as-fallback")
        self.assertEqual(runner.secondary_calls, [])


class ReportTests(unittest.TestCase):
    def test_report_contains_pr_sections_and_representative_url(self):
        record = {
            "feast_id": "st-example", "name": "St. Example", "status": "needs-human",
            "slots": [{**dossier()["fallbacks"][0], "decision": "needs-human", "error": "disagreement"}],
            "extra": [{"section": "proper-rubric", "hour": "lauds", "note": "special rubric"}],
        }
        report = discover.render_report([record], "pilot")
        for wanted in ("Feasts processed: 1", "Other observed sections: 1",
                       "https://office.fly.dev/lauds/2026-07-17", "## Same as fallback"):
            self.assertIn(wanted, report)


class NoRenderedEffectTests(unittest.TestCase):
    """The Little Hours derive their versicle from the hour's short responsory,
    so a versicle section written beside one is duplication the engine ignores."""

    def test_applier_reverts_and_drops_attestation_when_render_is_unchanged(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            texts = root / "data" / "texts" / "proper"
            texts.mkdir(parents=True)
            target = texts / "st-example.txt"
            target.write_text("# [versicle-terce]\n# Proper versicle at Terce.\n#\n", encoding="utf-8")
            ledger = root / "data" / "review" / "provenance.csv"
            ledger.parent.mkdir(parents=True)
            ledger.write_text("key,content_hash\nproper/st-example/versicle-terce,abc\n", encoding="utf-8")
            original_root, original_render = discover.ROOT, discover.render_hour
            discover.ROOT = root
            discover.render_hour = lambda date, hour: "identical output"
            try:
                applier = discover.CorpusApplier(root / "run")
                applier.scaffolded.add("st-example")
                written = []
                original_replace = discover.transcribe.replace_and_attest
                discover.transcribe.replace_and_attest = lambda *a: written.append(a)
                try:
                    with self.assertRaises(discover.NoRenderedEffect):
                        applier(
                            {"feast_id": "st-example"},
                            {"target_key": "proper/st-example/versicle-terce",
                             "contexts": [{"date": "2026-03-21", "hour": "terce"}]},
                            {"printed_page": "497", "text": "V. The Lord loved him."},
                            {"png": "page.png"},
                        )
                finally:
                    discover.transcribe.replace_and_attest = original_replace
                self.assertEqual(len(written), 1)
                restored = target.read_text(encoding="utf-8")
                self.assertNotIn("\n[versicle-terce]", restored)
                self.assertIn("# [versicle-terce]", restored)
                self.assertNotIn("proper/st-example/versicle-terce", ledger.read_text(encoding="utf-8"))
            finally:
                discover.ROOT, discover.render_hour = original_root, original_render


class QueueTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.run = self.root / "output" / "discover" / "pilot"
        self.run.mkdir(parents=True)
        for name in ("source.pdf", "index.json", "page.png"):
            (self.root / name).write_text(name)
        self.dossiers = []
        for number in range(3):
            d = dossier()
            d["feast_id"] = f"st-example-{number}"
            d["live_sections"] = {}
            request = d["fallbacks"][0]
            request["target_key"] = f"proper/{d['feast_id']}/collect"
            request["current_keys"] = [request["current_key"]]
            request["contexts"] = [{"date": request["date"], "hour": "lauds", "key": request["current_key"], "tier": "common"}]
            d["fallback_hashes"] = {request["current_key"]: hashlib.sha256(b"fallback").hexdigest()}
            d["pages"][0].update(png=str(self.root / "page.png"),
                                   png_sha256=discover.diurnal_pages.sha256_file(self.root / "page.png"))
            d["source_witness"] = {
                "source_pdf": str(self.root / "source.pdf"), "page_key": "test",
                "pdf_sha256": discover.diurnal_pages.sha256_file(self.root / "source.pdf"),
                "index_path": str(self.root / "index.json"),
                "index_sha256": discover.diurnal_pages.sha256_file(self.root / "index.json"),
            }
            d["packet_sha256"] = discover.packet_hash(d)
            discover.write_jsonl(self.run / "dossiers.jsonl", d)
            self.dossiers.append(d)
        self.current = copy.deepcopy(self.dossiers)
        self.runner = FakeRunner(primary("", printed=False))
        for name, value in (("ROOT", self.root), ("DISCOVER_ROOT", self.run.parent)):
            p = patch.object(discover, name, value)
            p.start()
            self.addCleanup(p.stop)
        for name, value in (("parse_inventory", {}), ("load_feast_catalog", {}),
                            ("build_dossiers", self.current), ("resolved_corpus_text", "fallback"),
                            ("ProviderRunner", self.runner)):
            p = patch.object(discover, name, return_value=value)
            p.start()
            self.addCleanup(p.stop)

    def resume(self, *flags):
        args = discover.build_parser().parse_args(["resume", "pilot", *flags])
        with redirect_stdout(io.StringIO()):
            return discover.resume_command(args)

    def test_small_batches_skip_finished_and_hold_uncertainty_until_explicit_retry(self):
        self.resume("--limit", "1")
        self.runner.first = primary("", printed=False, confidence="low")
        self.resume("--limit", "1")
        self.runner.first = primary("", printed=False)
        self.resume()
        self.resume()
        self.assertEqual(len(self.runner.primary_calls), 3)
        queue = (self.run / "queue.md").read_text()
        for wanted in ("Needs review", "proper/st-example-1/collect", "commons/confessor/collect",
                       "2026-07-17 lauds", "Printed 566 / PDF 11", "page.png", "does not approve the fallback"):
            self.assertIn(wanted, queue)
        with self.assertRaises(ValueError):
            self.resume("--retry")
        self.resume("--feasts", "st-example-1", "--retry")
        self.assertEqual(len(self.runner.primary_calls), 4)
        self.assertEqual(len(discover.latest_results(self.run)), 3)
        with redirect_stdout(io.StringIO()) as output:
            discover.report_command("pilot")
        self.assertIn("Feasts processed: 3", output.getvalue())

    def test_source_changes_invalidate_even_completed_searches(self):
        self.resume()
        for name in ("source.pdf", "index.json", "page.png"):
            path = self.root / name
            original = path.read_bytes()
            path.write_text("changed")
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, "source file changed"):
                self.resume()
            path.write_bytes(original)
        self.assertEqual(len(self.runner.primary_calls), 3)

    def test_missing_pages_are_visible_but_do_not_consume_readers(self):
        d = self.dossiers[0]
        d["pages"] = []
        d["packet_sha256"] = discover.packet_hash(d)
        (self.run / "dossiers.jsonl").write_text("".join(json.dumps(d) + "\n" for d in self.dossiers))
        self.resume()
        self.resume()
        self.assertEqual(len(self.runner.primary_calls), 2)
        self.assertIn("No pages — source research needed", (self.run / "queue.md").read_text())

    def test_changed_saved_task_or_concurrent_reader_is_rejected(self):
        with (self.run / ".resume.lock").open("w") as lock:
            discover.fcntl.flock(lock, discover.fcntl.LOCK_EX | discover.fcntl.LOCK_NB)
            with self.assertRaisesRegex(ValueError, "another reader"):
                self.resume()
        self.dossiers[0]["name"] = "Changed saved request"
        (self.run / "dossiers.jsonl").write_text("".join(json.dumps(d) + "\n" for d in self.dossiers))
        with self.assertRaisesRegex(ValueError, "dossier changed"):
            self.resume()
        self.assertEqual(self.runner.primary_calls, [])

    def test_changed_appointment_or_wording_blocks_reader(self):
        d = self.current[0]
        for field, changed in (("proper_name", "Different"), ("live_sections", {"collect": "new proper"})):
            original = d[field]
            d[field] = changed
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.resume()
            d[field] = original
        d["fallbacks"][0]["contexts"][0]["hour"] = "vespers"
        with self.assertRaisesRegex(ValueError, "appointment context"):
            self.resume()
        self.current[0] = copy.deepcopy(self.dossiers[0])
        with patch.object(discover, "resolved_corpus_text", return_value="changed"), self.assertRaisesRegex(ValueError, "fallback wording"):
            self.resume()
        self.assertEqual(self.runner.primary_calls, [])

    def test_saved_positive_still_needs_new_readers_and_agreement_to_apply(self):
        d = self.dossiers[0]
        discover.write_jsonl(self.run / "results.jsonl", {
            "packet_sha256": d["packet_sha256"], "status": "processed",
            "slots": [{"decision": "printed-proper"}],
        })
        self.runner.first = primary("Grant a singular grace to thy servants.")
        self.runner.second = {"found": True, "text": "An entirely different reading.",
                              "printed_page": "566", "pdf_page": 11, "confidence": "high"}
        with patch.object(discover, "CorpusApplier") as applier:
            self.resume("--feasts", d["feast_id"], "--apply")
            applier.return_value.assert_not_called()
        self.assertEqual(len(self.runner.primary_calls), 1)
        self.assertEqual(len(self.runner.secondary_calls), 1)
        self.assertEqual(discover.latest_results(self.run)[d["packet_sha256"]]["status"], "needs-human")

    def test_slot_filter_does_not_confuse_first_and_second_vespers(self):
        d = self.dossiers[0]
        d["fallbacks"] = [{"target_section": section} for section in (
            "chapter-lauds", "chapter-first-vespers", "chapter-vespers")]
        selected = discover.select_slots([d], {"chapter-vespers"})
        self.assertEqual(selected[0]["fallbacks"], [{"target_section": "chapter-vespers"}])
        self.assertEqual(discover.select_slots([d], {"collect"}), [])


if __name__ == "__main__":
    unittest.main()
