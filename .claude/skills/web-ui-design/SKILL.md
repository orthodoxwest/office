---
name: web-ui-design
description: >-
  Design principles and implementation guidance for the Daily Office web UI
  (layout, typography, themes, mobile prayer chrome, CSS tokens). Use when
  changing templates, style.css, app.js, PWA chrome, or visual UX — or when
  the user mentions design, beauty, reverence, light/dark, nave/apse, mobile
  layout, scroll progress, or runs /web-ui-design.
---

# Daily Office web UI design

Project skill for the AWRV Benedictine Office web app (`internal/web/`).
Read this before changing visual design or prayer-page chrome.

## Product posture

| Fact | Implication |
|------|-------------|
| Audience: lay people who know the office roughly | Compose for them; do not surface rubric engineering mid-prayer |
| Job: scroll assembled prayers without multi-book stress | **Prayer text is the product** |
| ~70–80% mobile | Design first at ~390px; desktop is a framed gift |
| High bar for beauty and reverence | Quiet materiality, not empty flatness or kitsch |

### Principles

1. **Prayer text is the product** — chrome shrinks once an hour is open.
2. **Beauty is quiet, not empty** — surfaces need material (tokens, hairlines, drop caps), not more widgets.
3. **Mobile is primary** — home CTA and first prayer win the first screen.
4. **Lay, not rubrics engineer** — Ordo/assurance are secondary rooms; assurance stays post-hour (do not bury for maintainers-only).
5. **Reverence > delight** — stillness over animation, streaks, or SaaS chrome.
6. **Parish as palette, not wallpaper** — lime wash, oak, gold, sage, terracotta, apse night sky → **CSS tokens + few ornaments**, never photos of the nave.

### Explicit non-goals

- Gamification, social, confetti, “Catholic app store” illustration
- Autoplay chant
- Stretching prose to full desktop width (keep ~38rem reading measure on `.elements`)
- Sticky bars that permanently eat phone viewport to “show more chrome”
- Query-param theme as the primary persistence mechanism (breaks offline/SW cache)

## Parish palette (implement in CSS variables)

| Mode | Story | Key tokens (approximate) |
|------|--------|---------------------------|
| **Light / Nave** | Rose plaster, oak, gold leaf | `--bg` warm plaster, `--accent` oak, `--gold` / `--gold-line`, `--surface` matte card |
| **Dark / Apse** | Night sky over the altar | Cooler **blue** night (`#121c28` family), not forest teal; gold carries hierarchy |

### Motif vocabulary (reuse; do not invent freely)

| Motif | Use |
|-------|-----|
| ✠ | Brand |
| Double gold hairline | Hour titles, major breaks |
| ✦ diamond | Footer / major separators |
| Liturgical color band | Top of hour pages (+ safe-area) |
| Gold drop caps | Psalm/chapter openings |
| Gold scroll progress hairline | Under color band on hour pages only |
| Sparse stars | Only if proven; prefer **not** on mobile margins |

Avoid: grain overlays in shipping PRs without a prototype, heavy wood textures, fitness-style progress rings, sun/moon icon toggles that read as SaaS.

## Page roles

| Surface | Role |
|---------|------|
| Home | Today + **Pray {hour}** primary CTA |
| Hour | The prayer itself; date-nav secondary (“Change date”) |
| Ordo | Advanced day inspection |
| Reminders | Habit / ICS feed |
| Assurance + report | After the hour, not mid-scroll |
| Under-construction banner | Temporary; may be unpersisted while collecting feedback |

## Implementation map

| Concern | Location |
|---------|----------|
| Tokens, layout, print | `internal/web/static/style.css` |
| Templates | `internal/web/templates/` (`layout`, `home`, `hour`, …) |
| Client behavior | `internal/web/static/app.js` |
| PWA / offline | `internal/web/static/sw.js`, `manifest.webmanifest` |
| Embedded assets | `//go:embed` in `internal/web/server.go` — **rebuild** after template/CSS/JS changes |

### Theme (Nave / Apse / System)

- **Preferred persistence:** `localStorage` key `office-theme` (`system` \| `light` \| `dark`).
- **FOUC prevention:** tiny **inline** pre-paint script in `layout.html` `<head>` before CSS.
- **`data-theme`** on `<html>`: set for `light`/`dark`; **remove** for system so `prefers-color-scheme` media rules apply.
- **Do not** put theme in every link URL for the new control. Legacy `?theme=` may still work for bookmarks; migrate into localStorage on first visit when sensible.
- Service worker precaches **unthemed** URLs — theme in query params multiplies cache keys and offline can flip theme. Keep appearance client-side.
- Labels: **System / Nave / Apse** (words), not sun/moon icons.
- **Placement:** footer on home / ordo / reminders (and empty-page errors) — **never** in the hour list nav, **never** on hour pages (prayer chrome stays lean).

### Hour pages

- Keep liturgical color band + optional gold scroll progress under it.
- Demote day switching (collapsed “Change date”); do not add sticky title chrome without measuring mobile pixels.
- **Wake Lock:** default on for `.office-hour` only; never home/ordo/reminders; graceful no-op if unsupported.
- Session prayers: collapsible, styled as **section headings**, not settings cards.
- Print: hide nav, progress, date-nav, banners; expand session prayers.

### Home

- Prayer card / **Pray now** leads (especially mobile).
- Preserve selectors used by `updatePrayNow()`: `.home-prayer-card[data-date-slug]`, `.pray-now`, `.home-hour-link[data-hour]`, `.home-hour-link-name`.

### Accessibility

- Keep `:focus-visible` rings; do not rely on gold alone for meaning.
- Re-check muted/secret-text contrast after any apse/token shift.
- Honor `prefers-reduced-motion` for transitions (progress bar, etc.).
- Touch targets ~2.75rem where interactive on mobile.

## Workflow when changing UI

1. Confirm the change serves **prayer focus** or **quiet beauty** — cut chrome that does not.
2. Touch the smallest surface: tokens/CSS first; DOM only if hierarchy requires it.
3. Test light + dark + narrow (≤540px) + one hour page + home.
4. `go test ./internal/web/`; rebuild (`make build`) if templates/static are embedded.
5. Golden files are composition output, not HTML — only regenerate when office text changes.
6. PR against `master` (no direct push); describe visual intent in the PR body.

## Anti-patterns (reject or prototype first)

| Idea | Why |
|------|-----|
| Sticky collapsing hour header | New permanent chrome; fights “chrome shrinks” |
| Always-on wake lock site-wide | Drains battery on ordo browsing |
| Theme only via `?theme=` | Offline/SW/cache and PWA `start_url` issues |
| Fixed decorative stars over body text | Collides with prayer on ~390px |
| Assurance moved to `?debug=` | Owner wants post-hour transparency for laity too |

## Related docs

- Architecture and commands: `CLAUDE.md`
- Ordo/rubrics verification: skill `ordo-verify`
- Human review process: `REVIEWING.md`
