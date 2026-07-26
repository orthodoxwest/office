package render

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestRenderPsalmLabelCarriesLatinIncipit(t *testing.T) {
	html := string(renderSectionElements([]models.OfficeElement{{
		Type:    models.Psalm,
		Label:   "Psalm 67",
		Incipit: "Deus misereatur nostri",
		Text:    "Psalm 67\n\n1. Be merciful unto us * and bless us.",
	}}))

	want := `<h3 class="item-label">Psalm 67` +
		`<span class="label-sep" aria-hidden="true"> · </span>` +
		`<span class="psalm-incipit" lang="la">Deus misereatur nostri</span></h3>`
	if !strings.Contains(html, want) {
		t.Errorf("expected the incipit inside the label:\nwant %s\ngot  %s", want, html)
	}
}

// Canticles take the same treatment as psalms — the diurnal labels the
// Magnificat by its incipit too.
func TestRenderCanticleLabelCarriesLatinIncipit(t *testing.T) {
	html := string(renderSectionElements([]models.OfficeElement{{
		Type:    models.Canticle,
		Label:   "Magnificat",
		Incipit: "Magnificat anima mea Dominum",
		Text:    "!Luke 1:46-55\n\n1. My soul doth magnify the Lord * and my spirit.",
	}}))

	if !strings.Contains(html, `<div class="canticle">`) {
		t.Fatalf("expected the canticle wrapper: %s", html)
	}
	if !strings.Contains(html, `<span class="psalm-incipit" lang="la">Magnificat anima mea Dominum</span>`) {
		t.Errorf("expected the canticle's incipit in the label: %s", html)
	}
}

// Without a recorded incipit the label must render exactly as before, with no
// empty span and no orphaned separator.
func TestRenderPsalmLabelWithoutIncipitIsUnchanged(t *testing.T) {
	html := string(renderSectionElements([]models.OfficeElement{{
		Type:  models.Psalm,
		Label: "Psalm 67",
		Text:  "Psalm 67\n\n1. Be merciful unto us * and bless us.",
	}}))

	if !strings.Contains(html, `<h3 class="item-label">Psalm 67</h3>`) {
		t.Errorf("expected the bare label: %s", html)
	}
	if strings.Contains(html, "label-sep") || strings.Contains(html, "psalm-incipit") {
		t.Errorf("expected no incipit markup at all: %s", html)
	}
}

// The incipit is attacker-adjacent only insofar as it is data, but it is
// still interpolated into markup, so it must be escaped like every other
// corpus-supplied string.
func TestRenderPsalmIncipitIsEscaped(t *testing.T) {
	html := string(renderSectionElements([]models.OfficeElement{{
		Type:    models.Psalm,
		Label:   "Psalm 67",
		Incipit: `Deus <b>misereatur</b> & nostri`,
		Text:    "Psalm 67\n\n1. Be merciful unto us * and bless us.",
	}}))

	if strings.Contains(html, "<b>misereatur</b>") {
		t.Errorf("incipit markup was not escaped: %s", html)
	}
	if !strings.Contains(html, "&lt;b&gt;misereatur&lt;/b&gt; &amp; nostri") {
		t.Errorf("expected the escaped incipit: %s", html)
	}
}
