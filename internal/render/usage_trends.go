package render

import (
	"strings"

	"github.com/orthodoxwest/office/internal/usage"
)

// UsageTrendGroup gives the browser only aggregate counts, in chronological
// order. Scope names and display labels stay with the server's vocabulary.
type UsageTrendGroup struct {
	Key, Label string
	Series     []string
	Points     []UsageTrendPoint
}

type UsageTrendPoint struct {
	Day    string
	Counts []int
}

func usageTrendGroups(rows []usage.Daily) []UsageTrendGroup {
	var groups []UsageTrendGroup
	for _, words := range usageSplitWords {
		dimension, ok := usage.DimensionByKey(words.Key)
		if !ok {
			continue
		}
		group := UsageTrendGroup{Key: words.Key, Label: strings.ToUpper(words.Key[:1]) + words.Key[1:], Series: words.Labels[:]}
		for i := len(rows) - 1; i >= 0; i-- {
			point := UsageTrendPoint{Day: rows[i].Day}
			for _, value := range dimension.Values {
				point.Counts = append(point.Counts, rows[i].Dimensions[dimension.Scope(value)])
			}
			group.Points = append(group.Points, point)
		}
		groups = append(groups, group)
	}
	forms := UsageTrendGroup{Key: "prayer-form", Label: "Prayer form", Series: []string{"Private", "Deacon", "Priest"}}
	offices := UsageTrendGroup{Key: "offices", Label: "Offices"}
	for _, hour := range usage.Hours {
		offices.Series = append(offices.Series, strings.ToUpper(hour[:1])+hour[1:])
	}
	for i := len(rows) - 1; i >= 0; i-- {
		forms.Points = append(forms.Points, UsageTrendPoint{Day: rows[i].Day, Counts: []int{
			rows[i].Dimensions["prayer-form:private"], rows[i].Dimensions["prayer-form:deacon"], rows[i].Dimensions["prayer-form:priest"],
		}})
		offices.Points = append(offices.Points, UsageTrendPoint{Day: rows[i].Day, Counts: append([]int(nil), rows[i].Hours[:]...)})
	}
	return append(groups, forms, offices)
}
