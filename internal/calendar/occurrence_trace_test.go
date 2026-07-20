package calendar

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

func traceFeast(id string, rank models.Rank, category models.FeastCategory) *models.Feast {
	return &models.Feast{ID: id, Name: id, Rank: rank, Category: category, Color: models.White}
}

func TestPrecedenceTraceRules(t *testing.T) {
	privileged := traceFeast("privileged-feria", models.PrivilegedFeria, models.CategoryFeria)
	double := traceFeast("double", models.Double, models.CategoryMartyr)
	second := traceFeast("second", models.Double2ndClass, models.CategoryMartyr)
	corpus := traceFeast("corpus-christi-octave-day-2", models.SemiDouble, models.CategoryLord)
	sunday := traceFeast("ordinary-sunday", models.SemiDouble, models.CategorySunday)
	greater := traceFeast("greater", models.GreaterDouble, models.CategoryMartyr)
	moveable := traceFeast("moveable", models.Double, models.CategoryMartyr)
	moveable.DateRule = "easter+1"
	lord := traceFeast("lord", models.Double, models.CategoryLord)
	equal := traceFeast("equal", models.Double, models.CategoryMartyr)

	tests := []struct {
		name       string
		challenger *models.Feast
		incumbent  *models.Feast
		wins       bool
		rule       string
	}{
		{"privileged feria wins", privileged, double, true, "occurrence:privileged-feria-below-second-class"},
		{"second class beats feria", second, privileged, true, "occurrence:second-class-over-privileged-feria"},
		{"corpus octave wins", corpus, double, true, "occurrence:corpus-octave-precedence"},
		{"sunday beats corpus octave", sunday, corpus, true, "occurrence:sunday-or-first-class-over-corpus-octave"},
		{"higher rank", second, greater, true, "occurrence:higher-rank"},
		{"sunday rank boost", sunday, double, true, "occurrence:sunday-rank-boost"},
		{"moveable tie break", moveable, double, true, "occurrence:temporal-tiebreak"},
		{"lord tie break", lord, equal, true, "occurrence:lord-tiebreak"},
		{"equal keeps incumbent", equal, double, false, "occurrence:equal-precedence-possession"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wins, decision := compareFeastPrecedenceWithDecision(tt.challenger, tt.incumbent)
			if wins != tt.wins || decision.Rule != tt.rule {
				t.Fatalf("wins=%t rule=%q, want wins=%t rule=%q", wins, decision.Rule, tt.wins, tt.rule)
			}
		})
	}
}

func TestColorTraceRules(t *testing.T) {
	lesser := traceFeast("lesser", models.Double, models.CategoryMartyr)
	lesser.Color = models.Red
	second := traceFeast("second", models.Double2ndClass, models.CategoryMartyr)
	second.Color = models.White

	tests := []struct {
		name    string
		winner  *models.Feast
		season  models.Season
		want    models.Color
		outcome string
	}{
		{"feria", nil, models.Epiphany, models.Green, "seasonal-feria"},
		{"penitential lesser feast", lesser, models.Lent, models.Violet, "penitential-season-over-lesser-feast"},
		{"celebration color", second, models.Lent, models.White, "celebration-color"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, decision := resolvedDayColorWithDecision(tt.winner, tt.season, tt.want)
			if got != tt.want || decision.Outcome != tt.outcome {
				t.Fatalf("color=%q outcome=%q, want color=%q outcome=%q", got, decision.Outcome, tt.want, tt.outcome)
			}
		})
	}
}

func TestCommemorationTraceRules(t *testing.T) {
	winner := traceFeast("winner", models.Double, models.CategorySunday)
	onlyWith := traceFeast("only-with", models.Commemoration, models.CategoryMartyr)
	onlyWith.OnlyWith = "someone-else"
	pentecostWinner := traceFeast("pentecost-octave-day-3", models.Double1stClass, models.CategoryLord)
	ember := traceFeast("whit-ember-wednesday", models.Commemoration, models.CategoryFeria)
	octave := traceFeast("ascension-octave-day-4", models.Commemoration, models.CategoryLord)
	privileged := traceFeast("ash-wednesday", models.Double1stClass, models.CategoryFeria)
	stGeorge := traceFeast("st-george-octave-day-2", models.Commemoration, models.CategoryMartyr)
	firstClass := traceFeast("easter-monday", models.Double1stClass, models.CategoryLord)
	lowRank := traceFeast("simple-comm", models.Commemoration, models.CategoryMartyr)

	tests := []struct {
		name   string
		winner *models.Feast
		comm   *models.Feast
		rule   string
	}{
		{"only-with", winner, onlyWith, "commemoration:only-with"},
		{"pentecost ember", pentecostWinner, ember, "commemoration:pentecost-ember"},
		{"same octave Sunday", winner, octave, "commemoration:same-octave-sunday"},
		{"St George octave", privileged, stGeorge, "commemoration:st-george-octave"},
		{"first class low rank", firstClass, lowRank, "commemoration:first-class-low-rank"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suppressed, decision := commemorationSuppressionDecision(tt.winner, tt.comm)
			if !suppressed || decision.Rule != tt.rule {
				t.Fatalf("suppressed=%t rule=%q, want %q", suppressed, decision.Rule, tt.rule)
			}
		})
	}

	matching := traceFeast("matching", models.Commemoration, models.CategoryMartyr)
	matching.Name = winner.Name
	duplicateA := traceFeast("duplicate-a", models.Commemoration, models.CategoryMartyr)
	duplicateB := traceFeast("duplicate-b", models.Commemoration, models.CategoryMartyr)
	duplicateA.Name, duplicateB.Name = "St Example", "St. Example"
	_, decisions := finalizeCommemorationsWithDecisions(winner, []*models.Feast{matching, duplicateA, duplicateB})
	assertTraceRule(t, decisions, "commemoration:matches-winner")
	assertTraceRule(t, decisions, "commemoration:duplicate-name")

	var many []*models.Feast
	for _, id := range []string{"one", "two", "three", "four", "five", "six", "seven"} {
		many = append(many, traceFeast(id, models.Commemoration, models.CategoryMartyr))
	}
	got, decisions := finalizeCommemorationsWithDecisions(nil, many)
	if len(got) != maxCommemorationsPerDay {
		t.Fatalf("got %d commemorations", len(got))
	}
	assertTraceRule(t, decisions, "commemoration:cap")
}

func TestResolveDayTraceIncludesTransfersAndDisposition(t *testing.T) {
	winner := traceFeast("winner", models.Double1stClass, models.CategoryLord)
	transferred := traceFeast("transferred", models.Double2ndClass, models.CategoryMartyr)
	comm := traceFeast("memorial", models.Commemoration, models.CategoryMartyr)
	day, transfers := ResolveDay(
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		[]*models.Feast{winner, comm}, models.Christmas, models.White, nil,
		[]*models.Feast{transferred},
	)
	if len(transfers) != 1 || transfers[0].ID != transferred.ID {
		t.Fatalf("transfers = %#v", transfers)
	}
	for _, rule := range []string{
		"occurrence:transfer-in",
		"occurrence:transfer-out",
		"occurrence:loser-disposition",
		"color:resolution",
	} {
		assertTraceRule(t, day.OccurrenceDecisions, rule)
	}
}

func TestResolveDayCanTransferBeyondYearBoundary(t *testing.T) {
	winner := traceFeast("december-31-winner", models.Double1stClass, models.CategoryLord)
	loser := traceFeast("cross-year-transfer", models.Double2ndClass, models.CategoryMartyr)
	day, transfers := ResolveDay(
		time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		[]*models.Feast{winner, loser}, models.Christmas, models.White, nil, nil,
	)
	if len(transfers) != 1 || transfers[0].ID != loser.ID {
		t.Fatalf("Dec 31 transfers = %#v, want %s", transfers, loser.ID)
	}
	assertTraceRule(t, day.OccurrenceDecisions, "occurrence:transfer-out")
}

func TestResolveDaySuppressesSeasonallyExcludedCommonVigil(t *testing.T) {
	tests := []struct {
		name       string
		season     models.Season
		candidates []*models.Feast
		wantWinner string
		vigilID    string
	}{
		{
			name:   "unimpeded vigil in Advent",
			season: models.Advent,
			candidates: []*models.Feast{
				{ID: "vigil-of-st-andrew", Rank: models.Simple, Category: models.CategoryFeria, IsVigil: true},
			},
			vigilID: "vigil-of-st-andrew",
		},
		{
			name:   "vigil on Ember day",
			season: models.Septuagesima,
			candidates: []*models.Feast{
				{ID: "lent-ember-wednesday", Rank: models.PrivilegedFeria, Category: models.CategoryFeria},
				{ID: "vigil-of-st-example", Rank: models.Simple, Category: models.CategoryFeria, IsVigil: true},
			},
			wantWinner: "lent-ember-wednesday",
			vigilID:    "vigil-of-st-example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, _ := ResolveDay(
				time.Date(2027, 11, 29, 0, 0, 0, 0, time.UTC),
				tt.candidates, tt.season, models.Violet, nil, nil,
			)
			gotWinner := ""
			if day.Celebration != nil {
				gotWinner = day.Celebration.ID
			}
			if gotWinner != tt.wantWinner {
				t.Fatalf("winner = %q, want %q", gotWinner, tt.wantWinner)
			}
			assertTraceDecision(t, day.OccurrenceDecisions, "commemoration:vigil-seasonal-exclusion", tt.vigilID)
		})
	}
}

func TestResolveDayRetainsPrivilegedVigilInAdvent(t *testing.T) {
	vigil := &models.Feast{
		ID:       "vigil-nativity",
		Rank:     models.Double1stClass,
		Category: models.CategoryFeria,
		IsVigil:  true,
	}
	day, _ := ResolveDay(
		time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC),
		[]*models.Feast{vigil}, models.Advent, models.Violet, nil, nil,
	)
	if day.Celebration != vigil {
		t.Fatalf("celebration = %#v, want privileged Vigil of the Nativity", day.Celebration)
	}
}

func TestCommemorationDedupeContainmentBehavior(t *testing.T) {
	short := traceFeast("st-john", models.Commemoration, models.CategoryMartyr)
	long := traceFeast("st-john-baptist", models.Commemoration, models.CategoryMartyr)
	short.Name = "St John"
	long.Name = "St John Baptist"

	got, decisions := dedupeCommemorationsWithDecisions(nil, []*models.Feast{short, long})
	if len(got) != 1 || got[0].ID != short.ID {
		t.Fatalf("deduped commemorations = %#v, want only %s", got, short.ID)
	}
	assertTraceRule(t, decisions, "commemoration:duplicate-name")
}

func assertTraceRule(t *testing.T, decisions []models.CompositionDecision, rule string) {
	t.Helper()
	for _, decision := range decisions {
		if decision.Rule == rule {
			return
		}
	}
	t.Errorf("missing trace rule %q in %#v", rule, decisions)
}

func assertTraceDecision(t *testing.T, decisions []models.CompositionDecision, rule, detail string) {
	t.Helper()
	for _, decision := range decisions {
		if decision.Rule == rule && decision.Detail == detail {
			return
		}
	}
	t.Errorf("missing trace rule %q with detail %q in %#v", rule, detail, decisions)
}
