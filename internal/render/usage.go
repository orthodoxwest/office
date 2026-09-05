package render

import (
	"time"

	"github.com/orthodoxwest/office/internal/usage"
)

type UsageData struct {
	NavDate, Theme, Page, SeasonClass string
	ShowBanner, ShowToday             bool
	Days, Max, Today, Yesterday       int
	Hours                             []string
	Rows                              []usage.Daily
	Chart                             []UsageBar
	FirstDate, LastDate, PeakDate     string
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
