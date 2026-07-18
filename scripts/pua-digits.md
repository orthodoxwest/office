# Recovering digits from the cycle PDFs

The four 2025 Temporal/Sanctoral Lauds/Vespers draft PDFs map the old-style
digits in their embedded AJensonPro subset to `U+F643`–`U+F64C`. Run:

```text
scripts/pua-digits.py BOOK.pdf > /tmp/book.txt
scripts/pua-digits.py BOOK.pdf /tmp/book.txt
```

The script requires Python 3 and Poppler's `pdftotext`. It invokes
`pdftotext -layout -enc UTF-8`, replaces digits from `pua-digits.json`, and
preserves Poppler's spacing and form-feed page boundaries. Warnings go to
stderr. Unmapped PUA characters are retained; these are normally notation
from the embedded Meinrad chant font. `U+F650` is separately replaced by
`#`, which is what the glyph visibly prints as in unresolved draft page
references such as `p. ##`.

## Derivation

To reproduce the map, pass all four PDFs to the derivation mode:

```text
scripts/pua-digits.py --derive \
  "/path/00. The Temporal Cycle - Lauds 2025 Jacob edit 03-31 v.11.pdf" \
  "/path/00. The Temporal Cycle - Vespers 2025 Jacob edit 03-28 v.12.pdf" \
  "/path/00. The Sanctoral Cycle - Lauds 2025 Jacob edit 03-31 v.10.pdf" \
  "/path/00. The Sanctoral Cycle - Vespers 2025 Jacob edit 03-31 v.10.pdf"
```

Derivation finds six Psalm headings, verifies an English marker beside each
heading, and turns the matched Coverdale/Hebrew corpus number into the number
printed by the Vulgate-numbered book. The constraints from Psalms 50, 117,
62, 131, 148, and 99 cover every digit and produce the contiguous map
`U+F643 = 0` through `U+F64C = 9`. Conflicting or incomplete constraints are
an error. Use `--write-map PATH` to save the derived JSON.

## Validation

The following recovered headings were checked against the indicated corpus
file. In every row the English opening beside the PDF heading matches the
corpus text; the middle range demonstrates the expected one-number Vulgate
offset. Psalm 115 begins at verse 10 of Coverdale Psalm 116, and Psalm 144.10
begins at verse 10 of Coverdale Psalm 145.

| PDF number | Corpus file | Matching opening |
|---:|---|---|
| 50 | `051.txt` | Have mercy upon me, O God |
| 62 | `063.txt` | O God, thou art my God |
| 109 | `110.txt` | The Lord said unto my Lord |
| 110 | `111.txt` | I will give thanks unto the Lord |
| 111 | `112.txt` | Blessed is the man that feareth the Lord |
| 112 | `113.txt` | Praise the Lord, ye servants |
| 115 | `116.txt`, verse 10 | I believed, and therefore will I speak |
| 121 | `122.txt` | I was glad when they said unto me |
| 126 | `127.txt` | Except the Lord build the house |
| 127 | `128.txt` | Blessed are all they that fear the Lord |
| 129 | `130.txt` | Out of the deep have I called unto thee |
| 131 | `132.txt` | Lord, remember David |
| 134 | `135.txt` | O praise the Lord, laud ye the Name |
| 135 | `136.txt` | O give thanks unto the Lord |
| 136 | `137.txt` | By the waters of Babylon |
| 137 | `138.txt` | I will give thanks unto thee, O Lord |
| 138 | `139.txt` | O Lord, thou hast searched me out |
| 144.10 | `145.txt`, verse 10 | All thy works praise thee, O Lord |
| 145 | `146.txt` | Praise the Lord, O my soul |
| 146/147 | `147a.txt`/`147b.txt` | O praise the Lord / Praise the Lord, O Jerusalem |

Page-number recovery was checked independently in two ways. In the Temporal
Lauds PDF, every printed footer from 107 through 333 equals its physical PDF
page ordinal (`pdfinfo` reports 333 pages), exercising all ten glyphs many
times. The later consolidated Lauds DOCX was also inspected through its Word
XML: overlapping section text retains decimal page fields, though later edits
shifted pagination. For example, the PDF's page 107 begins *January 2 — The
Holy Name of Jesus* while the DOCX places that material at pages 116–123;
*Sexagesima Sunday* begins at PDF page 150 and appears at DOCX pages 159–169;
and *The Summer Hymn* is PDF page 325 and DOCX page 344. The headings and
surrounding text agree, and the DOCX digits independently confirm the glyph
identifications used in those PDF page numbers.

All digit PUA codepoints in all four PDFs are mapped. The remaining unmapped
PUA codepoints are non-digit chant glyphs and are deliberately left unchanged.
