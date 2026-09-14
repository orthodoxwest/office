# Composition requirements and remaining review

Reviewed by Codex on 2026-09-13 against the local Monastic Diurnal page images
and 2026 archdiocesan ordo. The executable appointment cases are in
[composition-requirements.json](composition-requirements.json); their runner is
`TestCompositionRequirements`. The complete-calendar checks compose every hour
on every date in 2026, 2027, and 2032. Additional years exercise reviewed rules
and calendar collisions; they do not establish agreement with future ordos.

## Requirements checked in this repair series

| Requirement | Evidence and regression scope |
|---|---|
| Temporal commemoration uses its own proper, seasonal ordinary, or Sunday Psalter versicle | Diurnal General Rubrics X, p. xxix; pp. 41, 246, 248, 250–251. Six ordinary Sunday Lauds cases, Lent feria Lauds/Vespers, proper precedence controls, and an ordinary Sunday rule across three years. |
| Ordinary weekday Vespers uses its weekday responsory; Sunday has its own wording and response division | Diurnal pp. 114, 121, 125, 131, 135, 139. All applicable ferial weekdays across three years; Saturday, Lent and Sunday controls. |
| Ferial little hours use the Advent and Lenten chapters and versicles, and Lenten antiphons | Diurnal pp. 163–164 and 250–251; 2026 ordo pp. 44, 125. Direct chapter/versicle/antiphon assertions, full-calendar chapter/versicle checks, and the post-Ash-Wednesday/start-of-Lent-I boundary. |
| Paschal ferial chapters and direct versicles preserve Sunday and feast precedence | Diurnal pp. 372–374, 378. Direct-verse versus responsory precedence tests, ferial checks across three years, and Sunday/feast controls. |
| Easter week retains its distinct chapters and “This is the day” through Saturday None | Diurnal p. 365. Easter Day and Saturday None cases; explicit Monday/Tuesday reuse and existing octave inheritance; Low Sunday chapters and versicles as controls. |
| St Michael's little hours use their feast chapters | Diurnal pp. 610–611; 2026 ordo p. 102. Terce/Sext/None cases in 2026, 2027 and 2032. |
| All Saints and its octave use their proper little-hour versicles | Diurnal pp. 640–641; 2026 ordo pp. 112–113. November 1 and 3–7, with November 8 Sunday as an exclusion control. November 2 Office of the Dead is separate. |
| II Sunday after Easter Sext and None use their explicit chapters | Diurnal p. 378; 2026 ordo p. 58. April 26 source and chapter-before-versicle assertions. Terce is excluded pending clarification. |

| Sunday within the Nativity octave retains its proper Vespers, including when observed on a weekday | Diurnal pp. 184–185, 205–206; 2026 ordo p. 128. December 29 chapter/responsory/hymn/versicle and December 28 following-office commemoration antiphon. |

The earlier Sunday Lauds repair (#332) has its own direct appointment tests.
No row above certifies the entire hour or the entire source page.


## Source questions held separately

- **Winter Sunday Lauds hymn boundary:** [#331](https://github.com/orthodoxwest/office/issues/331).
- **April 26 Lauds/Terce chapter references:** [#335](https://github.com/orthodoxwest/office/issues/335).
  The ordo's Lauds citation points to p. 370 (Low Sunday, 1 John 5:4); the
  Sunday proper on p. 377 has 1 Peter 2:21–22, and p. 378 sends Terce back to
  Lauds. Neither reference supports the app's ordinary 1 John 4:16. No
  replacement has been selected while the intended appointment is unresolved.

## Next source-review passes

These are uncompleted requirements, not passing checks or adjudicated defects.
For each pass, record the cited requirement, expected behavior, representative
and boundary cases, then add an executable assertion or a linked unresolved
question. Expand the existing checks without deriving expectations from output.

1. **Remaining seasonal little-hour appointments:** Passiontide chapters and
   versicles (ordinary Passion week, Holy Monday–Wednesday, and the transition
   into the separate Triduum); remaining Easter Sunday chapter appointments,
   particularly May 10 and May 17, 2026. Complete the unreviewed scope of #314.
2. **Commons and proper inheritance:** the little-hour chapters and versicles
   of Apostles, Martyrs, Confessors, and the BVM, in and outside Paschaltide.
   Check printed cross-references, hour-specific exceptions, and octaves. A
   plausible reduction of a seeded responsory is not sufficient evidence.
3. **Ordinary structure at every weekday/hour:** openings, hymn placement,
   psalm/antiphon assignments, ferial versus festal canticles, repeated
   antiphons, chapter/versicle order, collects and conclusions. Prime and
   Compline received limited chapter/versicle controls in this audit only.
4. **Seasonal and concurrence boundaries at the major hours:** preces,
   suffrages, alleluias, doxologies, Marian antiphons, I/II Vespers, incoming
   versus outgoing commemorations, and season changes while another feast
   owns the office. Existing snapshots alone do not establish these rules.
5. **Prayer forms:** compare the private, deacon, and priest openings,
   greetings, confession, and endings against their actual appointments.

Known incomplete Triduum and All Souls little-hour work remains separate from
ordinary-year review. Follow the existing repair backlog and clergy questions;
do not silently resolve an ordo/source conflict by changing the calendar.

## Audit measurement correction

The initial fallback audit reported 24 weekday Vespers exposures. The direct
whole-calendar check identifies **45 in 2026: 43 unnamed ferias and two
September Ember days**. The ingestion inventory omits unnamed principal
owners; a named commemoration can nevertheless leave rows for that date/hour.
Supplementing only entirely absent date/hours therefore misses some principal
slots. Composition checks must inspect the full composed hour, even when the
inventory already contains rows for a commemoration. Keep the ingestion
inventory's owner filter intact for its own proposal-safety purpose.

Report checked requirements, outstanding defects, and unresolved source
questions separately. Counts of compositions, source attestations, and test
cases are not percentages of liturgical correctness.
