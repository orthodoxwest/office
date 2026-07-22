package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/review"
)

func TestSplitLeadingVerseNumber(t *testing.T) {
	tests := []struct {
		line     string
		wantNum  string
		wantRest string
		wantOK   bool
	}{
		{"2. That thy way may be known", "2", "That thy way may be known", true},
		{"10. Make me a clean heart", "10", "Make me a clean heart", true},
		{"2 O ye Angels of the Lord", "2", "O ye Angels of the Lord", true},
		{"20 Blessed art thou, O Lord", "20", "Blessed art thou, O Lord", true},
		{"O ALL ye Works of the Lord", "", "O ALL ye Works of the Lord", false},
		{"2.No space after period", "", "2.No space after period", false},
	}
	for _, tt := range tests {
		num, rest, ok := splitLeadingVerseNumber(tt.line)
		if ok != tt.wantOK || num != tt.wantNum || rest != tt.wantRest {
			t.Errorf("splitLeadingVerseNumber(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.line, num, rest, ok, tt.wantNum, tt.wantRest, tt.wantOK)
		}
	}
}

func TestRenderBenediciteSpaceNumberedVerses(t *testing.T) {
	html := string(renderPsalmVerses("Song of the Three Children\n\n" +
		"O ALL ye Works of the Lord, bless ye the Lord: * praise him, and magnify him forever.\n" +
		"2 O ye Angels of the Lord, bless ye the Lord: * O ye Heavens, bless ye the Lord.\n" +
		"10 O let the Earth bless the Lord: * yea, let it praise him, and magnify him for ever.\n"))

	if !strings.Contains(html, `<p class="verse numbered"><span class="verse-num">2</span>`) {
		t.Fatalf("expected space-style verse 2 to use verse-num: %s", html)
	}
	if !strings.Contains(html, `<span class="verse-num">10</span>`) {
		t.Fatalf("expected space-style verse 10 to use verse-num: %s", html)
	}
	if strings.Contains(html, `>2 O ye Angels`) {
		t.Fatalf("verse number must not remain in the body text: %s", html)
	}
}

func TestRenderSectionElementsMergesPsalmDoxologyIntoPsalmBlock(t *testing.T) {
	html := string(renderSectionElements([]models.OfficeElement{
		{Type: models.Psalm, Label: "Psalm 67", Text: "Psalm 67\n\n1. Be merciful unto us * and bless us."},
		{Type: models.PsalmDoxology, Text: "Glory be to the Father,\nas it was in the beginning."},
	}))

	if !strings.Contains(html, `<div class="psalm">`) {
		t.Fatalf("expected psalm wrapper in output: %s", html)
	}
	if !strings.Contains(html, `<div class="psalm"><h3 class="item-label">Psalm 67</h3>`) {
		t.Fatalf("expected psalm label inside wrapper: %s", html)
	}
	if !strings.Contains(html, `<div class="psalm-verses">`) {
		t.Fatalf("expected rendered psalm verses in output: %s", html)
	}
	if !strings.Contains(html, `<p class="gloria-patri">`) {
		t.Fatalf("expected gloria patri in output: %s", html)
	}
	if !strings.Contains(html, `</div><p class="gloria-patri">`) {
		t.Fatalf("expected gloria patri to be rendered immediately after the psalm verses inside the psalm block: %s", html)
	}
	if !strings.HasSuffix(html, `</p></div>`) {
		t.Fatalf("expected psalm block to close after the gloria patri: %s", html)
	}
}

func TestRenderCollectReflowsProseAndPreservesSemanticLines(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Collect,
		Text: "Almighty God, who hast brought us\nto the beginning of this day.\n\nV. O Lord, hear my prayer.\nR. And let my cry come unto thee.",
	}, "")

	if !strings.Contains(html, `<p class="plain-line">Almighty God, who hast brought us to the beginning of this day.</p>`) {
		t.Fatalf("expected source-wrapped prose to flow as one paragraph: %s", html)
	}
	if strings.Contains(html, `brought us<br>to`) {
		t.Fatalf("source wrapping must not produce a hard break: %s", html)
	}
	if !strings.Contains(html, `<span class="sigil">℣.</span>`) || !strings.Contains(html, `<span class="sigil">℟.</span>`) {
		t.Fatalf("expected versicle and response to remain semantic lines: %s", html)
	}
	if !strings.Contains(html, `<div class="liturgical-gap"></div>`) {
		t.Fatalf("expected a blank source line to retain paragraph spacing: %s", html)
	}
}

func TestRenderPrayerReflowsSourceLines(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Prayer,
		Text: "Thy kingdom come.\nThy will be done.",
	}, "")

	if !strings.Contains(html, `Thy kingdom come. Thy will be done.`) {
		t.Fatalf("expected prayer source lines to reflow: %s", html)
	}
}

func TestRenderSecretPrayerVoiceSpans(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Prayer,
		Text: "Our Father, who art in heaven.\nThy kingdom come.",
		Voice: []models.VoiceSpan{
			{Text: "Our Father", Spoken: true},
			{Text: ", who art in heaven.\nThy kingdom come.", Spoken: false},
		},
	}, "")

	if !strings.Contains(html, `<span class="spoken-text">Our Father</span>`) {
		t.Fatalf("expected spoken incipit: %s", html)
	}
	if !strings.Contains(html, `<span class="secret-text">, who art in heaven.</span>`) {
		t.Fatalf("expected silent body start: %s", html)
	}
	if !strings.Contains(html, `<span class="secret-text">Thy kingdom come.</span>`) {
		t.Fatalf("expected silent continuation after reflow: %s", html)
	}
	if !strings.Contains(html, `heaven.</span> <span class="secret-text">Thy kingdom`) {
		t.Fatalf("expected reflowed space between source lines: %s", html)
	}
}

func TestRenderPartlySecretPrayerVoiceSpans(t *testing.T) {
	html := renderOfficeElement(models.OfficeElement{
		Type: models.Prayer,
		Text: "Our Father, middle.\nAnd lead us not into temptation,\nBut deliver us from evil.",
		Voice: []models.VoiceSpan{
			{Text: "Our Father", Spoken: true},
			{Text: ", middle.\n", Spoken: false},
			{Text: "And lead us not into temptation,\nBut deliver us from evil.", Spoken: true},
		},
	}, "")

	if !strings.Contains(html, `<span class="spoken-text">Our Father</span><span class="secret-text">, middle.</span>`) {
		t.Fatalf("expected spoken incipit then silent middle: %s", html)
	}
	if !strings.Contains(html, `<span class="spoken-text">And lead us not into temptation,</span>`) {
		t.Fatalf("expected spoken tail start: %s", html)
	}
	if !strings.Contains(html, `<span class="spoken-text">But deliver us from evil.</span>`) {
		t.Fatalf("expected spoken tail continuation: %s", html)
	}
}

func TestRenderMarianAntiphonPreservesVerseAndReflowsPrayer(t *testing.T) {
	text := "[Ave Regina Caelorum]\n\nQueen of the heavens, we hail thee,\nHail thee, Lady of all the Angels;\n\nV. Vouchsafe that I may praise thee.\nR. Give me strength.\n\nLet us pray.\n\nGrant us, O merciful God, protection in our weakness:\nthat we may rise again from our sins."
	html := string(renderMarianAntiphon(text))

	if !strings.Contains(html, `<p class="chant-line">Queen of the heavens, we hail thee,</p><p class="chant-line">Hail thee, Lady of all the Angels;</p>`) {
		t.Fatalf("expected each opening Marian verse line as its own chant line: %s", html)
	}
	if !strings.Contains(html, `Grant us, O merciful God, protection in our weakness: that we may rise again from our sins.`) {
		t.Fatalf("expected the concluding Marian prayer to flow: %s", html)
	}
	if strings.Contains(html, `<br>`) {
		t.Fatalf("Marian antiphon should use per-line blocks, not hard breaks: %s", html)
	}
}

func TestRenderMarianAntiphonStylesIncipitMediant(t *testing.T) {
	text := "Mary we hail thee * Mother and Queen compassionate;\nMary our comfort, life, and hope, we hail thee."
	html := string(renderMarianAntiphon(text))

	if !strings.Contains(html, `<p class="chant-line">Mary we hail thee <span class="mediant">*</span> Mother and Queen compassionate;</p>`) {
		t.Fatalf("expected the incipit mediant styled like a psalm verse: %s", html)
	}
}

func TestRenderHymnStanzasPreservesVerseLines(t *testing.T) {
	html := string(renderHymnStanzas("Latin title\n\nFirst verse line,\nSecond verse line.\n\nAnother stanza."))

	if !strings.Contains(html, `<p class="hymn-stanza">First verse line,<br>Second verse line.</p>`) {
		t.Fatalf("expected hymn verse lines to remain hard-wrapped: %s", html)
	}
}

func TestShowVettingBannerDependsOnReviewHash(t *testing.T) {
	hour := &models.OfficeHour{
		Hour:   "lauds",
		Title:  "Lauds",
		Season: models.Pentecost,
		Feast:  "Trinity Sunday",
		Color:  models.White,
		Sections: []models.OfficeSection{
			{
				Label: "The Collect",
				Elements: []models.OfficeElement{
					{Type: models.Collect, Text: "Almighty and everlasting God..."},
				},
			},
		},
	}

	s := &Server{reviewed: map[string]bool{review.HashHour(hour): true}}
	if s.showVettingBanner(hour) {
		t.Fatal("expected vetted hour to hide the construction banner")
	}

	hour.Sections[0].Elements[0].Text = "Changed text."
	if !s.showVettingBanner(hour) {
		t.Fatal("expected changed or unreviewed hour to show the construction banner")
	}
}

func TestHourAssuranceCountsDependenciesWithoutSourceContents(t *testing.T) {
	hour := &models.OfficeHour{
		Hour: "lauds",
		Sections: []models.OfficeSection{{Elements: []models.OfficeElement{
			{Type: models.Collect, Text: "A collect.", SourceRef: "proper/example/collect", SourceRefs: []string{"proper/example/collect"}},
			{Type: models.Psalm, Text: "A psalm.", SourceRef: "psalms/001", SourceRefs: []string{"psalms/001"}},
			{Type: models.Chapter, Text: "A chapter.", SourceRef: "proper/example/chapter", SourceRefs: []string{"proper/example/chapter"}},
		}}},
		Decisions: []models.CompositionDecision{{Rule: "occurrence:higher-rank", Outcome: "challenger-wins"}},
	}
	s := &Server{provenance: map[string]review.EntryProvenance{
		"proper/example/collect": {Key: "proper/example/collect", Status: review.ProvenanceVerified},
		"psalms/001":             {Key: "psalms/001", Status: review.ProvenanceNeedsReview},
	}}
	got := s.hourAssurance(hour, "lauds", "2026-01-01")
	if got.Verified != 1 || got.NeedsReview != 1 || got.SourceUnknown != 1 || len(got.Dependencies) != 3 {
		t.Fatalf("assurance = %#v", got)
	}
	if len(got.Decisions) != 1 || got.Decisions[0].Rule != "occurrence:higher-rank" {
		t.Fatalf("decisions = %#v", got.Decisions)
	}
	foundPsalmReview := false
	for _, dependency := range got.Dependencies {
		if dependency.Key == "psalms/001" && strings.Contains(dependency.ReportURL, "psalms%2F001") {
			foundPsalmReview = true
		}
	}
	if !foundPsalmReview {
		t.Fatal("report URL does not identify the psalm dependency")
	}
}

func TestHourPageAssuranceDisclosureIsCollapsedAndSourceSafe(t *testing.T) {
	s, err := New("../../data", ":0")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleRoot(rec, httptest.NewRequest(http.MethodGet, "/lauds/2026-06-07", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<details class="assurance-panel">`,
		`<details class="site-menu" open>`,
		`class="today-link"`,
		`class="hour-continuation"`,
		`href="/prime/2026-06-07"`,
		"Text dependencies",
		"Composition decisions",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("hour page missing %q", want)
		}
	}
	for _, want := range []string{"need review", "source unknown"} {
		if !strings.Contains(body, want) {
			t.Errorf("hour page missing assurance category %q", want)
		}
	}
	for _, retired := range []string{" documented", "undocumented"} {
		if strings.Contains(body, retired) {
			t.Errorf("hour page contains retired assurance category %q", retired)
		}
	}
	for _, forbidden := range []string{"SOURCE:", ".txt", "/home/", "../resources"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("hour page leaks source metadata %q", forbidden)
		}
	}
}

func TestAdjacentHoursKeepDateAndTheme(t *testing.T) {
	// theme arg is ignored; appearance is client-side only (no ?theme= on links).
	previousName, previousLink, nextName, nextLink := adjacentHours("sext", "2026-06-07", "dark")
	if previousName != "Terce" || previousLink != "/terce/2026-06-07" {
		t.Errorf("previous hour = %q %q", previousName, previousLink)
	}
	if nextName != "None" || nextLink != "/none/2026-06-07" {
		t.Errorf("next hour = %q %q", nextName, nextLink)
	}

	previousName, previousLink, _, _ = adjacentHours("lauds", "2026-06-07", "")
	if previousName != "" || previousLink != "" {
		t.Errorf("lauds should not have a previous hour, got %q %q", previousName, previousLink)
	}

	_, _, nextName, nextLink = adjacentHours("compline", "2026-06-07", "")
	if nextName != "" || nextLink != "" {
		t.Errorf("compline should not have a next hour, got %q %q", nextName, nextLink)
	}
}

func TestHandle404DoesNotShowVettingBanner(t *testing.T) {
	s, err := New("../../data", ":0")
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	s.handle404(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), `id="site-banner"`) {
		t.Fatal("404 page should not show the vetting banner")
	}
}
