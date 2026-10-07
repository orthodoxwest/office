#!/usr/bin/env python3
"""Subset the ✠ (U+2720) glyph the web pages set in text.

EB Garamond has no Maltese cross, so each platform substituted its own
dingbat. This keeps one glyph everywhere, drawn from Noto Sans Symbols like
the LuaLaTeX booklet's cross. Requires fontTools and brotli
(tools/requirements.txt). The source may be a TTF/OTF or a WOFF2, e.g.
Fontsource's noto-sans-symbols-symbols-700-normal.woff2.
"""
import argparse
from pathlib import Path

from fontTools import subset


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("source", type=Path, help="Noto Sans Symbols font (weight 700)")
    parser.add_argument("--out", type=Path, default=Path("apps/office-web/static/fonts/noto-sans-symbols-cross.woff2"))
    args = parser.parse_args()
    options = subset.Options()
    options.flavor = "woff2"
    options.hinting = False
    options.desubroutinize = True
    options.layout_features = []
    options.name_IDs = [0, 1, 2, 3, 4, 5, 6]
    font = subset.load_font(str(args.source), options)
    subsetter = subset.Subsetter(options)
    subsetter.populate(unicodes=[0x2720])
    subsetter.subset(font)
    subset.save_font(font, str(args.out), options)
    print("wrote", args.out, args.out.stat().st_size, "bytes")


if __name__ == "__main__":
    main()
