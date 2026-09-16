package calendar

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestCommemorationHierarchyIndependentOfInputOrder(t *testing.T) {
	// General Rubrics XIV.14(a-n), including both privileged octave tiers.
	sunday := traceFeast("arbitrary-sunday", models.SemiDouble, models.CategorySunday)
	corpus := traceFeast("arbitrary-high-octave-day-4", models.SemiDouble, models.CategoryLord)
	corpus.OctaveClass = models.OctavePrivilegedSecond
	ember := traceFeast("ember-example", models.PrivilegedFeria, models.CategoryFeria)
	terminal := traceFeast("example-octave-day", models.GreaterDouble, models.CategoryApostle)
	greater := traceFeast("greater", models.GreaterDouble, models.CategoryConfessor)
	double := traceFeast("double", models.Double, models.CategoryConfessor)
	ascension := traceFeast("arbitrary-low-octave-day-4", models.SemiDouble, models.CategoryLord)
	ascension.OctaveClass = models.OctavePrivilegedThird
	friday := traceFeast("arbitrary-feria", models.SemiDouble, models.CategoryFeria)
	friday.CommemorationClass = models.CommemorationPostAscensionFeria
	common := traceFeast("common-octave-day-3", models.SemiDouble, models.CategoryApostle)
	feria := traceFeast(models.FeriaCommemorationID, models.Commemoration, models.CategoryFeria)
	vigil := traceFeast("arbitrary-vigil", models.Simple, models.CategoryFeria)
	vigil.IsVigil, vigil.VigilOf = true, "example"
	simpleOctave := traceFeast("explicit-terminal", models.Simple, models.CategoryMartyr)
	simpleOctave.OctaveClass = models.OctaveSimple
	simple := traceFeast("simple", models.Simple, models.CategoryVirgin)
	memorial := traceFeast("memorial", models.Commemoration, models.CategoryMartyr)
	want := []*models.Feast{sunday, corpus, ember, terminal, greater, double, ascension, friday, common, feria, vigil, simpleOctave, simple, memorial}
	rng := rand.New(rand.NewSource(399))
	for i := 0; i < 100; i++ {
		input := slices.Clone(want)
		rng.Shuffle(len(input), func(i, j int) { input[i], input[j] = input[j], input[i] })
		original := slices.Clone(input)
		got := orderCommemorations(input, commemorationOrderContext{season: models.Advent})
		if !slices.Equal(got, want) {
			t.Fatalf("order = %v, want %v", feastIDs(got), feastIDs(want))
		}
		if !slices.Equal(input, original) {
			t.Fatal("mutated input")
		}
	}
	// The same feria moves above Doubles in Lent, but stays below common
	// octaves in Advent/Septuagesima. It must be included in the Lauds sort.
	for _, season := range []models.Season{models.Advent, models.Septuagesima, models.Lent, models.Passiontide} {
		day := &models.CalendarDay{Season: season, Commemorations: []*models.Feast{double, memorial}, FeriaCommemoration: feria}
		want := []*models.Feast{double, feria, memorial}
		if season == models.Lent || season == models.Passiontide {
			want = []*models.Feast{feria, double, memorial}
		}
		if got := LaudsCommemorations(day); !slices.Equal(got, want) {
			t.Errorf("%s: %v", season, feastIDs(got))
		}
	}
}

func TestLordAndSundayCommemorationOrder(t *testing.T) {
	lord := traceFeast("example-lord", models.GreaterDouble, models.CategoryLord)
	sunday := traceFeast("example-sunday", models.SemiDouble, models.CategorySunday)
	octave := traceFeast("example-octave-day-3", models.SemiDouble, models.CategoryLord)
	octave.OctaveClass = models.OctavePrivilegedSecond
	for _, tc := range []struct {
		season models.Season
		want   []*models.Feast
	}{
		{models.Easter, []*models.Feast{lord, sunday, octave}},
		{models.Advent, []*models.Feast{sunday, octave, lord}},
	} {
		for _, input := range [][]*models.Feast{{octave, lord, sunday}, {sunday, octave, lord}, {lord, sunday, octave}} {
			if got := orderCommemorations(input, commemorationOrderContext{season: tc.season}); !slices.Equal(got, tc.want) {
				t.Errorf("%s: %v", tc.season, feastIDs(got))
			}
		}
	}
	// No lesser Sunday: do not promote the Lord's feast above a Corpus-type octave.
	if got := orderCommemorations([]*models.Feast{lord, octave}, commemorationOrderContext{season: models.Easter}); got[0] != octave {
		t.Fatal("unconditional Lord promotion")
	}
}

func TestCommemorationCompanionGroupsAndConcurrentOffice(t *testing.T) {
	parent := traceFeast("apostolic-office", models.GreaterDouble, models.CategoryApostle)
	companion := traceFeast("companion", models.Commemoration, models.CategoryApostle)
	companion.CompanionOf = parent.ID
	equal := traceFeast("unrelated-equal", models.GreaterDouble, models.CategoryConfessor)
	concurrent := traceFeast("concurrent", models.Commemoration, models.CategoryMartyr)
	got := orderCommemorations([]*models.Feast{parent, equal, companion, concurrent}, commemorationOrderContext{concurrent: concurrent})
	if want := []*models.Feast{concurrent, parent, companion, equal}; !slices.Equal(got, want) {
		t.Fatalf("group split: %v", feastIDs(got))
	}
	// With the parent owning the office, its companion precedes other prayers.
	got = orderCommemorations([]*models.Feast{equal, companion, concurrent}, commemorationOrderContext{concurrent: concurrent, winner: parent})
	if want := []*models.Feast{concurrent, companion, equal}; !slices.Equal(got, want) {
		t.Fatalf("companion priority: %v", feastIDs(got))
	}
}

func TestCommemorationOrderingPrecedesCap(t *testing.T) {
	var input []*models.Feast
	for i := 0; i < maxCommemorationsPerDay; i++ {
		input = append(input, traceFeast(fmt.Sprintf("memorial-%d", i), models.Commemoration, models.CategoryMartyr))
	}
	sunday := traceFeast("sunday", models.SemiDouble, models.CategorySunday)
	input = append(input, sunday)
	got, _ := orderedCommemorationsWithDecisions(nil, input, commemorationOrderContext{})
	if len(got) != maxCommemorationsPerDay || got[0] != sunday {
		t.Fatalf("capped before ordering: %v", feastIDs(got))
	}
}

func TestCommemorationSameTierSourceHolds(t *testing.T) {
	days, err := BuildCalendar(2026, findDataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range days {
		if d.Date.Format("01-02") == "03-17" {
			// #402: the ordo p.46 prints outgoing Patrick before incoming Cyril.
			want := []string{"st-patrick", "st-cyril-jerusalem", "comm-03-18-st-edward-king-and-martyr"}
			if !slices.Equal(feastIDs(d.Vespers.Commemorations), want) {
				t.Fatalf("changed held order: %v", feastIDs(d.Vespers.Commemorations))
			}
		}
	}
}

func TestCompanionTargetsValidated(t *testing.T) {
	parent := traceFeast("parent", models.Double, models.CategoryApostle)
	companion := traceFeast("companion", models.Commemoration, models.CategoryApostle)
	for _, target := range []string{"missing", companion.ID} {
		companion.CompanionOf = target
		if !strings.Contains(strings.Join(validateSemantics([]*models.Feast{parent, companion}), "\n"), "invalid CompanionOf") {
			t.Errorf("accepted %s", target)
		}
	}
	companion.CompanionOf = parent.ID
	if errs := validateSemantics([]*models.Feast{parent, companion}); len(errs) > 0 {
		t.Fatal(errs)
	}
	parent.CompanionOf = companion.ID
	if !strings.Contains(strings.Join(validateSemantics([]*models.Feast{parent, companion}), "\n"), "invalid CompanionOf") {
		t.Fatal("accepted companion cycle")
	}
}

func TestCommemorationOrphanNilAndConcurrentCompanion(t *testing.T) {
	parent := traceFeast("parent", models.Double, models.CategoryApostle)
	companion := traceFeast("companion", models.Commemoration, models.CategoryApostle)
	companion.CompanionOf = parent.ID
	sunday := traceFeast("sunday", models.SemiDouble, models.CategorySunday)
	got := orderCommemorations([]*models.Feast{nil, companion, sunday}, commemorationOrderContext{})
	if !slices.Equal(got, []*models.Feast{sunday, companion}) {
		t.Fatalf("orphan/nil: %v", feastIDs(got))
	}
	got = orderCommemorations([]*models.Feast{sunday, companion, parent}, commemorationOrderContext{concurrent: companion})
	if !slices.Equal(got, []*models.Feast{parent, companion, sunday}) {
		t.Fatalf("concurrent group: %v", feastIDs(got))
	}
}
