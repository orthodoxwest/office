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
