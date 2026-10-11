#!/usr/bin/env python3
"""Build the app's EB Garamond faces from the variable fonts.

EB Garamond's Regular was drawn for print and reads light on a phone's bright
screen, so the app sets its text a step heavier than Regular: TEXT_WEIGHT on
the face's weight axis (400 is Regular, 500 Medium). The instances are pinned
here, once, so the web, Android and iOS all set the very same glyphs and no
platform has to interpolate a variable font (Android's widgets and iOS's
UIFont cannot be asked for an axis value).

Three changes to the instances, beyond the weight:

* Names. Family "EB Garamond", styles Regular, Italic and Bold, PostScript
  names EBGaramond-Regular, -Italic and -Bold, which iOS asks for by name.
* Vertical metrics. The variable fonts carry a 1.007/0.298em ascent and
  descent, which made every native line box taller than the web's CSS line
  heights assume. Georg Duffner's original cut, which the app set before,
  used 0.71/0.29em; the instances keep that, so the native type scales
  (Theme.kt, Theme.swift) hold.
* Hints. TrueType instructions are dropped: no platform the app runs on
  applies them, and they are a third of the bytes.

The web then gets each face cut in two, as before: a core holding Latin-1,
the punctuation and the few symbols the corpus uses (℟ ℣ ⁵ → −), with only
the OpenType features browsers apply by default or style.css asks for; and
an "-ext" file holding every other glyph with all its features. style.css
declares both under one family with disjoint unicode-range, so a browser
fetches an -ext file only for a page that sets one of its characters, and
nothing the full face covered is lost. Bold covers the core alone. The
native apps take Regular and Italic as TrueType files cut to the core and
the extended Latin ranges (NATIVE).

Requires fontTools and brotli (tools/requirements.txt). Sources are the
variable fonts from Google Fonts (github.com/google/fonts, ofl/ebgaramond:
EBGaramond[wght].ttf and EBGaramond-Italic[wght].ttf, Octavio Pardo's
redraw of Duffner's design, version 1.003 or later). `--css` prints the
unicode-range lines for style.css.
"""
import argparse
import tempfile
from pathlib import Path

from fontTools import subset
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

# The weight axis value of the text faces (Regular and Italic), and of Bold.
TEXT_WEIGHT = 480
BOLD_WEIGHT = 700

# Vertical metrics, in a 1000-unit em: Duffner's EB Garamond 12.
ASCENT = 710
DESCENT = 290

FAMILY = "EB Garamond"
PS_FAMILY = "EBGaramond"

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

# Symbols the site sets that neither file draws (✠ has its own face; the
# disclosure triangles and the ✦ ornament come from system fonts). An -ext
# range must never claim them, or every page would fetch the file in vain.
UNDRAWN = {0x2500, 0x25B4, 0x25BE, 0x25C6, 0x2720, 0x2726}

# Runs of an -ext file's coverage closer than this are declared as one range;
# a page setting a stray code point in such a gap costs a download, not a glyph.
MERGE_GAP = 32

# What the native apps set: the core, and the extended Latin a saint's name
# might need. The rest of the face (Greek, Cyrillic, symbols) stays out of
# the app bundles.
NATIVE = CORE + [(0x0100, 0x024F), (0x1E00, 0x1EFF), (0x2C60, 0x2C7F), (0xA720, 0xA7FF), (0xFB00, 0xFB06)]


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


def set_names(font, style):
    """Family, style, full and PostScript names for one static instance."""
    name = font["name"]
    full = FAMILY if style == "Regular" else f"{FAMILY} {style}"
    for record in list(name.names):
        if record.nameID in (16, 17, 21, 22, 25) or record.nameID > 255:
            name.removeNames(nameID=record.nameID)
    for name_id, text in ((1, FAMILY), (2, style), (3, f"{FAMILY} {style}"), (4, full), (6, f"{PS_FAMILY}-{style}")):
        name.setName(text, name_id, 3, 1, 0x409)
        name.setName(text, name_id, 1, 0, 0)


def set_metrics(font):
    """Duffner's line metrics on every table a platform reads, and a Windows
    box that clips no glyph."""
    upem = font["head"].unitsPerEm
    ascent, descent = round(ASCENT * upem / 1000), round(DESCENT * upem / 1000)
    hhea, os2, head = font["hhea"], font["OS/2"], font["head"]
    hhea.ascent, hhea.descent, hhea.lineGap = ascent, -descent, 0
    os2.sTypoAscender, os2.sTypoDescender, os2.sTypoLineGap = ascent, -descent, 0
    os2.usWinAscent, os2.usWinDescent = max(head.yMax, ascent), max(-head.yMin, descent)
    os2.fsSelection |= 1 << 7  # USE_TYPO_METRICS


def strip_hints(font):
    for table in ("fpgm", "prep", "cvt ", "cvar", "hdmx", "LTSH", "VDMX"):
        if table in font:
            del font[table]
    glyf = font["glyf"]
    for name in glyf.keys():
        glyph = glyf[name]
        if hasattr(glyph, "program"):
            glyph.program.fromBytecode(b"")
    font["maxp"].maxZones = 1
    font["maxp"].maxTwilightPoints = 0
    font["maxp"].maxStorage = 0
    font["maxp"].maxFunctionDefs = 0
    font["maxp"].maxInstructionDefs = 0
    font["maxp"].maxStackElements = 0
    font["maxp"].maxSizeOfInstructions = 0


def instance(source, weight, style):
    """One static face from a variable font, pinned at `weight`."""
    font = instancer.instantiateVariableFont(TTFont(str(source)), {"wght": weight}, updateFontNames=False)
    strip_hints(font)
    set_names(font, style)
    set_metrics(font)
    font["OS/2"].usWeightClass = weight
    if style == "Bold":
        font["OS/2"].fsSelection = (font["OS/2"].fsSelection & ~(1 << 6)) | (1 << 5)
        font["head"].macStyle |= 1
    return font


def cut(source, out, unicodes, features, flavor="woff2"):
    options = subset.Options()
    options.flavor = flavor
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
    parser.add_argument("roman", type=Path, help="EBGaramond[wght].ttf")
    parser.add_argument("italic", type=Path, help="EBGaramond-Italic[wght].ttf")
    parser.add_argument("--weight", type=int, default=TEXT_WEIGHT, help="the text faces' weight axis value")
    parser.add_argument("--out", type=Path, default=Path("apps/office-web/static/fonts"))
    parser.add_argument("--native", type=Path, default=Path("apps/android/app/src/main/res/font"), help="where the apps' TrueType files go")
    parser.add_argument("--css", action="store_true", help="print the unicode-range lines for style.css")
    args = parser.parse_args()
    faces = [("regular", args.roman, args.weight, "Regular"), ("italic", args.italic, args.weight, "Italic"), ("bold", args.roman, BOLD_WEIGHT, "Bold")]
    css = [f"core: unicode-range: {css_range(CORE)};"]
    with tempfile.TemporaryDirectory() as tmp:
        for face, source, weight, style in faces:
            font = instance(source, weight, style)
            full = Path(tmp) / f"{face}.ttf"
            font.save(str(full))
            cmap = font.getBestCmap()
            if face != "bold":
                native = [cp for cp in cmap if any(lo <= cp <= hi for lo, hi in NATIVE)]
                cut(full, args.native / f"eb_garamond_{face}.ttf", native, CORE_FEATURES, flavor=None)
            core = [cp for cp in cmap if in_core(cp)]
            # The face draws some of the UNDRAWN symbols; they are left out so
            # no -ext range claims them.
            rest = [cp for cp in cmap if not in_core(cp) and cp not in UNDRAWN]
            cut(full, args.out / f"eb-garamond-{face}.woff2", core, CORE_FEATURES)
            if face != "bold" and rest:
                cut(full, args.out / f"eb-garamond-{face}-ext.woff2", rest, ["*"])
                css.append(f"{face}-ext: unicode-range: {css_range(ranges(rest, MERGE_GAP))};")
    if args.css:
        print("\n".join(css))


if __name__ == "__main__":
    main()
