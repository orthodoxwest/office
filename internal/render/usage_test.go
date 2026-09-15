package render

import (
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

func TestUsageDatesAcrossYears(t *testing.T) {
	crossing := NewUsageData([]usage.Daily{{Day: "2026-01-02", Users: 1}, {Day: "2025-12-31", Users: 1}}, 365)
	if crossing.FirstDate != "Dec 31, 2025" || crossing.LastDate != "Jan 2" {
		t.Fatalf("ambiguous window labels: %q to %q", crossing.FirstDate, crossing.LastDate)
	}
}

func TestUsagePeriodTotalsAndCompletedDayAverage(t *testing.T) {
	rows := []usage.Daily{
		{Day: "2026-09-05", Users: 100, Hours: [7]int{4, 1}, Ordo: 2, Reminders: 1},
		{Day: "2026-09-04", Users: 9, Hours: [7]int{3, 0, 0, 0, 0, 5}, Ordo: 3},
		{Day: "2026-09-03"},
	}
	d := NewUsageData(rows, 3)
	if d.DailyAverage != 4.5 || d.CompleteDays != 2 || d.BrowserDays != 109 {
		t.Fatalf("average must exclude today and include zero days: %+v", d)
	}
	if d.OfficeTotals[0].Count != 7 || d.OfficeTotals[0].Width != 100 || d.OfficeTotals[5].Count != 5 || d.OfficeTotals[2].Width != 0 || d.OrdoTotal != 5 || d.RemindersTotal != 1 {
		t.Fatalf("period totals: %+v", d)
	}
	for _, rows := range [][]usage.Daily{nil, {{Day: "2026-09-05"}}} {
		d := NewUsageData(rows, 7)
		if d.CompleteDays != 0 || d.DailyAverage != 0 || d.BrowserDays != 0 {
			t.Fatalf("empty average: %+v", d)
		}
		for _, office := range d.OfficeTotals {
			if office.Width != 0 {
				t.Fatalf("empty office comparison: %+v", office)
			}
		}
	}
}
