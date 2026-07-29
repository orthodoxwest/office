package office

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func makeDay(year int, month time.Month, day int, celebration *models.Feast, comms []*models.Feast, withinOctave string) *models.CalendarDay {
	return &models.CalendarDay{
		Date:           time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
		Season:         models.Lent,
		Celebration:    celebration,
		Commemorations: comms,
		Color:          models.Violet,
		WithinOctaveOf: withinOctave,
	}
}

func TestShouldSayPreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "ferial day — preces",
			day:  makeDay(2026, 3, 11, nil, nil, ""),
			want: true,
		},
		{
			name: "simple feast — preces",
			day: makeDay(2026, 3, 11,
				&models.Feast{ID: "test", Rank: models.Simple}, nil, ""),
			want: true,
		},
		{
			name: "semi-double feast — preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "test", Rank: models.SemiDouble}, nil, ""),
			want: true,
		},
		{
			name: "double feast — no preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "test", Rank: models.Double}, nil, ""),
			want: false,
		},
		{
			name: "1st class feast — no preces",
			day: makeDay(2026, 4, 12,
				&models.Feast{ID: "easter-sunday", Rank: models.Double1stClass}, nil, ""),
			want: false,
		},
		{
			name: "Sunday with elevated precedence rank — preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "lent-sunday-3", Rank: models.Double1stClass, Category: models.CategorySunday}, nil, ""),
			want: true,
		},
		{
			name: "feria with elevated precedence rank — preces",
			day: makeDay(2026, 2, 25,
				&models.Feast{ID: "ash-wednesday", Rank: models.Double1stClass, Category: models.CategoryFeria}, nil, ""),
			want: true,
		},
		{
			name: "elevated Sunday with double commemoration — no preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "lent-sunday-3", Rank: models.Double1stClass, Category: models.CategorySunday},
				[]*models.Feast{{ID: "test", Rank: models.Double}}, ""),
			want: false,
		},
		{
			name: "Vigil of Pentecost is a Double feria — no preces",
			day: makeDay(2026, 5, 30,
				&models.Feast{ID: "vigil-pentecost", Rank: models.Double1stClass, Category: models.CategoryFeria}, nil, ""),
			want: false,
		},
		{
			name: "Vigil of the Nativity is a Double feria — no preces",
			day: makeDay(2026, 12, 24,
				&models.Feast{ID: "vigil-nativity", Rank: models.Double1stClass, Category: models.CategoryFeria}, nil, ""),
			want: false,
		},
		{
			name: "All Souls is a Double feria — no preces",
			day: makeDay(2026, 11, 2,
				&models.Feast{ID: "all-souls", Rank: models.Double, Category: models.CategoryFeria}, nil, ""),
			want: false,
		},
		{
			name: "within octave — no preces",
			day:  makeDay(2026, 4, 13, nil, nil, "easter-sunday"),
			want: false,
		},
		{
			name: "simple octave-day office (Jan 2, Octave Day of St Stephen) — no preces",
			day: makeDay(2026, 1, 2,
				&models.Feast{ID: "octave-day-st-stephen", Rank: models.Simple}, nil, ""),
			want: false,
		},
		{
			name: "double commemoration — no preces",
			day: makeDay(2026, 3, 11, nil,
				[]*models.Feast{{ID: "test", Rank: models.Double}}, ""),
			want: false,
		},
		{
			name: "octave commemoration — no preces",
			day: makeDay(2026, 3, 11, nil,
				[]*models.Feast{{ID: "test-octave-day-3", Rank: models.SemiDouble}}, ""),
			want: false,
		},
		{
			name: "vigil of Epiphany (Jan 5) — no preces",
			day:  makeDay(2026, 1, 5, nil, nil, ""),
			want: false,
		},
		{
			name: "Friday after Ascension octave — no preces",
			day: makeDay(2026, 5, 29, // Easter+47 = Apr 12 + 47 = May 29
				nil, nil, ""),
			want: false,
		},
		{
			name: "Eastertide Sunday (IV after Easter) — no preces",
			day: func() *models.CalendarDay {
				d := makeDay(2026, 5, 10,
					&models.Feast{ID: "easter-sunday-4", Rank: models.SemiDouble, Category: models.CategorySunday},
					nil, "")
				d.Season = models.Easter
				return d
			}(),
			want: false,
		},
		{
			name: "Eastertide feria — preces",
			day: func() *models.CalendarDay {
				d := makeDay(2026, 5, 11, nil, nil, "")
				d.Season = models.Easter
				return d
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSayPreces(tt.day, moveable)
			if got != tt.want {
				t.Errorf("shouldSayPreces() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComposeComplineUsesEveningOfficeContext(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	days, err := calendar.BuildCalendar(2026, dataDir)
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	engine, err := NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)

	byDate := make(map[string]*models.CalendarDay, len(days))
	for i := range days {
		byDate[days[i].Date.Format("2006-01-02")] = &days[i]
	}
	compose := func(t *testing.T, date string) (*models.CalendarDay, *models.OfficeHour) {
		t.Helper()
		day := byDate[date]
		if day == nil {
			t.Fatalf("calendar day %s not found", date)
		}
		hour, err := engine.ComposeHour("compline", day, moveable)
		if err != nil {
			t.Fatalf("ComposeHour(compline): %v", err)
		}
		return day, hour
	}
	composeFirst := func(t *testing.T, date string) (*models.CalendarDay, *models.OfficeHour) {
		t.Helper()
		day, hour := compose(t, date)
		if day.Vespers.Owner != models.VespersIOfFollowing {
			t.Fatalf("%s Vespers owner = %v, want I Vespers of following", date, day.Vespers.Owner)
		}
		return day, hour
	}

	t.Run("feast and preces follow incoming office", func(t *testing.T) {
		day, hour := composeFirst(t, "2026-01-16")

		if hour.Date != day.Date {
			t.Errorf("Compline date = %s, want civil evening %s", hour.Date, day.Date)
		}
		if hour.Feast != day.Vespers.Feast.Name {
			t.Errorf("Compline feast = %q, want %q", hour.Feast, day.Vespers.Feast.Name)
		}
		if hour.Color != models.White || hour.Color != day.Vespers.Color {
			t.Errorf("Compline color = %s, want incoming white", hour.Color)
		}
		if hour.Season != day.Vespers.Season {
			t.Errorf("Compline season = %s, want incoming %s", hour.Season, day.Vespers.Season)
		}
		if hasOfficeSectionLabel(hour, "Preces") {
			t.Error("Compline includes preces at I Vespers of an incoming Double")
		}

		assertDecision(t, hour.Decisions, "context:office", "feria", "")
		assertDecision(t, hour.Decisions, "office-context:office", "celebration", day.Vespers.Feast.ID)
		assertDecision(t, hour.Decisions, "office-context:first-vespers", "yes", "")
		assertDecision(t, hour.Decisions, "preces", PrecesSuppressedDoubleOffice, "")
	})

	t.Run("seasonal opening follows incoming Sunday", func(t *testing.T) {
		_, hour := composeFirst(t, "2026-02-07")

		if hour.Season != models.Septuagesima || hour.Color != models.Violet {
			t.Fatalf("Compline season/color = %s/%s, want septuagesima/violet", hour.Season, hour.Color)
		}
		alleluia := officeElementBySlot(hour, "alleluia")
		if alleluia == nil {
			t.Fatal("Compline opening seasonal acclamation not found")
		}
		if alleluia.Text != "Praise be to thee, O Lord, King of eternal glory." {
			t.Errorf("Compline opening = %q, want Septuagesima acclamation", alleluia.Text)
		}
		if alleluia.SourceRef != "seasonal/septuagesima/alleluia" {
			t.Errorf("Compline opening source = %q, want seasonal/septuagesima/alleluia", alleluia.SourceRef)
		}
	})

	t.Run("Marian handoff remains Compline-specific", func(t *testing.T) {
		_, hour := composeFirst(t, "2026-02-01")

		if hour.Feast != "Purification of the Blessed Virgin Mary" {
			t.Errorf("Compline feast = %q, want Purification", hour.Feast)
		}
		marian := officeElementBySlot(hour, "marian-antiphon")
		if marian == nil {
			t.Fatal("Compline Marian antiphon not found")
		}
		if marian.SourceRef != "ordinary/marian/alma-redemptoris-christmas" {
			t.Errorf("Feb 1 Compline Marian source = %q, want Alma Redemptoris", marian.SourceRef)
		}
	})

	t.Run("Holy Saturday keeps its exceptional Compline form", func(t *testing.T) {
		_, hour := composeFirst(t, "2026-04-11")

		if hour.Feast != "Sunday of the Resurrection" || hour.Season != models.Easter {
			t.Errorf("Holy Saturday Compline office = %q/%s, want Easter office", hour.Feast, hour.Season)
		}
		nunc := officeElementBySlot(hour, "nunc-dimittis-antiphon")
		if nunc == nil {
			t.Fatal("Holy Saturday Compline lost its proper Nunc dimittis")
		}
		if nunc.SourceRef != "proper/holy-saturday/nunc-dimittis-antiphon" {
			t.Errorf("Holy Saturday Nunc source = %q, want proper/holy-saturday", nunc.SourceRef)
		}
		if officeElementByType(hour, models.Hymn) != nil {
			t.Error("Holy Saturday Compline unexpectedly includes the ordinary hymn")
		}
		marian := officeElementBySlot(hour, "marian-antiphon")
		if marian == nil || marian.SourceRef != "ordinary/marian/regina-caeli" {
			t.Errorf("Holy Saturday Marian antiphon = %#v, want Regina Caeli", marian)
		}
	})

	t.Run("second Vespers remains the current office", func(t *testing.T) {
		day, hour := compose(t, "2026-01-21")

		if day.Vespers.Owner != models.VespersIIOfPreceding {
			t.Fatalf("Vespers owner = %v, want II Vespers of preceding", day.Vespers.Owner)
		}
		if hour.Feast != day.Celebration.Name {
			t.Errorf("Compline feast = %q, want current office %q", hour.Feast, day.Celebration.Name)
		}
		if hour.Color != day.Color || hour.Season != day.Season {
			t.Errorf("Compline color/season = %s/%s, want current %s/%s",
				hour.Color, hour.Season, day.Color, day.Season)
		}
	})
}

func hasOfficeSectionLabel(hour *models.OfficeHour, label string) bool {
	for _, section := range hour.Sections {
		if section.Label == label {
			return true
		}
	}
	return false
}

func officeElementBySlot(hour *models.OfficeHour, slot string) *models.OfficeElement {
	for si := range hour.Sections {
		for ei := range hour.Sections[si].Elements {
			elem := &hour.Sections[si].Elements[ei]
			if elem.SlotRef == slot {
				return elem
			}
		}
	}
	return nil
}

func officeElementByType(hour *models.OfficeHour, typ models.ElementType) *models.OfficeElement {
	for si := range hour.Sections {
		for ei := range hour.Sections[si].Elements {
			elem := &hour.Sections[si].Elements[ei]
			if elem.Type == typ {
				return elem
			}
		}
	}
	return nil
}
