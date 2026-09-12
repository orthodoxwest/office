package render

import (
	"fmt"
	"html/template"
	"strconv"
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
	// those ourselves, as corner bosses, instead of letting them fall out of
	// the module loop as plain squares.
	qrFinderSize = 7
	// The emblem knockout: a centred square of modules left undrawn so the
	// cross medallion can sit in the middle of the code. Error-correction
	// level H recovers roughly 30% of a damaged symbol; 7x7 modules is 5-9%
	// of the versions this app's URLs produce, well inside that budget, and
	// the centre carries no format or finder information.
	// TestQRKnockoutBudget holds the ratio to a conservative ceiling.
	qrEmblemSize = 7
	// Half a module: an isolated module is drawn as a disc and a run of them
	// as a single capsule. This is the whole character of the artwork — the
	// code reads as ink laid in strokes rather than as a grid of cells.
	qrModuleRadius = 0.5
	// The corner bosses. 1.5 on a 3-unit pupil is half its width, so the
	// pupil is a disc like an isolated module; 2.2 on the 6-unit ring is a
	// softened square, which keeps the three corners reading as corners.
	qrFinderRingRadius  = 2.2
	qrFinderPupilRadius = 1.5
	// The medallion: two concentric hairlines, echoing the double rules the
	// stylesheet sets around an hour's title.
	qrMedallionGap   = 0.34
	qrMedallionRule  = 0.17
	qrEmblemCrossFit = 0.80 // cross size as a fraction of the knockout
)

// QRCodeSVG renders url as an inline SVG QR code in the app's visual
// language: modules merged into continuous strokes of dark ink on a light
// plate, corner bosses in place of the three finder squares, and the app's
// cross set in a double-ruled gold medallion at the centre.
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
	b.WriteString(qrDefs)

	// The plate is part of the graphic: the quiet zone must be light even
	// when the code is dropped onto a dark surface.
	fmt.Fprintf(&b, `<rect class="qr-plate" width="%d" height="%d" rx="2"/>`, extent, extent)

	fmt.Fprintf(&b, `<path class="qr-modules" d="%s"/>`, qrModulePath(code, emblemLow))

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

// qrDark reports a painted data module in symbol coordinates. Reserved areas
// — the three finder patterns and the emblem knockout — read as empty, so
// modules beside them round off against the gap instead of merging into it.
func qrDark(code *qr.Code, emblemLow, x, y int) bool {
	if x < 0 || y < 0 || x >= code.Size || y >= code.Size {
		return false
	}
	if qrInFinder(x, y, code.Size) || qrInEmblem(x+qrQuietZone, y+qrQuietZone, emblemLow) {
		return false
	}
	return code.Black(x, y)
}

// qrModulePath draws every data module as one subpath of a single path.
// Abutting subpaths union under the default nonzero fill rule, so a run of
// modules becomes one continuous stroke of ink with rounded ends.
//
// A corner is rounded only where the shape actually turns — that is, where
// both of the edge-neighbours meeting at it are absent. Where a run
// continues through a corner the corner stays square, so the stroke does not
// pinch at every module boundary. Inner corners, where two runs meet at a
// right angle, are left sharp: filleting them too reads as a liquid blob
// rather than as laid ink.
//
// Every subpath begins with a move to the module's top edge, at x = px or
// px+r and y = py exactly. TestQRCodeSVGPaintsTheSymbol reads the modules
// back out of the path by flooring those two numbers, so keep the first
// command of each subpath an absolute move to that point.
func qrModulePath(code *qr.Code, emblemLow int) string {
	var d strings.Builder
	const r = qrModuleRadius
	for y := range code.Size {
		for x := range code.Size {
			if !qrDark(code, emblemLow, x, y) {
				continue
			}
			px, py := float64(x+qrQuietZone), float64(y+qrQuietZone)
			up := qrDark(code, emblemLow, x, y-1)
			down := qrDark(code, emblemLow, x, y+1)
			left := qrDark(code, emblemLow, x-1, y)
			right := qrDark(code, emblemLow, x+1, y)
			corner := func(a, b bool) float64 {
				if !a && !b {
					return r
				}
				return 0
			}
			tl, tr := corner(up, left), corner(up, right)
			br, bl := corner(down, right), corner(down, left)

			d.WriteString("M" + qrNum(px+tl) + " " + qrNum(py))
			d.WriteString("H" + qrNum(px+1-tr))
			qrArc(&d, tr, px+1, py+tr)
			d.WriteString("V" + qrNum(py+1-br))
			qrArc(&d, br, px+1-br, py+1)
			d.WriteString("H" + qrNum(px+bl))
			qrArc(&d, bl, px, py+1-bl)
			d.WriteString("V" + qrNum(py+tl))
			qrArc(&d, tl, px+tl, py)
			d.WriteString("Z")
		}
	}
	return d.String()
}

// qrArc appends a clockwise quarter-turn of radius r ending at (x, y), or
// nothing when the corner is square.
func qrArc(d *strings.Builder, r, x, y float64) {
	if r == 0 {
		return
	}
	n := qrNum(r)
	d.WriteString("A" + n + " " + n + " 0 0 1 " + qrNum(x) + " " + qrNum(y))
}

// qrNum formats a path coordinate as briefly as SVG allows. Several hundred
// of these ride in every share page, so "4.5" beats "4.500" and ".5" beats
// "0.5".
func qrNum(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return strings.TrimPrefix(s, "0")
}

// qrInFinder reports whether module (x, y) falls inside one of the three
// finder patterns, which qrFinder redraws as corner bosses.
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
// disc, replacing the specification's plain concentric squares. The ring is
// stroked down its centre line so its outer edge lands exactly on the 7x7
// footprint a scanner expects.
func qrFinder(x, y int) string {
	return fmt.Sprintf(
		`<rect class="qr-finder-ring" x="%.1f" y="%.1f" width="6" height="6" rx="%.2f"/>`+
			`<rect class="qr-finder-pupil" x="%d" y="%d" width="3" height="3" rx="%.2f"/>`,
		float64(x)+0.5, float64(y)+0.5, qrFinderRingRadius,
		x+2, y+2, qrFinderPupilRadius)
}

// qrEmblem draws the medallion: the cross of the site favicon, in gold leaf,
// inside two concentric hairlines. The circle is inscribed in the square
// knockout, so the four corners of the knockout stay clear plate and read as
// a halo around the seal.
//
// The favicon fills its own 32-unit square edge to edge, which is right for a
// browser tab but crowds the inner rule here. qrEmblemCrossFit holds the
// cross back from it without touching the mark's own proportions.
func qrEmblem(low int) string {
	const faviconExtent = 32.0
	centre := float64(low) + qrEmblemSize/2.0
	outer := qrEmblemSize / 2.0

	scale := qrEmblemCrossFit * qrEmblemSize / faviconExtent
	margin := (qrEmblemSize - qrEmblemCrossFit*qrEmblemSize) / 2
	at := func(v float64) float64 { return float64(low) + margin + v*scale }
	span := func(v float64) float64 { return v * scale }

	return fmt.Sprintf(`<g class="qr-emblem">`+
		`<circle class="qr-medallion" cx="%.3f" cy="%.3f" r="%.3f"/>`+
		`<circle class="qr-medallion-inner" cx="%.3f" cy="%.3f" r="%.3f"/>`+
		`<g class="qr-emblem-cross">`+
		`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f"/>`+
		`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f"/>`+
		`</g></g>`,
		centre, centre, outer,
		centre, centre, outer-qrMedallionGap,
		// Upright: favicon x=13.5 w=5, y=4 h=24.
		at(13.5), at(4), span(5), span(24),
		// Traverse: favicon x=5.5 w=21, y=10 h=5.
		at(5.5), at(10), span(21), span(5))
}

// qrDefs is the gold-leaf gradient the cross is filled with: two stops about
// 18% apart in lightness, as on the drop caps, so the mark reads as leaf
// catching light rather than as flat paint.
//
// These are the emblem's own tokens rather than --ornament-hi/lo, which
// follow the theme. The medallion sits on the light plate in both themes, so
// its gilding must not change with the room around it.
const qrDefs = `<defs>` +
	`<linearGradient id="qr-leaf" x1="0" y1="0" x2="0" y2="1">` +
	`<stop offset="0" stop-color="var(--qr-emblem-hi,#b98d3c)"/>` +
	`<stop offset="1" stop-color="var(--qr-emblem-lo,#7d5c1c)"/>` +
	`</linearGradient></defs>`

// qrStyle is the SVG's self-contained stylesheet.
var qrStyle = fmt.Sprintf(`<style>`+
	`.qr-plate{fill:var(--qr-plate,#fdfaf3)}`+
	`.qr-modules{fill:var(--qr-ink,#2b2520)}`+
	`.qr-finder-ring{fill:none;stroke:var(--qr-ink,#2b2520);stroke-width:1}`+
	`.qr-finder-pupil{fill:var(--qr-ink,#2b2520)}`+
	`.qr-medallion{fill:var(--qr-plate,#fdfaf3);stroke:var(--qr-emblem-line,#c9ac72);stroke-width:%[1]s}`+
	`.qr-medallion-inner{fill:none;stroke:var(--qr-emblem-line,#c9ac72);stroke-width:%[2]s}`+
	`.qr-emblem-cross{fill:url(#qr-leaf)}`+
	`</style>`,
	qrNum(qrMedallionRule), qrNum(qrMedallionRule*0.75))
