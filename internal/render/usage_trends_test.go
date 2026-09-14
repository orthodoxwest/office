package render

import (
	"testing"

	"github.com/orthodoxwest/office/internal/usage"
)

func TestUsageTrendsKeepScopeCountsAndChronologicalDates(t *testing.T) {
	rows := []usage.Daily{
		{Day: "2026-09-14", Hours: [7]int{3, 0, 0, 0, 0, 2}, Dimensions: map[string]int{"appearance:nave": 4, "appearance:apse": 5, "screen:mobile": 8, "prayer-form:priest": 2}},
		{Day: "2026-09-13"},
	}
	groups := usageTrendGroups(rows)
	if len(groups) != 4 {
		t.Fatalf("groups: %+v", groups)
	}
	for _, group := range groups {
		if group.Points[0].Day != "2026-09-13" || group.Points[1].Day != "2026-09-14" || len(group.Points[0].Counts) != len(group.Series) {
			t.Fatalf("misaligned series: %+v", group)
		}
		for _, count := range group.Points[0].Counts {
			if count != 0 {
				t.Fatalf("missing observations invented: %+v", group)
			}
		}
	}
	if groups[0].Points[1].Counts[1] != 5 || groups[1].Points[1].Counts[1] != 8 || groups[2].Points[1].Counts[2] != 2 || groups[3].Points[1].Counts[5] != 2 {
		t.Fatalf("scope mapping: %+v", groups)
	}
	groups[3].Points[1].Counts[0] = 99
	if rows[0].Hours[0] != 3 {
		t.Fatal("trend payload aliases stored rows")
	}
}
