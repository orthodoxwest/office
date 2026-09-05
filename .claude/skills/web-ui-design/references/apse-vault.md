# Apse vault implementation

Read when changing the starfield, its masks, or post-office decoration.
The existing design is a geometric diaper: crossed diagonal hairlines with
principal stars at intersections and smaller stars at panel centres.

- The field uses twelve CSS gradients in one repeating cell. Keep colour in
  `var(--ornament)` via `color-mix`; SVG data URLs cannot inherit these tokens.
- In a square tile, the 45° stripe crosses at 50% of its axis; the 135° stripes
  cross at 25% and 75%. Principal stars sit at (25%,25%) and (75%,75%); smaller
  stars at (75%,25%) and (25%,75%). Keep stars inside the cell: corner/edge
  gradients paint half a star without completing it in the neighbouring tile.
- Declare the field on `body`. Custom properties resolve where declared;
  seasonal classes also live on `body`. A `none` on body overrides inheritance.
- Default-theme gating needs `prefers-color-scheme: dark`. Selecting every
  root without `data-theme="light"` also catches light-mode default users.
- Home uses one fixed field, anchored top centre, across viewport widths. Its
  opaque frontispiece and soft background-coloured shadow clear the content;
  phone gutters retain a hint of stars. Separate gap/footer pieces previously
  shifted the visible pattern as content and viewport heights changed.
- Hours admit the vault only in `.hour-epilogue`, after prayer. Keep the field
  transparent through continuation links, fade in around Assurance, and align
  its bottom with the footer continuation. On desktop these layers bleed to
  viewport edges without widening `.elements`.
- Masks clear the header and thin toward the footer. Avoid a narrow decorative
  band floating between blank margins. Keep the field static, and hide the
  footer diamond where the vault already provides ornament.
- Measure paint cost when adding layers. An earlier 172-layer version cost
  roughly 600ms of extra first paint; the current twelve-layer design cost
  about 24ms in that comparison. Check rendered pixels as well as valid CSS:
  sub-pixel radii and low opacity can make stars disappear.

These are constraints of the existing implementation, not a requirement to
add a vault to other pages. Administrative pages use the shared palette and
can remain visually simpler.
