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

// UsageSplit is one two-valued dimension of how the Office was read — which
// appearance was rendered, and whether the reading was phone-shaped. The
// period totals answer "which is more", and the daily mix answers "is that
// changing": a single figure for the window cannot tell a settled 60/40 from
// a migration that passed through it, and the longer the window the more of
// that movement it hides.
type UsageSplit struct {
	Title, Note         string
	Total               int
	Left, Right         UsageShare
	Mix                 []UsageMixDay
	FirstDate, LastDate string
	// Shares on the earliest and most recent days that reported anything, and
	// how many days those were: the drift the period totals cannot show.
	FirstShare, LastShare, Reported int
}

// UsageShare is one side of a UsageSplit over the whole period.
type UsageShare struct {
	Label   string
	Count   int
	Percent int
}

// UsageMixDay is one day's column of the mix band: the left side's share of
// that day, drawn full height so the band reads as proportion rather than
// volume — the daily totals above already carry volume. A day nobody reported
// stays unreported and leaves a gap rather than being drawn as an even split.
type UsageMixDay struct {
	Day                  string
	Left, Right, Percent int
	Reported             bool
	X, Width             float64
	LeftHeight           float64
	RightY, RightHeight  float64
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
	// The 366-day window reaches back into last year, where a bare "Sep 9"
	// cannot be told from this year's — so date the ones that need it.
	year := rows[0].Day[:4]
	label := func(day string) string {
		t, err := time.Parse(time.DateOnly, day)
		if err != nil {
			return day
		}
		if day[:4] != year {
			return t.Format("Jan 2, 2006")
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
	d.Splits = []UsageSplit{
		newUsageSplit("Nave vs Apse", "The appearance actually rendered, whether chosen or inherited from the device",
			rows, label, "Nave", "Apse", func(row usage.Daily) (int, int) { return row.Nave, row.Apse }),
		newUsageSplit("Desktop vs Mobile", "Phone-shaped reading: a narrow window, or any touch screen",
			rows, label, "Desktop", "Mobile", func(row usage.Daily) (int, int) { return row.Desktop, row.Mobile }),
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

// newUsageSplit lays out one dimension: its period totals and its day-by-day
// mix band, chronological like the trend chart above it. Percentages are made
// to sum to 100 rather than rounded independently. A dimension nothing has
// reported yet — a period before it was collected, or one served entirely to
// clients still on a cached app.js — draws nothing at all, so the template can
// say so instead of implying an even split.
func newUsageSplit(title, note string, rows []usage.Daily, label func(string) string,
	leftLabel, rightLabel string, pick func(usage.Daily) (int, int)) UsageSplit {
	split := UsageSplit{Title: title, Note: note,
		Left:      UsageShare{Label: leftLabel},
		Right:     UsageShare{Label: rightLabel},
		FirstDate: label(rows[len(rows)-1].Day), LastDate: label(rows[0].Day)}
	step := 720 / float64(len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		left, right := pick(rows[i])
		split.Left.Count += left
		split.Right.Count += right
		day := UsageMixDay{Day: rows[i].Day, Left: left, Right: right,
			X: float64(len(rows)-1-i) * step, Width: step}
		if total := left + right; total > 0 {
			day.Reported = true
			day.Percent = int(math.Round(100 * float64(left) / float64(total)))
			day.LeftHeight = 40 * float64(left) / float64(total)
			day.RightY = day.LeftHeight
			day.RightHeight = 40 - day.LeftHeight
			split.Reported++
			if split.Reported == 1 {
				split.FirstShare = day.Percent
			}
			split.LastShare = day.Percent
		}
		split.Mix = append(split.Mix, day)
	}
	split.Total = split.Left.Count + split.Right.Count
	if split.Total == 0 {
		return split
	}
	split.Left.Percent = int(math.Round(100 * float64(split.Left.Count) / float64(split.Total)))
	split.Right.Percent = 100 - split.Left.Percent
	return split
}
