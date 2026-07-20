package office

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestMinorHourWeekdayCondition(t *testing.T) {
	tests := []struct {
		name      string
		date      time.Time
		condition string
		want      bool
	}{
		{"Sunday matches weekday-sunday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-sunday", true},
		{"Sunday does not match weekday-monday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-monday", false},
		{"Tuesday matches weekday-tuesday", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC), "weekday-tuesday", true},
		{"Saturday matches weekday-saturday", time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC), "weekday-saturday", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date}
			got := evaluateCondition(tt.condition, day, nil)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestMinorHourPreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "ferial day — preces",
			day:  makeDay(2026, 3, 16, nil, nil, ""),
			want: true,
		},
		{
			name: "double feast — no preces",
			day:  makeDay(2026, 3, 15, &models.Feast{ID: "test", Rank: models.Double}, nil, ""),
			want: false,
		},
		{
			name: "within octave — no preces",
			day:  makeDay(2026, 4, 13, nil, nil, "easter-sunday"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition("if-preces", tt.day, moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(if-preces) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShortResponsoryVersicle(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
		ok   bool
	}{
		{
			name: "standard responsory breve",
			text: "R. God shall help her * with his countenance.\n" +
				"R. God shall help her * with his countenance.\n" +
				"V. God is in the midst of her, therefore shall she not be removed.\n" +
				"R. With his countenance.\n" +
				"Glory be to the Father.",
			want: "V. God shall help her with his countenance.\n" +
				"R. God is in the midst of her, therefore shall she not be removed.",
			ok: true,
		},
		{
			name: "seasonal responsory without marked repetition",
			text: "R. Redeem me, O Lord, and have mercy on me, alleluia, alleluia.\n" +
				"Redeem me, O Lord, and have mercy on me, alleluia, alleluia.\n" +
				"V. My foot standeth in an even place.",
			want: "V. Redeem me, O Lord, and have mercy on me, alleluia, alleluia.\n" +
				"R. My foot standeth in an even place.",
			ok: true,
		},
		{
			name: "malformed source",
			text: "No marked response or verse",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := shortResponsoryVersicle(tt.text)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("shortResponsoryVersicle() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestResolveMinorHourVersicle(t *testing.T) {
	t.Run("ordinary Sunday uses the direct parish versicle", func(t *testing.T) {
		corpus := texts.NewTestCorpus(map[string]string{
			"ordinary/terce/short-responsory": "R. Seeded ordinary response.\nV. Seeded ordinary verse.",
			"ordinary/terce/versicle":         "V. Weekday versicle.\nR. Weekday response.",
			"ordinary/terce/versicle-sunday":  "V. Sunday versicle.\nR. Sunday response.",
		})
		day := &models.CalendarDay{Date: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)}

		got := resolveMinorHourVersicle(day, "terce", corpus)
		if got.Type != models.Versicle ||
			got.Text != "V. Sunday versicle.\nR. Sunday response." ||
			got.SourceRef != "ordinary/terce/versicle-sunday" {
			t.Fatalf("ordinary Sunday versicle = %+v", got)
		}
	})

	t.Run("feast converts its proper responsory breve", func(t *testing.T) {
		corpus := texts.NewTestCorpus(map[string]string{
			"proper/example/short-responsory-terce": "R. Proper opening * response.\nR. Proper opening * response.\nV. Proper verse.\nR. Response.",
			"ordinary/terce/short-responsory":       "R. Ordinary response.\nV. Ordinary verse.",
			"ordinary/terce/versicle":               "V. Ordinary versicle.\nR. Ordinary response.",
		})
		day := &models.CalendarDay{
			Date:        time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
			Celebration: &models.Feast{ID: "example", Category: models.CategoryLord},
		}

		got := resolveMinorHourVersicle(day, "terce", corpus)
		if got.Type != models.Versicle ||
			got.Text != "V. Proper opening response.\nR. Proper verse." ||
			got.SourceRef != "proper/example/short-responsory-terce" {
			t.Fatalf("proper feast versicle = %+v", got)
		}
	})

	t.Run("Paschaltide decorates both lines once", func(t *testing.T) {
		corpus := texts.NewTestCorpus(map[string]string{
			"seasonal/easter/short-responsory-terce": "R. Seasonal response, alleluia, alleluia.\nSeasonal response, alleluia, alleluia.\nV. Seasonal verse.",
			"ordinary/terce/short-responsory":        "R. Ordinary response.\nV. Ordinary verse.",
			"ordinary/terce/versicle":                "V. Ordinary versicle.\nR. Ordinary response.",
		})
		day := &models.CalendarDay{
			Date:   time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			Season: models.Easter,
		}

		got := resolveMinorHourVersicle(day, "terce", corpus)
		if got.Text != "V. Seasonal response, alleluia.\nR. Seasonal verse, alleluia." {
			t.Fatalf("Paschal seasonal versicle = %q", got.Text)
		}
	})

	t.Run("Paschaltide decorates a feast common", func(t *testing.T) {
		corpus := texts.NewTestCorpus(map[string]string{
			"commons/confessor/short-responsory-terce": "R. The Lord loved him * and adorned him.\nR. The Lord loved him * and adorned him.\nV. He clothed him with a robe of glory.",
			"ordinary/terce/short-responsory":          "R. Ordinary response.\nV. Ordinary verse.",
			"ordinary/terce/versicle":                  "V. Ordinary versicle.\nR. Ordinary response.",
		})
		day := &models.CalendarDay{
			Date:        time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			Season:      models.Easter,
			Celebration: &models.Feast{ID: "example", Category: models.CategoryConfessor},
		}

		got := resolveMinorHourVersicle(day, "terce", corpus)
		if got.Text != "V. The Lord loved him and adorned him, alleluia.\nR. He clothed him with a robe of glory, alleluia." {
			t.Fatalf("Paschal common versicle = %q", got.Text)
		}
	})
}

func TestMinorHourResponsorySourcesAreConvertible(t *testing.T) {
	corpus, err := texts.LoadTexts(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}

	var checked int
	for _, ref := range corpus.References() {
		if !strings.HasSuffix(ref, "/short-responsory-terce") &&
			!strings.HasSuffix(ref, "/short-responsory-sext") &&
			!strings.HasSuffix(ref, "/short-responsory-none") {
			continue
		}
		checked++
		if _, ok := shortResponsoryVersicle(corpus.Get(ref)); !ok {
			t.Errorf("%s cannot be reduced to a Little Hours versicle", ref)
		}
	}
	if checked == 0 {
		t.Fatal("no Little Hours responsory sources checked")
	}
}
