package render

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"rsc.io/qr"
)

const testShareURL = "https://office.fly.dev/"

// moduleRect matches one data module's <rect x="N" y="N"/>. Geometry comes
// from the SVG's own stylesheet, so the element carries coordinates only.
var moduleRect = regexp.MustCompile(`<rect x="(\d+)" y="(\d+)"/>`)

// drawnModules reads back the set of data modules the SVG actually paints,
// in plate coordinates.
func drawnModules(t *testing.T, svg string) map[[2]int]bool {
	t.Helper()
	drawn := make(map[[2]int]bool)
	for _, m := range moduleRect.FindAllStringSubmatch(svg, -1) {
		var x, y int
		if _, err := fmt.Sscan(m[1], &x); err != nil {
			t.Fatalf("parsing x from %q: %v", m[0], err)
		}
		if _, err := fmt.Sscan(m[2], &y); err != nil {
			t.Fatalf("parsing y from %q: %v", m[0], err)
		}
		drawn[[2]int{x, y}] = true
	}
	return drawn
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

	emblemLow := (code.Size-qrEmblemSize)/2 + qrQuietZone
	if want := fmt.Sprintf(`class="qr-emblem-ground" x="%.3f" y="%.3f"`, float64(emblemLow), float64(emblemLow)); !strings.Contains(string(svg), want) {
		t.Errorf("emblem should sit on the knockout (%s)", want)
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
	for _, want := range []string{"<style>", "var(--qr-plate,#", "var(--qr-ink,#", "width:1px;height:1px"} {
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
