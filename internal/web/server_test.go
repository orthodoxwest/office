package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/review"
)

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
