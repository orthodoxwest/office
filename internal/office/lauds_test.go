package office

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestLaudsWeekdayCondition(t *testing.T) {
	composer := &LaudsComposer{}

	tests := []struct {
		name      string
		date      time.Time
		condition string
		want      bool
	}{
		{"Sunday matches weekday-sunday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-sunday", true},
		{"Sunday does not match weekday-monday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-monday", false},
		{"Monday matches weekday-monday", time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), "weekday-monday", true},
		{"Saturday matches weekday-saturday", time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC), "weekday-saturday", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date}
			got := evaluateCondition(tt.condition, day, composer.Moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestLaudsPreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)
	composer := &LaudsComposer{Moveable: moveable}

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
			got := evaluateCondition("if-preces", tt.day, composer.Moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(if-preces) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveProperText(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/psalm-antiphon":      "Ordinary antiphon",
		"seasonal/lent/psalm-antiphon":       "Lenten antiphon",
		"proper/christmas/psalm-antiphon":    "Christmas antiphon",
		"ordinary/lauds/benedictus-antiphon": "Ordinary Benedictus",
	})

	tests := []struct {
		name    string
		day     *models.CalendarDay
		ref     string
		wantTxt string
	}{
		{
			name:    "feast-specific proper wins",
			day:     &models.CalendarDay{Season: models.Lent, Celebration: &models.Feast{ID: "christmas"}},
			ref:     "psalm-antiphon",
			wantTxt: "Christmas antiphon",
		},
		{
			name:    "seasonal fallback when no feast proper",
			day:     &models.CalendarDay{Season: models.Lent, Celebration: &models.Feast{ID: "unknown-feast"}},
			ref:     "psalm-antiphon",
			wantTxt: "Lenten antiphon",
		},
		{
			name:    "ordinary fallback when no seasonal",
			day:     &models.CalendarDay{Season: models.Easter, Celebration: &models.Feast{ID: "unknown-feast"}},
			ref:     "psalm-antiphon",
			wantTxt: "Ordinary antiphon",
		},
		{
			name:    "ordinary fallback with no celebration",
			day:     &models.CalendarDay{Season: models.Pentecost},
			ref:     "benedictus-antiphon",
			wantTxt: "Ordinary Benedictus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTxt, _ := resolveProperText(tt.day, "lauds", tt.ref, corpus)
			if gotTxt != tt.wantTxt {
				t.Errorf("resolveProperText() text = %q, want %q", gotTxt, tt.wantTxt)
			}
		})
	}
}

func TestLaudsCommemorations(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/commemoration-antiphon":   "Default antiphon",
		"ordinary/lauds/commemoration-versicle":   "Default versicle",
		"ordinary/lauds/commemoration-collect":    "Default collect",
		"proper/st-andrew/commemoration-antiphon": "Andrew antiphon",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Commemorations: []*models.Feast{
			{ID: "st-andrew", Name: "St. Andrew"},
		},
	}

	elems := addCommemorations(day, "lauds", corpus)

	if len(elems) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(elems))
	}

	// Heading
	if elems[0].Type != models.Heading {
		t.Errorf("element 0: want Heading, got %s", elems[0].Type)
	}
	if elems[0].Text != "Commemoration of St. Andrew" {
		t.Errorf("element 0 text = %q", elems[0].Text)
	}

	// Antiphon — should use feast-specific
	if elems[1].Text != "Andrew antiphon" {
		t.Errorf("element 1 text = %q, want feast-specific antiphon", elems[1].Text)
	}

	// Versicle — should use default
	if elems[2].Text != "Default versicle" {
		t.Errorf("element 2 text = %q, want default versicle", elems[2].Text)
	}

	// Collect — should use default
	if elems[3].Text != "Default collect" {
		t.Errorf("element 3 text = %q, want default collect", elems[3].Text)
	}
}

func TestComposeLaudsSundayPsalmodyOmitsFestalPsalms(t *testing.T) {
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
		Season: models.Pentecost,
		Color:  models.Green,
	}

	hour, err := engine.ComposeHour("lauds", day, calendar.ComputeMoveableDates(2026))
	if err != nil {
		t.Fatalf("ComposeHour(lauds): %v", err)
	}

	var psalmLabels []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.Type == models.Psalm {
				psalmLabels = append(psalmLabels, elem.Label)
			}
		}
	}

	if len(psalmLabels) < 7 {
		t.Fatalf("got %d psalms, want at least 7: %v", len(psalmLabels), psalmLabels)
	}

	wantPrefix := []string{"Psalm 67", "Psalm 51", "Psalm 118", "Psalm 63"}
	for i, want := range wantPrefix {
		if psalmLabels[i] != want {
			t.Fatalf("psalm %d = %q, want %q (all psalms: %v)", i, psalmLabels[i], want, psalmLabels)
		}
	}

	for _, got := range psalmLabels {
		if got == "Psalm 93" || got == "Psalm 100" {
			t.Fatalf("Sunday Lauds still contains festal psalm %q: %v", got, psalmLabels)
		}
	}
}

func TestAddCommemorationsUsesProperIDAlias(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/pentecost-sunday-23/commemoration-antiphon": "XXIII Pentecost antiphon",
		"ordinary/lauds/commemoration-versicle":             "Default versicle",
		"ordinary/lauds/commemoration-collect":              "Default collect",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC),
		Season: models.Epiphany,
		Commemorations: []*models.Feast{
			{
				ID:       "epiphany-sunday-7",
				Name:     "VII Sunday after Epiphany",
				ProperID: "pentecost-sunday-23",
			},
		},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if len(elems) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(elems))
	}
	if elems[1].Text != "XXIII Pentecost antiphon" {
		t.Fatalf("element 1 text = %q, want ProperID-backed antiphon", elems[1].Text)
	}
}
