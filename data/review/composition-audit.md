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
| Passiontide little hours use their printed chapters, direct versicles and antiphons, with Sunday and Holy Week exceptions | Diurnal pp. 272–275, 279–280; 2026 ordo pp. 50–53. Passion/Palm Sunday Prime–None antiphons, ferial Passion-week antiphons, chapters through Holy Wednesday, Holy Monday–Wednesday Lauds antiphon mapping controls, Lent/Triduum boundaries, and calendar-wide versicle/chapter/antiphon rules in three years. |
| III and IV Sundays after Easter keep their own chapters, including Lauds and II Vespers cross-references | Diurnal pp. 380–383; 2026 ordo p. 62. May 10 five-hour chapter checks, III Sunday cases in 2027/2032, the May 3, 2026 Holy Cross precedence control and following ferial little hours. |
| V Sunday after Easter Lauds and II Vespers use their explicit chapter appointment | Diurnal pp. 385–386; 2026 ordo p. 64. May 17 Lauds/Vespers; little-hour chapters remain excluded pending #339. |

The earlier Sunday Lauds repair (#332) has its own direct appointment tests.
No row above certifies the entire hour or the entire source page.

The seasonal follow-up adds 118 explicit cases (220 total) and checks 78 Passiontide
slots in 2026 and 69 each in 2027/2032. A frozen before/after comparison of all
2,555 date/hours in 2026 finds 93 changed source selections and two existing
I Vespers chapters with added printed pointing. These are scoped regression
counts, not whole-office certification. Independent Codex and Sonnet image
readings agree on the 18 new canonical texts and three reused chapters.

## Source questions held separately

- **Winter Sunday Lauds hymn boundary:** [#331](https://github.com/orthodoxwest/office/issues/331).
- **April 26 Lauds/Terce chapter references:** [#335](https://github.com/orthodoxwest/office/issues/335).
  The ordo's Lauds citation points to p. 370 (Low Sunday, 1 John 5:4); the
  Sunday proper on p. 377 has 1 Peter 2:21–22, and p. 378 sends Terce back to
  Lauds. Neither reference supports the app's ordinary 1 John 4:16. No
  replacement has been selected while the intended appointment is unresolved.

- **May 17 little-hour chapter references:** [#339](https://github.com/orthodoxwest/office/issues/339).
  The ordo cites p. 383 (IV Sunday), while V Sunday has distinct chapters on
  pp. 385–386. All three little-hour chapters remain unchanged pending
  clarification; the Lauds/Vespers p. 385 references are unambiguous.

## Apostle little-hour data pilot

Reviewed on 2026-09-14: Diurnal pp. 11*-12* and 18*-19*, with the
2026 ordo's May 1 Hours (p. 59) and August 24 Hours (p. 92) as appointment
controls. Six explicit little-hour versicles now replace runtime responsory
reduction for the ordinary and Paschal Apostle commons. Terce reuses existing
identical verses through aliases; four entries store the other printed pairs.
The chapter references remain unchanged, including Terce's return to Lauds.
The bounded Codex and Sonnet page readers agree on the six pairs; two slots
required a fresh reading at 300 dpi to preserve archaic spelling and follow
a column break. Existing canonical source evidence is retained, with five
new hash-bound wording attestations. These do not certify other appointments
that happen to use the same canonical texts.

`TestApostleLittleHourAppointmentsAcrossCalendars` checks the common-appointed
Apostle versicles in all three prayer forms throughout 2026, 2027, and 2032.
Twelve new source-backed cases check chapters, versicles and their order;
three compatibility controls preserve Conversion of St Paul's proper priority.
The comparison of 23,016 complete offices found no rendered content or
structure changes. Source metadata changed for 405 offices (135 date/hours
in three forms), all in the intended Apostle little hours.

At the end of that pilot, Evangelist, Paschal Martyr, and Conversion of St Paul
entries still referenced the legacy responsories. In the same three-year
sweep, private-form reductions fell from 1,374 to
1,239. These are migration counts, not percentages of liturgical correctness.
Independent adversarial review confirmed that this pilot needs no engine
branch or scope-schema extension. The dependent-common migration below follows
these references before retiring legacy entries. Other common families were
not reviewed by that pilot.

## Dependent-common little-hour migration

The dependent-common migration continues the Apostle pilot: Evangelist
ordinary/Paschal and the One Martyr, Many Martyrs, and Bishop-Martyr Paschal
commons now have 15 direct versicle aliases. Diurnal pp. 6*, 513, and 626
establish Evangelist inheritance; pp. 31*-33* establish Paschal Martyr
inheritance with no little-hour chapter/versicle exception. The 2026 ordo
confirms the Hours for St George and St Mark (p. 57) and St Luke (p. 108).
Codex and Sonnet reviewed these bounded cross-references; the canonical
wording and its existing attestations are unchanged.

The alias scan and a two-stage comparison permit removal of 20 legacy
little-hour responsory entries, including the now-unused Apostle Paschal
None body. Apostle None remains for Conversion of St Paul; all major-hour
responsories remain. Across 23,016 complete offices in 2026, 2027, and 2032,
the alias migration changes only source metadata in 135 offices (45
date/hours in three forms). Subsequent deletion changes no complete-office
hash. Private-form compatibility reductions fall from 1,239 to 1,194.
The controlled common tests include Many Martyrs, which has no principal
Paschal occurrence in that three-year default-calendar sample. Twenty-seven
source-backed cases cover chapter/verse order, real appointments, and the
St George octave; proper priority and omission controls remain in place.
These checks cover the named little-hour appointments, not the major-hour
exceptions, all remaining commons, or future-ordo agreement.

## Ordinary Martyr little-hour migration

Diurnal pp. 21*-23* and 25*-27* appoint the One Martyr and Many Martyrs
little hours. The distinction between a Bishop and a non-Bishop changes the
collect, not these chapter/versicle appointments. Terce returns to the Lauds
chapter; Sext and None print their own chapters and simple V./R. pairs. The
2026 ordo confirms these Hours for Fabian and Sebastian (p. 30), Timothy
(p. 31), and Alban (p. 74).

The three ordinary Martyr commons now have nine direct verse appointments:
four canonical bodies and five aliases, reusing the existing Terce pairs.
Bounded Codex and Sonnet readings establish all six distinct pairs. Fresh
300 dpi readings resolve a Terce preposition disagreement and the two Sext
column crossings; the initial readings are retained in ignored run artifacts.
Five hash-bound source attestations cover the newly stored or reused wording.

Nine obsolete little-hour responsory entries and their four obsolete
attestations are retired. Those older attestations attributed full responsory
bodies to pp. 23* and 27*, which print only the simple pairs at those hours.
The new attestations concern the actual canonical verse bodies. Major-hour
responsories, proper overrides, and Paschal appointments remain intact.

Across 23,016 complete offices in 2026, 2027, and 2032, the direct appointments
change source metadata in 387 offices (129 date/hours in three forms), with
no rendered content or structure changes. Deleting the obsolete entries and
attestations changes no complete-office hash after that migration. Private-form
compatibility reductions fall from 1,194 to 1,065. Static dependency review,
proper-priority and omission controls, 18 source appointment/order cases,
three St Stephen compatibility cases, and all-form common tests cover this
migration. These counts do not certify proper or octave inheritance, the full
major hours, or future-ordo agreement.

## Next source-review passes

These are uncompleted requirements, not passing checks or adjudicated defects.
For each pass, record the cited requirement, expected behavior, representative
and boundary cases, then add an executable assertion or a linked unresolved
question. Expand the existing checks without deriving expectations from output.

1. **Commons and proper inheritance:** the little-hour chapters and versicles
   of Apostles, Martyrs, Confessors, and the BVM, in and outside Paschaltide.
   Check printed cross-references, hour-specific exceptions, and octaves. A
   plausible reduction of a seeded responsory is not sufficient evidence.
   Include the May 3, 2026 Holy Cross None ordinary-chapter fallback as an
   unreviewed candidate; this pass checked only its Terce precedence control.
2. **Ordinary structure at every weekday/hour:** openings, hymn placement,
   psalm/antiphon assignments, ferial versus festal canticles, repeated
   antiphons, chapter/versicle order, collects and conclusions. Prime and
   Compline received limited chapter/versicle controls in this audit only.
3. **Seasonal and concurrence boundaries at the major hours:** preces,
   suffrages, alleluias, doxologies, Marian antiphons, I/II Vespers, incoming
   versus outgoing commemorations, and season changes while another feast
   owns the office. Existing snapshots alone do not establish these rules.
4. **Prayer forms:** compare the private, deacon, and priest openings,
   greetings, confession, and endings against their actual appointments.

The merged Triduum little-hour structure is a boundary control in this pass;
remaining Triduum and All Souls work stays separate from ordinary-year review. Follow the existing repair backlog and clergy
questions;
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

The Passiontide chapter/versicle appointment scope of #314 is now covered.
Its remaining source-accounting work concerns the legacy seeded responsory
entries and Paschal Compline; this repair does not attest those unused
little-hour responsories as if the Diurnal printed them.
