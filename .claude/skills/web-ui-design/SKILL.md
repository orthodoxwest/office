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
sky—with EB Garamond, restrained rules, and generous reading space. Avoid
playful rewards, motion for its own sake, and decorative layers behind prayer.

Administrative pages should belong to the same app, but may use denser tables,
charts, and more direct language. They need not imitate a liturgical page.
Follow the user's requested scope; this skill does not authorize deployment
or impose a redesign on an unrelated change.

## Shared visual language

- Use existing tokens in `internal/web/static/style.css`, not copied hex values.
  Nave is warm plaster; Apse is cool blue, not forest green.
- `--accent` / `--oak` supply structure; `--border` / `--surface-edge` separate
  surfaces. Use the existing surface tokens for panels and tables.
- Functional gold (`--gold`, `--gold-line`) marks controls and selections.
  Ornament (`--ornament`, `--ornament-line`, `--ornament-hi/lo`) changes with
  Passiontide/Paschaltide. Decide which job a colour serves before choosing it.
  Dark inscription backgrounds use the seasonal Apse leaf colours in both themes.
- Current controls use a gold underline; disclosures use the existing caret.
  Keep Default / Nave / Apse labels and visible keyboard focus.
- The starfield belongs only to Apse home and the post-office epilogue.
  For changes to that field or its masks, read
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
  across the unequal 2/3/2 groups.
- Hours keep date switching secondary, wake lock scoped to `.office-hour`,
  and Assurance/reporting after the prayer. Print expands session prayers and
  hides navigation/progress controls. Don't add persistent mobile chrome
  without checking how much prayer remains visible.
- Ordo is a working page: keep columns near their content and the office
  digest within reading measure. Avoid dotted underlines on abbreviations.
- Lay broad surface washes once, sized to the viewport, rather than repeating
  texture tiles on long pages. Keep liturgical text on a flat field.

## Where to work

| Concern | Location |
|---------|----------|
| Tokens, layout, print | `internal/web/static/style.css` |
| Markup and view models | `internal/render/templates/`, `internal/render/` |
| Client behavior | `internal/web/static/app.js` |
| Offline behavior | `internal/web/static/sw.js`, `manifest.webmanifest` |
| Browser checks | `.web-tools/tests/ux.spec.js`, `visual.spec.js` |

## Validation and delivery

Choose checks for the changed surface. For layout work, inspect light/dark
and narrow/wide screens; measure overflow, alignment, and touch targets with
Playwright rather than relying only on screenshots. Check home and an hour
when shared CSS/chrome changes. Recheck seasonal colours only when changing
ornament or theme tokens; measure paint cost when adding decoration.

Run `go test ./internal/render/ ./internal/web/`, rebuild embedded assets,
and run `make test-ux` for UI changes. CI runs behavior tests from
`ux.spec.js`; add coverage there when warranted. Preserve meaningful
accessibility and visual checks, updating assertions only for deliberate
behavior changes. Composition goldens are not HTML snapshots.

Use a PR against `master`, describing the visual outcome and validation.
Keep the existing merge-to-master release flow. Repository instructions and
user authorization govern external actions; this skill adds no approval gate.
