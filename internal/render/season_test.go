package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestSeasonClassVeilsPassiontideAndBrightensPaschaltide(t *testing.T) {
	for _, tc := range []struct {
		season models.Season
		want   string
	}{
		{models.Passiontide, "season-passiontide"},
		{models.Easter, "season-eastertide"},
		// Every other season keeps the ordinary gold. Lent in particular is
		// deliberately unveiled: the veiling begins at Passion Sunday, not on
		// Ash Wednesday.
		{models.Lent, ""},
		{models.Advent, ""},
		{models.Christmas, ""},
		{models.Epiphany, ""},
		{models.Septuagesima, ""},
		{models.Pentecost, ""},
		{models.Season(""), ""},
		{models.Season("nonsense"), ""},
	} {
		if got := SeasonClass(tc.season); got != tc.want {
			t.Errorf("SeasonClass(%q) = %q, want %q", tc.season, got, tc.want)
		}
	}
}

func TestLayoutStampsSeasonClassOnBody(t *testing.T) {
	pages, err := New("test")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		seasonClass string
		want        string
	}{
		{"veiled", SeasonClassPassiontide, `<body class="page-home season-passiontide">`},
		{"bright", SeasonClassEastertide, `<body class="page-home season-eastertide">`},
		// No season class must leave the body class byte-identical to what it
		// was before seasonal variation existed — no stray trailing space.
		{"ordinary", "", `<body class="page-home">`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			data := HomeData{
				DateStr:     "Sunday, March 22, 2026",
				DateSlug:    "2026-03-22",
				Page:        "home",
				SeasonClass: tc.seasonClass,
			}
			if err := pages.Home(&buf, data); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), tc.want) {
				t.Errorf("home page body tag is not %q", tc.want)
			}
		})
	}
}

// The year calendar spans every season at once, so it must never be tinted.
func TestCalendarPageStaysSeasonNeutral(t *testing.T) {
	pages, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := pages.Calendar(&buf, CalendarData{Year: 2026, Page: "calendar"}); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	if !strings.Contains(body, `<body class="page-calendar">`) {
		t.Error("calendar body tag carries something other than its page class")
	}
	for _, forbidden := range []string{SeasonClassPassiontide, SeasonClassEastertide} {
		if strings.Contains(body, forbidden) {
			t.Errorf("calendar page carries season class %q", forbidden)
		}
	}
}
