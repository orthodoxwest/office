---
name: ordo-verify
description: Verify the app's calendar and office composition against the archdiocese's printed ordo PDFs in ../resources. Use when checking feast dates, ranks, colors, commemorations, preces/suffrage, antiphons, or Vespers precedence against parish practice, or after calendar/engine/text changes that should move the ordo-diff numbers.
---

# Verifying against the parish ordo

The archdiocesan ordo PDFs in `../resources/` are the ground truth for what
this app should produce. **The newest year is authoritative** — it reflects
current archdiocesan policy, which is revised over time. A feast, rank, or
discipline that held steady across older ordos and then differs in the newest
year may be a deliberate revision, so don't assume a typo just because it
changed — but typos happen every year too. Flag such a discrepancy for
confirmation (a priest ruling) rather than silently following either. Older
years stay useful for cross-checking anything unchanged and are always valid
for the temporal cycle (paschalion, moveable dates, Sunday counts).

One exception: **computus figures** (Golden Number, Dominical Letter, moveable
dates, Ember days) are mathematically determined, so a discrepancy there is a
genuine error in whichever ordo carries it — never a policy choice (this is how
the known-wrong Tabula values below are identifiable as typos).

## Authority hierarchy

1. Newest-year ordo PDF — what the archdiocese actually does.
2. `diurnal-rubrics.pdf` — normative general rubrics (§XXXVII preces,
   §XXXVIII suffrage). If the ordo contradicts it (it does, for Sunday
   preces), that's a question for the priest — file an issue, don't pick.
3. `additional-sunday-rubrics.pdf` — extra Epiphany Sundays rule. Trust its
   prose over its year table (the table misclassifies 2024).
4. Older ordos — always valid for the temporal cycle, and useful for
   confirming unchanged sanctoral/disciplinary content; where they differ from
   the newest year, treat the newest as current policy but flag the change for
   confirmation (it may be a deliberate revision or a typo).

## Workflow

```bash
pdftotext -layout ../resources/2026-ordo.pdf /tmp/2026-ordo.txt
./office ordo 2026 > /tmp/our-ordo.txt
./office rubrics 2026 > /tmp/our-rubrics.tsv   # per-day TSV incl. Ben/Mag antiphons

scripts/ordo-compare.py calendar  /tmp/2026-ordo.txt /tmp/our-ordo.txt      # headlines
scripts/ordo-compare.py rubrics   /tmp/2026-ordo.txt /tmp/our-rubrics.tsv   # preces/suffrage/comms
scripts/ordo-compare.py antiphons /tmp/2026-ordo.txt /tmp/our-rubrics.tsv   # Ben./Mag. incipits
scripts/ordo-compare.py colors    /tmp/2026-ordo.txt /tmp/our-ordo.txt
scripts/ordo-compare.py vespers   /tmp/2026-ordo.txt /tmp/our-ordo.txt      # I fol./II prec.
scripts/ordo-compare.py moveable ../resources 2017 2018 2019 2021 2022 2023 2024 2025 2026
```

`./office ordo` opens with a **Tabula Temporaria** header (Golden Number,
Dominical Letter, Sundays after Epiphany/Pentecost, moveable feasts, Ember
days) and renders each day as per-hour stanzas (Lauds/Hours/Vespers with
color, Ben/Mag incipit, preces + Suffrage, commemorations with incipits). The
compare parser reads this stanza format; `moveable` still cross-checks the
moveable dates, and the Tabula header can be eyeballed against the ordo's own
front matter (its computus figures verify `calendar.ComputeTabula`).

## Interpreting the diffs

None of the diffs go to zero — each carries known residue that is not a
regression. Judge a change by whether it moves its own cluster toward zero
without disturbing the others; capture a genuinely new divergence, or one
that needs a clergy decision, as a GitHub issue rather than here. The
durable residue categories:

- **calendar headlines** — sanctoral naming differences against older ordos
  are noise; only the newest year is authoritative for the sanctoral.
- **antiphons (Ben./Mag.)** — DO-vs-diurnal translation drift, per-saint
  propers Divinum Officium lacks, ruling-gated Lenten feria/feast days, and
  the per-annum monastic weekly antiphons absent from DO. Saturday I Vespers
  Magnificats follow the scripture cycle via `calendar.HistoriaWeekID` and
  `proper/historia-*` files.
- **colors** — mostly ordo-internal notation artifacts, where a printed
  "Vespers G" contradicts the ordo's own described office, plus ruling-gated
  Ember/All Souls rows.
- **Vespers designations** — parish rank demotions to memorial, the
  Ember-day precedence question, and ordo notation artifacts where the
  quoted content matches ours.
- **preces / suffrage** — governed by the diurnal-rubrics §XXXVII/§XXXVIII
  conditions; disagreements usually trace to a ruling-gated day.

**Known ordo front-matter errors** (do not treat as regressions): some
printed Tabula Temporaria values are transcription errors. The 2025 Golden
Number and Dominical Letter are copied from 2024 (correct: XII / E). The 2021
and 2024 "Sundays after Pentecost" figures disagree with every straightforward
calendar count and with the calendar builder's own reckoning.
`calendar.ComputeTabula` computes the correct values rather than reproducing
these.

## Parsing caveats

- Ordo rank codes: D1 D2 Gd D Sd M (memorial = commemoration-only)
  F2 (privileged feria) F3 V. A leading `‡`/`†` marks obligation/devotion
  days; `[bracketed]` feasts are scoped (e.g. "Monastics & Oblates Only").
- Day titles sometimes wrap or sit a line above/below the day number; the
  parser tolerates one skipped day (the 2026 PDF misprints Dec 24 as
  "23 Thu").
- The Epiphany proclamation (Jan 6) announces all moveable feasts and Holy
  Saturday mentions Easter — anchor searches must stay inside day-title
  blocks and respect month floors.
- Scanned pages (e.g. additional-sunday-rubrics.pdf) have no text layer:
  render with `pdftoppm -png -r 150` and read the image.
