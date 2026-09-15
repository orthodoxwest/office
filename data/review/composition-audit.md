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

These checks do not certify all Triduum wording, Tenebrae ceremonies, or the
Holy Saturday Vigil. In particular, the current Holy Saturday Vespers route
still lacks the abbreviated Vigil office explicitly appointed by the ordo.
That separate repair remains in `repair-backlog.csv`; the boundary tests above
preserve the existing route rather than certify its complete appointment.

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
[#361](https://github.com/orthodoxwest/office/issues/361). Those remain separate.
Ordinary Sunday Vespers' 2 Corinthians chapter agrees with Diurnal p. 114;
ordinary numbered Sunday propers inspected on pp. 232 and 437 do not print
replacement chapters. These negative findings are limited to those pages,
not clearance of every ordinary fallback in the inventory.
