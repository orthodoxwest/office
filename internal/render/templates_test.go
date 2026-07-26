package render

import (
	"strings"
	"testing"
)

func TestHourIncludesInlineConstructionBanner(t *testing.T) {
	src, err := TemplateSource("hour.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
		`{{if .ShowBanner}}`,
		`<aside class="site-banner"`,
		`class="site-banner"`,
		`id="site-banner"`,
		`aria-label="Development notice"`,
		`data-dismiss-banner`,
		`under active development`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("hour is missing inline construction banner markup %q", want)
		}
	}
	headerAt := strings.Index(body, `class="hour-header"`)
	bannerAt := strings.Index(body, `class="site-banner"`)
	elementsAt := strings.Index(body, `class="elements"`)
	if headerAt < 0 || bannerAt < 0 || elementsAt < 0 || !(headerAt < bannerAt && bannerAt < elementsAt) {
		t.Error("construction notice should sit after the hour header and before the prayer elements")
	}

	layout, err := TemplateSource("layout.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(layout), `class="site-banner"`) {
		t.Error("shared layout should not render the hour-only construction banner")
	}
}

func TestLayoutIncludesStampedNavigationAndAssets(t *testing.T) {
	src, err := TemplateSource("layout.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
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
			t.Errorf("layout is missing stamped navigation or asset markup %q", want)
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

func TestHomeGroupsHoursWithoutBreakingClientSelectors(t *testing.T) {
	src, err := TemplateSource("home.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
		`class="home-hero day-color-{{.Color}}"`,
		`class="home-prayer-card"`,
		`class="pray-now"`,
		`class="home-hour-links"`,
		`home-hour-group-morning`,
		`home-hour-group-day`,
		`home-hour-group-evening`,
		`id="home-hours-morning">Morning`,
		`id="home-hours-day">Day`,
		`id="home-hours-evening">Evening`,
		`data-hour="{{.Slug}}"`,
		`aria-current="time"`,
		`home-hour-link-name`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("home is missing grouped-hour prototype markup %q", want)
		}
	}

	dayAt := strings.Index(body, `class="home-summary`)
	prayerAt := strings.Index(body, `class="home-prayer-card"`)
	metaAt := strings.Index(body, `class="home-day-meta"`)
	if dayAt < 0 || prayerAt < 0 || metaAt < 0 || !(dayAt < prayerAt && prayerAt < metaAt) {
		t.Error("home source order should be day identity, prayer invitation, then date control")
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
