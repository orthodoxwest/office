package office

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// Diurnal pp. 360–361 and 2026 ordo p. 53: the Vigil supplies abbreviated
// Vespers. Outside Mass it has the Benedicamus with two Alleluias, Fidelium,
// and a final silent Pater, with nothing more. Easter still owns the office.
func TestHolySaturdayVigilVespers(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		moveable := calendar.ComputeMoveableDates(year)
		vigilDays := 0
		for i := range days {
			day := &days[i]
			if day.Celebration == nil || day.Celebration.ID != "holy-saturday" {
				continue
			}
			vigilDays++
			for _, form := range models.PrayerForms {
				t.Run(fmt.Sprintf("%d/%s", year, form), func(t *testing.T) {
					hour, err := engine.ComposeHourWithOptions("vespers", day, moveable, ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					if hour.Season != models.Easter || hour.Color != models.White || hour.Feast != day.Vespers.Feast.Name || !hour.Date.Equal(day.Date) {
						t.Fatalf("Vigil lost Easter ownership or civil date: %+v", hour)
					}
					greeting := "shared/leader/greeting-clergy"
					if form == models.PrayerPrivate {
						greeting = "shared/leader/greeting-lay"
					}
					want := []string{
						"ordinary/shared/our-father", "ordinary/shared/hail-mary",
						"shared/formulas/paschal-psalm-antiphon", "psalms/117", "ordinary/shared/gloria-patri", "shared/formulas/paschal-psalm-antiphon",
						"proper/holy-saturday/vigil-magnificat-antiphon", "canticles/magnificat", "ordinary/shared/gloria-patri", "proper/holy-saturday/vigil-magnificat-antiphon",
						greeting, "shared/leader/let-us-pray", "proper/holy-saturday/vigil-collect", "shared/formulas/collect-conclusion-through-spirit",
						greeting, "proper/holy-saturday/vigil-dismissal", "shared/formulas/faithful-departed", "ordinary/shared/our-father",
					}
					var refs []string
					var elements []models.OfficeElement
					var doxologies int
					for _, section := range hour.Sections {
						for _, elem := range section.Elements {
							refs = append(refs, elem.SourceRef)
							elements = append(elements, elem)
							if strings.Contains(elem.Text, "[Text not found:") || elem.Text == "" {
								t.Errorf("unresolved element: %+v", elem)
							}
							if elem.Type == models.PsalmDoxology {
								doxologies++
							}
							if elem.Type == models.Collect && len(elem.Voice) > 0 {
								t.Error("Vigil collect inherited a silent conclusion")
							}
						}
					}
					if !slices.Equal(refs, want) {
						t.Fatalf("sequence = %v, want %v", refs, want)
					}
					if doxologies != 2 {
						t.Errorf("got %d doxologies, want Psalm and Magnificat", doxologies)
					}
					if summary := SummarizeHour(hour); summary.GospelAntFull != strings.ReplaceAll(elements[6].Text, "\n", " ") {
						t.Errorf("ordo summary lost the fixed Magnificat antiphon: %+v", summary)
					}
					if !strings.HasPrefix(elements[6].Text, "In the end of the sabbath") {
						t.Error("wrong Magnificat antiphon")
					}
					if !strings.HasPrefix(elements[12].Text, "Pour down upon us") {
						t.Error("wrong Vigil collect")
					}
					if strings.Count(strings.ToLower(elements[15].Text), "alleluia") != 4 {
						t.Error("dismissal must have two Alleluias in both versicle and response")
					}
					last := elements[len(elements)-1]
					if len(last.Voice) != 1 || last.Voice[0].Spoken || last.Voice[0].Text != last.Text {
						t.Error("final Pater must be wholly silent")
					}
					if !slices.ContainsFunc(hour.Decisions, func(d models.CompositionDecision) bool {
						return d.Rule == "context:office-form" && d.Outcome == "holy-saturday-vigil"
					}) {
						t.Error("missing office-form explanation")
					}
				})
			}
			// The special form belongs to this evening alone. Both adjacent Vespers
			// and every other hour of Holy Saturday retain their own definitions.
			for _, tc := range []struct {
				day  *models.CalendarDay
				hour string
			}{
				{&days[i-1], "vespers"}, {&days[i+1], "vespers"},
				{day, "lauds"}, {day, "prime"}, {day, "terce"}, {day, "sext"}, {day, "none"}, {day, "compline"},
			} {
				t.Run(tc.day.Date.Format(time.DateOnly)+"/"+tc.hour+"/boundary", func(t *testing.T) {
					hour, err := engine.ComposeHour(tc.hour, tc.day, moveable)
					if err != nil {
						t.Fatal(err)
					}
					for _, section := range hour.Sections {
						for _, elem := range section.Elements {
							if strings.HasPrefix(elem.SourceRef, "proper/holy-saturday/vigil-") {
								t.Errorf("Vigil proper leaked: %s", elem.SourceRef)
							}
						}
					}
				})
			}
		}
		if vigilDays != 1 {
			t.Errorf("%d: got %d Holy Saturdays, want one", year, vigilDays)
		}
	}
}
