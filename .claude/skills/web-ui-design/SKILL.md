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
| ✦ diamond | Footer / major separators; the raised points either side of an inscription |
| Inscription band | Dark oak course, full-bleed, gold small-caps between ✦ points. Section heads in a framed object — the parish's most distinctive mark, so spend it rarely |
| Liturgical color band | Top of hour pages (+ safe-area) |
| Gold drop caps | Psalm/chapter openings |
| Gold scroll progress hairline | Under color band on hour pages only |
| Gold underline (`inset 0 -1px 0`) | **The** marker for "this is the chosen one" — current hour, selected control. Never a filled cell |
| Gold caret `▾` / `▴` | **The** disclosure marker, everywhere. Never the native `▶`, never `+` / `−` |
| Apse starfield | **Desktop Apse home only** — the page field around the frontispiece, never the phone, never a prayer page, never Nave. See below |

Two weights of line, and they mean different things: **oak** (`--oak`, near-charcoal warm brown) is structure — the header beam, an inscription course. **Pale tan hairlines** (`--border`, `--surface-edge`) are surfaces and separators. The building is emphatically structural; if a page feels boneless, it is usually missing oak, not missing more hairlines.

Avoid: grain overlays in shipping PRs without a prototype, heavy wood textures, fitness-style progress rings, sun/moon icon toggles that read as SaaS.

### The Apse starfield (settled — do not relitigate)

The half-dome over the altar is a slate vault of gold stars — the one place in
the building where ornament sits on **open field** rather than on structure.
The app's version is **geometric, not naturalistic**: four-pointed stars on a
quincunx lattice with fainter points between, as painted vaults do
(Sainte-Chapelle, Giotto's Scrovegni, Salisbury). Ordered geometry also matches
what the rest of the app is made of — courses, aligned frames, banded groups.

- **Draw it as one repeating cell.** A quincunx is periodic, so two stars and
  two points tile the whole field in eight gradients. Hand-placing every star
  cost 172 background layers and pushed first contentful paint from 88ms to
  **696ms** — six hundred milliseconds of blank screen for decoration. A fixed
  px tile also holds its geometry at any width, where percentage positions
  stretch. Keep it a tile.
- **Size it from the parish**, which sets its stars at ~0.7% of the vault's
  width and packs them densely. A first pass at 2–4× smaller was not subtle,
  it was invisible: measure lit pixels, do not judge from a downscaled
  screenshot.
- **Crossed elliptical gradients, not an SVG data URI.** An SVG cannot read
  `var(--ornament)`, so it would silently break the seasonal veil.
- **Declare the field on `body`, not `:root`** — twice-bitten. A `none` default
  on `body` beats an inherited `:root` value outright (the vault never
  appears), and a custom property's `var()` resolves against the element it is
  declared on, while the seasonal `--ornament` lands on `body`.
- **Guard with `prefers-color-scheme: dark`.** `:root:not([data-theme="light"])`
  matches when no choice is stored, so without it a Default-theme reader on a
  light device gets gold stars across the plaster.
- **The field is desktop Apse home only; the phone gets a course instead.**
  Three mobile treatments were built and rejected — a page-level field is
  occluded by the card, stars on the card become specks on paper, and a
  *border* in the 16px side gutters reads as debris pinned against the frame.
  What works is a single course of stars in the open ground **below** the card.
  Position it **out of flow**: laid out in flow it pushes an 844px phone past
  its viewport and makes home scroll for an ornament. Gate it on the theme
  rather than on `--apse-vault`, since an empty block still takes space in
  Nave, and on a `min-height` — the gap it sits in comes from `main`'s
  `min-height` and narrows to ~40px on a short phone, where the course would
  land on the footer.
- **Let the field reach the edges.** A mask window that opens and closes inside
  the viewport leaves the stars as a band across the middle with bare ground
  above and below, which reads as a mistake rather than restraint. Clear the
  header at the top, thin toward the footer at the foot.
- Static. A twinkling vault is the opposite of stillness.

### Gilding vs functional gold

Two gold families, and picking the wrong one is silent — it only shows up in Passiontide.

| Family | Tokens | Moves with the season? | For |
|--------|--------|------------------------|-----|
| **Gilding** | `--ornament`, `--ornament-line`, `--ornament-hi/lo` | **Yes** — veils grey-violet in Passiontide, warms in Paschaltide | Ornament: ✠, ✦, drop caps, hour-title hairlines, inscription ink, the frontispiece invitation's border |
| **Functional** | `--gold`, `--gold-line` | No | Chrome that happens to be gold: disclosure carets, ordo rank hairlines, today's ordo row |

Ask "is this ornament, or is it a control that happens to be gold?" Ornament veils; controls do not. Never write a literal gold hex into a rule — that is how something ends up gold on Good Friday.

**Gold on a dark ground takes the Apse leaf colours in *both* themes.** An inscription band's course is dark in Nave and Apse alike, so its ink needs one seasonal pair, not two, and the dark token block should not restate it. Check contrast against both grounds.

Verify by probing, not by reading tokens: load an ordinary day, a Passiontide day and a Paschaltide day and compare computed styles. `body` carries `season-passiontide` / `season-eastertide` (server-stamped per date, so it is service-worker safe).

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
| Templates | `internal/render/templates/` (`layout`, `home`, `hour`, …) |
| Client behavior | `internal/web/static/app.js` |
| PWA / offline | `internal/web/static/sw.js`, `manifest.webmanifest` |
| Template rendering | `internal/render/` (view models, FuncMap, text-to-HTML) |
| Embedded assets | `//go:embed` in `internal/render/render.go` (templates) and `internal/web/server.go` (static) — **rebuild** after template/CSS/JS changes |

### Page frame (do not move the measure back onto `body`)

The reading measure lives on `main` and `footer` (46rem; the ordo widens its
`main` only). `body` is full width. The header is deliberately outside that
column: `.site-header` is full-bleed so its hairline runs wall to wall like the
timber it echoes, and `.site-nav-shell` holds its own wider max-width so the
nav has **one geometry on every page**.

- Putting the measure on `body` again wraps the nav — the brand plus nine links
  do not fit a 46rem column — and, because the ordo widens, wraps it on *some*
  pages only, so the header changes shape as you navigate.
- Gutters come from `--page-gutter` (narrowed once at ≤540px), read by header,
  `main` and `footer` alike. Do not hard-code side padding on `body`.
- `.site-nav-shell` is the positioning ancestor for the mobile menu panel;
  offset the panel by the gutter, not `right: 0`, or it sits on the screen edge.
- Print resets `main`/`footer` alongside `body`.

### Page-scale material (washes)

- **Lay the field once — never tile it.** The ordo runs tens of thousands of
  pixels on a phone; any repeating tile becomes a visible periodic stripe. Use
  `no-repeat` over the flat `--bg`.
- **Size it to the viewport** (`100%` width), not a fixed `rem` tile. A `body`
  background propagates to the canvas and is positioned against the *root* box,
  so a tile wider than the screen shows only its middle slice — and which slice
  shifts with device width, so what you tuned is not what ships.
- Non-liturgical rooms only. Prayer pages stay a flat diurnal field.

### Theme (Default / Nave / Apse)

- **Persistence:** `localStorage` key `office-theme` (`default` \| `light` \| `dark`). Write **only** on explicit control click — never invent a choice on passive load.
- **FOUC prevention:** tiny **inline** pre-paint script in `layout.html` `<head>` before CSS.
- **`data-theme`** on `<html>`: set for `light`/`dark`; **remove** for default so `prefers-color-scheme` media rules apply.
- **No URL theme:** do not stamp `?theme=` on nav or content links. Ignore any query param if present. Appearance is client-side only.
- Service worker precaches unthemed URLs; keep it that way.
- Labels: **Default / Nave / Apse** (words), not sun/moon icons. Use `title` tooltips for plain-language meaning (device setting / light plaster / night sky).
- **Placement:** footer on **all** pages (including hours) — quiet, below the fold; **never** in the primary hour nav.

### Hour pages

- Keep liturgical color band + optional gold scroll progress under it.
- Demote day switching (collapsed “Change date”); do not add sticky title chrome without measuring mobile pixels.
- **Wake Lock:** default on for `.office-hour` only; never home/ordo/reminders; graceful no-op if unsupported.
- Session prayers: collapsible, styled as **section headings**, not settings cards.
- Print: hide nav, progress, date-nav, banners; expand session prayers.

### Home

- Prayer card / **Pray now** leads (especially mobile). Nothing else in the
  frontispiece may out-shout it — keep an inscription band thin, keep markers quiet.
- Preserve selectors used by `updatePrayNow()`: `.home-prayer-card[data-date-slug]`, `.pray-now`, `.home-hour-link[data-hour]`, `.home-hour-link-name`.
- **The current-hour marker exists twice** — server-rendered in `home.html` and
  rebuilt client-side by `setHourCurrent()` in `app.js`. Change both, or the
  marker silently reverts once JS runs. (Same trap for anything app.js recreates.)
- The hour directory is **banded, not gridded**: the periods hold 2, 3 and 2
  hours, so rules between hours imply columns that cannot align across rows and
  it reads as a mis-set table. Horizontal band separators only.
- Period labels are a fixed index column — size it to the longest word
  (“Morning”) with real breathing room, not to within a pixel of its box.

### Ordo

- A working room, but still the parish's: it gets the same carets, hairlines
  and underlines as everywhere else, not browser defaults.
- Keep the table near the content. Most days have no fast/abstinence/rank
  value, so extra width opens a bare gutter with three lonely flags in it.
- The office digest is prose — cap it at the reading measure even though the
  table around it is wider.
- Never `text-decoration: underline dotted` on abbreviations: it is the
  browser's spell-check idiom and makes `2cl` / `sd` read as flagged typos.

### Accessibility

- Keep `:focus-visible` rings; do not rely on gold alone for meaning.
- Re-check muted/secret-text contrast after any apse/token shift.
- Honor `prefers-reduced-motion` for transitions (progress bar, etc.).
- Touch targets ~2.75rem where interactive on mobile.

## Workflow when changing UI

1. Confirm the change serves **prayer focus** or **quiet beauty** — cut chrome that does not.
2. Touch the smallest surface: tokens/CSS first; DOM only if hierarchy requires it.
3. Test light + dark + narrow (≤540px) + one hour page + home.
4. `go test ./internal/render/ ./internal/web/`; rebuild (`make build`) if templates/static are embedded.
5. Golden files are composition output, not HTML — only regenerate when office text changes.
6. `make test-ux` (Playwright). It carries axe baselines and structural
   assertions; a test asserting the thing you just deliberately removed should
   be **updated to the new intent with the reasoning inline**, not deleted.
7. PR against `master` (no direct push); describe visual intent in the PR body.

### Measure it; do not eyeball it

Screenshots show that something is off. Driving the page tells you what and why,
and the answer is usually a number worth putting in the commit message.

- Serve locally and drive with the Playwright browser already in `.web-tools/`:
  read `getComputedStyle`, `getBoundingClientRect`, `scrollHeight`.
- **Kill the old server before re-measuring.** CSS and templates are `go:embed`ed,
  and Playwright reuses an existing listener, so a stale process happily serves
  the previous build and you will "verify" a change that never applied.
- Sweep a real width ladder (≈320 → 1920), not one phone and one desktop.
  Assert no horizontal overflow at each.
- Checks that repay the effort: does the nav wrap at any width? do elements that
  should align actually share an edge? does a label overflow its box? does a
  background tile repeat over the page's true height? does a gold accent move
  across the three seasons?
- **Decoration has a paint cost — measure it.** Compare median first
  contentful paint with the ornament present and absent, and again under CPU
  throttling. Many CSS gradient layers are cheap to write and expensive to
  rasterize; 172 of them cost 600ms of blank screen. Anything above ~30ms of
  added FCP wants a cheaper form.
- **Count lit pixels, not declared layers.** CSS that parses is not CSS that
  renders: sub-pixel radii and low alpha can leave an effect entirely
  invisible while every layer is present in the computed style.

## Anti-patterns (reject or prototype first)

| Idea | Why |
|------|-----|
| Sticky collapsing hour header | New permanent chrome; fights “chrome shrinks” |
| Always-on wake lock site-wide | Drains battery on ordo browsing |
| Stamping `?theme=` on every link | Offline/SW cache-key bloat; use localStorage only |
| Fixed decorative stars over body text | Collides with prayer on ~390px |
| Assurance moved to `?debug=` | Owner wants post-hour transparency for laity too |
| Reading measure back on `body` | Wraps the nav, and wraps it on some pages only |
| Repeating background tile | Periodic stripe down the ordo's tens of thousands of pixels |
| Native `▶` / `+` / `−` disclosure marks | The app has one caret, in gold |
| Filled cell for a selected control | Selection is a gold underline; a fill reads as a settings panel |
| A literal gold hex in a rule | Bypasses the seasons — it will still be gold on Good Friday |

## Related docs

- Architecture and commands: `CLAUDE.md`
- Ordo/rubrics verification: skill `ordo-verify`
- Human review process: `REVIEWING.md`
