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

## Blessed Virgin common little-hour migration

Diurnal p. 67* prints the three Blessed Virgin feast-common V./R. pairs;
Terce returns to the Lauds chapter and Sext/None have their own chapters.
The Saturday office explicitly borrows these chapters and verses on p. 69*.
Its After Christmas and Paschal variants do not replace them (pp. 70*-71*);
p. 72* begins the Office of the Dead. The 2026 ordo confirms the common Hours
for January 31, May 16, and June 20 Saturdays (pp. 33, 63, 73), the Visitation
(p. 77), Nativity BVM (p. 96), and Holy Name of Mary (p. 97).

Three direct appointments replace the BVM little-hour responsory aliases.
Terce reuses an existing attested canonical verse; Sext and None have two
new canonical bodies and hash-bound attestations. Bounded Codex and Sonnet
image readings agree on the pairs and Saturday cross-references. The initial
None reading selected the neighboring Sext verse; a corrected bounded reading
agrees with Sonnet and the page image. The three old aliases are retired;
their Virgin-Martyr targets remain required by other commons.

The comparison of 23,016 complete offices in 2026, 2027, and 2032 finds 666
source-metadata changes (222 date/hours in three forms), with no rendered
content or structure changes. Removing the aliases changes no complete-office
hash after the direct migration. Private-form reductions fall from 1,065 to
843. Thirty-six appointment/order cases, six proper-priority controls, and
all-form verse checks cover the named dates, including the existing Paschal
alleluia decoration. This does not certify the full Saturday office, Marian
feast propers, octave inheritance, or future-ordo agreement.

## Next source-review passes

These are uncompleted requirements, not passing checks or adjudicated defects.
For each pass, record the cited requirement, expected behavior, representative
and boundary cases, then add an executable assertion or a linked unresolved
question. Expand the existing checks without deriving expectations from output.

1. **Commons and proper inheritance:** the little-hour chapters and versicles
   of Apostles, Martyrs, Confessors, and the BVM, in and outside Paschaltide.
   Check printed cross-references, hour-specific exceptions, and octaves. A
   plausible reduction of a seeded responsory is not sufficient evidence.
   The May 3 Holy Cross None fallback is repaired in the Cross pass below;
   other proper and octave inheritance remains to be reviewed.
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


## Triduum review of PR #360

The shared `usesTriduumForm` policy bounds psalm doxology omission and the
silent collect conclusion independently. All seven hour definitions keep the
complete Christus factus est ending together in `[Triduum-Ending]`.

Reviewed by Codex on 2026-09-15 against the Monastic Diurnal page images and
2026 archdiocesan ordo pp. 15–16 and 53. This review is separate from the ordinary
year assessment above.

| Requirement and source | Cases, expected behavior, and regression |
|---|---|
| Lauds begins with its first antiphon and uses five proper frames (Diurnal pp. 305–311, 336–341, 356–359) | April 9–11, 2026: omit Psalm 67 and festal psalms; use each day's appointed psalms, canticle, and divided Laudate psalm. `TestTriduumPsalmodyAppointments` also checks 2027 and 2032. Existing Lauds antiphon wording remains subject to corpus verification. |
| Thursday and Friday Vespers use the same five psalms and antiphons (p. 315; ordo p. 53) | April 9–10: Psalms 116b, 120, 140, 141, 142; five repeated proper antiphon frames. The ending adds Psalm 51 as the sixth psalm. `TestTriduumPsalmodyAppointments` and both Vespers sweep tests cover this distinction. New antiphons have independent Codex/Sonnet image readings. |
| All hours end with the daily Christus factus est, entirely silent Our Father, Miserere, and the shared Almighty God collect with silent conclusion (p. 313; ordo Sacred Triduum notes) | All seven hours on April 9–10 and Lauds–None on April 11, in private/deacon/priest forms. `TestTriduumMajorHoursOpeningsEndings` checks the exact ending sequence, collect source and conclusion, voice partitions, and omission of ordinary greetings, commemorations and Marian endings. The three collect keys use the same independently agreed p. 313 reading, appointed to all three days by the printed rubric. |
| Compline begins with confession/absolution, then Psalms 4, 91, 134 and Nunc dimittis (p. 316) | April 9–10, all forms: confession is the only opening prayer; the canticle immediately follows the three psalms before Christus factus est. The same sequence test includes priest and deacon substitutions. |
| Lauds alone adds the candle/noise instruction (p. 313) | Exactly one rubric at each Triduum Lauds, none at other hours. Bounded readings distinguish this sentence from the preceding general ending instruction. |
| The ending stops at Holy Saturday None (p. 313; #276) | Wednesday and Easter Sunday controls retain ordinary openings/endings; Holy Saturday Vespers keeps Easter ownership and Compline keeps its proper Nunc dimittis antiphon. HTML/TeX tests and the browser Triduum test check silent prayer presentation. |

The ordo appoints the p. 313 ending and lists St Leo on April 11 without an
explicit Lauds commemoration. That listing alone does not override the printed
ending or establish a source conflict. The omitted feria/following-office
commemorations were ordinary composition artifacts; the review found no
contrary appointment for those hours.

These checks do not certify all Triduum wording or Tenebrae ceremonies.
The subsequent Holy Saturday Vigil review below addresses the abbreviated
Vespers form that was outside PR #360's scope.

## Cross composition repairs

Diurnal pp. 43 and 146–147 appoint distinct Paschal Cross antiphons at Lauds
and Vespers, followed by a shared verse, invitation, collect, and abbreviated
Through-the-same conclusion. The old shared antiphon and verse were wrong;
the raw collect also omitted both its invitation and its conclusion. The hour
definitions now appoint the two antiphons and reuse the already attested
invitation and conclusion formulas. No additional engine mechanism is needed.
Three new hash-bound attestations cover the antiphons and verse; the existing
collect body and formula attestations remain intact. Bounded Codex and Sonnet
readings, with rejected incomplete/column-crossing readings retained in ignored
artifacts, establish the new wording and the printed conclusion appointment.

Diurnal p. 524 appoints Philippians 2:8–9 at Finding of the Holy Cross None.
The 2026 ordo p. 60 assigns the Hours of the feast to that page. The missing
chapter now aliases the identical, already attested Exaltation chapter from
p. 600. This replaces the ordinary fallback without duplicating the body.

Across 23,016 complete offices in 2026, 2027, and 2032, 309 rendered offices
change: 100 Lauds/Vespers date-hours and three Holy Cross None date-hours,
each in private, deacon, and priest forms. Cross presence is unchanged.
After excluding the Cross section and the repaired Holy Cross None chapter,
all other composed content is identical. Tests assert the complete Cross
sequence and text in both hours and all forms, and the proper None chapter.
These comparisons establish repair scope, not whole-office correctness.

Eligibility remains unresolved in [issue #356](https://github.com/orthodoxwest/office/issues/356).
The Diurnal p. 146 suppresses Cross for a commemorated Double, including
concurrence. The 2026 ordo p. 61 explicitly includes it at May 9 Vespers with
Gregory Nazianzen (Double), while p. 64 omits it at May 17 Vespers with
Venantius (Double). The engine also ends the seasonal interval three days
early. Extending the interval alone would incorrectly add Cross on May 18;
blanket suppression would contradict the newest ordo on May 9. The proposed
eligibility changes are therefore deferred together pending clergy resolution.
No calendar or eligibility rule is changed by this repair.


## Collect invitations and sequence

The Diurnal General Rubrics XII (p. xxxi) concludes only the first and last
collects. The ordinary of Lauds (pp. 42–43) and Vespers (pp. 144–147) places
commemorations and the suffrage/Cross before the closing greeting, with
Let us pray before each suffrage collect. Breviary General Rubrics
XXXIII.3,5 (printed p. 50) explicitly requires that invitation before every
collect, including intermediate commemorations without a conclusion.

The app omitted the invitation in every generated commemoration and in both
forms of the Suffrage of All Saints. The four static suffrage sections now
reference the existing attested invitation. Generated commemorations resolve
that same shared prayer and retain the commemoration owner. No corpus body,
conclusion-selection rule, commemoration eligibility, or prayer-form rule changes.

The before/after comparison covers 23,016 offices across 2026, 2027, and 2032
in three forms. It adds 7,257 invitations in 4,911 offices: 2,490 Lauds and
2,421 Vespers. Removing invitation elements from both captures leaves every
complete-office hash identical, including all other text, sources, decisions,
and commemorations. Thirteen calendar fixtures in all three forms assert
invitations, commemoration ownership, and first/last-only conclusions, including
multiple commemorations, ordinary/BVM suffrages, and the Cross. The full
private-form collect-run audit finds no misplaced conclusions before or after.
The sequence audit covers 2,189 private-form runs and 4,708 collects; the
three All Souls Lauds offices, which lack that normal closing boundary, are
excluded. Remaining Triduum structural work stays under its existing backlog. These checks
establish ordering and repair scope, not the correctness of every appointment
or collect's wording. Cross eligibility remains deferred in #356.


## Proper little-hour chapter fallbacks

Seven missing appointments used ordinary little-hour chapters despite printed
propers. Circumcision Sext/None (Diurnal p. 213; 2026 ordo p. 25) now reuse
the existing Christmas Hebrews chapters. Transfiguration None (p. 584;
ordo p. 87) now supplies Revelation 21:10,11,23. Sexagesima None (pp. 239–240)
now supplies 2 Corinthians 12:9; the ordo p. 38 references the Sunday psalter
without appointing a different chapter. Nativity Sunday's Terce/Sext/None
(pp. 206–207; ordo p. 128) now supply Galatians 4:1–2, 4:4–5, and 4:7,
including when that Sunday office is observed on a weekday.

Five aliases reuse existing attested chapter wording and its current pointing;
only the Transfiguration and Sexagesima chapters add new bodies and hash-bound
attestations. Bounded Codex and Sonnet readings agree after comparison
normalization. Initial word-order and opening-word disagreements were resolved
by corrected readings; rejected readings and page identities remain in ignored
artifacts. No canonical alias target, engine rule, or feast metadata changes.

The comparison of 23,016 complete offices in 2026, 2027, and 2032 finds exactly
63 changes: these seven appointments in three years and three forms. Removing
chapter elements from both captures leaves every complete-office hash identical.
`TestProperLittleHourChapters` checks all 63 cases, including source selection,
chapter wording, response, and chapter-before-versicle order. These cases fail
against the pre-repair corpus. Additional years exercise the appointed office,
not agreement with future ordos.

The initial candidate inventory contains 130 ordinary chapter resolution rows
outside Prime/Compline, including legitimate weekday/vigil/Sunday fallbacks
and unresolved source questions. It is not a defect count. This pass reviews
only the seven appointments above; #335 and #339 remain untouched. The discovery
catalog omits the computed Nativity Sunday owner, and its Circumcision dossier
stops at p. 209 before the little hours on p. 213. The full resolution inventory
and parent-verified page selection supplement those limited discovery results;
neither negative searches nor discovery queue membership certify a fallback.


## Sunday within the Corpus Christi octave

Diurnal pp. 417–421 and the 2026 ordo pp. 71–72 appoint a distinct Sunday
within this octave. Terce, Sext and None used ordinary antiphons, chapters
and verses; I Vespers paired the proper antiphons with Saturday psalms, and
II Vespers used ordinary psalmody, chapter, responsory, hymn and verse. Nineteen explicit
data appointments repair those fallbacks with sixteen aliases and three new
bodies: the Sext/None chapters (1 John 3:16 and 3:18) and None verse. No engine
rule, feast metadata, shared psalm text or canonical alias target changes.

Terce and II Vespers now refer to the Sunday Lauds chapter. The little hours
reuse their appointed Lauds antiphons and existing simple verses. Both Vespers
use Psalms 110, 111 and 116:10ff; I Vespers ends with 147:12ff, while II Vespers
ends with 128. The existing Corpus Christi psalmody declarations supply these
sequences, with the feast's II Vespers profile used for Sunday I Vespers and
its I Vespers profile used for Sunday II Vespers. II Vespers also reuses the
existing Pange lingua hymn, Vespers responsory and proper antiphons. Prime's
already correct antiphon and psalms are regression controls.

Bounded Codex and Sonnet readings agree on the new wording and printed
appointment rubrics. Rejected readings include a punctuation disagreement,
an adjacent-verse overrun, confusion about the name “II Sunday after
Pentecost,” and a failed rubric reading. Corrected readings and a 300 dpi
None image resolve them; all attempts and document/page identities remain
in ignored run artifacts. Three new hash-bound attestations concern wording;
the alias targets retain their existing evidence. These do not attest full
psalm bodies or certify every inherited appointment.

Across 23,016 complete offices in 2026, 2027 and 2032, 27 offices change:
five principal date/hours on June 13–14, 2026, and four Vespers commemorations
on July 3–4 in 2027/2032, each in three prayer forms. The latter now use the
Sunday's own verse under General Rubrics X, p. xxix. All other elements and
metadata compare identically after excluding the repaired slots and their
psalmody decisions. Direct tests exercise the appointed Sunday in 2026 and
2028 in all forms, its commemoration in 2027/2032, and its displacement by the
transferred Visitation. Additional years exercise rule boundaries, not
agreement with future ordos.

The responsory citation required an additional boundary check, documented in
[#362](https://github.com/orthodoxwest/office/issues/362). Diurnal p. 418 contains
both the continuation of I Vespers' “He gave them” and the opening of Lauds'
“He fed them.” The 2026 ordo p. 72's reference to p. 418 therefore does not
establish a conflicting appointment. The explicit II Vespers responsory on
p. 421 agrees with I Vespers on pp. 417–418; II Vespers now reuses that existing
body. Independent adversarial review confirmed this pagination distinction.
This pass does not certify the entire office, octave commemoration wording,
or the wider octave.

The audit also identified four missing Epiphany Sunday II Vespers appointments
and a distinct little-hour source conflict, tracked in
[#361](https://github.com/orthodoxwest/office/issues/361). The II Vespers
appointments were repaired by #364; the little-hour chapter question remains
in the repair backlog pending clergy clarification.
Ordinary Sunday Vespers' 2 Corinthians chapter agrees with Diurnal p. 114;
ordinary numbered Sunday propers inspected on pp. 232 and 437 do not print
replacement chapters. These negative findings are limited to those pages,
not clearance of every ordinary fallback in the inventory.


## Holy Saturday Vigil Vespers

The 2026 archdiocesan ordo p. 53 appoints abbreviated Vespers at the Vigil.
Diurnal pp. 360–361 also give its form apart from Mass, which is the form
supplied by the app. The civil Holy Saturday evening selects
`office/vespers-holy-saturday.txt`; Easter retains liturgical ownership, colour
and season. The separate definition is preloaded and validated with the seven
ordinary definitions, and `review explain` records its office-form decision.

| Source requirement | Appointment and verification |
| --- | --- |
| Psalm 117 with the triple Alleluia, then Magnificat with its proper antiphon; Gloria Patri follows each (p. 360) | `TestHolySaturdayVigilVespers` checks the exact complete sequence in private/deacon/priest forms in 2026, 2027 and 2032. The existing shared triple-Alleluia text matches the page. The Magnificat antiphon has independent Codex/Sonnet agreement. |
| The first printed collect, with the conclusion naming the same Holy Spirit (p. 360) | The new `vigil-collect` uses the independently agreed Diurnal text and the existing through-spirit formula. The ordo's “Pour forth” names this appointment; it does not print a competing text. The alternative P B Collect is excluded. |
| Apart from Mass: Benedicamus with two Alleluias in both versicle and response, Fidelium, then Our Father secretly and nothing more (p. 361) | Exact sequence and fully silent final-prayer assertions; independently agreed dismissal text. The ordinary opening, chapter, hymn, responsory, extra psalms, commemorations, Marian antiphon and final peace are absent. |
| The special form belongs to Holy Saturday Vespers | Adjacent evenings and all six other hours on Holy Saturday are boundary controls. Concurrent composition preserves the cached calendar and definitions. The annual Vespers count expects one psalm here. |

The ordo summary reads the actual fixed Magnificat frame, so direct corpus
references remain visible in its antiphon column. These references appear in
composition and provenance evidence; the dynamic resolution inventory covers
resolver-selected slots and does not enumerate this fixed form's references.

The 2026 before/after comparison changes only April 11 Vespers among the 21
rendered hours checked on April 10–12. Calendar, colour, ownership and
commemoration comparisons use the same reference and comparator before and
after. Goldens include the complete rendered April 11 Vespers and the 28-year
parity snapshot. Broader corpus wording remains a separate source-review task:
the Holy Saturday Compline follow-up below repairs its Nunc dimittis antiphon
and the transition to the collect.


## Holy Saturday Compline antiphon and collect transition

Diurnal pp. 361–362 prints the Nunc dimittis antiphon “In the end of the
sabbath,” also printed at Vigil Vespers on p. 360. The 2026 ordo p. 53 appoints
that proper Compline. Its antiphon now aliases the already reviewed Vigil
body, replacing the unrelated “Now Thou dost dismiss Thy servant” text.
Independent bounded Codex and Sonnet page readings agree on every word;
the existing canonical punctuation, pointing and attestation are retained.
The independent readings differ only in punctuation after “sabbath.”
No new canonical body, wording attestation or engine rule is introduced.

After the repeated antiphon, p. 362 directs the greeting, invitation and
collect at once. The extra Kyrie came from a dedicated `Chapter-Easter-Eve`
section; removing that section restores the printed transition. Compline's
opening, three psalms, four doxologies, Easter ownership, Regina caeli and
closing prayers remain intact. The Marian reference includes the ordinary
“May the divine help” conclusion appointed on p. 156; its position and wording
are explicit regression controls.

The comparison covers 23,016 complete offices across 2026, 2027 and 2032 in
private, deacon and priest forms. Only nine rendered offices change: Holy
Saturday Compline in each year/form, with two antiphon occurrences replaced
and one Kyrie removed. The other 3,279 Compline compositions lose only the
obsolete section's omitted decision-trace entry. After excluding these two
repairs and that trace entry, every composed-office comparison is identical.
All other hours retain identical complete-office hashes.

`TestHolySaturdayComplineAppointments` checks the exact canticle-to-collect
sequence, full antiphon and canonical source, retained psalmody and Marian
conclusion, evening ownership and adjacent-day boundaries. It fails against
the pre-repair data. These checks establish the repaired appointments and
retained structure, not complete wording or prayer-form certification. The
resolved antiphon item is removed from the outstanding repair backlog.


## Scholastica, octave antiphons, and seasonal cleanup

This pass repairs source-backed appointments with corpus bodies and aliases;
no production engine or ingestion code changes. The local 2026 archdiocesan
ordo remains the calendar authority. Additional years exercise boundaries,
not agreement with future ordos.

| Source requirement | Repair and verification |
|---|---|
| Scholastica I Vespers takes the Lauds chapter and antiphons, omitting the fourth Lauds antiphon; II Vespers repeats I except Magnificat (Diurnal pp. 475, 477, 479; ordo p. 36) | Vespers chapter aliases Lauds and the fourth Vespers antiphon aliases the fifth Lauds antiphon. Existing Common psalms 110, 113, 122, 127 remain. |
| Scholastica has full proper hymns and a Vespers responsory; Terce repeats the Lauds chapter (pp. 475–478) | Added both six-stanza hymns and the printed abbreviated Vespers responsory; Terce chapter aliases Lauds. Lauds' Common responsory already agrees with the scan. |
| Scholastica's Lauds ending never changes (p. 478); Vespers has its own six-line ending (pp. 475–476) | Hour-specific `@omit` doxology declarations protect the inline endings in Epiphany years, without suppressing seasonal endings at Prime, Terce, Sext, None or Compline. |
| Peter and Paul within-octave gospel antiphons differ from the feast; the terminal octave day retains them (pp. 558–559; ordo pp. 77–78) | Two canonical texts, with aliases for all six within-octave days, principal offices and commemorations. An explicit octave-day I Vespers alias also handles its anticipated commemoration on July 5, 2026. The parent feast stays unchanged. |
| Sexagesima and Pentecost have specific Magnificat wording (pp. 240, 399; ordo pp. 38, 68) | Replaced the unsupported wording with the printed texts. |
| Compline's Keep us verse adds alleluias in Paschaltide (p. 151), continuing through Pentecost octave Saturday None (2026 ordo p. 18) | Aligned the Easter verse's punctuation and added one Pentecost alias inherited throughout its octave. Compline had lost the alleluias from Pentecost I Vespers onward; Trinity I Vespers now ends the appointment at the correct boundary. |

The new hymn, responsory and antiphon readings have independent Codex and
Claude Sonnet readings with cited page images. Eight canonical bodies receive
hash-bound source attestations; aliases do not duplicate those bodies.
Artifacts, including a corrected hymn continuation reading, are retained under
ignored `output/composition-cleanup/`. The Scholastica responsory retains the
book's abbreviated repeated lines, as elsewhere in the corpus.

`TestScholasticaAppointments`, `TestScholasticaSmallHourHymnEndings`,
`TestPeterPaulOctaveGospelAntiphons`, `TestReviewedSundayAntiphonWording`, and
`TestPaschalComplineVerse` check text, source, principal/commemorated ownership,
and neighboring boundaries in all three prayer forms. They use 2026, 2027 and
2032, including the Epiphany hymn-ending cases and daily Compline from Easter
through Trinity Monday. Peter/Paul day II Vespers is not selected in that
calendar sample; a direct resolver check covers its data appointments.

A before/after comparison of all **23,016** seven-hour compositions in those
three years and forms finds **633 changed / 22,383 identical**: 63 Vespers,
66 Lauds, 9 Terce and 495 Compline. Full saved snapshots for Lauds, Vespers,
Scholastica and 2026 Compline have changes only in the repaired elements;
calendar metadata, section structure, decisions and all other elements match.
The remaining Compline changes are covered by the daily focused tests.
Holy Name outputs are identical after replacing two duplicated doxology
bodies with `@omit`: those declarations preserve the proper inline ending
and must not simply be deleted.

The unchanged 2026 ordo comparator clears five canticle-incipit findings and
introduces none: February 15, May 31, July 4 Benedictus, and both July 6
canticles. Strict comparable assertions improve from 3438/3626 to 3443/3626;
remaining mismatches fall from 188 to 183. These are incipit/appointment
measurements, not whole-office certification.

The fourteen remaining canticle-incipit differences are now explicitly
classified. February 23 matches the locally supplied Chair of Peter supplement
(II Vespers, final page), despite a different ordo incipit. September 4 is a
spelling error in the ordo's incipit; its cited Diurnal p. 23* agrees with the
app. April 1's apostolic I Vespers entry conflicts with the next day's feria
(ordo pp. 50–51), and September 18 repeats Thursday Vespers on Friday (ordo
p. 99 versus Diurnal pp. 136, 140). Those two appointment conflicts remain
unchanged and are recorded for clergy. Six previously unclassified antiphon
findings are consequences of the already tracked Vespers-concurrence (#62)
and Einsiedeln calendar-scope (#11) questions, not separate wording repairs.

A fresh 2026–2053 zero-occurrence inventory identifies the six legacy Easter
and Passiontide little-hour responsories as shadowed by their direct verses.
Their hash-bound dispositions are recorded; their wording is neither deleted
nor newly attested. Four stale disposition hashes are refreshed, and the two
Holy Name duplicate-body classifications are removed with those bodies.

The backlog drops completed Scholastica/seasonal targets and the stale Holy
Name of Mary collect target, replaces the general antiphon bucket with specific
source conflicts, and restores the unresolved Epiphany Sunday little-hour
question. **Peter and Paul's other octave appointments remain a concrete repair
target**: Common psalmody, chapters, hymns, responsories, verses, commemoration
verses and the terminal collect still require appointment-by-appointment review
against pp. 558–560. The gospel-antiphon fix does not certify those slots.

Validation: `make golden`, `make check`, and `make test-coverage` pass.
The 28-year parity snapshot changes only Lauds, Terce, Vespers and Compline
content/source digests; all calendar, composition-decision and
commemoration-merge digests match. Assurance reports eight additional verified
bodies, zero stale attestations, and zero stale zero-occurrence classifications.

## Peter and Paul octave; ordinary Prime and Compline (2026-09-15)

The remaining explicit octave appointments are checked against Diurnal
pp. 558–560 and Common of Apostles pp. 7*–16*, alongside the 2026 ordo
pp. 77–78. Existing per-day files now alias the Common for Lauds/little-hour
antiphons, within-octave Vespers antiphons, chapters and hymns. The octave
reverses the Common's Lauds/Vespers responsories and uses its own verse
appointments, including commemorations. The terminal day has the p. 560
collect and its second-person conclusion. Its I Vespers antiphons are explicit.
No new octave inheritance layer or feast-specific engine branch is added.

**Terminal II Vespers psalmody remains held.** The 2026 ordo p. 78 appoints
antiphons and psalms from Common I Vespers (7*); Diurnal p. 559 directs I and
II Vespers to 7* and 12*, respectively. Existing terminal II Vespers psalmody
is preserved pending clergy clarification. Hour-qualified terminal aliases
prevent the repaired Lauds appointments from changing that held selection.
The remaining octave backlog row now names this specific conflict.

The new collect body has independent Codex and Sonnet readings and a
hash-bound attestation. Sonnet agrees on the body but abbreviates the printed
conclusion differently; the attestation covers the body only. The conclusion
registration follows direct inspection of the printed abbreviation.

Prime and Compline were checked against Diurnal pp. 1–10, 14, 17, 20, 24,
29, 81–84 and 147–152, with the locally supplied parish drafts and printing
feedback as additional witnesses. Repairs are:

- Ordinary Sunday Prime uses its threefold Alleluia, preserving proper,
  Common and seasonal precedence. Paschal Prime retains the O Christ arise
  alleluias through Pentecost octave Saturday; Trinity restores the ordinary.
- The preces Creed at both hours has a spoken incipit, silent middle and
  spoken final articles, retaining Amen. Compline's opening Pater is entirely
  silent; its closing secret prayers retain their spoken incipits.
- The optional Martyrology notice/preview follows ordinary Prime's closing
  versicles. This does not implement the book's complete Capitular Office;
  the existing optional supplement follows the completed ordinary hour.

The weekly Prime psalter, Wednesday's joined Psalms 9b/10, ordinary Compline's
three psalms without antiphons or Nunc dimittis, and its omission of Faithful
departed already agree. The parish Prime draft supports the existing hymn
ending; later corporate-prayer feedback supports the modeled Lord's Prayer.
Those differences from the Diurnal are preserved. This is an appointment and
structure audit, not a fresh transcription of every ordinary text.

Tests exercise all seven generated octave identities, real principal and
commemorated offices in 2026/2027/2032, terminal I Vespers, all prayer forms,
Prime antiphon precedence, the complete Easter-to-Trinity Prime verse boundary,
weekly psalmody, secret-prayer spans and Martyrology placement. The independent
engine reviewer found no substantive blocker. Evidence and before/after
snapshots remain under ignored `output/octave-prime-compline/`.

The web renderer's existing voice spans now receive the correct prayer data.
TeX additionally honors valid inline silent spans, fixing the same Creed
presentation and the pre-existing omission of secret continuations in print.
Malformed partitions retain ordinary rendering; corporate speaker roles and
fully silent/collect handling keep their existing paths. Focused HTML and TeX
checks protect the spoken ending and its Amen.

The before/after sweep covers **23,016 offices** across 2026, 2027 and 2032 in
all three forms: **6,651 changed / 16,365 identical** (3,288 Prime, 3,261
Compline, 57 Lauds, 27 Vespers, and 6 each Terce/Sext/None). Prime's declaration
reordering also changes decision order, including on days where those sections
are omitted; the decision sets themselves match. Saved snapshots confirm
unchanged calendar metadata, element counts and all unrelated Prime/Compline
elements. Every changed office outside Prime/Compline falls June 30–July 6.
The 28-year parity calendars and commemoration-merge digests are unchanged;
only Prime's reordered declarations change decision digests.

The unchanged 2026 ordo comparator remains **3443/3626**, with 183 differences;
its incipit/appointment checks do not certify these newly reviewed ordinary
and octave slots. Source assurance has one additional verified body and zero
stale attestations. These measurements do not resolve the held terminal
II Vespers psalmody conflict or the other clergy questions in the backlog.

Validation: `make golden`, `make check` and `make test-coverage` pass. The
post-change 2026 ordo findings CSV matches the merged baseline byte-for-byte.
Independent review of the final preces data and TeX changes found no blocker.

## Weekday Lauds on lesser feasts (2026-09-16)

Lauds had treated almost every saint's office as having Sunday festal psalmody.
Diurnal pp.44,46 instead retain the weekday psalter on lesser Doubles without
proper psalm antiphons. Their weekday festal canticle is also appointed on
Simple octave days and anticipated Saturday Sundays (p.79). The 2026 ordo
explicitly confirms the weekday psalter with festal canticle on 27 affected
dates, including simple octave days (p.25), lesser Doubles (pp.29–30), and
St Sylvester within the Nativity octave (p.129). Being within another feast's
octave does not itself change this lesser Double appointment.

The hour definition now selects the weekday psalms and canticle frame. Five
missing festal canticles are supplied from pp.47–48,53–54,58–59,65,71–72,
along with their antiphons and printed Paschaltide additions. Direct page-image
readings were compared with independent Sonnet readings; two Judith details
were adjudicated by re-reading p.59. Saturday uses the existing Sirach canticle
and Psalm143 divisions, with its p.79 antiphon independently read. These 17
new text bodies have hash-bound source attestations. No source pages or reader
output are committed; evidence remains under `output/ordinary-lauds/`.

The selection rule distinguishes proper antiphons from Common fallbacks;
resolved proper aliases count as explicit appointments, so those aliases must
represent printed directions. Greater feasts, proper psalmody, ordinary ferias,
and the separate Saturday BVM office retain their previous selection. The
antiphon override is confined to composing Lauds: Prime still borrows the
Feast/Common's Lauds antiphon. No feast-ID list is added to the engine.

The Saturday Laudate group now follows the selected psalmody, rather than the
`is-feast` flag. This also repairs Pentecost vigil: its existing Ascension
psalmody was followed by antiphon4 instead of antiphon5. Diurnal pp.392,394 and
2026 ordo p.67 agree on its festal appointment.

**Two new source conflicts remain held.** The current ordo appoints Friday
Lauds on Saturday May2 (Athanasius, p.59), and Tuesday Lauds on Wednesday
December2 (Peter Chrysologus, p.121). Both feast files explicitly retain their
previous festal selection through a validated data declaration. These holds
apply in every year until clergy settle the appointment and its scope; they
are neither source attestations nor a claim that the preserved selection is
correct. The repair backlog records both questions.

A before/after comparison of **23,016 offices** across 2026, 2027 and 2032 in
all three forms finds **252 Lauds with changed elements** (84,81,87); this is
28 dates in 2026, including the vigil fix. Every one of the **19,728 other
hours is byte-identical**, as are Lauds elements from the chapter onward and
all calendar metadata. The new conditional section boundaries change decision
traces on every Lauds, including days with unchanged prayers. The unchanged
2026 ordo comparator remains **3443/3626**; its finding CSV matches the merged
baseline byte-for-byte. That comparator does not check this psalmody distinction.

The remaining Paschaltide ferial canticle-antiphon fallback is a separate
source-review target: ordinary Lauds currently gives it the shared Alleluia,
whereas the psalter prints canticle antiphons with a P.T. addition. This PR does
not certify every ordinary or seasonal antiphon. Other clergy-held repairs in
the backlog remain unresolved.

The 28-year parity calendars, commemoration-merge digests and all non-Lauds
hour digests are unchanged. `make golden`, `make check` and
`make test-coverage` validate the repair; independent adversarial review found
no blocker in the Lauds-only boundary or the explicit conflict holds.

## Paschaltide ferial and Saturday BVM psalmody (2026-09-16)

Diurnal pp.44,46,48 distinguishes the psalm groups' shared Alleluia from the
weekday canticle's own antiphon with its P.T. addition. Six ferial canticle
antiphons are now recorded from pp.46,52,57,63,69,75; direct page-image readings
agree with independent Sonnet readings. The new bodies have hash-bound source
attestations. The 2026 ordo pp.56,60–64 confirms the corresponding weekday
ferial canticles, including Rogation Monday and the Ascension vigil.

Seasonal lookup now accepts an hour-and-weekday-qualified entry before its
shared hour/generic entries. Proper and Common precedence and seasonal scope
gates remain in place. The Lauds-only weekday helper shares this lookup for
ordinary Simple offices, while lesser Doubles retain their festal weekday
canticle antiphons. Tests cover all six weekdays and prayer forms, proper and
Common precedence, explicit omission, wrong season/weekday/hour, source traces,
and the separation of the two Alleluia psalm groups around the canticle.

The Saturday BVM office also had an explicit Paschaltide fallback to Sunday
psalms and Benedicite. Diurnal pp.68*–69*,71* directs it to the Saturday psalter;
p.79 appoints the festal Sirach canticle and Psalm143 divided at verse8. The
current ordo agrees on May16 (p.63). The two fallback sections are removed;
the existing Saturday sections now apply in every season. Four seasonal
proper aliases reuse existing Alleluia and festal canticle texts. Actual
calendar and synthetic tests protect Psalm67/51/143a/143b, Sirach, the joined
Laudate group, seven Glorias including Benedictus, and the non-Paschal office.
No new feast branch or psalmody abstraction is introduced.

The **23,016-office** before/after sweep across 2026,2027,2032 and all three
forms finds **150 offices with changed elements** (42,54,54): 13 ferial dates
plus Saturday BVM on May16 in 2026, and 15 ferial plus three Saturday BVM dates
in each other sampled year. All **19,728 other hours are byte-identical**;
Lauds elements from the chapter onward and calendar metadata are unchanged.
Deleting the obsolete sections changes Lauds decision traces even on days
with unchanged prayers. The 28-year calendar, commemoration-merge and
non-Lauds parity digests are unchanged. The 2026 ordo findings CSV remains
byte-identical at **3443/3626**; that comparator does not assess these psalmody
and canticle-antiphon distinctions.

Validation: `make golden`, `make check`, `make test-coverage`, and independent
adversarial review. Evidence and frozen comparisons remain under ignored
`output/paschal-ferial/`. Existing clergy-held appointments are unchanged.

A separate Sunday appointment gap was identified while checking the boundary:
Diurnal p.370 prints a distinct Benedicite antiphon and a ninefold Alleluia
for the first psalm group on Low Sunday, whereas the current Low Sunday data
aliases every slot to the shared threefold Alleluia. The current ordo p.56
cites that appointment. Follow its use on the subsequent Easter Sundays,
check the exact texts independently, and preserve Vespers/little-hour
appointments when repairing the Lauds aliases. This is recorded in the repair
backlog and is not certified by the weekday repair.

## Paschaltide Sunday psalmody and antiphons (2026-09-16)

Low Sunday through V Sunday after Easter now follow the appointments in
Diurnal pp.34–37,370–371,377,380,382,385. Lauds has Psalms93,100,63 after
Psalm67, followed by Benedicite and the joined Laudate psalms. The first three
main psalms share a ninefold Alleluia, Benedicite has its own “Christ is risen”
antiphon, and Laudate keeps the threefold Alleluia. Previously Low Sunday used
the threefold form throughout, while II–V Sundays also retained the ordinary
Sunday Psalms51 and118 and lacked the separate canticle antiphon frame.

The Sunday little hours take a fourfold Alleluia (pp.83,371). Explicit
proper-hour aliases prevent Prime from borrowing the new Lauds ninefold form.
The 2026 ordo pp.56,58,62,64 agrees on these Sunday psalter and Paschaltide
antiphon appointments. Its separate II/V Easter chapter-reference conflicts
remain held in the backlog; no chapter is changed here.

Direct Codex page-image readings agree with an independent Sonnet reading of
the three new text bodies, including Alleluia counts and grouping. Their source
attestations are hash-bound; evidence, page identities and reader results remain
under ignored `output/paschal-sunday/`. Pointing is normalized to the corpus's
asterisk convention. These attestations verify wording, not entire offices.

The existing `lauds-psalmody` declaration now supplies the explicit Sunday
appointments. The three formerly hardcoded octave-Sunday exceptions move into
their proper files (Nativity p.206, Epiphany p.229, Ascension p.392). This removes
the engine's feast-ID list and preserves ProperID redirects, including the
Friday after Ascension's octave and Pentecost vigil. Existing clergy-held
weekday appointments retain their declarations.

Source requirement checks, reviewed by Codex with an independent adversarial
agent review:

- Lauds structure and counts: `TestPaschalSundayPsalmody` checks all five proper
  IDs in actual 2026/2027/2032 calendars and all three prayer forms, including
  Psalm67, Benedicite without a Gloria, and six antiphon frames.
- Little-hour distinction: the same test checks fourfold antiphons in
  Prime/Terce/Sext/None, their hour-specific source selection and canonical alias.
- Data selection: `TestExplicitSundayLaudsPsalmody` checks direct declarations,
  ProperID redirects, the three migrated appointments, and ordinary Sunday
  fallback when no declaration exists.
- Boundaries: the frozen 23,016-office sweep changes exactly 210 offices on
  14 dates: Lauds and four little hours, in three forms. There are four affected
  Sundays in 2026 (April19/26, May10/17); Finding of the Cross displaces III
  Sunday on May3. All five Sundays occur in both other sampled years.
  The remaining 22,806 offices are byte-identical, including every Vespers and
  Compline and all migrated octave appointments. Calendar metadata is unchanged.
  Little hours change only psalm antiphons; Lauds changes only those antiphons
  and frames and the two wrongly selected psalms. All chapter conflicts remain
  byte-identical to the baseline.

Validation includes `make golden`, `make check` and `make test-coverage`.
The unchanged 2026 ordo comparator still reports 3443/3626, with a byte-identical
finding CSV; it does not inspect these psalmody/antiphon distinctions. The
28-year parity review preserves calendar, commemoration, Vespers and Compline
digests. This closes the Paschal Sunday backlog row, without certifying other
Paschaltide appointments or resolving the clergy-held questions.

## Paschaltide ferial little-hour antiphons (2026-09-16)

Diurnal p.374 explicitly gives four Alleluias at Prime, Terce, Sext and None,
while Vespers keeps three. The 2026 ordo pp.62–64 appoints these Paschaltide
Hours on the ferias, Rogation Monday and the Ascension vigil. The app previously
used its generic threefold seasonal antiphon at each little hour.

Four hour-specific seasonal aliases now reuse the independently verified
fourfold formula added in the Sunday repair. Prime's existing seasonal window
(Monday after Low Sunday through the Ascension vigil) now selects its qualified
key. No new branch, scope rule, canonical wording or source attestation is
introduced. Proper and Common precedence, Sunday appointments and major-hour
antiphons retain their previous behavior.

Codex read p.374 directly and compared the little-hour wording with a bounded
independent Sonnet reading. The initial independent reader miscounted Vespers;
a focused 300-dpi re-reading agrees with the threefold antiphon visible on the
page. Both results and the adjudication are retained under ignored
`output/paschal-weekday/`; unsolicited contextual claims in reader notes are
not adopted as source evidence. The existing canonical formula's hash-bound
attestation is unchanged, and the aliases cite the new appointment page.

Requirement checks, reviewed by Codex and an independent adversarial agent:

- `TestPaschalFerialLittleHourAntiphons` checks all applicable default-calendar
  ferias in 2026/2027/2032, including named ferias, all four hours and all three
  forms: four Alleluias, two antiphon frames, hour-qualified selected source,
  and the canonical alias target.
- `TestPaschalFerialAntiphonHourBoundaries` exercises all six weekdays,
  threefold Vespers, and proper/Common precedence. The existing
  `TestResolvePrimePsalmAntiphon` continues to protect Prime's seasonal boundary.
- A frozen 23,016-office comparison changes exactly 516 offices on 43 dates:
  13 dates in 2026, 15 in 2027 and 15 in 2032, each at four hours in three forms.
  Changes are confined to the psalm antiphons and their resolution evidence;
  section structure, all other elements and calendar metadata are unchanged.
  All other 22,500 offices are byte-identical, including Lauds, Vespers,
  Compline, Sundays, Saturday BVM, feasts and the post-Ascension appointments.
  This preserves those boundaries without certifying their entire offices.

Validation: `make golden`, `make check`, `make test-coverage`, and adversarial
review. The 28-year calendars, commemoration digests, Lauds, Vespers and Compline
parity digests remain unchanged. The 2026 ordo finding CSV remains byte-identical
at 3443/3626; this comparator does not assess these antiphon repetition counts.
Existing clergy-held questions are unchanged.
