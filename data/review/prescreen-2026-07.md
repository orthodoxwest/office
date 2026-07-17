# Pre-screen of Divinum Officium–seeded texts — July 2026

An automated read-through of all 982 corpus sections carrying a
`# SOURCE: divinum-officium` marker, flagging texts that look wrong before
volunteers spend book-time on them. **Nothing here is authoritative** — every
item still needs checking against the diurnal/supplement — but these rows
should jump the review queue, and rows not listed here are likelier to be
quick confirms.

> **Status (2026-07-16):** this document is now narrative history. Its
> still-valid findings were re-verified against the current corpus and moved
> into the machine-read ledger `data/review/prescreen.csv`, which feeds the
> provenance queue's suspect tier (`make review-suspects`) and the hour
> pages' Assurance disclosures. Many items below (the six truncated collects
> except Epiphany, the Pentecost/All Saints hymns, items 8, 10, 12, 16
> mostly, 20, 21, 22) had already been fixed by draft-book reconciliation
> and were not carried into the ledger. New read-through findings should be
> recorded with `./office review flag`, not appended here.

Fixed directly in this PR (mechanical, no book needed):
- `proper/epiphany.txt [hymn-vespers]` — title line contained a literal `\n`
  escape that rendered to users.
- `proper/assumption-bvm.txt [hymn-lauds]` — backticks used as apostrophes.

## High priority — garbled, truncated, or wrongly rendered text

1. **Six collects end mid-sentence** with no termination formula
   ("…deliver us;"). DO stores the ending (`Per Dominum…`) separately and the
   seeder did not append it. Verify what ending the diurnal prints and add it:
   - `proper/advent-sunday-1.txt [collect]`
   - `proper/advent-sunday-2.txt [collect]`
   - `proper/epiphany.txt [collect]`
   - `proper/circumcision.txt [collect]`
   - `proper/holy-name-jesus.txt [collect]`
   - `proper/st-stephen.txt [collect]`

2. **Hymns headed by rubric lines instead of a title** — the renderer treats
   the first line as the hymn's Latin title, so users see a rubric where the
   title belongs, and the real titles (*Ave maris stella*, *Veni Creator
   Spiritus*) are absent:
   - `commons/blessed-virgin.txt [hymn-vespers]` — begins
     `/:Prima stropha sequentis hymni dicitur flexis genibus.:/`
   - `proper/pentecost.txt [hymn-vespers]` — same pattern; its Latin rubric is
     also misspelled ("flexibus genibus").

3. `proper/finding-holy-cross.txt [hymn-vespers]` — stanza 6 contains inline
   untranslated DO rubrics `(sed tempore Passionis)` / `(sed tempore
   Paschali)` with three alternative opening lines; this renders literally.
   Needs restructuring (seasonal variants) or a fixed text per the diurnal.

4. `proper/christ-the-king.txt [chapter-sext]` — text begins "LL things"
   (dropped "A"); citation reads `Col 1;16-18` (semicolon for colon).

5. `proper/corpus-christi.txt [short-responsory-terce]` — the repeated R.
   lines differ ("the bread" vs "of the bread") and the final line has
   "Aleluia" (single l).

6. **Modern-English versicle amid archaic register** in three Peter offices:
   "V. You are Peter. R. And on this rock I am going to build my church."
   - `proper/chains-st-peter.txt [versicle-vespers]`
   - `proper/chair-peter-antioch.txt [versicle-vespers]`
   - `proper/chair-peter-rome.txt [versicle-vespers]`

7. `proper/all-saints.txt [hymn-vespers]` (*Placare, Christe, servulis*) —
   dropped word: "And plead for us when death is nigh, / [When] our
   all-searching judge appears"; also the only hymn in the corpus with no
   doxology stanza and no "Amen." — check whether the diurnal's final stanza
   was lost in seeding.

8. `proper/st-john-evangelist.txt [psalm-antiphon-3]` — "This is My disciple
   * if I will that he tarry till I come?" reads as a garble of John 21:22
   ("If I will that he tarry till I come, what is that to thee?").

9. `proper/st-john-evangelist.txt [chapter-sext]` — ends "…wholesome wisdom
   to drink; God, our Lord." — dangling fragment appended to Sir 15:3.

10. `proper/ascension.txt [psalm-antiphon-2]` — "And while they looked
    steadfastly towards heaven, as He went up, they said, alleluia" — the
    middle of the verse (the two men in white apparel) appears to be missing;
    "they said" hangs without content.

11. `commons/bishop-martyr.txt [short-responsory-sext]` — division garbled:
    "R. O Lord, Thou hast set * Upon his head. / V. A Crown of precious
    stones." (stray capitals, wrong split of Ps 21:3).

12. `proper/vigil-nativity.txt [collect]` — "O God, thou Who gladden us" —
    ungrammatical ("gladdenest" / "who dost gladden").

13. `proper/st-martin-tours.txt [psalm-antiphon-2]` — "I refuse not * to
    work. thy will be done." — punctuation/capitalization garbled.

## Medium priority — register, spelling, and consistency

14. **Modern pronouns in apostle/evangelist antiphons**
    (`commons/apostle.txt`, `commons/evangelist.txt` psalm-antiphons 1, 3, 5):
    "that you love one another", "You are my friends", "In your patience you
    shall possess your souls" — mixed with "hath" in the same set. The
    diurnal presumably has thou-form throughout.

15. **Douay-Rheims phrasing in chapters** — seeded chapters follow DR
    ("Brothers:", "the domestics of God", "one towards another"). If the
    diurnal prints KJV-style lessons this is a systematic word-for-word
    difference across most seeded chapters; settle the policy once before
    volunteers file dozens of identical issues.

16. **American spellings scattered among British** — "honor/Splendor" in
    `proper/christmas.txt [hymn-lauds]`/`[hymn-vespers]`,
    `proper/holy-innocents.txt` hymns, `commons/dedication.txt [hymn-lauds]`,
    `proper/vigil-epiphany.txt` hymns; "scepter" in
    `proper/christ-the-king.txt [hymn-vespers]`. Also "Holy Zions Help"
    (missing apostrophe) in `commons/dedication.txt [hymn-lauds]`.

17. **Unpointed antiphons** (no ` * ` mediant) — now surfaced automatically by
    `./office lint` as `unpointed-antiphon` (11 hits), e.g.
    `proper/apparition-st-michael.txt`/`proper/dedication-st-michael.txt`
    `[psalm-antiphon-4]`, `proper/conversion-st-paul.txt [psalm-antiphon-4]`,
    `proper/st-agatha.txt [psalm-antiphon-4]`,
    `proper/guardian-angels.txt [psalm-antiphon-5]` (which also lacks
    punctuation: "all His Angels Praise ye Him").

18. **Short responsories with inconsistent pointing/repeats** —
    `proper/nativity-john-baptist.txt` (terce and none have no asterisk at
    all; sext's final repeat drops it), `commons/dedication.txt
    [short-responsory-sext]` (second line missing final period).

19. **"R. Thanks be to God." inconsistently present after chapters** — e.g.
    `commons/apostle.txt [chapter-lauds]` lacks it while `[chapter-sext]` has
    it; the Phil 2:5-7 chapters in `proper/exaltation-holy-cross.txt` and
    `proper/finding-holy-cross.txt` lack it. Decide whether the response
    belongs in the data or the hour definition, then normalize.

20. `proper/christ-the-king.txt` hymns — "coersion" (coercion), "we do
    remit;—" punctuation, and metrically garbled lines ("As Thy one sheepfold
    be we embraced") — the whole pair of hymns deserves a word-for-word check.

21. `proper/advent-sunday-3.txt [psalm-antiphon-5]` — "We should live *
    righteously and godly…" — Titus 2:12 usually has "soberly, righteously,
    and godly"; check whether "soberly" was dropped.

22. `proper/all-saints.txt [psalm-antiphon-1]` — "lo, great multitude"
    (missing "a").

23. `commons/confessor.txt [psalm-antiphon]`/`[psalm-antiphon-1]` — "Lord,
    Thou deliverest unto me five talents" — Matt 25:20 wants the past tense
    "deliveredst".

24. **Parenthesized "(Alleluia.)"** in versicles/antiphons
    (`proper/annunciation.txt`, `proper/exaltation-holy-cross.txt`,
    `proper/st-gabriel-archangel.txt`, `proper/apparition-st-michael.txt`,
    …) — confirm these render conditionally in Eastertide rather than
    printing the parentheses year-round.

## Also machine-flagged (see `./office lint`)

- 9 `truncated` entries (missing final punctuation), including the
  `commons/blessed-virgin` / `commons/holy-woman` "spikenard" antiphons and
  three hymns ending "Amen" without a period.
- 38 `near-duplicate` pairs — same text seeded twice with small differences;
  the 1.00-similarity pairs differ only in case/punctuation and one side of
  each pair is presumably wrong.
