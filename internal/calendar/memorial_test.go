package calendar

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

// Diurnal VIII, pp. xxviii-xxix, confirmed in #138. Test arbitrary feast
// IDs so this remains a rank/category rule, not a list of 2026 collisions.
func TestFirstClassFeastMemorialSuppression(t *testing.T) {
	memorial := &models.Feast{ID: "memorial", Rank: models.Commemoration, Category: models.CategoryMartyr}
	for _, tc := range []struct {
		name   string
		winner *models.Feast
		comm   *models.Feast
		want   bool
	}{
		{"Lord feast", traceFeast("lord", models.Double1stClass, models.CategoryLord), memorial, true},
		{"saint feast", traceFeast("saint", models.Double1stClass, models.CategoryConfessor), memorial, true},
		{"Marian feast", traceFeast("marian", models.Double1stClass, models.CategoryBlessedVirgin), memorial, true},
		{"second class retains at Lauds", traceFeast("second", models.Double2ndClass, models.CategoryLord), memorial, false},
		{"Greater Sunday precedence", traceFeast("sunday", models.Double1stClass, models.CategorySunday), memorial, false},
		{"privileged feria precedence", traceFeast("feria", models.Double1stClass, models.CategoryFeria), memorial, false},
		{"later Easter octave weekday", traceFeast("easter-sunday-octave-day-6", models.Double1stClass, models.CategoryLord), memorial, false},
		{"later Pentecost octave weekday", traceFeast("pentecost-octave-day-5", models.Double1stClass, models.CategoryLord), memorial, false},
		{"Low Sunday is Greater Double", traceFeast("low-sunday", models.Double1stClass, models.CategoryLord), memorial, false},
		{"no principal feast", nil, memorial, false},
		{"perpetual apostolic companion separate", traceFeast("lord", models.Double1stClass, models.CategoryLord), &models.Feast{ID: "companion", Rank: models.Commemoration, IsApostolicCompanion: true}, false},
		{"Double rule separate", traceFeast("lord", models.Double1stClass, models.CategoryLord), traceFeast("double", models.Double, models.CategoryMartyr), false},
		{"protected Sunday", traceFeast("lord", models.Double1stClass, models.CategoryLord), traceFeast("sunday", models.Commemoration, models.CategorySunday), false},
		{"protected feria", traceFeast("lord", models.Double1stClass, models.CategoryLord), traceFeast("feria", models.Commemoration, models.CategoryFeria), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, decision := commemorationSuppressionDecision(tc.winner, tc.comm)
			if got != tc.want {
				t.Fatalf("suppressed = %v, want %v: %+v", got, tc.want, decision)
			}
			if tc.want && decision.Rule != "commemoration:memorial-under-first-class-feast" {
				t.Errorf("unexpected explanation: %+v", decision)
			}
		})
	}
}

func TestMemorialSuppressionPreservesUnresolvedScopes(t *testing.T) {
	memorial := traceFeast("memorial", models.Commemoration, models.CategoryMartyr)
	for _, id := range []string{"easter-monday", "easter-tuesday", "pentecost-octave-day-2", "pentecost-octave-day-3", "solemnity-st-joseph"} {
		t.Run(id, func(t *testing.T) {
			if suppressed, decision := commemorationSuppressionDecision(traceFeast(id, models.Double1stClass, models.CategoryLord), memorial); suppressed {
				t.Fatalf("#378/#380 appointment changed: %+v", decision)
			}
		})
	}
}
