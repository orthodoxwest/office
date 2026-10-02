# Apse vault implementation

Read when changing the starfield, its masks, or post-office decoration.
The field is the parish apse's painted vault: eight-ray stars (four long
rays, four short diagonals) in three sizes, set by hand rather than on a
lattice, each at its own opacity. A ribbed diaper with principal stars at the
crossings came first and read as printed wallpaper.

- The tile is `ornaments/vault.svg` (`--apse-star-tile`), written by
  `tools/genornaments.py` (seeded), used as a **mask**, never as a background
  image. The mask supplies only shape and alpha; the colour is the
  pseudo-element's `background-color`, painted through it. An SVG image
  cannot read tokens, so painting it directly would freeze the stars
  through the seasons. A star crossing the tile's edge is drawn again on
  the far side, so the repeat is seamless.
- The hours' ending reads `--wall-field`, `--wall-ink`,
  `--wall-tile-size`, `--wall-leaf` and the fades (`--wall-end-fade`,
  `--wall-foot-fade`), declared on
  `body`: by night the vault (`--apse-vault`, `--apse-ink`, the seasonal
  `--ornament`, `--apse-tile`, the leaf, soft fades), by day the Nave's
  powdering (`ornaments/powder.svg`, one rosette to each cell of a 96×168px
  quincunx tile that never scales, in `--rubric` at 12%, no leaf, cut
  square below the hour navigation at the ending, which lives in the epilogue's 3.25rem bottom padding and the
  footer). The Nave gate mirrors the Apse one (`prefers-color-scheme:
  light` with `:root:not([data-theme="dark"])`, and
  `:root[data-theme="light"]`), and by day also sets `--apse-copy-shadow`
  (the footer lettering's clearing). The wide hours' side and ending fields read `--apse-vault`
  directly, so by day the margins beside the prayer stay plain.
  Home has no field: its leaf is lettering on the bare wall in both themes
  (an earlier home set its frontispiece under the vault, lit from the
  altar, and on a powdered wall by day).
- Each field's fade is a second mask layer (`linear-gradient`, sized 100%),
  intersected with the tile (`mask-composite: intersect` plus
  `-webkit-mask-composite: source-in`). Give both layers the tile's position
  so the phase reads the same on every layer.
- Every field reads one `--apse-tile` from `body`: 528px on phones, 704px
  from 701px, then 832px from 1800px and 960px from 2400px, so on very wide
  screens the vault reads as a painted ceiling rather than a wallpaper of
  small stars. Keep the steps whole pixels; fractional tiles can seam.
- Every field adds `--apse-leaf` as a further mask layer: `leaf.png`, 64px
  of seamless low-frequency noise (`tools/genplaster --leaf`) drawn at
  1024px, so each star catches its own share of light (about 60-100%) as
  hand-laid leaf does. It takes each field's tile position, which keeps the
  epilogue and footer continuous. An SVG `feTurbulence` mask gave the same
  look but cost about 50ms of first paint and most of a full scroll's raster
  time in software; the small image costs neither.
- Declare `--apse-ink` and `--apse-vault` on `body`. Custom properties
  resolve where declared; seasonal classes also live on `body`. A `none` on
  body overrides inheritance.
- Default-theme gating needs `prefers-color-scheme: dark`. Selecting every
  root without `data-theme="light"` also catches light-mode default users.
- Hours admit the vault only in `.hour-epilogue`, after prayer. Keep the field
  transparent through the end mark and the continuation links (it begins in
  the epilogue's bottom padding), and align its bottom with the footer
  continuation, which carries the colophon and the report line. On desktop these layers bleed to
  viewport edges without widening `.elements`. The stars sit straight on
  the wall. A flat night ground once eased in beneath them
  (each host's outset `border-image`), but its fade banded (see below).
- From 1680px an hour also shows the vault in the margins beside the prayer,
  at half strength, running down to meet the ending. Both are fixed layers
  on one phase: the sides (`.office-hour::after`) and the ending
  (`.office-hour::before`), which there replaces the scrolling epilogue and
  footer fields. A field scrolling with the page repainted every strip of a
  long hour (about ten times the raster time of a full scroll of Lauds), and
  a fixed field meets a scrolling one on the same phase at only one scroll
  position; an earlier version faded the sides out before the epilogue for
  that reason, which left the margins empty around the ending. Scroll-driven
  opacity fades the sides in over the first half-screen and the ending in
  over the last 9rem, on the compositor. The ending's mask and phase are
  set in rem from the screen's foot (`--apse-rest-seam`, the epilogue/footer
  seam at rest), which holds at every text size, so at rest it reproduces
  the scrolling fields. `mask-position` takes the four-value form
  (`left 50% bottom …`); the three-value form is invalid there and silently
  drops to `0% 0%`. Without scroll timelines (`@supports`) the margins stay
  plain and the ending scrolls. Below 1680px the margin holds a column of
  stars or less, which reads as an accident.
- Masks clear the header and thin toward the footer. Avoid a narrow decorative
  band floating between blank margins. Keep the field static. The footer's
  tailpiece (`footer::before`) sits on the field in both themes at one
  geometry; `--apse-rest-seam` counts its height, so retune the seam if the
  footer's height changes.
- Software rasterization (headless Chromium, the snapshot tests, and any
  browser drawing without a GPU) truncates when it blends a feathered layer
  over a smooth dark field: a full-width fade steps down in one-level
  columns or rows that read as hairlines. GPU rasterization does not, but
  readers do meet software drawing, so Apse carries no full-width feathered
  flat layer over its wall: no softened prayer band and no ground under the
  epilogue. Star fields are safe, since their fades act only on sparse
  shapes, and Nave's texture breaks the steps up. `ux.spec.js` measures
  column and row stripes against the plain wall at the end of Lauds.
- Measure paint cost when adding layers. An earlier 172-layer version cost
  roughly 600ms of extra first paint; one repeating tile stays within noise
  of a bare page. Check rendered pixels as well as valid CSS: sub-pixel
  shapes and low opacity can make stars disappear.

These are constraints of the existing implementation, not a requirement to
add a vault to other pages. Administrative pages use the shared palette and
can remain visually simpler.
