# Parish Lauds ingestion pilot

This report records a bounded trial of the diurnal ingestion workflow against
the parish's 2025 draft Lauds booklets. It contains aggregate results only.
The PDFs, extracted text, rendered pages, witness coordinates, model prompts,
and model responses remain in ignored `output/` and must not be committed.

## Scope

The mechanical intake covered all three draft volumes:

| Volume | Pages | Native route | Weak/needs review |
| --- | ---: | ---: | ---: |
| Temporal | 333 | 330 | 3 |
| Sanctoral | 252 | 251 | 1 |
| Commons | 129 | 128 | 1 |
| **Total** | **714** | **709** | **5** |

Native extraction handled 99.3% of pages. Visual inspection of all five weak
routes found legible, chant-dense pages; private-use chant glyphs, rather than
blank or unreadable scans, triggered the noise threshold. Tesseract was not
available in the pilot shell, so QA correctly remained partial instead of
manufacturing OCR text.

The runtime baseline swept 28 years beginning in 2026. It produced 25,009
deduplicated resolution rows representing 559,632 occurrences. The Sanctoral
semantic profile mapped office headings plus the Chapter, Short Responsory,
Hymn, Versicle, Benedictus antiphon, and structural prayer boundary.

## Discovery results

The Sanctoral pass emitted 295 immutable review witnesses:

| Classification | Count |
| --- | ---: |
| Existing text differs | 106 |
| Rubrical or extraction-complex | 117 |
| Unmodeled structural slot | 52 |
| Missing proper override | 10 |
| Unknown feast | 5 |
| Ambiguous owner/context | 3 |
| Known owner not observed at runtime | 2 |

These are triage classifications, not attestations. In particular,
`existing-different` includes chant-underlay cleanup and other extraction
effects that require inspection of the rendered witness.

The 10 missing-override candidates are concentrated in five offices:

- St. Benedict: Chapter and Hymn.
- Ss. Philip and James: Chapter, Short Responsory, Hymn, and Versicle.
- Nativity of St. John Baptist: Short Responsory.
- Ss. Peter and Paul: Versicle.
- St. Lawrence: Short Responsory and Hymn.

In each case the runtime trace selected a common while the draft booklet
contained feast-specific material. A maintainer must still verify the printed
witness and its rubrics before creating any corpus key or provenance record.

## Agent trial

Luna at medium reasoning reviewed the 10 missing-override packets. The durable
collection contained:

| Advisory verdict | Count |
| --- | ---: |
| Missing override | 7 |
| Fallback equal | 1 |
| Needs modeling | 1 |
| Ambiguous | 1 |

Mean reported confidence was 0.93. Five proposals supplied a non-null target
suggestion; the other five withheld one and asked for human review. One packet exceeded
the 128 KiB native-output cap on its first attempt and completed on the single
permitted retry.

A separate ambiguity smoke test routed one page-level packet to both Luna and
Grok 4.5. Luna returned `ambiguous`; Grok returned `needs-ruling`, and the
coordinator surfaced the disagreement without selecting a winner. Both
providers required one bounded retry: Luna first exceeded the output cap, and
Grok first reached its two-turn ceiling. Claude adjudication remained disabled
to preserve its limited quota.

No agent result edited the corpus, recorded a decision, or created an
attestation.

## Defects exposed and fixed

The trial found three correctness issues in the foundation tooling:

1. Runtime inventory lookup omitted the office hour, allowing a Lauds witness
   to be compared with the same owner's Vespers resolution.
2. Continuation pages without mapped boundaries could clear a valid carried
   owner, and a page with several unambiguous headings did not safely carry its
   final heading to the next page.
3. The provider-neutral proposal schema allowed optional object properties,
   but Codex strict structured output requires every declared property to
   appear in `required` (nullable where appropriate).

Regression tests now cover all three cases. The full repository check passes.

## Follow-up

- Human-review the 10 missing-override witnesses before proposing text keys.
- Refine segmentation for collect bodies and multi-office transition pages.
- Treat chant-glyph density separately from prose legibility so native chant
  pages do not automatically request OCR.
- Use Grok replicas selectively; the smoke test validated authentication and
  disagreement handling, but the two-turn ceiling is tight.
- Keep Claude disabled unless a completed disagreement genuinely warrants
  adjudication.
