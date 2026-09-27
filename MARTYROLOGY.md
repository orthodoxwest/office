# Prime Martyrology pilot

Prime normally ends with a rubric pointing to the Martyrology. With
`?preview=martyrology` in the URL, it instead reads the reviewed announcement
for the **following civil date**, if one exists along with the common
conclusion and response. Otherwise, and in all CLI output, the rubric stays.
The Triduum still suppresses the whole section. Readings follow civil dates
regardless of feast transfers; nothing else in the calendar or other hours
changes.

The trial covers September 7–9 and September 28–December 31, read at Prime
on the preceding day; try `/prime/2026-09-07?preview=martyrology`. Within the
autumn batch, October 8 and 12, November 6, 10 and 30, and December 2 hold a
notice with no usable chronology; December 8, 15 and 25 await clergy review of
their announcements. Held days keep the rubric. It is an unlinked review feature, not
authentication: the parameter isn't persisted, previews bypass the service
worker and send `Cache-Control: private, no-store` and
`X-Robots-Tag: noindex, nofollow`. Programmatic review uses
`Engine::compose_hour_with_options` with
`ComposeOptions { martyrology_preview: true, ..Default::default() }`. The flag can go once
clergy approve; making the section collapsible is a separate decision.

Tests cover next-day selection, year and leap-year rollover, civil time across
DST, missing-text fallback, the Triduum, autumn coverage with its held days,
and omitted post-1200 notices.

## Eligibility

The 1916 Roman Martyrology is the base text. Notices keep their traditional
dates and order; retained wording is not edited.

- Judge each person by death, never canonization, relic translation, or a
  feast's institution. Keep a notice whose whole plausible death range is at
  or before 1200 (an exact year is unnecessary: "under Diocletian" or "fourth
  century" suffices). Omit it if the range reaches past 1200. Flag it for
  human review only when there is no usable chronology at all.
- Identify by name, place, companions, and date together; homonyms are common.
- When a retained notice mentions a later saint, omit the smallest clause that
  still reads correctly. Never invent a revised companion count; send
  inseparable cases to review.
- Feast announcements, octaves, vigils, and translations are not people.
  Review their AWRV observance and wording separately.
- Absence from the AWRV office calendar is no reason to omit an eligible
  notice, and presence in it is no exception to the cutoff.

## Adding dates

1. Cache pages and take independent image readings per
   [scripts/DIURNAL-PIPELINE.md](scripts/DIURNAL-PIPELINE.md); keep raw
   readings in ignored `output/`.
2. Record a keep/omit/flag decision per notice with its cited death date or
   range, in ignored `output/`.
3. Add only days with no flagged notice as `ordinary/martyrology/MM-DD`, and
   attest the source wording with the document hash and an explicit note of
   omissions. A partly decided day stays out of the corpus.
4. Check Prime on the preceding civil date and add tests. Any dated entry
   becomes live in the preview; the corpus is curated, not filtered at runtime.

A full year also needs the moveable-feast announcements (checked against the
current ordo), the source's leap-year conventions, the special Christmas
announcement, and reviewed December 8 wording. The rollover tests exercise
lookup only, not those rules.
