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
  ends each hour and is the brand mark; Lauds' headpiece carries
  the sun and Vespers' and Compline's the moon. One mark per threshold.
- Current controls use a gold underline (the header's current page takes the
  lining's terracotta); disclosure carets take their label's ink at 70%.
  Keep Default / Nave / Apse labels and visible keyboard focus.
- The starfield belongs to the post-office epilogue and, on screens 1680px
  and wider, the margins beside an hour's prayer, which run down into the
  epilogue's field. Home has neither stars nor powdering: its leaf is
  lettering on the bare wall. By day the epilogue's field carries the
  Nave's powdering instead: one six-petal rosette to each cell
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
- Home's selectors (`.home-prayer-card[data-date-slug]`, `.pray-now`,
  `.home-hour-link[data-hour]`, `.home-hour-link-name`) are used by app.js.
  `.pray-now` is the hour row the leaf points at (`.is-now`), or, after
  midnight while yesterday's Compline is still the hour, a `.leaf-late`
  link in the leaf's head. Current-hour and pointer markup exists in both
  the template and `updatePrayNow()` / `setHourCurrent()`; keep both
  consistent, and keep the server's hour words in `presentation`.
- Home is the day's leaf, a kalendar page lettered on the wall (no panel,
  niche, powdering or vault; header beam and footer as everywhere): the
  rubric date line (the ordo link), the title with a versal graded by rank
  (`.leaf-g0`–`.leaf-g4` from `presentation::leaf_rank`: a feria none; simple
  to double an IM Fell versal in the day's ink; the second class gilt; the
  first class a gilt Goudy Initialen letter in a square two title lines
  tall, with a penwork bar border, `.leaf-bar`, down the margin), the rank
  line, a double rule in the day's colour, the horarium (each hour's
  initial red and blue by turns, its time in the old reckoning; the
  pointed hour 1.7rem with ☞ in the margin and "pray now", or "begin here"
  on another day; stronger rules over Terce and Vespers), the Lauds collect
  under a titulus, and the season with Change date at the foot. A title
  opening with a numeral (`.leaf-numeral`) and a black day (`.leaf-bare`)
  take no versal. Versals are fitted per letter from the faces' outlines
  (`--vh/--vt/--vl/--vr`); recalibrate from outlines, not by eye. One
  column at every width, 36rem centred from 1000px; on a phone the leaf
  scrolls and the footer follows it. Apse keeps the night wall; red turns
  gold and blue silver. The leaf's inks never change with the season or
  the hour.
- The time of day (`time-dawn`, `-day`, `-dusk`, `-night` on home's body,
  set by the server and corrected by app.js) tints the Nave's wall only:
  each pair of `--bg` / `--plaster-tint` is solved so the wall still
  averages its `--bg`, and `main::before` adds a soft cast. Never in Apse.
- From 701px to 959px the hours' header sets its link list as a centred
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
- Every page closes on the footer's tailpiece (`footer::before`, a
  quatrefoil between two painted rules, in the lining): the same geometry
  in both themes, so the footer never moves with the theme, and set in rem
  so the wide hours' vault seam (`--apse-rest-seam`) holds. The hours'
  "Report a problem" line is footer matter under the tailpiece and the
  colophon (`layout.html`'s `footer_matter` block), not part of the
  epilogue. The header beam is a 4px course of oak (`--beam`) with
  `--material-highlight` catching its upper edge. The ordo's month headings
  and the Tabula's are tituli; the year's figures sit on a painted tablet
  (`.tabula-figures`: the surface thinned, a frame ruled twice with
  quatrefoil knops, one mask over one pseudo-element); today is a painted
  band (the frieze's wash inset 8px from the phone's gutters, ruled in
  `--gold-line`, the number in gold), not a selected row, and carries no
  further mark.
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
  clouds); a symmetric texture there is invisible. `ux.spec.js` measures
  the rendered wall's mean and contrast in both themes.
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
