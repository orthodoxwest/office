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

func composePrimeWithMartyrology(t *testing.T, day *models.CalendarDay, entries map[string]string) *models.OfficeSection {
	t.Helper()
	sections, err := ParseHourDefinition(filepath.Join("..", "..", "data", "office", "prime.txt"))
	if err != nil {
		t.Fatalf("ParseHourDefinition: %v", err)
	}
	corpusEntries := map[string]string{
		"ordinary/prime/martyrology-rubric": "Read the Martyrology.",
		"ordinary/martyrology/conclusion":   "Many other holy ones.",
		"ordinary/martyrology/response":     "Thanks be to God.",
	}
	for key, value := range entries {
		corpusEntries[key] = value
	}
	hour, err := (&PrimeComposer{MartyrologyPreview: true}).Compose(day, sections, texts.NewTestCorpus(corpusEntries), calendar.ComputeMoveableDates(day.Date.Year()))
	if err != nil {
		t.Fatalf("Prime.Compose: %v", err)
	}
	for i := range hour.Sections {
		if len(hour.Sections[i].Elements) == 0 {
			continue
		}
		for _, elem := range hour.Sections[i].Elements {
			if elem.SourceRef == "ordinary/prime/martyrology-rubric" || strings.HasPrefix(elem.SourceRef, "ordinary/martyrology/") {
				return &hour.Sections[i]
			}
		}
	}
	return nil
}

func TestResolvePrimeMartyrologyUsesFollowingCivilDate(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/prime/martyrology-rubric": "Read the Martyrology.",
		"ordinary/martyrology/09-08":        "Saint of the eighth.",
		"ordinary/martyrology/conclusion":   "Many other holy ones.",
		"ordinary/martyrology/response":     "Thanks be to God.",
	})
	day := &models.CalendarDay{Date: time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)}
	got := resolvePrimeMartyrology(day, corpus)
	if len(got) != 4 {
		t.Fatalf("got %d elements, want heading, readings, and response: %+v", len(got), got)
	}
	if got[0].Type != models.Heading || got[0].Text != "Martyrology — September 8" {
		t.Fatalf("heading = %+v", got[0])
	}
	if got[1].Type != models.Reading || got[1].Text != "Saint of the eighth." || got[1].SourceRef != "ordinary/martyrology/09-08" {
		t.Fatalf("date chapter = %+v", got[1])
	}
	if got[2].Type != models.Reading || got[2].Text != "Many other holy ones." || got[2].SourceRef != "ordinary/martyrology/conclusion" {
		t.Fatalf("conclusion = %+v", got[2])
	}
	if got[3].Type != models.Response || got[3].Text != "R. Thanks be to God." || got[3].SourceRef != "ordinary/martyrology/response" {
		t.Fatalf("response = %+v", got[3])
	}
}

func TestResolvePrimeMartyrologyFallsBackToRubric(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/prime/martyrology-rubric": "Read the Martyrology.",
	})
	day := &models.CalendarDay{Date: time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)}
	got := resolvePrimeMartyrology(day, corpus)
	if len(got) != 1 || got[0].Type != models.Rubric || got[0].Text != "Read the Martyrology." {
		t.Fatalf("fallback = %+v", got)
	}
}

func TestResolvePrimeMartyrologyHandlesLeapDay(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/prime/martyrology-rubric": "Read the Martyrology.",
		"ordinary/martyrology/02-29":        "Leap day saint.",
		"ordinary/martyrology/conclusion":   "Many other holy ones.",
		"ordinary/martyrology/response":     "Thanks be to God.",
	})
	day := &models.CalendarDay{Date: time.Date(2028, time.February, 28, 0, 0, 0, 0, time.UTC)}
	got := resolvePrimeMartyrology(day, corpus)
	if len(got) != 4 || got[1].Text != "Leap day saint." {
		t.Fatalf("leap-day result = %+v", got)
	}
}

func TestPrimeComposeMartyrologyRolloverAndDST(t *testing.T) {
	zone := time.FixedZone("EDT", -4*60*60)
	tests := []struct {
		name  string
		date  time.Time
		entry string
		want  string
	}{
		{"December rollover", time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC), "ordinary/martyrology/01-01", "Martyrology — January 1"},
		{"non-leap February", time.Date(2027, time.February, 28, 0, 0, 0, 0, time.UTC), "ordinary/martyrology/03-01", "Martyrology — March 1"},
		{"leap February", time.Date(2028, time.February, 28, 0, 0, 0, 0, time.UTC), "ordinary/martyrology/02-29", "Martyrology — February 29"},
		{"DST boundary", time.Date(2026, time.March, 7, 0, 0, 0, 0, zone), "ordinary/martyrology/03-08", "Martyrology — March 8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := composePrimeWithMartyrology(t, &models.CalendarDay{Date: tt.date, Season: models.Lent}, map[string]string{tt.entry: "Reviewed entry."})
			if section == nil || len(section.Elements) != 4 {
				t.Fatalf("Martyrology section = %+v", section)
			}
			if section.Elements[0].Text != tt.want || section.Elements[1].Text != "Reviewed entry." ||
				section.Elements[2].Text != "Many other holy ones." || section.Elements[3].Text != "R. Thanks be to God." {
				t.Fatalf("Martyrology elements = %+v", section.Elements)
			}
		})
	}
}

func TestPrimeComposeOmitsMartyrologyOnTriduum(t *testing.T) {
	day := &models.CalendarDay{
		Date:        time.Date(2026, time.April, 2, 0, 0, 0, 0, time.UTC),
		Season:      models.Lent,
		Celebration: &models.Feast{ID: "holy-thursday", Rank: models.Double1stClass},
	}
	sections, err := ParseHourDefinition(filepath.Join("..", "..", "data", "office", "prime.txt"))
	if err != nil {
		t.Fatalf("ParseHourDefinition: %v", err)
	}
	hour, err := (&PrimeComposer{MartyrologyPreview: true}).Compose(day, sections, texts.NewTestCorpus(map[string]string{
		"ordinary/prime/martyrology-rubric": "Read the Martyrology.",
		"ordinary/martyrology/04-03":        "Reviewed entry.",
	}), calendar.ComputeMoveableDates(2026))
	if err != nil {
		t.Fatalf("Prime.Compose: %v", err)
	}
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.SourceRef == "ordinary/prime/martyrology-rubric" || strings.HasPrefix(elem.SourceRef, "ordinary/martyrology/") {
				t.Fatal("Prime included Martyrology during the Triduum")
			}
		}
	}
}

func TestPrimeComposeReviewedSeptemberMartyrology(t *testing.T) {
	sections, err := ParseHourDefinition(filepath.Join("..", "..", "data", "office", "prime.txt"))
	if err != nil {
		t.Fatalf("ParseHourDefinition: %v", err)
	}
	corpus, err := texts.LoadTexts(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	tests := []struct {
		name    string
		date    time.Time
		heading string
		keep    []string
		omit    []string
	}{
		{
			name:    "September 7 announces September 8",
			date:    time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC),
			heading: "Martyrology — September 8",
			keep:    []string{"The Nativity", "St. Corbinian"},
			omit:    []string{"Thomas Villanova", "Peter Claver"},
		},
		{
			name:    "September 8 announces September 9",
			date:    time.Date(2026, time.September, 8, 0, 0, 0, 0, time.UTC),
			heading: "Martyrology — September 9",
			keep:    []string{"St. Kieran"},
			omit:    []string{"Thomas Villanova", "Peter Claver"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date, Season: models.Pentecost}
			hour, err := (&PrimeComposer{MartyrologyPreview: true}).Compose(day, sections, corpus, calendar.ComputeMoveableDates(tt.date.Year()))
			if err != nil {
				t.Fatalf("Prime.Compose: %v", err)
			}
			var body strings.Builder
			headingCount, conclusionCount, responseCount := 0, 0, 0
			for _, section := range hour.Sections {
				for _, elem := range section.Elements {
					body.WriteString(elem.Text)
					body.WriteByte('\n')
					switch {
					case elem.Type == models.Heading && elem.Text == tt.heading:
						headingCount++
					case elem.SourceRef == "ordinary/martyrology/conclusion":
						conclusionCount++
					case elem.SourceRef == "ordinary/martyrology/response":
						responseCount++
					}
				}
			}
			if headingCount != 1 || conclusionCount != 1 || responseCount != 1 {
				t.Fatalf("Martyrology counts heading=%d conclusion=%d response=%d", headingCount, conclusionCount, responseCount)
			}
			text := body.String()
			for _, want := range tt.keep {
				if !strings.Contains(text, want) {
					t.Errorf("Martyrology omitted expected notice %q", want)
				}
			}
			for _, unwanted := range tt.omit {
				if strings.Contains(text, unwanted) {
					t.Errorf("Martyrology retained omitted notice %q", unwanted)
				}
			}
		})
	}
}
