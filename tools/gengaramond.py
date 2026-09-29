#!/usr/bin/env python3
"""Split EB Garamond into the Latin core every page needs and the rest.

The full faces carry Greek, Cyrillic, and extended Latin the office never
sets, and at 334 KB they were most of a first visit's bytes. Each face is
cut in two: a core holding Latin-1, the punctuation, and the few symbols
the corpus uses (℟ ℣ ⁵ → −), with only the OpenType features browsers apply
by default or style.css asks for; and an "-ext" file holding every other
glyph with all its features. style.css declares both under one family with
disjoint unicode-range, so a browser fetches an -ext file only for a page
that sets one of its characters, and nothing the full face covered is lost.

Requires fontTools and brotli (tools/requirements.txt). Sources are the
Debian fonts-ebgaramond OpenType files (EBGaramond12-Regular.otf, -Italic,
-Bold) or full WOFF2 conversions of them — never the core outputs, which
no longer hold the -ext glyphs. `--css` prints the unicode-range lines for
style.css.
"""
import argparse
from pathlib import Path

from fontTools import subset
from fontTools.ttLib import TTFont

# Google Fonts' "latin" range, plus superscript digits, № ℟ ℣, and arrows.
CORE = [
    (0x0000, 0x00FF), (0x0131, 0x0131), (0x0152, 0x0153), (0x02BB, 0x02BC), (0x02C6, 0x02C6),
    (0x02DA, 0x02DA), (0x02DC, 0x02DC), (0x0304, 0x0304), (0x0308, 0x0308), (0x0329, 0x0329),
    (0x2000, 0x206F), (0x2070, 0x209F), (0x20AC, 0x20AC), (0x2116, 0x2116), (0x211F, 0x211F),
    (0x2122, 0x2123), (0x2190, 0x2195), (0x2212, 0x2212), (0x2215, 0x2215), (0xFEFF, 0xFEFF),
    (0xFFFD, 0xFFFD),
]

# Features a browser applies unprompted, and those style.css requests:
# small caps (smcp, c2sc) and numeral styles. Swashes, stylistic sets, and
# discretionary ligatures are never switched on, so the core omits them.
CORE_FEATURES = [
    "ccmp", "locl", "mark", "mkmk", "kern", "liga", "clig", "calt", "rlig", "rvrn",
    "smcp", "c2sc", "lnum", "onum", "tnum", "pnum", "frac", "numr", "dnom", "sups", "subs", "ordn", "case",
]

FACES = ["regular", "italic", "bold"]

# Symbols the site sets that neither file draws (✠ has its own face; the
# disclosure triangles and the ✦ ornament come from system fonts). An -ext
# range must never claim them, or every page would fetch the file in vain.
UNDRAWN = {0x2500, 0x25B4, 0x25BE, 0x25C6, 0x2720, 0x2726}

# Runs of an -ext file's coverage closer than this are declared as one range;
# a page setting a stray code point in such a gap costs a download, not a glyph.
MERGE_GAP = 32


def in_core(cp):
    return any(lo <= cp <= hi for lo, hi in CORE)


def bridgeable(lo, hi):
    """A gap may join two runs if it holds no UNDRAWN or core code point, so
    no -ext range overlaps the core's."""
    return not any(lo < u < hi for u in UNDRAWN) and not any(lo < c_hi and c_lo < hi for c_lo, c_hi in CORE)


def ranges(cps, gap=0):
    """Collapses code points into (lo, hi) runs, bridging bridgeable gaps of
    at most `gap`."""
    out = []
    for cp in sorted(cps):
        if out and cp - out[-1][1] - 1 <= gap and bridgeable(out[-1][1], cp):
            out[-1][1] = cp
        else:
            out.append([cp, cp])
    return [tuple(r) for r in out]


def css_range(runs):
    return ", ".join(f"U+{lo:04X}" if lo == hi else f"U+{lo:04X}-{hi:04X}" for lo, hi in runs)


def cut(source, out, unicodes, features):
    options = subset.Options()
    options.flavor = "woff2"
    options.desubroutinize = True
    options.layout_features = features
    options.name_IDs = ["*"]
    options.notdef_outline = True
    font = subset.load_font(str(source), options)
    subsetter = subset.Subsetter(options)
    subsetter.populate(unicodes=unicodes)
    subsetter.subset(font)
    subset.save_font(font, str(out), options)
    print("wrote", out, out.stat().st_size, "bytes")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("regular", type=Path, help="full EB Garamond Regular (OTF or WOFF2)")
    parser.add_argument("italic", type=Path, help="full EB Garamond Italic")
    parser.add_argument("bold", type=Path, help="full EB Garamond Bold")
    parser.add_argument("--out", type=Path, default=Path("apps/office-web/static/fonts"))
    parser.add_argument("--css", action="store_true", help="print the unicode-range lines for style.css")
    args = parser.parse_args()
    css = [f"core: unicode-range: {css_range(CORE)};"]
    for face, source in zip(FACES, [args.regular, args.italic, args.bold]):
        cmap = TTFont(str(source)).getBestCmap()
        core = [cp for cp in cmap if in_core(cp)]
        rest = [cp for cp in cmap if not in_core(cp)]
        cut(source, args.out / f"eb-garamond-{face}.woff2", core, CORE_FEATURES)
        if rest:
            cut(source, args.out / f"eb-garamond-{face}-ext.woff2", rest, ["*"])
            css.append(f"{face}-ext: unicode-range: {css_range(ranges(rest, MERGE_GAP))};")
    if args.css:
        print("\n".join(css))


if __name__ == "__main__":
    main()
