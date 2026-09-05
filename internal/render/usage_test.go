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
