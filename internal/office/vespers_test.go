package office

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestVespersWeekdayCondition(t *testing.T) {
	composer := &VespersComposer{}

	tests := []struct {
		name      string
		date      time.Time
		first     bool
		condition string
		want      bool
	}{
		{"Sunday matches weekday-sunday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), false, "weekday-sunday", true},
		{"Sunday does not match weekday-monday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), false, "weekday-monday", false},
		{"Monday matches weekday-monday", time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), false, "weekday-monday", true},
		{"Saturday matches weekday-saturday", time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC), false, "weekday-saturday", true},
		{"Sunday first Vespers uses civil Saturday", time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC), true, "weekday-saturday", true},
		{"Sunday first Vespers is not civil Sunday", time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC), true, "weekday-sunday", false},
		{"Low Sunday first Vespers uses civil Saturday", time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC), true, "weekday-saturday", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date, FirstVespers: tt.first}
			if tt.first {
				day.Celebration = &models.Feast{Category: models.CategorySunday}
				if tt.name == "Low Sunday first Vespers uses civil Saturday" {
					day.Celebration.Category = models.CategoryLord
				}
			}
			got := evaluateCondition(tt.condition, day, composer.Moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestVespersPreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)
	composer := &VespersComposer{Moveable: moveable}

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

func TestVespersProperAntiphonFallback(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/vespers/psalm-antiphon":      "Vespers antiphon",
		"ordinary/lauds/psalm-antiphon":        "Lauds antiphon",
		"ordinary/vespers/magnificat-antiphon": "Vespers Magnificat",
	})

	day := &models.CalendarDay{Season: models.Pentecost}

	// Vespers should use ordinary/vespers/, not ordinary/lauds/
	gotTxt, _ := resolveProperText(day, "vespers", "psalm-antiphon", corpus)
	if gotTxt != "Vespers antiphon" {
		t.Errorf("resolveProperText(vespers) = %q, want %q", gotTxt, "Vespers antiphon")
	}

	// Lauds should still use ordinary/lauds/
	gotTxt, _ = resolveProperText(day, "lauds", "psalm-antiphon", corpus)
	if gotTxt != "Lauds antiphon" {
		t.Errorf("resolveProperText(lauds) = %q, want %q", gotTxt, "Lauds antiphon")
	}
}

func TestVespersUsesFollowingOfficeWhenConcurrenceSaysFirstVespers(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers": "Joseph psalm antiphon",
		"proper/st-cyril/psalm-antiphon-1-vespers":  "Cyril psalm antiphon",
		"proper/st-joseph/collect":                  "Joseph collect",
		"proper/st-cyril/collect":                   "Cyril collect",
		"ordinary/vespers/opening-versicle":         "Opening versicle",
	})
	sections := []HourSection{
		{
			Name:      "Psalmody-Thursday",
			Label:     "Psalmody",
			Condition: "weekday-thursday",
			Elements: []HourElement{
				{Type: "proper-antiphon", Ref: "psalm-antiphon-1"},
			},
		},
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Color:  models.Violet,
		Celebration: &models.Feast{
			ID:       "st-cyril",
			Name:     "St Cyril",
			Rank:     models.Double,
			Color:    models.White,
			Category: models.CategoryConfessorDoctor,
		},
		Commemorations: []*models.Feast{
			{ID: "st-edward", Name: "St Edward", Rank: models.Commemoration},
		},
		Vespers: models.VespersDesignation{
			Owner:  models.VespersIOfFollowing,
			Feast:  &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:  models.White,
			Season: models.Lent,
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	if hour.Date != day.Date {
		t.Fatalf("hour date = %s, want civil date %s", hour.Date, day.Date)
	}
	if hour.Feast != "St Joseph" {
		t.Fatalf("hour feast = %q, want St Joseph", hour.Feast)
	}
	if hour.Color != models.White {
		t.Fatalf("hour color = %s, want white", hour.Color)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Joseph psalm antiphon") || !strings.Contains(joined, "Joseph collect") {
		t.Fatalf("expected following feast texts, got:\n%s", joined)
	}
	if strings.Contains(joined, "Cyril") || strings.Contains(joined, "St Edward") {
		t.Fatalf("expected preceding office texts and commemorations to be omitted, got:\n%s", joined)
	}
}

func TestVespersCommemoratesOutgoingOfficeAtFirstVespersOfFollowing(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers": "Joseph psalm antiphon",
		"proper/st-joseph/collect":                  "Joseph collect",
		"commons/confessor/commemoration-antiphon":  "Cyril commemoration antiphon",
		"commons/confessor/commemoration-collect":   "Cyril commemoration collect",
	})
	sections := []HourSection{
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Celebration: &models.Feast{
			ID: "st-cyril", Name: "St Cyril", Rank: models.Double, Category: models.CategoryConfessor,
		},
		Vespers: models.VespersDesignation{
			Owner:          models.VespersIOfFollowing,
			Feast:          &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:          models.White,
			Season:         models.Lent,
			Commemorations: []*models.Feast{{ID: "st-cyril", Name: "St Cyril", Rank: models.Double, Category: models.CategoryConfessor}},
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Commemoration of St Cyril") {
		t.Fatalf("expected commemoration of the outgoing office, got:\n%s", joined)
	}
}

func TestVespersWithoutOwnerUsesFollowingDayCommemorations(t *testing.T) {
	current := &models.Feast{ID: "current", Name: "Current Memorial", Rank: models.Commemoration}
	incoming := &models.Feast{ID: "incoming", Name: "Incoming Memorial", Rank: models.Commemoration}
	day := &models.CalendarDay{
		Date:           time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC),
		Season:         models.Epiphany,
		Color:          models.Green,
		Commemorations: []*models.Feast{current},
		Vespers: models.VespersDesignation{
			Owner:          models.VespersNotApplicable,
			Commemorations: []*models.Feast{incoming},
			Rule:           "concurrence:neither-office-has-rights",
		},
	}

	officeDay := vespersOfficeDay(day)
	if officeDay == day {
		t.Fatal("expected a Vespers-specific day copy")
	}
	if len(officeDay.Commemorations) != 1 || officeDay.Commemorations[0] != incoming {
		t.Fatalf("commemorations = %#v, want incoming memorial only", officeDay.Commemorations)
	}
	if officeDay.Date != day.Date || officeDay.Color != day.Color {
		t.Fatalf("no-owner Vespers changed office context: %#v", officeDay)
	}
}

func TestVespersCommemoratesIncomingOfficeAtSecondVespersOfPreceding(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers": "Joseph psalm antiphon",
		"proper/st-joseph/collect":                  "Joseph collect",
		"commons/confessor/commemoration-antiphon":  "Turibius commemoration antiphon",
		"commons/confessor/commemoration-collect":   "Turibius commemoration collect",
	})
	sections := []HourSection{
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Celebration: &models.Feast{
			ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White,
		},
		Vespers: models.VespersDesignation{
			Owner:          models.VespersIIOfPreceding,
			Feast:          &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:          models.White,
			Season:         models.Lent,
			Commemorations: []*models.Feast{{ID: "st-turibius", Name: "St Turibius", Rank: models.Double, Category: models.CategoryConfessor}},
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Commemoration of St Turibius") {
		t.Fatalf("expected commemoration of the incoming office, got:\n%s", joined)
	}
}

func TestVespersUsesPrecedingOfficeWhenConcurrenceSaysSecondVespers(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers":   "Joseph psalm antiphon",
		"proper/st-joseph/collect":                    "Joseph collect",
		"proper/st-cuthbert/psalm-antiphon-1-vespers": "Cuthbert psalm antiphon",
		"proper/st-cuthbert/collect":                  "Cuthbert collect",
	})
	sections := []HourSection{
		{
			Name:      "Psalmody-Thursday",
			Label:     "Psalmody",
			Condition: "weekday-thursday",
			Elements: []HourElement{
				{Type: "proper-antiphon", Ref: "psalm-antiphon-1"},
			},
		},
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Color:  models.White,
		Celebration: &models.Feast{
			ID:    "st-joseph",
			Name:  "St Joseph",
			Rank:  models.Double2ndClass,
			Color: models.White,
		},
		Vespers: models.VespersDesignation{
			Owner:  models.VespersIIOfPreceding,
			Feast:  &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:  models.White,
			Season: models.Lent,
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	if hour.Date != day.Date {
		t.Fatalf("hour date = %s, want civil date %s", hour.Date, day.Date)
	}
	if hour.Feast != "St Joseph" {
		t.Fatalf("hour feast = %q, want St Joseph", hour.Feast)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Joseph psalm antiphon") || !strings.Contains(joined, "Joseph collect") {
		t.Fatalf("expected preceding feast texts, got:\n%s", joined)
	}
	if strings.Contains(joined, "Cuthbert") {
		t.Fatalf("expected following feast texts to be omitted, got:\n%s", joined)
	}
}
