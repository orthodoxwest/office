EB Garamond (Georg Duffner, Octavio Pardo)
Copyright 2017 The EB Garamond Project Authors
(https://github.com/octaviopardo/EBGaramond12)
Licensed under the SIL Open Font License, Version 1.1.
See OFL-1.1.txt in this directory.

Source: the variable fonts from Google Fonts (github.com/google/fonts,
ofl/ebgaramond: EBGaramond[wght].ttf and EBGaramond-Italic[wght].ttf,
version 1.003). tools/gengaramond.py pins the weight axis a step heavier
than Regular (480; Bold 700), names the instances "EB Garamond", sets
compact vertical metrics (0.71/0.29em, as Duffner's cut), drops the
TrueType hints, and splits each face: eb-garamond-{regular,italic,bold}.woff2
hold the Latin core every page sets; eb-garamond-{regular,italic}-ext.woff2
hold the remaining glyphs, which style.css loads only for pages that use
them. The same tool writes the native apps' eb_garamond_{regular,italic}.ttf.
Regenerate them all from the variable fonts, never from these files.

Noto Sans Symbols (The Noto Project Authors)
Copyright 2022 The Noto Project Authors (https://github.com/notofonts/symbols)
Licensed under the SIL Open Font License, Version 1.1.
See OFL-1.1.txt in this directory.

noto-sans-symbols-cross.woff2 holds only U+2720 (✠) from the Bold (700)
instance, subset by tools/gencross.py from Fontsource's
@fontsource/noto-sans-symbols package. The LuaLaTeX booklet sets the same
glyph from the Black weight.
