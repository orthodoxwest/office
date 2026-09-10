package review

import (
	"reflect"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// The ingestion inventory deliberately counts calendar occurrences once.
// Check that changing the prayer form cannot alter its dynamic proper rows.
func TestPrayerFormsPreserveDynamicResolutionInventory(t *testing.T) {
	eng, err := office.NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	moveable := calendar.ComputeMoveableDates(2026)
	for i := range days {
		for _, name := range HourNames {
			var baseline []office.ProperResolutionTrace
			for _, form := range models.PrayerForms {
				hour, err := eng.ComposeHourWithOptions(name, &days[i], moveable, office.ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var traces []office.ProperResolutionTrace
				for _, section := range hour.Sections {
					for _, element := range section.Elements {
						if element.SlotRef == "" {
							continue
						}
						trace := traceInventoryElement(eng, &days[i], name, element)
						if isDynamicResolutionRef(element.SourceRef) || trace.SelectedTier == "not-found" {
							traces = append(traces, trace)
						}
					}
				}
				if form == models.PrayerPrivate {
					baseline = traces
				} else if !reflect.DeepEqual(baseline, traces) {
					t.Fatalf("%s %s: %s changed dynamic resolutions", days[i].Date, name, form)
				}
			}
		}
	}
}
