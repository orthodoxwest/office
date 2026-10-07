#!/usr/bin/env python3
"""Static contract between the font split in style.css and the site's own chrome."""
from pathlib import Path
import re
import unittest

STATIC = Path(__file__).resolve().parent.parent / "apps/office-web/static"


class StaticContracts(unittest.TestCase):
    def test_font_ranges_keep_every_page_on_the_core_files(self):
        """The -ext faces must load only for rare characters: none the site's
        own chrome sets may fall in their ranges (tools/genjunicode.py)."""
        css = (STATIC / "style.css").read_text()
        faces = re.findall(r'src: url\("fonts/(junicode-[a-z-]+)\.woff2"\) format\("woff2"\);\n  unicode-range: ([^;]+);', css)

        def parse(ranges):
            spans = []
            for part in ranges.split(", "):
                lo, _, hi = part[2:].partition("-")
                spans.append((int(lo, 16), int(hi or lo, 16)))
            return spans

        core = {name: parse(r) for name, r in faces if not name.endswith("-ext")}
        ext = {name: parse(r) for name, r in faces if name.endswith("-ext")}
        self.assertEqual(sorted(ext), ["junicode-italic-ext", "junicode-regular-ext"])
        inside = lambda cp, spans: any(lo <= cp <= hi for lo, hi in spans)
        for name, spans in ext.items():
            base = core[name[:-len("-ext")]]
            for lo, hi in spans:
                with self.subTest(face=name, span=hex(lo)):
                    self.assertFalse(any(lo <= b_hi and b_lo <= hi for b_lo, b_hi in base), "core and -ext ranges overlap")
        root = STATIC.parent.parent.parent
        plain = re.sub(r"/\*.*?\*/", "", css, flags=re.S)
        chrome = set("".join(re.findall(r'content:\s*"([^"]*)"', plain)))
        chrome |= {chr(int(h, 16)) for h in re.findall(r'content:\s*"\\([0-9a-fA-F]{4,6})', plain)}
        for path in [*(root / "crates/render-html/templates").glob("*.html"), root / "crates/render-html/src/html.rs",
                     STATIC / "app.js", STATIC / "leader.js"]:
            chrome |= set(path.read_text())
        for c in sorted(chrome):
            for name, spans in ext.items():
                with self.subTest(char=c, face=name):
                    self.assertFalse(inside(ord(c), spans), f"U+{ord(c):04X} would fetch {name} on every page; add it to CORE in tools/genjunicode.py")


if __name__ == '__main__':
    unittest.main()
