package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Traverse the complete calendar, including offices with no named owner.
// Assertions come from reviewed source rules, not the resolver's selected tier.
// Future years exercise rule interactions; only 2026 has current-ordo evidence.
func TestCalendarCompositionRequirements(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}
	for _, year := range []int{2026, 2027, 2032} {
		yd := buildYear(t, year)
		hours, checked, seasonal := 0, 0, 0
		for i := range yd.days {
			day := &yd.days[i]
			for _, name := range names {
				hour, err := eng.ComposeHour(name, day, yd.moveable)
				if err != nil {
					t.Fatalf("%s %s: %v", day.Date, name, err)
				}
				hours++
				// Ferial little-hour chapters and versicles: Diurnal pp. 163-164,
				// 250-251 and 372-374. Vigils and octave offices have separate propers.
				feria := day.Celebration == nil || (day.Celebration.Category == models.CategoryFeria &&
					(strings.Contains(day.Celebration.ID, "feria") || strings.Contains(day.Celebration.ID, "ember")))
				little := name == "terce" || name == "sext" || name == "none"
				inLent := day.Season == models.Lent && day.Date.After(yd.moveable.Lent1)
				inEaster := day.Season == models.Easter && day.Date.After(yd.moveable.LowSunday) && day.Date.Before(yd.moveable.Ascension)
				if little && feria && (inLent || inEaster || day.Season == models.Advent) {
					for _, slot := range []string{"chapter", "versicle"} {
						assertPrincipalSource(t, hour, slot, "seasonal/"+string(day.Season)+"/"+slot+"-"+name)
						seasonal++
					}
				}
				// Diurnal pp. 121, 125, 131, 135, 139: per-annum Monday-Friday
				// use I will bless, while a following feast or season supplies its own.
				if name == "vespers" && day.Vespers.Feast == nil && (day.Celebration == nil || day.Celebration.Category == models.CategoryFeria) && day.Date.Weekday() >= time.Monday && day.Date.Weekday() <= time.Friday &&
					(day.Season == models.Pentecost || day.Season == models.Epiphany || day.Season == models.Septuagesima) {
					want := "ordinary/vespers/short-responsory-" + strings.ToLower(day.Date.Weekday().String())
					found := false
					for _, sec := range hour.Sections {
						for _, elem := range sec.Elements {
							if !elem.IsCommemoration && elem.SlotRef == "short-responsory" {
								found = true
								checked++
								if elem.SourceRef != want || !strings.Contains(elem.Text, "I will bless the Lord") {
									t.Errorf("Diurnal weekday Vespers: %s uses %s, want %s", day.Date.Format("2006-01-02"), elem.SourceRef, want)
								}
							}
						}
					}
					if !found {
						t.Errorf("%s: missing weekday Vespers responsory", day.Date)
					}
				}
			}
		}
		if checked == 0 {
			t.Errorf("%d: weekday Vespers rule did not exercise any cases", year)
		}
		t.Logf("%d: composed %d date/hours; checked %d weekday Vespers responsories and %d seasonal little-hour slots", year, hours, checked, seasonal)
	}
}

func assertPrincipalSource(t *testing.T, hour *models.OfficeHour, slot, want string) {
	t.Helper()
	found := false
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if !elem.IsCommemoration && elem.SlotRef == slot {
				found = true
				if elem.SourceRef != want {
					t.Errorf("%s %s %s: got %s, source requirement wants %s", hour.Date.Format("2006-01-02"), hour.Hour, slot, elem.SourceRef, want)
				}
			}
		}
	}
	if !found {
		t.Errorf("%s %s: missing appointed %s", hour.Date.Format("2006-01-02"), hour.Hour, slot)
	}
}
