package render

import (
	"strings"
	"testing"
)

func TestLayoutIncludesConstructionBanner(t *testing.T) {
	src, err := TemplateSource("layout.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
		`class="site-banner"`,
		`id="site-banner"`,
		`data-dismiss-banner`,
		`under active development`,
		// Build-stamped static URLs.
		`{{static "style.css"}}`,
		`{{static "app.js"}}`,
		// data-nav markers for client-side dated href stamping.
		`data-nav="home"`,
		`data-nav="hour"`,
		`data-hour="lauds"`,
		`data-nav="calendar"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout is missing construction banner markup %q", want)
		}
	}
}

func TestLayoutIncludesTextSizeControl(t *testing.T) {
	src, err := TemplateSource("layout.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
		// Footer control, mirroring the appearance switch.
		`class="text-size-switch" role="group" aria-label="Text size"`,
		`data-text-size-choice="small"`,
		`data-text-size-choice="default"`,
		`data-text-size-choice="large"`,
		// Glyph buttons must still name themselves for screen readers.
		`aria-label="Smaller text"`,
		`aria-label="Default text size"`,
		`aria-label="Larger text"`,
		`title="Smaller text"`,
		`title="Larger text"`,
		`aria-pressed="false"`,
		// Pre-paint: size applied before the stylesheet loads, no flash.
		`localStorage.getItem("office-text-size")`,
		`setAttribute("data-text-size", s)`,
		`removeAttribute("data-text-size")`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout is missing text size control markup %q", want)
		}
	}

	// The pre-paint script must run before the stylesheet, or the size flashes.
	prePaint := strings.Index(body, `localStorage.getItem("office-text-size")`)
	styles := strings.Index(body, `{{static "style.css"}}`)
	if prePaint < 0 || styles < 0 || prePaint > styles {
		t.Errorf("text size pre-paint script must precede the stylesheet link")
	}

	// Appearance stays client-side: no query parameter, no server round trip.
	if strings.Contains(body, "text-size=") && !strings.Contains(body, `data-text-size=`) {
		t.Errorf("text size must not be stamped onto URLs")
	}
}

func TestCalendarFishUsesSprite(t *testing.T) {
	src, err := TemplateSource("calendar.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if !strings.Contains(body, `id="icon-fish"`) {
		t.Errorf("calendar should define a single fish symbol")
	}
	if !strings.Contains(body, `href="#icon-fish"`) {
		t.Errorf("calendar fish instances should use <use href=\"#icon-fish\">")
	}
	// Path data should appear once (in the symbol), not in the fish template body.
	if strings.Count(body, `M1 6 C5 1.2`) != 1 {
		t.Errorf("fish path data should appear once in the sprite, not per instance")
	}
}
