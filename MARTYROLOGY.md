# Prime Martyrology pilot

Prime replaces its final Martyrology rubric only when the URL contains
`?preview=martyrology` and a reviewed reading exists for the **following civil
date**. Ordinary pages and command-line exports retain the rubric. Preview
requests are online-only, bypass the service worker cache, and carry
`Cache-Control: private, no-store` and `X-Robots-Tag: noindex, nofollow`. The
parameter does not persist in cookies or device settings; navigation to a URL
without it exits the preview. It is an unlinked review feature, not authentication. The initial readings are September 7–9, shown
at Prime on September 6–8, including September 7, 2026. Other dates retain the
existing rubric. This is a three-day trial, not a complete annual Martyrology.

The common conclusion and response must also be present, or Prime retains the
rubric. The existing Triduum condition still suppresses the entire section.
Readings remain tied to civil dates, independently of feast transfers. The
calendar, feast metadata, and other hours are unchanged.

To extend the trial:

1. Follow `scripts/DIURNAL-PIPELINE.md` for page caching and independent image
   readings. Keep raw readings and research in ignored `output/`.
2. Resolve identities by name, place, and date, review the cutoff and incidental
   later-saint references, and record exclusion reasons. Hold unresolved
   cutoff decisions for review rather than inferring dates from names.
3. Add only complete reviewed selections as `ordinary/martyrology/MM-DD`,
   using `office corpus put`, then attest the selected source wording with
   an explicit description of omissions and the document hash.
4. Check the rendered Prime for the preceding civil date and add coverage
   tests. Adding a dated corpus entry makes it available within the preview: this
   is a curated corpus, not a runtime death-year filter.

Full-year expansion still needs the separate movable-feast announcements
described in the introduction, checked against the current parish ordo, plus
the source's leap-year conventions and special Christmas announcement. The
current civil-date rollover tests exercise lookup mechanics; they do not
constitute approval of those additional liturgical rules. No such seasonal
announcement falls within this September pilot.

Preview today's trial at `/prime/2026-09-07?preview=martyrology` in the
running app. Removing the parameter returns to the ordinary Prime. CLI output
remains ordinary; programmatic review uses `ComposeHourWithOptions` with
`MartyrologyPreview: true`. The flag can be removed after clergy approval;
collapsibility is a separate future presentation decision. Unit/integration tests cover next-day selection, year and
leap-year rollover, civil time across DST, missing-text fallback, the Triduum,
and the pilot exclusions.
