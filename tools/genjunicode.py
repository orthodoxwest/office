#!/usr/bin/env python3
"""Cut the app's Junicode faces from Peter S. Baker's variable fonts.

Junicode 2 ships as two variable fonts (JunicodeVF-Roman, JunicodeVF-Italic:
axes wght 300-700, wdth 75-125, ENLA 0-100) holding some five thousand glyphs
for medievalists. The office needs one weight of each, at normal width, so
this tool pins the axes, gives each instance a plain family name, and writes:

  * for the web (apps/office-web/static/fonts/), each face split in two, as
    the stylesheet loads it: a core holding Latin-1, the punctuation and the
    few symbols the corpus uses (℟ ℣ ⁵ → −), with only the OpenType features
    browsers apply by default or style.css asks for; and an "-ext" file
    holding every other glyph with all its features. style.css declares both
    under one family with disjoint unicode-range, so a browser fetches an -ext
    file only for a page that sets one of its characters. The bold, which
    only the ordo's fasting mark and the usage page set, is the core alone,
    without features (its figures stay lining, as Junicode's default are).
  * for the native apps (apps/android/app/src/main/res/font/, which the iOS
    project also bundles), Regular and Italic as TrueType: the core and the
    rest of the Latin script (NATIVE), with the core's features.

Five changes to the instances, each for the app's sake:

  * The text weight is a step above Junicode's 400 (`--weight`): the reason
    for leaving EB Garamond was its thin colour on a phone, and 400 still
    sets a little light at 19-20px on a bright screen.
  * The face is drawn on EB Garamond's body: its em is enlarged from 1000
    units to 1034 (UPEM), so at any font size its letters are 3.3% smaller
    than Junicode's own. Junicode has the larger x-height (415 units against
    EB Garamond's 400) and sets 3.3% wider; at 1034 its x-height is 0.401em
    and its average advance over the corpus within 0.2% of EB Garamond's.
    Every size in the web's and the apps' type scales, every measure fitted
    to them (the home arch's date and feast, hymn columns, the verse-number
    gutter) and the line breaks of the prayer keep what they were tuned to,
    and the extra colour comes from the weight alone.
  * Q is drawn with Junicode's long sweeping tail (Q.alt2, the roman's cv33
    alternate 3; the italic's swash Q). EB Garamond's long-tailed Q was a
    favourite. The outlines are exchanged under the glyph names, so the
    default Q is the long one everywhere: no stylesheet, Compose style or
    UIFont descriptor needs a feature switched on (a font-feature-settings
    value does not merge, so every rule that set its own would have had to
    repeat it), and the native apps' ink measurement of a dropped initial,
    which reads the default glyph, sees the tail it paints. The roman's
    advances are equal, so its kerning and small caps (c2sc maps the glyph
    named Q) stand.
  * The figures are old-style by default, as EB Garamond's are and as every
    front assumes: the web asks for lining-nums, Compose and UIKit for lnum,
    only where lining figures are wanted (the ordo's day numbers, times,
    the usage page). Junicode's default figures are tabular lining, so the
    regular and italic map the digits' code points to its proportional
    old-style glyphs (zero.osf ...). Its numeral lookups already lead from
    those (lnum to zero.lf, tnum to zero.tosf, both to zero), and every
    other single substitution from a default digit (small caps, sups, numr)
    is extended to them. The bold, without features, keeps lining figures.
  * The vertical metrics are EB Garamond's 0.71/0.29em. Junicode's own
    860/410 units make room for stacked medieval diacritics the office never
    sets, and the apps' line boxes (an iOS single-line Text is the font's own
    line height) and the web's two-line initial (its float's line-height is
    worked from ascent and cap height) were built on EB Garamond's 1em box.
    Junicode's ascenders (0.702em) and descenders (0.264em) stand inside it.
    Windows clipping metrics (usWin*) are left as drawn.

Requires fontTools and brotli (tools/requirements.txt). Sources are the
JunicodeVF-Roman and JunicodeVF-Italic fonts (TTF or WOFF2) from Junicode
2's release (https://github.com/psb1558/Junicode-font). `--css` prints the
unicode-range lines for style.css.
"""
import argparse
from pathlib import Path

from fontTools import subset
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

# Google Fonts' "latin" range, plus superscript digits, № ℟ ℣, and arrows.
CORE = [
    (0x0000, 0x00FF), (0x0131, 0x0131), (0x0152, 0x0153), (0x02BB, 0x02BC), (0x02C6, 0x02C6),
    (0x02DA, 0x02DA), (0x02DC, 0x02DC), (0x0304, 0x0304), (0x0308, 0x0308), (0x0329, 0x0329),
    (0x2000, 0x206F), (0x2070, 0x209F), (0x20AC, 0x20AC), (0x2116, 0x2116), (0x211F, 0x211F),
    (0x2122, 0x2123), (0x2190, 0x2195), (0x2212, 0x2212), (0x2215, 0x2215), (0xFEFF, 0xFEFF),
    (0xFFFD, 0xFFFD),
]

# Features a browser applies unprompted, and those style.css requests:
# small caps (smcp, c2sc) and numeral styles. Junicode's character variants
# and stylistic sets are never switched on, so the core omits them, but for
# the Q's: cv33 and swsh hold its alternates (one of which the default Q now
# draws) and dlig the italic's Qu, kept so a rule can still ask for them.
CORE_FEATURES = [
    "ccmp", "locl", "mark", "mkmk", "kern", "liga", "clig", "calt", "rlig", "rvrn",
    "smcp", "c2sc", "lnum", "onum", "tnum", "pnum", "frac", "numr", "dnom", "sups", "subs", "ordn", "case",
    "cv33", "swsh", "dlig",
]

# Symbols the site sets that the core does not hold (✠ has its own face; the
# disclosure triangles, ─ and the ◆ ✦ ornaments come from system fonts, as
# they did beside EB Garamond). An -ext range must never claim them, or every
# page would fetch the file for a glyph drawn in a different hand.
UNDRAWN = {0x2500, 0x25B4, 0x25BE, 0x25C6, 0x2720, 0x2726}

# Runs of an -ext file's coverage closer than this are declared as one range;
# a page setting a stray code point in such a gap costs a download, not a glyph.
MERGE_GAP = 32

# The native apps' faces: the core and the rest of the Latin script (its
# extended letters and combining marks, the letterlike symbols, the f
# ligatures). The apps set the same English corpus as the web, which never
# reaches past these; Junicode's runes, Gothic and medieval abbreviations
# would treble the files for nothing.
NATIVE = CORE + [(0x0100, 0x024F), (0x0300, 0x036F), (0x1E00, 0x1EFF), (0x2100, 0x214F), (0xFB00, 0xFB06)]

# See the module docstring. Junicode draws on 1000 units per em; its glyphs
# stand on a 1034-unit em, with EB Garamond's 0.71/0.29em ascent and descent.
UPEM = 1034
ASCENT, DESCENT = 734, 300

# The long-tailed Q each face draws by default.
LONG_Q = {"roman": "Q.alt2", "italic": "Q.swash"}

FAMILY = "Junicode"


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


def swap_glyphs(font, a, b):
    """Exchanges two glyphs' outlines and metrics, leaving their names (and so
    every lookup that names them) where they were."""
    glyf, hmtx = font["glyf"], font["hmtx"]
    glyf[a], glyf[b] = glyf[b], glyf[a]
    hmtx[a], hmtx[b] = hmtx[b], hmtx[a]
    if "gvar" in font:
        raise ValueError("swap the glyphs of an instance, not a variable font")


DIGITS = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"]


def oldstyle_default(font):
    """Maps 0-9 to the proportional old-style figures, and lets every single
    substitution that starts from a default digit start from those too."""
    for table in font["cmap"].tables:
        for i, name in enumerate(DIGITS):
            if 0x30 + i in table.cmap:
                table.cmap[0x30 + i] = f"{name}.osf"
    gsub = font["GSUB"].table
    numerals = set()
    for record in gsub.FeatureList.FeatureRecord:
        if record.FeatureTag in ("lnum", "onum", "pnum", "tnum"):
            numerals.update(record.Feature.LookupListIndex)
    for index, lookup in enumerate(gsub.LookupList.Lookup):
        if index in numerals:
            continue
        for sub in lookup.SubTable:
            if sub.LookupType == 7:
                sub = sub.ExtSubTable
            mapping = getattr(sub, "mapping", None)
            if sub.LookupType != 1 or mapping is None:
                continue
            for name in DIGITS:
                if name in mapping and f"{name}.osf" not in mapping:
                    mapping[f"{name}.osf"] = mapping[name]


def instance(source, weight, style):
    """A static instance of `source` at `weight`, normal width, no enlarged
    letters, named, measured and with its long Q, as the module describes."""
    font = TTFont(str(source))
    font = instancer.instantiateVariableFont(font, {"wght": weight, "wdth": 100, "ENLA": 0}, updateFontNames=False)
    if style != "bold":
        swap_glyphs(font, "Q", LONG_Q["italic" if style == "italic" else "roman"])
        oldstyle_default(font)

    subfamily = {"regular": "Regular", "italic": "Italic", "bold": "Bold"}[style]
    postscript = f"{FAMILY}-{subfamily}"
    version = font["name"].getDebugName(5) or "Version 2"
    name = font["name"]
    for record in list(name.names):
        if record.nameID in (1, 2, 3, 4, 6, 16, 17, 21, 22, 25) or record.nameID > 255:
            name.removeNames(nameID=record.nameID)
    name.setName(FAMILY, 1, 3, 1, 0x409)
    name.setName(subfamily, 2, 3, 1, 0x409)
    name.setName(f"{version.removeprefix('Version ')};{postscript};wght{weight}", 3, 3, 1, 0x409)
    name.setName(f"{FAMILY} {subfamily}", 4, 3, 1, 0x409)
    name.setName(postscript, 6, 3, 1, 0x409)
    for table in ("STAT",):
        if table in font:
            del font[table]

    os2, hhea, head = font["OS/2"], font["hhea"], font["head"]
    head.unitsPerEm = UPEM
    os2.usWeightClass = 700 if style == "bold" else 400
    # Bits: 0 italic, 5 bold, 6 regular, 7 use typo metrics.
    selection = os2.fsSelection & ~0b1100001
    selection |= {"regular": 1 << 6, "italic": 1 << 0, "bold": 1 << 5}[style]
    os2.fsSelection = selection | 1 << 7
    head.macStyle = {"regular": 0, "italic": 2, "bold": 1}[style]
    os2.sTypoAscender, os2.sTypoDescender, os2.sTypoLineGap = ASCENT, -DESCENT, 0
    hhea.ascent, hhea.descent, hhea.lineGap = ASCENT, -DESCENT, 0
    return font


def cut(font, out, unicodes, features, flavor):
    options = subset.Options()
    options.flavor = flavor
    options.layout_features = features
    options.name_IDs = ["*"]
    options.name_languages = ["*"]
    options.notdef_outline = True
    options.hinting = False
    options.glyph_names = False
    subsetter = subset.Subsetter(options)
    subsetter.populate(unicodes=unicodes)
    subsetter.subset(font)
    subset.save_font(font, str(out), options)
    print("wrote", out, out.stat().st_size, "bytes")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("roman", type=Path, help="JunicodeVF-Roman (TTF or WOFF2)")
    parser.add_argument("italic", type=Path, help="JunicodeVF-Italic")
    parser.add_argument("--weight", type=float, default=440, help="text weight on Junicode's wght axis (default 440)")
    parser.add_argument("--web", type=Path, default=Path("apps/office-web/static/fonts"))
    parser.add_argument("--native", type=Path, default=Path("apps/android/app/src/main/res/font"))
    parser.add_argument("--css", action="store_true", help="print the unicode-range lines for style.css")
    args = parser.parse_args()
    css = [f"core: unicode-range: {css_range(CORE)};"]
    faces = [("regular", args.roman, args.weight), ("italic", args.italic, args.weight), ("bold", args.roman, 700)]
    for style, source, weight in faces:
        def fresh():
            return instance(source, weight, style)
        cmap = fresh().getBestCmap()
        core = [cp for cp in cmap if in_core(cp)]
        if style == "bold":
            cut(fresh(), args.web / "junicode-bold.woff2", core, ["kern"], "woff2")
            continue
        rest = [cp for cp in cmap if not in_core(cp) and cp not in UNDRAWN]
        cut(fresh(), args.web / f"junicode-{style}.woff2", core, CORE_FEATURES, "woff2")
        cut(fresh(), args.web / f"junicode-{style}-ext.woff2", rest, ["*"], "woff2")
        css.append(f"{style}-ext: unicode-range: {css_range(ranges(rest, MERGE_GAP))};")
        native = [cp for cp in cmap if any(lo <= cp <= hi for lo, hi in NATIVE)]
        cut(fresh(), args.native / f"junicode_{style}.ttf", native, CORE_FEATURES, None)
    if args.css:
        print("\n".join(css))


if __name__ == "__main__":
    main()
