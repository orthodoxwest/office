package office

import (
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
	"testing"
)

func TestTemporalCommemorationKeepsItsOwnProper(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/example/versicle-lauds-lent": "Proper seasonal verse",
		"proper/example/versicle-lauds":      "Proper verse",
		"seasonal/lent/versicle-lauds":       "Seasonal verse",
		"ordinary/lauds/versicle-sunday":     "Sunday verse",
		"ordinary/lauds/versicle":            "Weekday verse",
	})
	for _, tc := range []struct {
		id       string
		category models.FeastCategory
		season   models.Season
		want     string
	}{
		{"example", models.CategorySunday, models.Lent, "proper/example/versicle-lauds-lent"},
		{"example", models.CategorySunday, models.Pentecost, "proper/example/versicle-lauds"},
		{"privileged-lenten-feria", models.CategoryFeria, models.Lent, "seasonal/lent/versicle-lauds"},
		{"other", models.CategorySunday, models.Pentecost, "ordinary/lauds/versicle-sunday"},
		{"other", models.CategoryFeria, models.Pentecost, "ordinary/lauds/versicle"},
	} {
		feast := &models.Feast{ID: tc.id, Category: tc.category}
		if tc.id == "privileged-lenten-feria" {
			feast.ProperID = "example"
		}
		_, ref := lookupTemporalCommemorationVersicle(feast, tc.season, "lauds", corpus)
		if ref != tc.want {
			t.Errorf("%s/%s: got %s, want %s", tc.id, tc.season, ref, tc.want)
		}
	}
}
