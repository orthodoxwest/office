---
name: web-ui-design
description: >-
  Design and implement the Daily Office web UI: layout, typography, themes,
  mobile prayer chrome, and administrative pages. Use for visual changes to
  templates, CSS, browser behavior, or PWA presentation.
---

# Daily Office web UI

## Design intent

Prayer is the product. Most readers are lay people on phones: keep the text
central, navigation quiet, and administrative material after the office.
Use the parish's material palette—warm plaster, oak, gold, and a blue night
sky—with EB Garamond, restrained rules, and generous reading space. Ornament
is English wall painting on that limewash: earth pigments drawn thinly on a
pale ground, painted into the wall rather than laid on it. Avoid playful
rewards, motion for its own sake, and decorative layers behind prayer.

Administrative pages should belong to the same app, but may use denser tables,
charts, and more direct language. They need not imitate a liturgical page.
Follow the user's requested scope; this skill does not authorize deployment
or impose a redesign on an unrelated change.

## Shared visual language

- Use existing tokens in `apps/office-web/static/style.css`, not copied hex values.
  Nave is warm plaster; Apse is cool blue, not forest green.
- `--accent` / `--oak` supply structure; `--border` / `--surface-edge` separate
  surfaces. Use the existing surface tokens for panels and tables.
- Functional gold (`--gold`, `--gold-line`) marks controls and selections.
  Ornament (`--ornament`, `--ornament-line`, `--ornament-hi/lo`) changes with
  Passiontide/Paschaltide. Decide which job a colour serves before choosing it.
  Dark inscription backgrounds use the seasonal Apse leaf colours in both themes.
- Painted ornament is one painter's logic: terracotta `--lining` for rules,
  linings and crosses (opaque, since thinned crimson goes rose on this
  plaster, and it does not veil with the season); `--titulus` for tituli
  (section headings, psalm numbers, "Ant."), red ochre on Nave and gilt on
  Apse, clear of the rubrics' red; gold for initials, lettering and the hour
  glyphs, never hardware on a rule. Shapes are masks in `static/ornaments/`
  (`tools/genornaments.py`), so colours stay tokens. The consecration cross
  ends each hour, crowns home and is the brand mark; Lauds' headpiece carries
  the sun and Vespers' and Compline's the moon. One mark per threshold.
- Current controls use a gold underline (the header's current page takes the
  lining's terracotta); disclosure carets take their label's ink at 70%.
  Keep Default / Nave / Apse labels and visible keyboard focus.
- The starfield belongs to Apse home (on a phone, the apse sky inside the
  head instead: see the painted page below), the post-office epilogue, and, on
  screens 1680px and wider, the margins beside an hour's prayer, which run
  down into the epilogue's field. By day the same home and epilogue fields
  carry the Nave's powdering instead: one six-petal rosette to each cell
  of a quincunx lattice (`ornaments/powder.svg`, 96×84px cells, alternate
  rows set half a cell over), in the rubrics' red at 12% (`--wall-field`,
  `--wall-ink`), cut square under the beam rather than faded, with no
  leaf; a scattered powdering in three sizes read as confetti. On the
  hours it shows only below the hour navigation. The wide hours' margins
  stay plain by day.
  For changes to those fields or their masks, read
  [references/apse-vault.md](references/apse-vault.md).

## Layout and behavior invariants

- `body` is full width. The reading measure belongs to `main` and `footer`
  (normally 46rem), and prayer text to `.elements` (about 38rem). Working pages
  may widen their own main. Keep the shared header geometry independent.
- Use `--page-gutter`. The mobile menu is positioned relative to
  `.site-nav-shell`; its inset follows the gutter. Controls need comfortable
  touch targets (about 2.75rem) without consuming the prayer viewport.
- Theme is client-side: `office-theme` in localStorage, `data-theme` on html,
  and the pre-paint script in `layout.html`. Persist only explicit choices.
  No theme query strings; keep service-worker page keys unthemed.
- Static files and templates are embedded. Rebuild/restart before measuring;
  Playwright can otherwise reuse a stale listener.
- Home's Pray-now selectors (`.home-prayer-card[data-date-slug]`, `.pray-now`,
  `.home-hour-link[data-hour]`, `.home-hour-link-name`) are used by app.js.
  Current-hour markup exists in both the template and `setHourCurrent()`;
  keep both consistent. The hour directory has horizontal bands, not columns
  across the unequal 2/3/2 groups. Nothing is appended after the groups
  that a `:first-child`/`:last-child` rule would miss: the ruled layer is an
  SVG, so the group rules use `:first-of-type`/`:last-of-type`.
- On a phone home's frontispiece is a painted panel with a pointed head:
  a steep four-centred arch rising 0.44 of the card's width, its haunches
  leaving the jambs without a kink. The head is drawn by
  `tools/genarch.py`, which writes clip-path polygons into style.css's
  `genarch` block, and the same shapes' arcs into the native apps'
  `Arch.kt` and `Arch.swift`; never edit them by hand. A box-shadow or border-radius
  cannot follow a pointed head, so each course is its own layer clipped
  to the same arch offset by its own distance (`--arch-d`): the day's
  ring (`.home-arch::before`, the halo's source), the frame
  (`.home-hero::after`), the panel (`.home-arch-fill`) with the shade its
  head casts (`.home-arch-shade`, the wall outside the arch drawn only for
  its drop-shadow), and the lining's band and hairline
  (`.home-lining::before/::after`). The card is an inline-size container
  so the layers can scale the head by its width (`--arch-w`). The day's
  colour is the 1.5px ring (mixed 30% toward the frame, so a red or
  green day edges the head without outshouting the cross) and the
  lining's inner hairline
  (`--lining-day`; a white day takes `--gold-line` by day); the terracotta
  lining runs round the head `--panel-inset` inside the edge, ending at the
  inscription band, the consecration cross sits in the point, and the day
  block is set low enough that the head is wide enough for it
  (`--head-pad`) and centred in what is left. The day block is at least
  the head's rise tall, so the band never crosses the arch. Halo and
  shade are filter tokens (`--panel-halo`, `--panel-shade`) that the Apse
  rules restate. On a phone by night the gold ring alone read as a line
  drawn on the sky, so the Apse sets a slate course 5px wide inside the
  ring (`--arch-stone`, lit from above as far as the springing): round the
  head it is the shade layer's wall brought 4px in and painted, down the
  sides and foot an inset shadow on the panel, so no box moves between
  themes. The panel stands a step above the sky (`--arch-ground`) with
  moonlight under the cross beneath the plaster (`--panel-glow`), the
  head's shade is soft and wide (a tight black one read as a dark band
  between the stone and the lining), and the card casts a shadow on the
  sky under a clearing of the sky's own colour, which keeps the stars off
  its edge; the day's red work is lifted to keep its contrast on the
  moonlight. The native apps draw the same. The date's weekday and the
  rest of it are each kept whole (`.home-date-weekday`,
  `.home-date-rest`), inside one span so the phone's flex touch target
  keeps their space. The day's words stand
  `--day-clear` in from the card's edge (the hairline, then 0.75rem of
  air; the native apps' `dayClear`), so a long name such as the ordo's
  "Commemoration of All the Faithful Departed (All Souls' Day)" breaks
  into balanced lines rather than crossing the lining. Higher up the head
  the date's and feast's measures are fitted to the arch: the hairline's
  width where each first line stands, less that air (the native apps take
  it from `archChord`), and `ux.spec.js` probes the hairline itself. A
  trailing familiar name in parentheses (`presentation::split_alias`) is
  set in italic beside the name, or on a line of its own, never broken.
  The date's tap box keeps its line at the foot, with `--head-pad` less
  the box's slack, so the feast stands close under it. On a phone the
  weekday is a titulus in spaced small caps over the rest of the date set
  larger (`.home-date-comma` keeps the comma for readers only).
  **The painted page** ("Home: the painted page" in style.css, app.js's
  block of that name, `HomePaint.kt`/`HomePaint.swift`): the panel's
  furniture is paint on limewash, not hardware. Lines are ruled by hand:
  app.js's `rule()` draws each as a seeded pen stroke that breathes and
  lifts to a point at its ends (the apps' `ruledLine`, same seeds: frieze
  11–14, invitation 21–28, hours 31, 32, 41+), laid in an `svg.home-rules`
  and marking its host `.is-ruled` so the CSS borders stand down (they stay
  the no-script fallback). The lining and the cross lie through the
  pigment (`pigment.png`), the invitation on a thin wash (`tablet.png`,
  `--tablet-wash`: ochre by day, moonlit blue by night; none on a bare
  black day) in a double frame of the lining and the day's colour
  (`--frontal`) stopping short of the corners. The hour table is ruled
  above, below and between the periods, never upright; the period labels
  have no cell; gilt points part the hours; the hour that is now lies
  under a pool of light with no edge (`--now-pool`), its name in
  `--titulus-now`. The frieze's letters are leaf lit at their heads; the
  foot sets the season and "Change date" in small caps parted by a gilt
  point. Masks come from `tools/genpaint.py`. On a first-class day
  (`presentation::home_first_class`: the rank signal, excluding the
  Triduum; `body.rank-first-class`, the apps' `HomeView.first_class`) the
  cross is gilt (`gild.png`) and the night's stars a size larger and
  fully lit.
  **The apse sky.** By night the head is the parish apse's painted sky
  (`.home-sky`), on a phone and in the wide niche alike: slate blue
  (`--sky-deep`, `--sky`, `--sky-low`) with the vault's brushwork
  (`sky-wash.jpg`, `tools/genplaster.py --sky-of`) in soft light, ringed by
  `--sky-rim`, a matte leaf fillet and the day's hairline (lime white on a
  white day, so the fillet stays the one gold line). Its gold-leaf stars
  are one fixed, seeded field per head (`tools/genstars.py`, measured from
  the vault's photograph; the niche heads' fields are sparser than the
  phone's, 1.6 against 2.0 times the vault's density, as their heads are
  larger; `ornaments/stars.json` and the
  apps' `Stars.kt`/`.swift`), and every front leaves out those in the
  painter's reserve: each line of the day's words padded 14×6px, the cross
  padded 11px, and 9px above the frieze. The leaf catches less light in the
  quiet seasons and Passiontide (`--leaf-catch`); a black day has no stars.
  Home's own starfield is off by night at every width, so the wall round
  the panel or niche is plain dark plaster and the night has one set of
  stars. An earlier verdict kept the desktop niche's head plaster under a
  starred wall because a sky there read as a sticker; that was a sky
  pasted into a stone niche among other stars. With the wall plain and the
  niche by night the phone's own panel (slate course, single fillet, sky),
  phone and desk read as one building, and the verdict is superseded. It must still fit a
  375×667 viewport whole. The phone's home (700px and below, all three
  fronts) carries no colophon: the arch alone stands from the header to the
  screen's foot, short of it by the 16px side gutter (`--home-card-least`
  on the web, `PhoneHome`'s `foot` in the apps), so a long day's heading
  has that height to grow into. With it, every day of a year fits 375×667
  and 390×844 as today; the wide home keeps its colophon.
  Taller phones get a taller design, not the small one stretched: from
  800px and 880px high the head rises further to a sharper point
  (genarch's "tall" and "taller" shapes, 131° and 123°), the cross, date,
  feast and invitation grow, the hours' rows get a fixed taller minimum
  (never stretched), and the card stands down to the screen's foot,
  the spare height going to the head, two parts above the day block to
  three below so the title reads with the cross. The larger type waits
  for 375px wide and 830px high; narrower or shorter phones need the
  height for wrapped lines. On a past date "Go to today" follows the
  day's facts (feast, fasts, commemorations) rather than parting them.
  The card's controls share the date's gold focus ring, and on Apse the
  card lifts red, green and violet as the rails lift white and black.
  From 701px the same object becomes one lancet niche cut into the wall
  ("Home niche" in style.css), its head a genarch shape for the screen's
  orientation: "niche" (rise 0.50, 137°) on a landscape screen, "upright"
  (rise 0.62, 125°) on a portrait one, each written by `tools/genarch.py`
  to style.css, `Arch.kt` and `Arch.swift`. By day its depth is told by
  light on the reveal, not by a moulding: the arris catches the light on
  the right and is lost on the left, the left jamb lies in shade and the
  right is lit, the soffit turns from the light toward the point and the
  sill is mitred between the jambs and faces the light (`--reveal-*`); the
  day's colour is one fillet at the back wall's edge, the back wall the
  phone's plaster with the shade the head and left jamb cast on it. A
  first-class day gilds one fillet round the head only, down to the frieze
  (`.home-summary::before`, the apps' `giltHead`/`GiltHead`), as the
  parish's chancel arch is gilt; the jambs keep the day's fillet. By night
  the reveal and fillet go and the niche is the phone's panel and sky. The
  room is lit toward it: `body.page-home::after` (warm pool, shaded edges)
  and `.home::before` (a shaft from above).
  The niche's title is the phone's weekday titulus over the date; every
  hour carries its gloss (never under 12 CSS px on the glass), the hour
  that is now is set larger as well as lit and rubricated, and the period
  labels are larger. The hours are the glossed table, or one to a line
  (the list of seven: label in the margin beside its hours, name and gloss
  parted by a gilt point, the two read as one block centred in the niche)
  where the list's heaviest day fits: from 1000px high upright, 1146px on
  its side. A heavy day (a fast with a commemoration, or a past date with a
  fast, commemoration or octave) closes the title up.
  The whole niche scales with one computed zoom, no step tables:
  `--niche-zoom: clamp(least, min((100vh - above - below) / (fit + air),
  share × 100vw / card), 1.5)` (CSS `tan(atan2())` divides lengths;
  `@property` makes it a number). What is not zoomed stands apart from what
  is: `above` the header and its margin (5.0625rem), `below` the footer, its
  fixed margin and the body's padding (6.375rem); `fit` the niche's height
  at 1 on its heaviest day of the year plus the footer's zoomed margin, and
  `air` the zoomed space above it. Figures (`--card-w`, fit, air, least,
  share): landscape 37.5, 47.25, 1.6rem, 0.96, 0.62; its list 37.5, 61; upright
  34.5, 48.75, 2.4rem, 0.9, 0.8; its list 34.5, 62.5. Each rule is marked
  `NicheFigures.<name>`, and Android's `TokensTest` holds style.css,
  `Niche.kt` and `Niche.swift` to the same figures and `--reveal-*` colours.
  Where the heaviest day fits, a lighter day is centred in the same room
  and the colophon stands at the foot; where it does not (a laptop's
  1280×800: the least keeps the niche near 576px wide), `--niche-fits` is 0
  and main stands a screen tall, so the hours stay above the fold and the
  colophon waits wholly under it, never cut through by it. A short screen
  (701px wide and at most 560px high: a phone on its side) is fitted down to
  the invitation instead (fit 35.25rem, least 0.5), so the day and "Pray"
  are above the fold on every day; there every small word (glosses,
  labels, weekday, feast, fasts, commemorations, frieze, foot) stands back
  from the zoom just enough to keep 12px on the glass (an element `zoom` of
  `max(1, 12 / (zoom × its size))`, `legible()` in the apps), hour targets
  keep 44px, and a gloss wider than its cell takes a second line. Compose
  (`nicheZoom` in `Niche.kt`, carried by a zoomed `LocalDensity`) and SwiftUI
  (`nicheZoom` in `Niche.swift`, carried by zoomed `Metrics`) use the same
  formula, and take a square screen as upright, as CSS's `orientation`
  does. Under `zoom`, app.js places the hour rules from `offsetTop` (in the
  zoomed box's own pixels) and divides star measures by the zoom; the phone
  keeps client rects so it stays pixel-identical. At 1280×800, 1180×820,
  1024×768 and 820×1180 the date, feast, "Pray" and all seven hours stand
  above the fold on every day of the year; on the heaviest days "Change
  date" and the niche's foot may not.
  Drawn architecture around it (columns, sconces, sills, arches) has been
  tried and read as illustration; prefer light and tone to objects. By day
  fills in its head (a mosaic conch, ochre voussoirs, glory rays) were
  tried and read as stickers or sunbursts: keep the day's head plaster. The
  mosaic belongs to the app icon (`tools/genicons.py`), an object seen at
  one scale.
- From 701px to 999px the hours' header sets its link list as a centred
  rank of its own under the brand and Settings; left to wrap, it stranded
  the last link on a line. Under a coarse pointer the header's links and
  the ordo's day links answer to a touch-target box drawn by a
  pseudo-element, so type and header height stay as the mouse has them.
- The footer's colophon and report line stand on a reserve: a feathered
  clearing of `--bg`, so no rosette or star runs under lettering. Its
  padding is returned by a negative margin; the footer's height feeds the
  wide hours' vault seam and must not change.
- Notices (404 and other errors) are `page-notice`: the wall, the
  headpiece, a title in the text's ink, the verse and its reference as red
  work. Form controls are set by hand where the platform's would break the
  theme (the prayer form's radios, Reminders' checkboxes): a ruled box
  filled with oak inside a margin of ground, with `appearance: auto`
  restored under forced colours.
- Every page but the phone's home closes on the footer's tailpiece
  (`footer::before`, a quatrefoil between two painted rules, in the
  lining): the same geometry
  in both themes, so the footer never moves with the theme, and set in rem
  so the wide hours' vault seam (`--apse-rest-seam`) holds. The hours'
  "Report a problem" line is footer matter under the tailpiece and the
  colophon (`layout.html`'s `footer_matter` block), not part of the
  epilogue. The header beam is a 4px course of oak (`--beam`) with
  `--material-highlight` catching its upper edge. The ordo's month headings
  and the Tabula's are tituli; the year's figures sit on a painted tablet
  (`.tabula-figures`: the surface thinned, a frame ruled twice with
  quatrefoil knops, one mask over one pseudo-element); today is a painted
  band (home's wash thinned, `--today-wash`, thinning toward the phone's
  gutters, ruled in `--gold-line`, the number in gold), not a selected row.
  No row keeps a line for "Today": on a phone it is said once at the right
  of today's second line, from 1000px it is a note in the left margin, and
  between them it keeps the reserved line under the number. The phone's
  date and "Office details" boxes are drawn round them, not laid out (the
  thumb-target test measures the drawn box). The feast page's standing is
  drawn as home's invitation (feathered wash, the lining's rule, a finer
  one in the colour of the day it is next kept).
- Filled actions are home's tablet, not slabs (`.reminder-subscribe`,
  `/beta`'s steps, the apps' "Turn on reminders"); a second action is a
  worded link with the gold underline. Reminders is ruled as home's hours
  (titulus over a lining rule, hairlines, a closing rule); a chosen time
  is the accent's figures on the gold rule with the caret, the platform's
  clock invisible over the field. Its foot carries the site's one link to
  Privacy. By night the menu panel is home's blue plaster.
- An hour's way on is set as home's hours: small-capital labels, names at
  1.06rem, gilt points between the three, and one rule struck under them
  with home's pen (`rule()` in app.js, seed 51; a border without script).
- Hours keep date switching secondary, wake lock scoped to `.office-hour`,
  and issue reporting after the prayer. "Change date" unfolds the
  hand-set month grid from `app.js` (days are links, the grid pattern's
  keys); the native date field is only the no-script fallback, since its
  popup is the platform's and belongs to no theme here. Print expands session prayers and
  hides navigation/progress controls. Don't add persistent mobile chrome
  without checking how much prayer remains visible.
- Ordo is a working page: keep columns near their content and the office
  digest within reading measure. Avoid dotted underlines on abbreviations.
  It sets one month to a page (`/calendar/YYYY/MM`, ~1,200 elements where
  the year was ~14,000); `/calendar/YYYY` is the frontispiece (Tabula
  Temporaria and the month strip) and `/calendar/YYYY/all` the whole year
  for print and find-in-page. Links to a day go to its month's page.
- Lay broad surface washes once, sized to the viewport, rather than repeating
  texture tiles on long pages. Keep liturgical text on a quiet field.
- The limewash wall is the parish nave photograph, high-passed to grey
  fields by `tools/genplaster` (portrait for phones, a landscape crop from
  1000px) and coloured by `--plaster-*` tokens. Always cover-fit; never
  stretch or tile it. The tokens are solved against the texture's pixels
  so the wall averages exactly `--bg` (see the token comment): retune one
  and re-solve the others, or the softened prayer band and the flat iOS
  sticky headings show edges. Near-white Nave needs its knee (flat ground, darker
  clouds); a symmetric texture there is invisible. No test measures the
  wall any more; check it by eye in the visual snapshots in both themes.
  All textured pages share `--plaster-strength`, composed into the opaque
  wall so sticky headings match. Wide Nave hours soften it beneath prayer
  with a pre-blurred copy (`genplaster --soft-of`) in a feathered band;
  regenerate the copies whenever a field changes. Apse's wall is already
  quiet, and a feathered layer over it bands in software rendering (see the
  vault reference), so Apse has no band. Never blur the wall live: a
  backdrop-filter over the fixed layer re-blurs it on every scrolled frame.

## Where to work

| Concern | Location |
|---------|----------|
| Tokens, layout, print | `apps/office-web/static/style.css` |
| Markup and view models | `crates/render-html/templates/`, `crates/render-html/src/` |
| Client behavior | `apps/office-web/static/app.js` |
| Offline behavior | `apps/office-web/static/sw.js`, `manifest.webmanifest` |
| Browser checks | `.web-tools/tests/ux.spec.js`, `visual.spec.js` |

## Validation and delivery

Choose checks for the changed surface. For layout work, inspect light/dark
and narrow/wide screens; measure overflow, alignment, and touch targets with
Playwright rather than relying only on screenshots. Check home and an hour
when shared CSS/chrome changes. Recheck seasonal colours only when changing
ornament or theme tokens; measure paint cost when adding decoration.

Run `cargo test -p render-html -p office-web`, rebuild embedded assets,
and run `make test-ux` for UI changes. CI runs behavior tests from
`ux.spec.js`; add coverage there when warranted. Preserve meaningful
accessibility and visual checks, updating assertions only for deliberate
behavior changes. Composition goldens are not HTML snapshots.

Use a PR against `master`, describing the visual outcome and validation.
Keep the existing merge-to-master release flow. Repository instructions and
user authorization govern external actions; this skill adds no approval gate.
