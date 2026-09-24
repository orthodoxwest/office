# Seasonal appointment scopes

`appointment-scopes.json` says when seasonal fallback texts are eligible. The
resolver still tries proper and common texts first; an inactive scope skips
only the seasonal tier and falls through to the ordinary (it doesn't omit the
slot). `@omit` keeps its usual meaning. Records hold configuration and source
rationale, never wording or attestations.

For example, the Paschal ferial chapters begin on Monday after Low Sunday:

```json
{
  "id": "paschal-ferial-chapters",
  "source": "Diurnal pp. 372-374: ferial chapters after Low Sunday.",
  "season": "easter",
  "hours": ["terce", "sext", "none"],
  "slots": ["chapter"],
  "require_ferial": true,
  "exclude_weekdays": ["sunday"],
  "from_easter": 8
}
```

A record applies when the season, one listed hour, and one listed slot match;
then every restriction must hold:

- `require_ferial` matches the hour-definition `is-ferial` condition: an
  unnamed day or any celebration with category feria.
- `exclude_weekdays` uses lowercase English names and the civil weekday.
- `from_easter` (inclusive) and `until_easter` (exclusive) are offsets from
  Easter of the office date's year; an absent bound is open. Weekday
  exclusions apply independently of date bounds.

With no matching record, seasonal lookup is unrestricted. Scopes don't affect
proper/common precedence, redirects, the Prime explicit-proper rule, or
Vespers selection. Current records, all at Terce, Sext, and None:

| Season and slots | Easter offset | Other restrictions |
|---|---|---|
| Lent chapters and psalm antiphons | from -42 | Ferias, excluding Sunday |
| Lent versicles | from -42 | None |
| Passiontide chapters | [-14, -3) | Ferias, excluding Sunday |
| Passiontide psalm antiphons | [-14, -7) | Ferias, excluding Sunday |
| Passiontide versicles | [-14, -3) | None |
| Easter chapters | from +8 | Ferias, excluding Sunday |

## Validation

Slot selectors are `chapter`, `versicle`, and `psalm-antiphon*` (the whole
psalm-antiphon family); hour-qualified or numbered keys aren't selectors. Each
record needs an ID, source, hours, slots, and at least one restriction; season
and hour names must be known, and each selector must have a seasonal corpus
candidate. Loading rejects unknown fields, duplicate IDs/hours/weekdays/
selectors, empty or invalid intervals, and offsets outside [-366, 366]. Only
one scope may cover a season/hour/slot family, even with disjoint dates: there
are no ordered rules or interval unions. A valid file replaces the index
atomically.

`NewEngine` requires the file (an explicit `[]` is valid) and fails if it is
missing or malformed. `LoadTexts` and `NewTestCorpus` tolerate a missing file
for small fixtures and text-only tools, so tests of scoped composition should
call `LoadAppointmentScopes` explicitly, before sharing the corpus with
concurrent composers.

Extend the data with a source-backed requirement and independent expected
appointments at its boundaries, feast collisions, and fallbacks; extend the
schema only when a requirement needs it.
