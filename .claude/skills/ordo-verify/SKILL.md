---
name: ordo-verify
description: Verify the app's calendar and office composition against the archdiocese's printed ordo PDFs in ../resources. Use when checking feast dates, ranks, colors, commemorations, preces/suffrage, antiphons, or Vespers precedence against parish practice, or after calendar/engine/text changes that should move the ordo-diff numbers.
---

# Verifying against the parish ordo

The archdiocesan ordo PDFs in `../resources/` are the ground truth for what
this app should produce. **Only the newest year is authoritative for the
sanctoral calendar** (the archdiocese has revised it over the years);
older years are still valid for the temporal cycle (paschalion, moveable
dates, Sunday counts).

## Authority hierarchy

1. Newest-year ordo PDF — what the archdiocese actually does.
2. `diurnal-rubrics.pdf` — normative general rubrics (§XXXVII preces,
   §XXXVIII suffrage). If the ordo contradicts it (it does, for Sunday
   preces), that's a question for the priest — file an issue, don't pick.
3. `additional-sunday-rubrics.pdf` — extra Epiphany Sundays rule. Trust its
   prose over its year table (the table misclassifies 2024).
4. Older ordos — temporal cycle only; sanctoral diffs are usually noise.

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

Baseline numbers (2026, as of PR #14 era) so regressions are obvious:
moveable all-OK; calendar headlines clean after PR #8; Hours preces 47;
Vespers suffrage 10 and Vespers comm 136 (after PR #21); Lauds comm 47;
Ben antiphons 109/288; Mag antiphons 205/335 (after ferial, Sunday, octave,
weekly-temporal, O-antiphon and Saturday historia seeding — Saturday
I Vespers Magnificats follow the scripture cycle via calendar.HistoriaWeekID
and proper/historia-* files; residue is DO-vs-diurnal translation drift,
per-saint propers DO lacks, ruling-gated Lenten feria/feast days, and the
per-annum monastic weekly antiphons absent from DO); colors 31/712 (mostly
ordo-internal notation artifacts where the printed "Vespers G" contradicts
the ordo's own described office, plus ruling-gated Ember/All Souls rows) and
Vespers designations 225 agree / 6 disagree (after the §XIII concurrence
rewrite; residue is parish rank demotions to memorial, the Ember-day
precedence question, and three ordo notation artifacts where the quoted
content matches ours: 01-04, 05-01, 08-23). Open issues:
#9 #10 #11 #12 #13 (rulings needed), #15 #17 #20 (engine/data work). A fix should move its cluster toward zero without
regressing the others.

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
