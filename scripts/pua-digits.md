# Recovering digits from the cycle PDFs

The four 2025 Temporal/Sanctoral Lauds/Vespers draft PDFs encode old-style
digits from their embedded AJensonPro subset as `U+F643`–`U+F64C` (0–9).
`pua-digits.py` (Python 3 and Poppler's `pdftotext`) extracts text with
`pdftotext -layout -enc UTF-8` and maps them back via `pua-digits.json`:

```text
scripts/pua-digits.py BOOK.pdf > /tmp/book.txt
scripts/pua-digits.py BOOK.pdf /tmp/book.txt
```

Spacing and form-feed page breaks are preserved and warnings go to stderr.
`U+F650` becomes `#`, as printed in unresolved references like `p. ##`. Other
PUA characters are Meinrad chant-font notation and are left unchanged.

## Derivation

```text
scripts/pua-digits.py --derive [--write-map PATH] \
  "/path/00. The Temporal Cycle - Lauds 2025 Jacob edit 03-31 v.11.pdf" \
  "/path/00. The Temporal Cycle - Vespers 2025 Jacob edit 03-28 v.12.pdf" \
  "/path/00. The Sanctoral Cycle - Lauds 2025 Jacob edit 03-31 v.10.pdf" \
  "/path/00. The Sanctoral Cycle - Vespers 2025 Jacob edit 03-31 v.10.pdf"
```

Derivation finds six Psalm headings (50, 117, 62, 131, 148, 99), checks the
English opening beside each against the corpus, and converts the matching
Coverdale number to the book's Vulgate number. Together they fix every digit;
conflicting or incomplete constraints are an error.

## Validation

Twenty recovered psalm headings (Vulgate 50–146/147) were matched to their
corpus files by opening words, showing the expected one-number offset,
including the split psalms (Vulgate 115 = Coverdale 116:10, 144.10 = 145:10,
146/147 = `147a`/`147b`). Independently, every Temporal Lauds footer from 107
to 333 equals its PDF page ordinal, and the later Lauds DOCX's decimal page
fields confirm the same glyphs where sections overlap. All digit codepoints in
all four PDFs are mapped.
