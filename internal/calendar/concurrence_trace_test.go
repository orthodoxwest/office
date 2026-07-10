package calendar

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestBoundaryCommemorationTraceRules(t *testing.T) {
	winner := traceFeast("winner", models.Double2ndClass, models.CategoryMartyr)
	loser := traceFeast("loser", models.Simple, models.CategoryMartyr)
	incoming := traceFeast("incoming", models.Commemoration, models.CategoryMartyr)
	following := &models.CalendarDay{Commemorations: []*models.Feast{incoming}}

	comms, decisions := boundaryCommemorationsWithDecisions(winner, loser, following, true)
	if len(comms) != 0 {
		t.Fatalf("commemorations = %#v", comms)
	}
	assertTraceRule(t, decisions, "commemoration:non-concurrence-loser-at-second-vespers")
	assertTraceRule(t, decisions, "commemoration:incoming-at-second-vespers")
}

func TestResolvedConcurrenceCarriesBoundaryTrace(t *testing.T) {
	winner := traceFeast("winner", models.Double2ndClass, models.CategoryMartyr)
	incoming := traceFeast("incoming", models.Commemoration, models.CategoryMartyr)
	preceding := &models.CalendarDay{Celebration: winner, Color: models.Red, Season: models.Pentecost}
	following := &models.CalendarDay{Commemorations: []*models.Feast{incoming}, Color: models.Green, Season: models.Pentecost}

	got := resolveConcurrence(preceding, following)
	if got.Rule != "concurrence:preceding-only" {
		t.Fatalf("rule = %q", got.Rule)
	}
	assertTraceRule(t, got.Decisions, "commemoration:incoming-at-second-vespers")
}
