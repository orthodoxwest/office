# Prime Martyrology pilot

Prime normally ends with a rubric pointing to the Martyrology. A reader who
turns **Martyrology** on in Settings (off by default; web, Android and iOS)
instead hears the reviewed announcement for the **following civil date**, if
one exists, along with the common conclusion and response. Otherwise, and in
all CLI output, the rubric stays. The Triduum still suppresses the whole
section. Readings follow civil dates regardless of feast transfers; nothing
else in the calendar or other hours changes.

The trial covers the whole civil year except the held days below, read at
Prime on the preceding day; try `/prime/2026-09-07` with the setting on. Terse
early-layer notices with no later claim are kept without an exact chronology
(proposed answer to #476, awaiting confirmation). May 29 (Eleutherius at Arce),
July 5 (Philomena at San Severino) and July 29 (Seraphina at Mamia) stay held:
the place is no ancient see or cannot be identified. December 8, 15 and 25 await
clergy review of their announcements. Held days keep the rubric.

Leap years follow the source's bissextile rule (January–June, p. 15), as the
calendar does for the feasts: February 24 reads `02-24-bissextile`, and
February 25–29 read the printed 24th–28th. Holy Name (`holy-name`, worded as
every ordo since 2021 prints it) and the vigils (`vigil-MM-DD`, keyed by
printed date) are read in the first place on the day the calendar keeps them:
a common vigil falling on Sunday on the Saturday before, the Epiphany's on
January 5, and St Matthias's on February 24 in leap years. Only vigils the AWRV
keeps are listed. Both rules are proposed answers awaiting clergy confirmation
(#475, #477).

On the web the setting lives in the browser (`office-martyrology` in
localStorage, like the theme), so the page never varies by reader: Prime carries
the Martyrology section twice, as the rubric and as the reading
(`data-martyrology-variant`), and the root's `data-martyrology` shows one before
first paint. The offline copy therefore serves either setting. The native apps
compose Prime again when the setting changes. Programmatic use goes through
`Engine::compose_hour_with_options` with
`ComposeOptions { martyrology: true, ..Default::default() }`. The earlier
`?preview=martyrology` URL flag is retired and ignored. Making the section
collapsible is a separate decision.

The usage report's **Martyrology at Prime** breakdown (`martyrology:shown|hidden`)
counts Prime readers on days with a reading, by whether the setting showed it;
see README, "Usage metrics".

Tests cover next-day selection, year and leap-year rollover, civil time across
DST, missing-text fallback, the Triduum, and (over 2026–2053) that Holy Name and
every vigil are announced exactly on the days the calendar keeps them.

## Eligibility

The 1916 Roman Martyrology is the base text. Notices keep their traditional
dates and order; retained wording is not edited.

- Judge each person by death, never canonization, relic translation, or a
  feast's institution. Keep a notice whose whole plausible death range is at
  or before 1200 (an exact year is unnecessary: "under Diocletian" or "fourth
  century" suffices). Omit it if the range reaches past 1200. A terse notice
  from the early layers (a name at an ancient church, no later claim) counts
  as early; flag for human review only a notice that is neither.
- Identify by name, place, companions, and date together; homonyms are common.
- When a retained notice mentions a later saint, omit the smallest clause that
  still reads correctly. Never invent a revised companion count; send
  inseparable cases to review.
- Feast announcements, octaves, vigils, and translations are not people.
  Review their AWRV observance and wording separately.
  A feast, octave or devotion announced on its Roman date stays only when the
  AWRV calendar keeps that observance on that date. Vigil lines are left out
  of the day's entry; Prime adds them from `vigil-MM-DD` on the day the
  calendar keeps the vigil.
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

A full year also needs any further moveable-feast announcements (checked
against the current ordo), the special Christmas announcement, and reviewed
December 8 wording.
