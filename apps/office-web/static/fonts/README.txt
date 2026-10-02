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

eb-garamond-manicule.woff2 holds only U+261E (☞) from EB Garamond 12
Regular, cut by tools/genmanicule.py from eb-garamond-regular-ext.woff2 so
home's leaf can point at the present hour without fetching the whole -ext
face. Same font, author and licence as EB Garamond above.

IM FELL French Canon (Igino Marini)
Copyright 2007 Igino Marini (www.iginomarini.com), with Reserved Font Name
"IM FELL French Canon Roman".
Licensed under the SIL Open Font License, Version 1.1.
See OFL-1.1.txt in this directory.

im-fell-french-canon-caps.woff2 holds the capitals A–Z only of the Roman,
subset from the Google Fonts build. The OFL treats a subset as a Modified
Version, so the file's internal names read "Office Fell Caps" rather than
the Reserved Font Name. Home's leaf sets them as versals (the feast's
initial and the collect's).

Goudy Initialen (Frederic W. Goudy, 1917; digitised by Dieter Steffmann)
Distributed by its digitiser as free to use; this is a freeware licence,
not the SIL Open Font License, and the font carries no licence text of its
own. Taken from TeX Live's "initials" package (GoudyIn), converted from
Type 1 to WOFF2.

goudy-initialen.woff2 holds the capitals A–Z only. Home's leaf sets one as
the woodcut initial of a first-class feast; style.css loads it only then.
