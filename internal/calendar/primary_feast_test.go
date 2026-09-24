package calendar

import (
	"slices"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// General Rubrics X (p. xxx), confirmed in #138: an occurring Greater or
// Lesser Double is not commemorated on a Primary Feast of Our Lord. The rule
// reads the source-defined PrimaryOfOurLord flag, never feast IDs.
func TestPrimaryFeastDoubleSuppression(t *testing.T) {
	primary := traceFeast("primary", models.Double1stClass, models.CategoryLord)
	primary.PrimaryOfOurLord = true
	for _, tc := range []struct {
		name       string
		winner     *models.Feast
		comm       *models.Feast
		suppressed bool
	}{
		{"Double", primary, traceFeast("double", models.Double, models.CategoryConfessorDoctor), true},
		{"Greater Double", primary, traceFeast("greater", models.GreaterDouble, models.CategoryConfessorBishop), true},
		{"other first-class feast of Our Lord", traceFeast("secondary", models.Double1stClass, models.CategoryLord), traceFeast("double", models.Double, models.CategoryMartyr), false},
		{"apostle held in #379", primary, traceFeast("apostle", models.GreaterDouble, models.CategoryApostle), false},
		{"Sunday protected", primary, traceFeast("sunday", models.SemiDouble, models.CategorySunday), false},
		{"feria protected", primary, traceFeast("feria", models.PrivilegedFeria, models.CategoryFeria), false},
		{"Memorial left to its own rule", primary, traceFeast("memorial", models.Commemoration, models.CategoryMartyr), false},
		{"semidouble not a Double", primary, traceFeast("semidouble", models.SemiDouble, models.CategoryMartyr), false},
		{"no winner", nil, traceFeast("double", models.Double, models.CategoryMartyr), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kept, decisions := primaryFeastDoublesWithDecisions(tc.winner, []*models.Feast{tc.comm})
			if got := len(kept) == 0; got != tc.suppressed {
				t.Fatalf("suppressed = %v, want %v", got, tc.suppressed)
			}
			if tc.suppressed && (len(decisions) != 1 || decisions[0].Rule != "commemoration:double-under-primary-feast-of-our-lord") {
				t.Errorf("unexpected explanation: %+v", decisions)
			}
		})
	}
}

// Ordo witnesses for the rule and its held boundaries.
func TestPrimaryFeastDoubleAppointments(t *testing.T) {
	for _, tc := range []struct {
		name, date, comm string
		kept             bool
	}{
		// 2018 ordo: Whitsunday "No Comm." (Bede), and its I Vespers.
		{"Bede on Whitsunday", "2018-05-27", "st-bede-venerable", false},
		// 2021 ordo: Easter Sunday "No Comm." (Athanasius).
		{"Athanasius on Easter Sunday", "2021-05-02", "st-athanasius", false},
		{"Basil on Trinity Sunday", "2020-06-14", "st-basil-great", false},
		// 2017 and 2023 ordos keep Barnabas on Trinity; 2026 on Corpus Christi (#379).
		{"Barnabas on Trinity Sunday", "2017-06-11", "st-barnabas", true},
		{"Barnabas on Corpus Christi", "2026-06-11", "st-barnabas", true},
		// Pentecost Monday is an octave weekday, not the Feast (#380 holds its scope).
		{"Boniface on Pentecost Monday", "2017-06-05", "st-boniface", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tc.date)
			if err != nil {
				t.Fatal(err)
			}
			days, err := BuildCalendar(date.Year(), findDataDir(t))
			if err != nil {
				t.Fatal(err)
			}
			i := slices.IndexFunc(days, func(d models.CalendarDay) bool { return d.Date.Equal(date) })
			if i < 0 {
				t.Fatalf("no calendar day %s", tc.date)
			}
			ids := feastIDs(days[i].Commemorations)
			if got := slices.ContainsFunc(ids, func(id string) bool { return id == tc.comm }); got != tc.kept {
				t.Fatalf("%s commemorated = %v, want %v (commemorations %v, office %s)", tc.comm, got, tc.kept, ids, days[i].Celebration.ID)
			}
		})
	}
}
