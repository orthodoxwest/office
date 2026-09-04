#!/usr/bin/env python3
"""Offline unit tests for diurnal-discover.py."""

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


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
                       "all from the Common", "extra", "stop before its conclusion cue"):
            self.assertIn(wanted, prompt_text)


class GateTests(unittest.TestCase):
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
        for wanted in ("Feasts processed: 1", "Extra unmodelled sections: 1",
                       "https://office.fly.dev/lauds/2026-07-17", "## Same as fallback"):
            self.assertIn(wanted, report)


if __name__ == "__main__":
    unittest.main()


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
