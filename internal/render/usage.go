package render

import (
	"math"
	"time"

	"github.com/orthodoxwest/office/internal/usage"
)

type UsageData struct {
	NavDate, Theme, Page, SeasonClass string
	// UsageWhen dates the page for the usage beacon (see HomeData.UsageWhen).
	UsageWhen                     string
	ShowBanner, ShowToday         bool
	Days, Max, Today, Yesterday   int
	Hours                         []string
	Rows                          []usage.Daily
	Chart                         []UsageBar
	Splits                        []UsageSplit
	FirstDate, LastDate, PeakDate string
}

// UsageSplit is one two-valued dimension of how the Office was read over the
// whole window — which appearance was rendered, and whether the reading was
// phone-shaped. Both sides are browser-days summed across the period, so they
// compare with each other rather than with the daily unique counts above.
type UsageSplit struct {
	Title, Note string
	Total       int
	Left, Right UsageShare
}

// UsageShare is one side of a UsageSplit: its count, its rounded percentage,
// and its segment of the 720-unit bar.
type UsageShare struct {
	Label    string
	Count    int
	Percent  int
	X, Width float64
}

type UsageBar struct {
	Day                 string
	Users               int
	X, Y, Width, Height float64
	Today               bool
}

// NewUsageData keeps the chart chronological while the detail table stays
// newest first. The peak is a daily count, never a sum of overlapping users.
func NewUsageData(rows []usage.Daily, days int) UsageData {
	d := UsageData{Page: "usage", Days: days, Rows: rows, Hours: usage.Hours}
	if len(rows) == 0 {
		return d
	}
	label := func(day string) string {
		t, err := time.Parse(time.DateOnly, day)
		if err != nil {
			return day
		}
		return t.Format("Jan 2")
	}
	d.Today = rows[0].Users
	if len(rows) > 1 {
		d.Yesterday = rows[1].Users
	}
	d.LastDate = label(rows[0].Day)
	d.FirstDate = label(rows[len(rows)-1].Day)
	for _, row := range rows {
		if row.Users > d.Max {
			d.Max = row.Users
			d.PeakDate = label(row.Day)
		}
	}
	var nave, apse, desktop, mobile int
	for _, row := range rows {
		nave += row.Nave
		apse += row.Apse
		desktop += row.Desktop
		mobile += row.Mobile
	}
	d.Splits = []UsageSplit{
		newUsageSplit("Nave vs Apse", "The appearance actually rendered, whether chosen or inherited from the device", "Nave", nave, "Apse", apse),
		newUsageSplit("Desktop vs Mobile", "Phone-shaped reading: a narrow window, or any touch screen", "Desktop", desktop, "Mobile", mobile),
	}
	scale := d.Max
	if scale == 0 {
		scale = 1
	}
	step := 720 / float64(len(rows))
	gap := step * .18
	for i := len(rows) - 1; i >= 0; i-- {
		height := 160 * float64(rows[i].Users) / float64(scale)
		d.Chart = append(d.Chart, UsageBar{Day: rows[i].Day, Users: rows[i].Users,
			X: float64(len(rows)-1-i)*step + gap/2, Y: 160 - height, Width: step - gap, Height: height, Today: i == 0})
	}
	return d
}

// newUsageSplit lays out one dimension bar. The percentages are made to sum
// to 100 rather than rounded independently, and an unreported dimension (an
// older cached client, or a period before it was collected) stays at zero
// width so the template can say so instead of drawing a half-empty bar.
func newUsageSplit(title, note, leftLabel string, left int, rightLabel string, right int) UsageSplit {
	split := UsageSplit{Title: title, Note: note, Total: left + right,
		Left:  UsageShare{Label: leftLabel, Count: left},
		Right: UsageShare{Label: rightLabel, Count: right}}
	if split.Total == 0 {
		return split
	}
	split.Left.Percent = int(math.Round(100 * float64(left) / float64(split.Total)))
	split.Right.Percent = 100 - split.Left.Percent
	split.Left.Width = 720 * float64(left) / float64(split.Total)
	split.Right.X = split.Left.Width
	split.Right.Width = 720 - split.Left.Width
	return split
}
