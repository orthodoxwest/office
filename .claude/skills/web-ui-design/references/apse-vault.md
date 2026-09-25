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
  The tile scales with `mask-size` (132px phones, 176px wider), so star size
  scales with it.
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
  transparent through continuation links, fade in around Assurance, and align
  its bottom with the footer continuation. On desktop these layers bleed to
  viewport edges without widening `.elements`. The flat night ground that
  eases in under desktop stars cannot share the star layer's mask, so it is
  each host's own `border-image`, outset to the viewport edges with the same
  fade; it adds no layout or scrollable overflow.
- Masks clear the header and thin toward the footer. Avoid a narrow decorative
  band floating between blank margins. Keep the field static, and hide the
  footer diamond where the vault already provides ornament.
- Measure paint cost when adding layers. An earlier 172-layer version cost
  roughly 600ms of extra first paint; one repeating tile stays within noise
  of a bare page. Check rendered pixels as well as valid CSS: sub-pixel
  shapes and low opacity can make stars disappear.

These are constraints of the existing implementation, not a requirement to
add a vault to other pages. Administrative pages use the shared palette and
can remain visually simpler.
