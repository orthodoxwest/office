package render

import (
	"fmt"
	"html/template"
	"strings"

	"rsc.io/qr"
)

// QR geometry, in module units (one "module" is one cell of the code).
//
// The quiet zone is the four-module margin the QR specification requires
// around the symbol; scanners use it to find the code's edge, so it is part
// of the graphic and not a CSS margin that a stylesheet could shave off.
const (
	qrQuietZone = 4
	// Every QR symbol carries a 7x7 finder pattern in three corners. We draw
	// those ourselves, as rounded bosses, instead of letting them fall out of
	// the module loop as plain squares.
	qrFinderSize = 7
	// The emblem knockout: a centred square of modules left undrawn so the
	// cross can sit in the middle of the code. Error-correction level H
	// recovers roughly 30% of a damaged symbol; 7x7 modules is 5-9% of the
	// versions this app's URLs produce, well inside that budget, and the
	// centre carries no format or finder information. TestQRKnockoutBudget
	// holds the ratio to a conservative ceiling.
	qrEmblemSize = 7
	// Corner softening on data modules, as a fraction of one module. Enough
	// to read as ink on laid paper rather than pixels; far below the radius
	// at which isolated modules lose the dark area scanners need.
	qrModuleRadius = 0.18
)

// QRCodeSVG renders url as an inline SVG QR code in the app's visual
// language: dark modules on a light plate, rounded corner bosses in place of
// the three finder squares, and the app's cross knocked out of the centre.
//
// The SVG carries its own <style> element rather than relying on the site
// stylesheet, so the code still renders — and still scans — in a printed
// page, a saved copy, or a context where style.css never loaded. Colours
// read theme tokens with literal fallbacks for exactly that reason.
//
// The plate stays light in both themes. Inverted codes (light modules on a
// dark ground) defeat scanners that do not try both polarities, so the Apse
// theme dims the plate rather than flipping it.
func QRCodeSVG(url string) (template.HTML, error) {
	code, err := qr.Encode(url, qr.H)
	if err != nil {
		return "", fmt.Errorf("encoding QR code for %q: %w", url, err)
	}

	size := code.Size
	extent := size + 2*qrQuietZone
	emblemLow := (size-qrEmblemSize)/2 + qrQuietZone

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="qr-code" viewBox="0 0 %d %d" role="img" `+
		`aria-label="QR code linking to %s" xmlns="http://www.w3.org/2000/svg">`,
		extent, extent, template.HTMLEscapeString(url))
	b.WriteString(qrStyle)

	// The plate is part of the graphic: the quiet zone must be light even
	// when the code is dropped onto a dark surface.
	fmt.Fprintf(&b, `<rect class="qr-plate" width="%d" height="%d" rx="2"/>`, extent, extent)

	// Data modules. Finder patterns and the emblem knockout are drawn (or
	// deliberately left blank) separately, so skip those cells here.
	b.WriteString(`<g class="qr-modules">`)
	for y := range size {
		for x := range size {
			if !code.Black(x, y) || qrInFinder(x, y, size) {
				continue
			}
			px, py := x+qrQuietZone, y+qrQuietZone
			if qrInEmblem(px, py, emblemLow) {
				continue
			}
			fmt.Fprintf(&b, `<rect x="%d" y="%d"/>`, px, py)
		}
	}
	b.WriteString(`</g>`)

	b.WriteString(`<g class="qr-finders">`)
	for _, corner := range [][2]int{
		{qrQuietZone, qrQuietZone},
		{extent - qrQuietZone - qrFinderSize, qrQuietZone},
		{qrQuietZone, extent - qrQuietZone - qrFinderSize},
	} {
		b.WriteString(qrFinder(corner[0], corner[1]))
	}
	b.WriteString(`</g>`)

	b.WriteString(qrEmblem(emblemLow))
	b.WriteString(`</svg>`)

	// Safe as template.HTML: url is the only caller-supplied value and it is
	// escaped above; everything else is geometry this function generated.
	return template.HTML(b.String()), nil
}

// qrInFinder reports whether module (x, y) falls inside one of the three
// finder patterns, which qrFinder redraws as rounded bosses.
func qrInFinder(x, y, size int) bool {
	near := func(v int) bool { return v < qrFinderSize || v >= size-qrFinderSize }
	inCorner := func(cx, cy int) bool {
		return x >= cx && x < cx+qrFinderSize && y >= cy && y < cy+qrFinderSize
	}
	if !near(x) || !near(y) {
		return false
	}
	return inCorner(0, 0) || inCorner(size-qrFinderSize, 0) || inCorner(0, size-qrFinderSize)
}

// qrInEmblem reports whether a plate-space cell falls in the centre knockout.
func qrInEmblem(px, py, low int) bool {
	return px >= low && px < low+qrEmblemSize && py >= low && py < low+qrEmblemSize
}

// qrFinder draws one corner boss: a rounded ring one module thick around a
// rounded 3x3 centre, replacing the specification's plain concentric squares.
// The ring is stroked down its centre line so its outer edge lands exactly on
// the 7x7 footprint a scanner expects.
func qrFinder(x, y int) string {
	return fmt.Sprintf(
		`<rect class="qr-finder-ring" x="%.1f" y="%.1f" width="6" height="6" rx="1.7"/>`+
			`<rect class="qr-finder-pupil" x="%d" y="%d" width="3" height="3" rx="0.8"/>`,
		float64(x)+0.5, float64(y)+0.5, x+2, y+2)
}

// qrEmblem draws the cross of the site favicon inside the knockout: the same
// proportions as static/favicon.svg, scaled to the emblem square and ringed
// with a hairline so the mark reads as inset rather than as lost modules.
//
// The favicon fills its own 32-unit square edge to edge, which is right for a
// browser tab but crowds the hairline here. qrEmblemFill holds the cross back
// from the ring without touching the mark's own proportions.
func qrEmblem(low int) string {
	const faviconExtent = 32.0
	const qrEmblemFill = 0.76
	scale := qrEmblemFill * qrEmblemSize / faviconExtent
	margin := (qrEmblemSize - qrEmblemFill*qrEmblemSize) / 2
	at := func(v float64) float64 { return float64(low) + margin + v*scale }
	span := func(v float64) float64 { return v * scale }

	return fmt.Sprintf(`<g class="qr-emblem">`+
		`<rect class="qr-emblem-ground" x="%.3f" y="%.3f" width="%d" height="%d" rx="0.9"/>`+
		`<g class="qr-emblem-cross">`+
		`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f"/>`+
		`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f"/>`+
		`</g></g>`,
		float64(low), float64(low), qrEmblemSize, qrEmblemSize,
		// Upright: favicon x=13.5 w=5, y=4 h=24.
		at(13.5), at(4), span(5), span(24),
		// Traverse: favicon x=5.5 w=21, y=10 h=5.
		at(5.5), at(10), span(21), span(5))
}

// qrStyle is the SVG's self-contained stylesheet. Module geometry lives here
// as CSS geometry properties so each data module costs one short <rect> with
// nothing but coordinates — a few hundred of them ride in every page.
var qrStyle = fmt.Sprintf(`<style>`+
	`.qr-plate{fill:var(--qr-plate,#fdfaf3)}`+
	`.qr-modules rect{width:1px;height:1px;rx:%.2fpx;fill:var(--qr-ink,#2b2520)}`+
	`.qr-finder-ring{fill:none;stroke:var(--qr-ink,#2b2520);stroke-width:1}`+
	`.qr-finder-pupil{fill:var(--qr-ink,#2b2520)}`+
	`.qr-emblem-ground{fill:var(--qr-plate,#fdfaf3);stroke:var(--qr-emblem-line,#c9ac72);stroke-width:.12}`+
	`.qr-emblem-cross rect{fill:var(--qr-emblem-ink,#9a7328)}`+
	`</style>`, qrModuleRadius)
