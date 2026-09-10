package render

import (
	"github.com/orthodoxwest/office/internal/usage"
	"testing"
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

func TestUsageSplitsSumOverThePeriod(t *testing.T) {
	// Day one had a silent, un-refreshed browser: it lands in Users without
	// landing in either half of a dimension.
	rows := []usage.Daily{
		{Day: "2026-09-05", Users: 4, Nave: 1, Apse: 2, Desktop: 1, Mobile: 2},
		{Day: "2026-09-04", Users: 3, Nave: 2, Apse: 1, Desktop: 0, Mobile: 3},
	}
	d := NewUsageData(rows, 7)
	appearance, screen := d.Splits[0], d.Splits[1]
	if appearance.Total != 6 || appearance.Left.Count != 3 || appearance.Right.Count != 3 {
		t.Fatalf("appearance: %+v", appearance)
	}
	if appearance.Left.Percent+appearance.Right.Percent != 100 || appearance.Left.Percent != 50 {
		t.Fatalf("percentages do not close: %+v", appearance)
	}
	if screen.Left.Count != 1 || screen.Right.Count != 5 || screen.Left.Percent != 17 || screen.Right.Percent != 83 {
		t.Fatalf("screen: %+v", screen)
	}
	for _, split := range d.Splits {
		if split.Left.X != 0 || split.Left.Width+split.Right.Width != 720 || split.Right.X != split.Left.Width {
			t.Fatalf("bar segments do not tile: %+v", split)
		}
	}
	// Before any client reports a dimension the bar stays empty rather than
	// implying an even split.
	quiet := NewUsageData([]usage.Daily{{Day: "2026-09-05", Users: 2}}, 7)
	for _, split := range quiet.Splits {
		if split.Total != 0 || split.Left.Width != 0 || split.Right.Width != 0 || split.Left.Percent != 0 {
			t.Fatalf("unreported dimension drew a bar: %+v", split)
		}
	}
}
