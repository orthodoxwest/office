# Public API

A read-only JSON API for other sites and apps that want to show the Office in their own
layout. It is served by the web app on every host, needs no key, and any origin may read it
from a browser (`Access-Control-Allow-Origin: *`). Replies may be cached for an hour.

## Endpoints

| Path | Returns |
| --- | --- |
| `GET /api/v1/days/YYYY-MM-DD` | One day |
| `GET /api/v1/calendar/YYYY/MM` | `{ "year", "month", "days": [day, …] }`, every day of the month |

Dates are civil dates: the API has no notion of "today", because that depends on the reader's
time zone, so the caller supplies the date. Errors are `{"error": "…"}` with a 4xx or 5xx status.

## A day

```json
{
  "date": "2026-10-09",
  "weekday": "Friday",
  "name": "Feria",
  "rank": "", "rank_name": "",
  "color": "green",
  "fast": false, "abstinence": true,
  "commemorations": ["Ss Denys, Rusticus & Eleutherius, Martyrs"],
  "monastic": [],
  "lauds": {
    "gospel_antiphon": "Through the tender mercy",
    "preces": false, "suffrage": true,
    "commemorations": [{ "name": "Ss Denys, Rusticus & Eleutherius, Martyrs", "incipit": "The very hairs of your head" }]
  },
  "vespers": {
    "gospel_antiphon": "O blessed Mother",
    "preces": false, "suffrage": true,
    "commemorations": [],
    "note": "I Vespers of Saturday Office of the B.V.M."
  },
  "hours_preces": true,
  "links": { "calendar": "https://…/calendar/2026/10#d-2026-10-09", "lauds": "https://…/lauds/2026-10-09", "…": "…" }
}
```

- `rank` is the ordo's abbreviation and `rank_name` its full name; both are empty on a plain feria.
- `color` is one of `white`, `red`, `green`, `violet`, `rose`, `black`.
- `monastic` lists observances the ordo brackets "(Monastics & Oblates Only)":
  `{ "name", "rank", "rank_name", "office" }`.
- `lauds` and `vespers` digest the two major hours as the ordo does. `gospel_antiphon` is the
  Benedictus or Magnificat antiphon's incipit. Vespers' `note` (omitted when empty) says when
  the evening belongs to the next day's office.
- `links` are absolute URLs on this site for the day's ordo row and each hour.

## Stability

Within `v1` a field keeps its name, type and meaning, and no field is removed. New fields and
new resources may be added, so clients should ignore what they don't know. A change that would
break a client is published as `v2`, alongside `v1`.

## Room to grow

Not built yet, but the layout leaves room for them. Anything about a day hangs beneath
`/api/v1/days/{date}`:

- `GET /api/v1/days/{date}/collect`: the day's collect, as text, with the office it belongs to.
- `GET /api/v1/days/{date}/hours/{hour}`: a composed hour as a list of typed elements (heading,
  antiphon, psalm, collect, …), the same document the site and the native apps render.
  `?form=private|deacon|priest` as on the site.
- `GET /api/v1/calendar/YYYY`: the year's moveable feasts, Ember days and computus figures (the
  ordo's Tabula Temporaria).

Serving texts brings questions the calendar does not: the licensing of each translation, and
whether to serve texts not yet checked against the printed diurnal. Those get settled before a
text endpoint ships.
