package render

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"rsc.io/qr"
)

const testShareURL = "https://office.fly.dev/"

// moduleMove matches the absolute move that opens each module's subpath.
// Modules are merged into continuous strokes, so there is no per-module
// element to count: the subpaths of the single <path> are the modules.
var moduleMove = regexp.MustCompile(`M(\d*\.?\d+) (\d*\.?\d+)`)

// drawnModules reads back the set of data modules the SVG actually paints,
// in plate coordinates. Each subpath opens at the module's top edge — x is
// px or px+radius, y is px exactly — so flooring both recovers the cell.
// See the contract on qrModulePath.
func drawnModules(t *testing.T, svg string) map[[2]int]bool {
	t.Helper()
	start := strings.Index(svg, `class="qr-modules" d="`)
	if start < 0 {
		t.Fatal("no module path in the SVG")
	}
	d := svg[start:]
	d = d[:strings.Index(d, `"/>`)]

	drawn := make(map[[2]int]bool)
	for _, m := range moduleMove.FindAllStringSubmatch(d, -1) {
		x, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatalf("parsing x from %q: %v", m[0], err)
		}
		y, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			t.Fatalf("parsing y from %q: %v", m[0], err)
		}
		if y != math.Trunc(y) {
			t.Fatalf("subpath %q should open on a module's top edge, but y is fractional", m[0])
		}
		drawn[[2]int{int(math.Floor(x)), int(y)}] = true
	}
	return drawn
}

// TestQRModulesMergeIntoStrokes guards the artwork's defining property: a
// corner is rounded only where the ink actually turns. If every corner
// rounded, adjacent modules would pinch into beads instead of reading as one
// stroke; if none did, this would be a plain grid again.
func TestQRModulesMergeIntoStrokes(t *testing.T) {
	svg, err := QRCodeSVG(testShareURL)
	if err != nil {
		t.Fatal(err)
	}
	code, err := qr.Encode(testShareURL, qr.H)
	if err != nil {
		t.Fatal(err)
	}
	emblemLow := (code.Size-qrEmblemSize)/2 + qrQuietZone

	// An isolated module is a disc: four arcs. A module in the middle of a
	// horizontal run turns nowhere along that run and has none.
	var isolated, interior int
	for y := range code.Size {
		for x := range code.Size {
			if !qrDark(code, emblemLow, x, y) {
				continue
			}
			up := qrDark(code, emblemLow, x, y-1)
			down := qrDark(code, emblemLow, x, y+1)
			left := qrDark(code, emblemLow, x-1, y)
			right := qrDark(code, emblemLow, x+1, y)
			switch {
			case !up && !down && !left && !right:
				isolated++
			case left && right && !up && !down:
				interior++
			}
		}
	}
	if isolated == 0 || interior == 0 {
		t.Fatalf("test needs both shapes present in the sample symbol "+
			"(isolated=%d, run-interior=%d)", isolated, interior)
	}

	// Four arcs per isolated module, none for a run interior. Counting arcs
	// in the whole path pins the rule without re-deriving the geometry.
	start := strings.Index(string(svg), `class="qr-modules" d="`)
	d := string(svg)[start:]
	d = d[:strings.Index(d, `"/>`)]
	arcs := strings.Count(d, "A")
	if arcs < 4*isolated {
		t.Errorf("path has %d arcs, fewer than the %d the %d isolated modules alone require",
			arcs, 4*isolated, isolated)
	}
	// Every module would carry four arcs if corners rounded unconditionally.
	if ceiling := 4 * len(drawnModules(t, string(svg))); arcs >= ceiling {
		t.Errorf("path has %d arcs of a possible %d: corners are rounding even where "+
			"a run continues, so modules will pinch instead of merging", arcs, ceiling)
	}
}

// TestQRCodeSVGPaintsTheSymbol is the correctness test for the artwork: every
// module the encoder marks black is painted, and nothing else is. A rounded
// corner or a bespoke finder may change how a module looks, but changing
// *which* modules are dark would silently encode a different URL.
func TestQRCodeSVGPaintsTheSymbol(t *testing.T) {
	svg, err := QRCodeSVG(testShareURL)
	if err != nil {
		t.Fatal(err)
	}
	code, err := qr.Encode(testShareURL, qr.H)
	if err != nil {
		t.Fatal(err)
	}

	drawn := drawnModules(t, string(svg))
	emblemLow := (code.Size-qrEmblemSize)/2 + qrQuietZone

	for y := range code.Size {
		for x := range code.Size {
			px, py := x+qrQuietZone, y+qrQuietZone
			// Finders are redrawn as bosses and the emblem square is
			// deliberately blank; both are checked separately below.
			if qrInFinder(x, y, code.Size) || qrInEmblem(px, py, emblemLow) {
				if drawn[[2]int{px, py}] {
					t.Errorf("module (%d,%d) painted inside a reserved area", x, y)
				}
				continue
			}
			if got, want := drawn[[2]int{px, py}], code.Black(x, y); got != want {
				t.Errorf("module (%d,%d): painted=%v, encoder says black=%v", x, y, got, want)
			}
		}
	}
}

// TestQRCodeSVGQuietZone guards the four-module margin scanners use to find
// the symbol's edge: no module may be painted inside it, and the plate must
// cover it. A code cropped to its modules is a code that stops scanning.
func TestQRCodeSVGQuietZone(t *testing.T) {
	svg, err := QRCodeSVG(testShareURL)
	if err != nil {
		t.Fatal(err)
	}
	code, err := qr.Encode(testShareURL, qr.H)
	if err != nil {
		t.Fatal(err)
	}
	extent := code.Size + 2*qrQuietZone

	// Stated as a literal rather than against qrQuietZone: the specification's
	// four-module minimum is the thing being defended, and an assertion
	// derived from the constant would move along with a bug that shrank it.
	if qrQuietZone < 4 {
		t.Errorf("quiet zone is %d modules; the QR specification requires at least 4", qrQuietZone)
	}

	if want := fmt.Sprintf(`viewBox="0 0 %d %d"`, extent, extent); !strings.Contains(string(svg), want) {
		t.Errorf("viewBox should span the symbol plus both quiet zones (%s)", want)
	}
	if want := fmt.Sprintf(`<rect class="qr-plate" width="%d" height="%d"`, extent, extent); !strings.Contains(string(svg), want) {
		t.Errorf("plate should cover the whole viewBox (%s)", want)
	}

	for pos := range drawnModules(t, string(svg)) {
		for _, v := range pos {
			if v < qrQuietZone || v >= extent-qrQuietZone {
				t.Errorf("module at %v intrudes on the quiet zone", pos)
				break
			}
		}
	}
}

// TestQRCodeSVGFindersAndEmblem checks the two bespoke pieces are present and
// where a scanner expects them: three corner bosses on the 7x7 footprints,
// and the cross centred in its knockout.
func TestQRCodeSVGFindersAndEmblem(t *testing.T) {
	svg, err := QRCodeSVG(testShareURL)
	if err != nil {
		t.Fatal(err)
	}
	code, err := qr.Encode(testShareURL, qr.H)
	if err != nil {
		t.Fatal(err)
	}
	extent := code.Size + 2*qrQuietZone
	far := float64(extent-qrQuietZone-qrFinderSize) + 0.5

	for _, want := range []string{
		fmt.Sprintf(`<rect class="qr-finder-ring" x="%.1f" y="%.1f"`, float64(qrQuietZone)+0.5, float64(qrQuietZone)+0.5),
		fmt.Sprintf(`<rect class="qr-finder-ring" x="%.1f" y="%.1f"`, far, float64(qrQuietZone)+0.5),
		fmt.Sprintf(`<rect class="qr-finder-ring" x="%.1f" y="%.1f"`, float64(qrQuietZone)+0.5, far),
	} {
		if !strings.Contains(string(svg), want) {
			t.Errorf("missing corner boss: %s", want)
		}
	}
	if n := strings.Count(string(svg), `class="qr-finder-pupil"`); n != 3 {
		t.Errorf("expected 3 finder pupils, got %d", n)
	}

	// The medallion is inscribed in the knockout: concentric with it, and
	// never wider, or it would cover live modules.
	emblemLow := (code.Size-qrEmblemSize)/2 + qrQuietZone
	centre := float64(emblemLow) + qrEmblemSize/2.0
	for _, want := range []string{
		fmt.Sprintf(`class="qr-medallion" cx="%.3f" cy="%.3f" r="%.3f"`, centre, centre, qrEmblemSize/2.0),
		fmt.Sprintf(`class="qr-medallion-inner" cx="%.3f" cy="%.3f" r="%.3f"`, centre, centre, qrEmblemSize/2.0-qrMedallionGap),
	} {
		if !strings.Contains(string(svg), want) {
			t.Errorf("medallion should sit concentric in the knockout (%s)", want)
		}
	}
	// The knockout is centred, so the margin either side must be equal.
	if lead, trail := emblemLow-qrQuietZone, code.Size-qrEmblemSize-(emblemLow-qrQuietZone); lead != trail {
		t.Errorf("emblem off centre: %d modules before, %d after", lead, trail)
	}
}

// TestQRKnockoutBudget keeps the cross well inside what error correction can
// recover. Level H tolerates roughly 30% damage; this holds the knockout to a
// third of that, leaving the rest of the budget for the real world — a creased
// card, a photocopy, a phone camera at an angle.
func TestQRKnockoutBudget(t *testing.T) {
	const ceiling = 0.10
	// The smallest symbol is the worst case: a fixed knockout is a larger
	// share of a smaller code. Short URLs are the ones parishes will use.
	for _, url := range []string{"https://a.co", testShareURL, "https://office.example.org/"} {
		code, err := qr.Encode(url, qr.H)
		if err != nil {
			t.Fatal(err)
		}
		ratio := float64(qrEmblemSize*qrEmblemSize) / float64(code.Size*code.Size)
		if ratio > ceiling {
			t.Errorf("%s: knockout covers %.1f%% of a %dx%d symbol, over the %.0f%% ceiling",
				url, ratio*100, code.Size, code.Size, ceiling*100)
		}
	}
}

// TestQRCodeSVGSelfContained: the SVG must render on its own. It is printed,
// and it is served to a browser that may not have style.css yet, so geometry
// and colour fallbacks travel with it.
func TestQRCodeSVGSelfContained(t *testing.T) {
	svg, err := QRCodeSVG(testShareURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<style>", "var(--qr-plate,#", "var(--qr-ink,#",
		"var(--qr-emblem-line,#", "<defs>", "var(--qr-emblem-hi,#", "url(#qr-leaf)",
	} {
		if !strings.Contains(string(svg), want) {
			t.Errorf("SVG should carry %q so it renders without the site stylesheet", want)
		}
	}
}

// TestQRCodeSVGEscapesURL: the encoded address is derived from the request
// host, so it must not be able to break out of the aria-label attribute.
func TestQRCodeSVGEscapesURL(t *testing.T) {
	svg, err := QRCodeSVG(`https://x/" onload="alert(1)`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(svg), `onload="alert(1)"`) || strings.Contains(string(svg), `/" onload`) {
		t.Errorf("URL escaped out of the aria-label: %s", firstTag(string(svg)))
	}
	if !strings.Contains(string(svg), "&#34;") {
		t.Errorf("expected the quote to be escaped: %s", firstTag(string(svg)))
	}
}

func firstTag(svg string) string {
	if i := strings.Index(svg, ">"); i >= 0 {
		return svg[:i+1]
	}
	return svg
}
