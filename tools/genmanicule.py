#!/usr/bin/env python3
"""Subset the ☞ (U+261E) glyph home's leaf points at the present hour with.

The pointing hand is EB Garamond's own, but it lives in the -ext face
(eb-garamond-regular-ext.woff2, about 140 KB), which a page fetches only for
a character it sets. Home sets the hand on every visit, so this keeps that
one glyph in a face of its own, as the ✠ is kept (tools/gencross.py).
Requires fontTools and brotli (tools/requirements.txt). The source is the
Debian fonts-ebgaramond EBGaramond12-Regular.otf, or any face that still
holds the glyph, such as the -ext WOFF2 tools/gengaramond.py writes.
"""
import argparse
from pathlib import Path

from fontTools import subset


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("source", type=Path, help="EB Garamond Regular (or its -ext WOFF2)")
    parser.add_argument("--out", type=Path, default=Path("apps/office-web/static/fonts/eb-garamond-manicule.woff2"))
    args = parser.parse_args()
    options = subset.Options()
    options.flavor = "woff2"
    options.hinting = False
    options.desubroutinize = True
    options.layout_features = []
    options.name_IDs = [0, 1, 2, 3, 4, 5, 6, 13, 14]
    font = subset.load_font(str(args.source), options)
    subsetter = subset.Subsetter(options)
    subsetter.populate(unicodes=[0x261E])
    subsetter.subset(font)
    subset.save_font(font, str(args.out), options)
    print("wrote", args.out, args.out.stat().st_size, "bytes")


if __name__ == "__main__":
    main()
