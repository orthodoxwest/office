package calendar

import (
	"slices"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestPrivilegedOctaveCommemorationsAtSecondVespers(t *testing.T) {
	// Entitlement follows the builder's trait, independent of names, ID
	// spelling, category, and the synthetic precedence rank.
	for _, id := range []string{"unrelated-id", "christmas-octave-day-3"} {
		for _, rank := range []models.Rank{models.Double1stClass, models.Double2ndClass} {
			t.Run(id+"/"+string(rank), func(t *testing.T) {
				winner := traceFeast("feast", rank, models.CategoryMartyr)
				comm := traceFeast(id, models.SemiDouble, models.CategoryLord)
				comm.IsPrivilegedOctaveDay = true
				if included, rule := occurrenceCommemoratedAtSecondVespers(winner, comm); !included {
					t.Fatalf("privileged octave suppressed: %s", rule)
				}
			})
		}
	}
	for _, id := range []string{"christmas-octave-day-3", "ss-peter-paul-octave-day-3"} {
		for _, rank := range []models.Rank{models.Double1stClass, models.Double2ndClass} {
			comm := traceFeast(id, models.SemiDouble, models.CategoryLord)
			if included, _ := occurrenceCommemoratedAtSecondVespers(traceFeast("feast", rank, models.CategoryMartyr), comm); included {
				t.Errorf("unmarked %s admitted as privileged under %s", id, rank)
			}
		}
	}
	if isPrivilegedOctaveCommemoration(nil) {
		t.Fatal("nil commemoration admitted as privileged")
	}
}

func TestOctaveVespersCommemorations2026(t *testing.T) {
	// Printed 2026 appointments: Jan 11, Apr 26, Jun 26, Jul 3, Aug 16,
	// Dec 25-29. Keep the next feast first, followed by the current octave;
	// do not commemorate two successive weekdays of the same octave.
	days := buildCalendar2026(t)
	for _, tc := range []struct {
		month, day int
		want       []string
	}{
		{1, 11, []string{"epiphany-octave-day-6", "comm-01-12-st-benedict-biscop-abbot"}},
		{4, 26, []string{"st-george-octave-day-4"}},
		{6, 26, []string{"nativity-john-baptist-octave-day-3"}},
		{7, 3, []string{"ss-peter-paul-octave-day-5"}},
		{8, 16, []string{"pentecost-sunday-11"}},
		{12, 25, []string{"st-stephen"}},
		{12, 26, []string{"st-john-evangelist", "christmas-octave-day-2"}},
		{12, 27, []string{"holy-innocents", "christmas-octave-day-3"}},
		{12, 28, []string{"nativity-sunday-within-octave", "christmas-octave-day-4"}},
		{12, 29, []string{"christmas-octave-day-5"}},
	} {
		d := findDay(days, 2026, tc.month, tc.day)
		var got []string
		for _, comm := range d.Vespers.Commemorations {
			got = append(got, comm.ID)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s commemorations = %v, want %v", d.Date.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestFollowingOctaveAtSecondVespers(t *testing.T) {
	privileged := traceFeast("example-octave-day-3", models.SemiDouble, models.CategoryLord)
	privileged.IsPrivilegedOctaveDay = true
	for _, tc := range []struct {
		name      string
		winner    *models.Feast
		following *models.Feast
		want      bool
	}{
		{"common excluded by first class", traceFeast("feast", models.Double1stClass, models.CategoryMartyr), traceFeast("all-saints-octave-day-3", models.SemiDouble, models.CategoryMartyrs), false},
		{"common excluded by second class", traceFeast("feast", models.Double2ndClass, models.CategoryConfessor), traceFeast("assumption-bvm-octave-day-3", models.SemiDouble, models.CategoryBlessedVirgin), false},
		{"privileged retained", traceFeast("feast", models.Double2ndClass, models.CategoryMartyr), privileged, true},
		{"common retained under ordinary Double", traceFeast("feast", models.Double, models.CategoryMartyr), traceFeast("ss-peter-paul-octave-day-3", models.SemiDouble, models.CategoryApostle), true},
		{"terminal day has its own concurrence", traceFeast("feast", models.Double2ndClass, models.CategoryMartyr), traceFeast("ss-peter-paul-octave-day", models.GreaterDouble, models.CategoryApostle), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, rule := followingOfficeCommemoratedAtSecondVespers(tc.winner, &models.CalendarDay{Celebration: tc.following})
			if got != tc.want {
				t.Fatalf("included = %v, want %v (%s)", got, tc.want, rule)
			}
		})
	}
}
