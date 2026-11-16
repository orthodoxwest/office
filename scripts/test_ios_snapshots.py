#!/usr/bin/env python3
"""Tests for the iOS screenshot baselines' comparison (scripts/ios-snapshots.py)."""

import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest

try:
    from PIL import Image, ImageDraw
except ImportError:  # CI runs these where the comparison runs, in ios.yml, with Pillow.
    Image = None

SCRIPT = pathlib.Path(__file__).with_name("ios-snapshots.py")
SPEC = importlib.util.spec_from_file_location("ios_snapshots", SCRIPT)
SNAP = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = SNAP
SPEC.loader.exec_module(SNAP)

ENV = {"xcode": "26.1 17B55", "runtime": "com.apple.CoreSimulator.SimRuntime.iOS-26-1",
       "phone": "iPhone 16", "ipad": "iPad Pro 11-inch (M5)"}


@unittest.skipIf(Image is None, "Pillow is not installed")
class CompareTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        root = pathlib.Path(self.tmp.name)
        self.raw, self.base, self.updates, self.diffs = (root / n for n in ("raw", "base", "updates", "diffs"))
        self.raw.mkdir()
        self.summary = root / "summary.md"

    def tearDown(self):
        self.tmp.cleanup()

    def shot(self, name, text="Vespers", size=(393 * 3, 120 * 3), env=ENV):
        """A full-size simulator shot: plaster-ish noise-free page with a word on it."""
        image = Image.new("RGB", size, (244, 238, 226))
        # Below the status bar's 70 points, which the blank test leaves out.
        ImageDraw.Draw(image).text((60, 240), text, fill=(40, 30, 20), font_size=90)
        image.save(self.raw / f"{name}.png")
        (self.raw / "environment.json").write_text(json.dumps(env))

    def run_compare(self):
        return SNAP.compare(self.raw, self.base, self.updates, self.diffs, self.summary)

    def seed(self):
        """Takes the run's updates as the baselines, as the label's commit does."""
        status, _, _ = self.run_compare()
        self.updates.rename(self.base)
        for p in self.raw.iterdir():
            p.unlink()
        return status

    def test_no_baselines_passes_with_a_notice_and_offers_every_shot(self):
        self.shot("home-nave")
        self.shot("ipad-home-nave", size=(834 * 2, 200 * 2))
        status, state, lines = self.run_compare()
        self.assertEqual(status, 0)
        self.assertIn("::notice::No iOS baselines yet", lines[0])
        self.assertEqual(state, "no-baselines")
        self.assertIn("no baselines", self.summary.read_text())
        with Image.open(self.updates / "home-nave.png") as a, Image.open(self.updates / "ipad-home-nave.png") as b:
            self.assertEqual(a.size, (393, 120))
            self.assertEqual(b.size, (834, 200))
        self.assertEqual(json.loads((self.updates / "environment.json").read_text()), ENV)

    def test_same_shots_match_and_keep_the_baseline_bytes(self):
        self.shot("home-nave")
        self.seed()
        before = (self.base / "home-nave.png").read_bytes()
        self.shot("home-nave")
        status, state, lines = self.run_compare()
        self.assertEqual(status, 0, lines)
        self.assertEqual(state, "matched")
        self.assertIn("matched", self.summary.read_text())
        self.assertEqual((self.updates / "home-nave.png").read_bytes(), before)
        self.assertEqual(list(self.diffs.iterdir()), [])

    def test_small_noise_is_tolerated(self):
        self.shot("home-nave")
        self.seed()
        self.shot("home-nave")
        path = self.raw / "home-nave.png"
        with Image.open(path) as image:
            image = image.copy()
        px = image.load()
        # A channel moved by a few levels everywhere, and a handful of pixels moved a lot.
        for x in range(0, image.width, 7):
            for y in range(0, image.height, 5):
                r, g, b = px[x, y]
                px[x, y] = (min(r + 30, 255), g, b)
        for x in range(30):
            px[x, 0] = (0, 0, 0)
        image.save(path)
        status, state, lines = self.run_compare()
        self.assertEqual(status, 0, lines)

    def test_a_changed_word_fails_with_a_diff_picture(self):
        self.shot("vespers-nave", "Vespers")
        self.shot("lauds-nave", "Lauds")
        self.seed()
        self.shot("vespers-nave", "Compline")
        self.shot("lauds-nave", "Lauds")
        status, state, lines = self.run_compare()
        self.assertEqual((status, state), (1, "differ"))
        self.assertTrue(any("vespers-nave.png differs" in line for line in lines), lines)
        self.assertEqual([p.name for p in self.diffs.iterdir()], ["vespers-nave.png"])
        with Image.open(self.diffs / "vespers-nave.png") as sheet:
            self.assertEqual(sheet.size, (393 * 3 + 16, 120))
        self.assertIn("`vespers-nave` | changed", self.summary.read_text())

    def test_new_and_gone_shots_fail_and_updates_follow_the_shots(self):
        self.shot("home-nave")
        self.shot("about-nave")
        self.seed()
        self.shot("home-nave")
        self.shot("ordo-nave")
        status, state, lines = self.run_compare()
        self.assertEqual(status, 1)
        self.assertTrue(any("ordo-nave.png has no baseline" in line for line in lines), lines)
        self.assertTrue(any("about-nave.png has a baseline but was not taken" in line for line in lines), lines)
        self.assertEqual(sorted(p.name for p in self.updates.glob("*.png")), ["home-nave.png", "ordo-nave.png"])

    def test_resized_shot_fails(self):
        self.shot("home-nave")
        self.seed()
        self.shot("home-nave", size=(393 * 3, 121 * 3))
        status, state, lines = self.run_compare()
        self.assertEqual(status, 1)
        self.assertTrue(any("size 393x120 -> 393x121" in line for line in lines), lines)

    def test_another_environment_is_not_compared(self):
        self.shot("home-nave")
        self.seed()
        self.shot("home-nave", "Something else", env={**ENV, "runtime": "com.apple.CoreSimulator.SimRuntime.iOS-27-0"})
        status, state, lines = self.run_compare()
        self.assertEqual(status, 0)
        self.assertIn("::warning::", lines[0])
        self.assertEqual(state, "not-compared")
        self.assertIn("not compared", self.summary.read_text())
        self.assertIn("iOS-27-0", (self.updates / "environment.json").read_text())

    def test_malformed_environment_is_refused(self):
        for env in ({"xcode": "26.1"}, {**ENV, "phone": "iPhone 16\n::error::x"}, {**ENV, "ipad": ""}, {**ENV, "xcode": 26}):
            with self.subTest(env=env):
                self.shot("home-nave", env=env)
                with self.assertRaises(SystemExit):
                    self.run_compare()

    def test_a_blank_shot_fails_and_is_never_offered(self):
        self.shot("home-nave")
        # The bare wall, with the island's black above the status line.
        wall = Image.new("RGB", (393 * 3, 120 * 3), (244, 238, 226))
        ImageDraw.Draw(wall).rectangle((450, 30, 730, 100), fill=(0, 0, 0))
        wall.save(self.raw / "compline-nave.png")
        status, state, lines = self.run_compare()
        self.assertEqual((status, state), (2, "blank"))
        self.assertTrue(any("compline-nave.png is blank" in line for line in lines), lines)
        self.assertEqual(list(self.updates.glob("*.png")), [])
        self.assertIn("blank", self.summary.read_text())

    def test_blank_is_one_colour_below_the_status_bar(self):
        page = Image.new("RGB", (393, 852), (244, 238, 226))
        self.assertTrue(SNAP.is_blank(page))
        ImageDraw.Draw(page).rectangle((150, 10, 240, 40), fill=(0, 0, 0))
        self.assertTrue(SNAP.is_blank(page), "the island alone is still blank")
        for y in range(100, 800, 30):
            ImageDraw.Draw(page).text((20, y), "O God, make speed to save me", fill=(40, 30, 20), font_size=16)
        self.assertFalse(SNAP.is_blank(page))

    def test_state_goes_to_the_output_file(self):
        self.shot("home-nave")
        output = pathlib.Path(self.tmp.name) / "output"
        status = SNAP.main(["compare", str(self.raw), str(self.base), "--updates", str(self.updates),
                            "--diffs", str(self.diffs), "--output", str(output)])
        self.assertEqual(status, 0)
        self.assertEqual(output.read_text(), "state=no-baselines\n")

    def test_excluded_shots_are_not_compared(self):
        self.shot("home-nave")
        self.shot("reminders-nave")
        SNAP.EXCLUDED["reminders-nave"] = "test"
        try:
            self.run_compare()
        finally:
            del SNAP.EXCLUDED["reminders-nave"]
        self.assertEqual([p.name for p in self.updates.glob("*.png")], ["home-nave.png"])



@unittest.skipIf(Image is None, "Pillow is not installed")
class StoreTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.raw = pathlib.Path(self.tmp.name) / "raw"
        self.out = pathlib.Path(self.tmp.name) / "out"
        self.raw.mkdir()
        for name in SNAP.STORE_SHOTS["iphone"]:
            Image.new("RGBA", (1320, 2868), (240, 230, 220, 255)).save(self.raw / f"{name}.png")
        for name in SNAP.STORE_SHOTS["ipad"]:
            Image.new("RGBA", (2064, 2752), (240, 230, 220, 255)).save(self.raw / f"ipad-{name}.png")

    def tearDown(self):
        self.tmp.cleanup()

    def test_sets_are_numbered_sized_and_opaque(self):
        SNAP.store(self.raw, self.out)
        for folder, size in SNAP.STORE_SIZES.items():
            files = sorted((self.out / folder).iterdir())
            self.assertEqual(len(files), 7, folder)
            self.assertTrue(files[0].name.startswith("1-"))
            for f in files:
                with Image.open(f) as image:
                    self.assertEqual((image.size, image.mode), (size, "RGB"), f)

    def test_a_shot_at_the_wrong_size_stops(self):
        Image.new("RGB", (1206, 2622)).save(self.raw / "home-nave.png")
        with self.assertRaises(SystemExit):
            SNAP.store(self.raw, self.out)

if __name__ == "__main__":
    unittest.main()
