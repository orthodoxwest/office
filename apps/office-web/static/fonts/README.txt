EB Garamond (Georg Duffner)
Copyright 2010-2012 Georg Duffner
Licensed under the SIL Open Font License, Version 1.1.
See OFL-1.1.txt in this directory.

Source: http://www.georgduffner.at/ebgaramond/
Packaged as WOFF2 from the Debian fonts-ebgaramond OpenType files
(EBGaramond12 Regular / Italic / Bold) and split by tools/gengaramond.py:
eb-garamond-{regular,italic,bold}.woff2 hold the Latin core every page
sets; eb-garamond-{regular,italic}-ext.woff2 hold the remaining glyphs,
which style.css loads only for pages that use them. Regenerate both from
the full Debian faces, never from the core files.

Noto Sans Symbols (The Noto Project Authors)
Copyright 2022 The Noto Project Authors (https://github.com/notofonts/symbols)
Licensed under the SIL Open Font License, Version 1.1.
See OFL-1.1.txt in this directory.

noto-sans-symbols-cross.woff2 holds only U+2720 (✠) from the Bold (700)
instance, subset by tools/gencross.py from Fontsource's
@fontsource/noto-sans-symbols package. The LuaLaTeX booklet sets the same
glyph from the Black weight.
