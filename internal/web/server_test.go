package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/review"
)

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
		`<details class="site-menu">`,
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
