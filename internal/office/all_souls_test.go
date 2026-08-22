package office

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestAllSoulsHourOwnershipFollowsCurrentOrdo(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	days, err := calendar.BuildCalendar(2026, dataDir)
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	var day *models.CalendarDay
	for i := range days {
		if days[i].Date.Month() == 11 && days[i].Date.Day() == 2 {
			day = &days[i]
			break
		}
	}
	if day == nil || day.Celebration == nil || day.Celebration.ID != "all-souls" {
		t.Fatalf("Nov 2 celebration = %#v, want all-souls", day)
	}

	octaveDay := allSaintsOctaveOfficeDay(day)
	if octaveDay == day {
		t.Fatal("Nov 2 daytime office was not promoted from its calendar commemoration")
	}
	if octaveDay.Celebration == nil || octaveDay.Celebration.ID != allSaintsOctaveDay2ID {
		t.Fatalf("daytime celebration = %#v, want %s", octaveDay.Celebration, allSaintsOctaveDay2ID)
	}
	if octaveDay.Color != models.White {
		t.Fatalf("daytime color = %s, want white", octaveDay.Color)
	}
	for _, comm := range octaveDay.Commemorations {
		if comm.ID == allSaintsOctaveDay2ID {
			t.Fatal("promoted octave day remained as its own commemoration")
		}
	}

	engine, err := NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)

	lauds, err := engine.ComposeHour("lauds", day, moveable)
	if err != nil {
		t.Fatalf("ComposeHour(lauds): %v", err)
	}
	if lauds.Feast != day.Celebration.Name || lauds.Color != models.Black {
		t.Fatalf("Lauds = %q/%s, want All Souls/black", lauds.Feast, lauds.Color)
	}

	for _, hourName := range []string{"prime", "terce", "sext", "none", "vespers"} {
		hour, err := engine.ComposeHour(hourName, day, moveable)
		if err != nil {
			t.Fatalf("ComposeHour(%s): %v", hourName, err)
		}
		if hour.Feast != octaveDay.Celebration.Name || hour.Color != models.White {
			t.Errorf("%s = %q/%s, want %q/white", hourName, hour.Feast, hour.Color, octaveDay.Celebration.Name)
		}
		if hourName == "vespers" {
			assertAllSaintsVespers(t, hour)
		}
	}
}

func TestAllSoulsPromotionFailsClosedWithoutResolvedOctave(t *testing.T) {
	day := &models.CalendarDay{Celebration: &models.Feast{ID: "all-souls"}}
	if got := allSaintsOctaveOfficeDay(day); got != day {
		t.Fatal("All Souls without the resolved octave commemoration must remain unchanged")
	}
}

func TestAllSoulsPromotionIsDrivenByResolvedCalendarContext(t *testing.T) {
	day := &models.CalendarDay{
		Celebration: &models.Feast{ID: "all-souls"},
		Commemorations: []*models.Feast{{
			ID: allSaintsOctaveDay2ID,
		}},
	}
	if got := allSaintsOctaveOfficeDay(day); got == day || got.Celebration.ID != allSaintsOctaveDay2ID {
		t.Fatalf("resolved octave context was not promoted: %#v", got)
	}
}

func assertAllSaintsVespers(t *testing.T, hour *models.OfficeHour) {
	t.Helper()
	var psalms int
	var magnificatSource string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.Type == models.Psalm {
				psalms++
			}
			if strings.Contains(elem.SourceRef, "all-souls") || elem.SourceRef == "shared/formulas/rest-eternal" {
				t.Errorf("Vespers retained Office-of-the-Dead source %q", elem.SourceRef)
			}
			if elem.SlotRef == "magnificat-antiphon" {
				magnificatSource = elem.SourceRef
			}
		}
	}
	if psalms != 4 {
		t.Errorf("All Saints octave Vespers has %d psalms, want 4", psalms)
	}
	if magnificatSource != "proper/all-saints/magnificat-antiphon" {
		t.Errorf("Magnificat source = %q, want All Saints proper", magnificatSource)
	}
}
