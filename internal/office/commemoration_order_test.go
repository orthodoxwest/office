package office

import (
	"slices"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestComposedCommemorationOrderAndConclusions(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	// 2026 ordo pp.66,72; Rubrics XIV.14 and XXXIII.5. Each collect's
	// conclusion must follow its new position in the actual rendered hour.
	cases := []struct {
		date, hour string
		want       []string
	}{
		{"05-24", "lauds", []string{"ascension-octave-day-4", "comm-05-24-st-vincent-of-lerins-confessor"}},
		{"06-14", "lauds", []string{"corpus-christi-octave-day-4", "st-basil-great", "comm-extra-06-14-all-saints-of-antioch"}},
		{"05-26", "vespers", []string{"st-augustine-canterbury", "ascension-octave-day-7", "comm-extra-05-27-st-john-i-pope-and-martyr"}},
		{"01-18", "vespers", []string{"chair-peter-rome", "comm-01-18-commemoration-of-st-paul", "comm-01-19-ss-marius-martha-audifax-and-abachum-martyrs", "comm-01-19-st-mark-of-ephesus-bishop-and-confessor"}},
	}
	for _, tc := range cases {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				var day *models.CalendarDay
				for i := range days {
					if days[i].Date.Format("01-02") == tc.date {
						day = &days[i]
						break
					}
				}
				hour, err := engine.ComposeHourWithOptions(tc.hour, day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var ids []string
				var concluded []string
				for _, section := range hour.Sections {
					for _, e := range section.Elements {
						if e.SlotRef != "commemoration-collect" {
							continue
						}
						ids = append(ids, e.CommemorationOwnerID)
						if slices.ContainsFunc(e.SourceRefs, func(s string) bool { return strings.HasPrefix(s, "shared/formulas/collect-conclusion-") }) {
							concluded = append(concluded, e.CommemorationOwnerID)
						}
					}
				}
				if !slices.Equal(ids, tc.want) {
					t.Fatalf("collect order = %v, want %v", ids, tc.want)
				}
				if want := tc.want[len(tc.want)-1:]; !slices.Equal(concluded, want) {
					t.Errorf("conclusions = %v, want %v", concluded, want)
				}
			})
		}
	}
}
