package render

import (
	"math"
	"testing"

	"github.com/orthodoxwest/office/internal/usage"
)

func TestUsageSummaryAndChronologicalChart(t *testing.T) {
	rows := []usage.Daily{{Day: "2026-09-05", Users: 3}, {Day: "2026-09-04", Users: 12}, {Day: "2026-09-03", Users: 0}}
	d := NewUsageData(rows, 7)
	if d.Today != 3 || d.Yesterday != 12 || d.Max != 12 || d.PeakDate != "Sep 4" {
		t.Fatalf("summary: %+v", d)
	}
	if d.Chart[0].Day != "2026-09-03" || !d.Chart[2].Today || d.Rows[0].Day != "2026-09-05" {
		t.Fatal("chart and table order")
	}
	for _, bar := range d.Chart {
		if bar.Height < 0 || bar.Height > 160 || bar.X < 0 || bar.X+bar.Width > 720 {
			t.Fatalf("bar outside plot: %+v", bar)
		}
	}
	empty := NewUsageData([]usage.Daily{{Day: "2026-09-05"}}, 7)
	if empty.Max != 0 || empty.Chart[0].Height != 0 || empty.PeakDate != "" {
		t.Fatalf("empty report implies activity: %+v", empty)
	}
}

func TestUsageSplitsCarryPeriodTotalsAndDailyMix(t *testing.T) {
	// A mix that reverses across the window: a period figure alone would call
	// this an even split and hide the migration entirely.
	mix := func(nave, apse, desktop, mobile int) map[string]int {
		return map[string]int{"appearance:nave": nave, "appearance:apse": apse,
			"screen:desktop": desktop, "screen:mobile": mobile}
	}
	rows := []usage.Daily{
		{Day: "2026-09-06", Users: 5, Dimensions: mix(1, 4, 1, 4)},
		{Day: "2026-09-05", Users: 4},
		{Day: "2026-09-04", Users: 5, Dimensions: mix(4, 1, 0, 5)},
	}
	d := NewUsageData(rows, 3)
	appearance, screen := d.Splits[0], d.Splits[1]
	if appearance.Total != 10 || appearance.Left.Count != 5 || appearance.Right.Count != 5 {
		t.Fatalf("appearance totals: %+v", appearance)
	}
	if appearance.Left.Percent+appearance.Right.Percent != 100 || appearance.Left.Percent != 50 {
		t.Fatalf("percentages do not close: %+v", appearance)
	}
	// The band runs oldest first, like the trend chart above it, so the drift
	// the period totals conceal is visible and described.
	if appearance.Mix[0].Day != "2026-09-04" || appearance.Mix[2].Day != "2026-09-06" {
		t.Fatalf("band is not chronological: %+v", appearance.Mix)
	}
	if appearance.FirstShare != 80 || appearance.LastShare != 20 || appearance.Reported != 2 {
		t.Fatalf("drift not reported: %+v", appearance)
	}
	if screen.Left.Count != 1 || screen.Right.Count != 9 || screen.Left.Percent != 10 {
		t.Fatalf("screen: %+v", screen)
	}
	for _, split := range d.Splits {
		var width float64
		for _, day := range split.Mix {
			width += day.Width
			if !day.Reported {
				// A day nobody reported draws nothing rather than an even split.
				if day.LeftHeight != 0 || day.RightHeight != 0 || day.Percent != 0 {
					t.Fatalf("unreported day drawn: %+v", day)
				}
				continue
			}
			if day.LeftHeight+day.RightHeight != 40 || day.RightY != day.LeftHeight {
				t.Fatalf("column does not fill the band: %+v", day)
			}
		}
		if math.Abs(width-720) > 1e-9 {
			t.Fatalf("band does not span the plot: %v", width)
		}
	}
	// A window that reaches back into last year dates its far end, so the two
	// ends of a 366-day band cannot read as the same "Sep 9".
	crossing := NewUsageData([]usage.Daily{
		{Day: "2026-01-02", Users: 1, Dimensions: mix(1, 0, 0, 0)},
		{Day: "2025-12-31", Users: 1, Dimensions: mix(0, 1, 0, 0)},
	}, 366)
	if crossing.FirstDate != "Dec 31, 2025" || crossing.LastDate != "Jan 2" {
		t.Fatalf("ambiguous window labels: %q to %q", crossing.FirstDate, crossing.LastDate)
	}
	if crossing.Splits[0].FirstDate != "Dec 31, 2025" {
		t.Fatalf("band labels drift from the chart: %+v", crossing.Splits[0])
	}

	// Before any client reports a dimension there is nothing to draw at all.
	quiet := NewUsageData([]usage.Daily{{Day: "2026-09-05", Users: 2}}, 7)
	for _, split := range quiet.Splits {
		if split.Total != 0 || split.Reported != 0 || split.Mix[0].Reported {
			t.Fatalf("unreported dimension drew a band: %+v", split)
		}
	}
}

// The report and the storage vocabulary are written in different packages and
// have to name the same families: a dimension with no words would silently
// stop being drawn, and words for a retired dimension would draw an empty
// band forever.
func TestUsageSplitsCoverTheVocabulary(t *testing.T) {
	if len(usageSplitWords)+1 != len(usage.Dimensions) {
		t.Fatalf("%d dimensions, %d described", len(usage.Dimensions), len(usageSplitWords))
	}
	for _, words := range usageSplitWords {
		dimension, ok := usage.DimensionByKey(words.Key)
		if !ok {
			t.Fatalf("report draws %q, which the vocabulary does not declare", words.Key)
		}
		for i, label := range words.Labels {
			if label == "" {
				t.Fatalf("%s has no word for %q", words.Key, dimension.Values[i])
			}
		}
		if words.Title == "" || words.Note == "" {
			t.Fatalf("%s is unlabelled: %+v", words.Key, words)
		}
	}
}
