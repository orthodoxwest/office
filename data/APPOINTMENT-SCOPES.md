# Seasonal appointment scopes

`appointment-scopes.json` describes when seasonal fallback texts are eligible.
The resolver still tries proper and common appointments first. An inactive
scope skips the seasonal tier and continues to the ordinary fallback; it does
not omit the slot. An active `@omit` retains its existing omission semantics.
These records contain configuration and source rationales, not corpus wording
or source attestations.

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

The season, one listed hour, and one listed slot must match; all restrictions
then apply. `require_ferial` uses
the same meaning as the hour-definition `is-ferial` condition: an unnamed day
or a celebration whose category is feria, including named and synthesized
ferias. Weekdays use lowercase English names and the composer's civil weekday.
Date bounds use the office date and Easter in that date's year:
`from_easter` is inclusive, `until_easter` exclusive, and an absent bound is
open. Sunday exclusions apply independently of date bounds.

The initial records preserve these previously implemented restrictions at
Terce, Sext, and None:

| Season and slots | Easter-relative interval | Additional restrictions |
|---|---|---|
| Lent chapters and psalm antiphons | from -42 | Ferias, excluding Sunday |
| Lent versicles | from -42 | None |
| Passiontide chapters | [-14, -3) | Ferias, excluding Sunday |
| Passiontide psalm antiphons | [-14, -7) | Ferias, excluding Sunday |
| Passiontide versicles | [-14, -3) | None |
| Easter chapters | from +8 | Ferias, excluding Sunday |

The season selector also bounds every rule. No matching record means the
seasonal lookup is unrestricted. The data does not alter proper/common
precedence, redirects, the Prime explicit-proper rule, or Vespers selection.

## Validation and extension

The first schema supports only the migrated base-slot selectors `chapter`,
`versicle`, and `psalm-antiphon*`. The last selects the psalm-antiphon family;
hour-qualified and numbered corpus keys are not selectors. Season and hour
names must be known, and each selector must have a seasonal corpus candidate.
IDs, source rationales, hours, slots, and at least one restriction are required.

Loading rejects unknown fields, duplicate IDs/hours/weekdays/selectors, invalid
or empty date intervals, and offsets outside [-366, 366]. There is at most one
scope for each season/hour/slot family. Overlapping selectors are rejected even
when their date intervals are disjoint; v1 has no ordered rules or unions of
intervals. A complete valid file replaces the scope index atomically without
changing text, aliases, or omissions.

Production `NewEngine` requires the sidecar and fails on missing or malformed
configuration. An explicit `[]` is valid. Raw `LoadTexts` and `NewTestCorpus`
allow missing scope metadata for small corpus fixtures and text-only tools;
tests of scoped composition must load it explicitly with `LoadAppointmentScopes`.
This setup method must run before sharing a corpus with concurrent composers.

Extend the data with a source-backed requirement and independent expected
appointments at its boundaries, feast collisions, and fallback cases. Extend
the schema only when those requirements need additional semantics. Existing
output parity demonstrates preservation, not independent liturgical accuracy.
