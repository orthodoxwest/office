Junicode (Peter S. Baker)
Copyright 2025 by Peter S. Baker
Licensed under the SIL Open Font License, Version 1.1, with no Reserved
Font Name. See OFL-1.1.txt in this directory.

Source: https://github.com/psb1558/Junicode-font (Junicode 2, version
2.003: JunicodeVF-Roman and JunicodeVF-Italic). tools/genjunicode.py pins
the variable axes (wght 440, bold 700; wdth 100; ENLA 0), names the
instances "Junicode", draws Q with the face's long-tailed alternate, makes
old-style figures the default, enlarges the em to 1034 units so the face
sets at EB Garamond's x-height and width, sets compact vertical metrics
(0.71/0.29em), and splits each face:
junicode-{regular,italic,bold}.woff2 hold the Latin core every page sets;
junicode-{regular,italic}-ext.woff2 hold the remaining glyphs, which
style.css loads only for pages that use them. The same tool writes the
native apps' junicode_{regular,italic}.ttf. Regenerate them all from the
variable fonts, never from these files.

Noto Sans Symbols (The Noto Project Authors)
Copyright 2022 The Noto Project Authors (https://github.com/notofonts/symbols)
Licensed under the SIL Open Font License, Version 1.1.
See OFL-1.1.txt in this directory.

noto-sans-symbols-cross.woff2 holds only U+2720 (✠) from the Bold (700)
instance, subset by tools/gencross.py from Fontsource's
@fontsource/noto-sans-symbols package. The LuaLaTeX booklet sets the same
glyph from the Black weight.
