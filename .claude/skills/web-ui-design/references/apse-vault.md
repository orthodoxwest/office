# Apse vault implementation

Read when changing the starfield, its masks, or post-office decoration.
The existing design is a geometric diaper: crossed diagonal hairlines with
principal stars at intersections and smaller stars at panel centres. The star
shapes follow the parish apse: eight-ray principal stars (four long rays,
four short diagonals) and small four-ray sparks.

- The cell is one SVG data URL (`--apse-star-tile`) used as a **mask**, never
  as a background image. The mask supplies only shape and alpha; the colour
  is the pseudo-element's `background-color: var(--apse-ink)`, which is
  `--ornament` in Apse and transparent in Nave. An SVG image cannot read
  tokens, so painting it directly would freeze the stars through the seasons.
  CSS gradients keep the colour too but cannot draw diagonal rays.
- Each field's fade is a second mask layer (`linear-gradient`, sized 100%),
  intersected with the tile (`mask-composite: intersect` plus
  `-webkit-mask-composite: source-in`). Give both layers the tile's position
  so the phase reads the same on every layer.
- Ribs run corner to corner and through the edge midpoints. Principal stars
  sit at (25%,25%) and (75%,75%); smaller stars at (75%,25%) and (25%,75%).
  Keep stars inside the cell, where no neighbour tile is needed to finish them.
  The tile scales with `mask-size`, so star size scales with it. Every field
  reads one `--apse-tile` from `body`: 132px on phones, 176px from 701px,
  then 208px from 1800px and 240px from 2400px, so on very wide screens the
  diaper reads as a painted ceiling rather than a wallpaper of small stars.
  Keep the steps whole pixels; fractional tiles can seam. Each principal star carries a soft radial halo inside the
  cell (28% at the centre, gone by 12 units), so the gold reads as catching
  light; keep it interior to the cell like the stars.
- Every field adds `--apse-leaf` as a further mask layer: `leaf.png`, 64px
  of seamless low-frequency noise (`tools/genplaster --leaf`) drawn at
  1024px, so each star catches its own share of light (about 60-100%) as
  hand-laid leaf does. It takes each field's tile position, which keeps the
  epilogue and footer continuous. An SVG `feTurbulence` mask gave the same
  look but cost about 50ms of first paint and most of a full scroll's raster
  time in software; the small image costs neither.
- Home's field is lit from the altar: its background is `--apse-gild`, a
  radial gradient over the seasonal `--ornament-hi/lo` pair centred on the
  frontispiece, and a third mask layer lets the light fall off toward the
  corners. `--apse-gild` is `none` outside Apse, like `--apse-vault`; a
  background image without that gate would paint gold over the Nave.
- Declare `--apse-ink` and `--apse-vault` on `body`. Custom properties
  resolve where declared; seasonal classes also live on `body`. A `none` on
  body overrides inheritance.
- Default-theme gating needs `prefers-color-scheme: dark`. Selecting every
  root without `data-theme="light"` also catches light-mode default users.
- Home uses one fixed field, anchored top centre, across viewport widths. Its
  opaque frontispiece and soft background-coloured shadow clear the content;
  phone gutters retain a hint of stars. Separate gap/footer pieces previously
  shifted the visible pattern as content and viewport heights changed.
- Hours admit the vault only in `.hour-epilogue`, after prayer. Keep the field
  transparent through continuation links, fade in around the report link, and align
  its bottom with the footer continuation. On desktop these layers bleed to
  viewport edges without widening `.elements`. The flat night ground that
  eases in under desktop stars cannot share the star layer's mask, so it is
  each host's own `border-image`, outset to the viewport edges with the same
  fade; it adds no layout or scrollable overflow.
- From 1680px an hour also shows the vault in the margins beside the prayer:
  one fixed layer (`.office-hour::after`), outside the softened band, at half
  strength. A field scrolling with the page could share the epilogue's
  phase, but it repainted every strip of a long hour (about ten times the
  raster time of a full scroll of Lauds); a fixed one is drawn once. A
  scroll-driven opacity fades it in over the first half-screen and out
  before the epilogue scrolls into view, so it never meets the ending's
  field at a different phase. Both fades run on the compositor; without
  scroll timelines (`@supports`) the margins stay plain. Below 1680px the
  margin holds a column of stars or less, which reads as an accident.
- Masks clear the header and thin toward the footer. Avoid a narrow decorative
  band floating between blank margins. Keep the field static, and hide the
  footer diamond where the vault already provides ornament.
- Headless Chromium rasterizes in software, which truncates when it blends a
  feathered mask over a smooth dark field: screenshots show one-level
  vertical stripes where the band or a field fades, which GPU rasterization
  (every ordinary desktop browser) does not. Before chasing them, re-shoot
  with `--use-angle=swiftshader --enable-gpu-rasterization`.
- Measure paint cost when adding layers. An earlier 172-layer version cost
  roughly 600ms of extra first paint; one repeating tile stays within noise
  of a bare page. Check rendered pixels as well as valid CSS: sub-pixel
  shapes and low opacity can make stars disappear.

These are constraints of the existing implementation, not a requirement to
add a vault to other pages. Administrative pages use the shared palette and
can remain visually simpler.
