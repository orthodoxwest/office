# Prime Martyrology pilot

Prime normally ends with a rubric pointing to the Martyrology. With
`?preview=martyrology` in the URL, it instead reads the reviewed announcement
for the **following civil date**, if one exists along with the common
conclusion and response. Otherwise, and in all CLI output, the rubric stays.
The Triduum still suppresses the whole section. Readings follow civil dates
regardless of feast transfers; nothing else in the calendar or other hours
changes.

The trial covers September 7–9, read at Prime on September 6–8; try
`/prime/2026-09-07?preview=martyrology`. It is an unlinked review feature, not
authentication: the parameter isn't persisted, previews bypass the service
worker and send `Cache-Control: private, no-store` and
`X-Robots-Tag: noindex, nofollow`. Programmatic review uses
`ComposeHourWithOptions` with `MartyrologyPreview: true`. The flag can go once
clergy approve; making the section collapsible is a separate decision.

Tests cover next-day selection, year and leap-year rollover, civil time across
DST, missing-text fallback, the Triduum, and the pilot exclusions.

## Adding dates

1. Cache pages and take independent image readings per
   [scripts/DIURNAL-PIPELINE.md](scripts/DIURNAL-PIPELINE.md); keep raw
   readings in ignored `output/`.
2. Identify each saint by name, place, and date; review the cutoff and
   incidental later-saint references, and record exclusion reasons. Hold
   unresolved cutoff decisions rather than inferring dates from names.
3. Add only complete reviewed selections as `ordinary/martyrology/MM-DD` with
   `office corpus put`, and attest the source wording with the document hash
   and an explicit note of omissions.
4. Check Prime on the preceding civil date and add tests. Any dated entry
   becomes live in the preview; the corpus is curated, not filtered at runtime.

A full year also needs the moveable-feast announcements (checked against the
current ordo), the source's leap-year conventions, and the special Christmas
announcement. The rollover tests exercise lookup only, not those rules.
