package render

import (
	"time"

	"github.com/orthodoxwest/office/internal/usage"
)

type UsageData struct {
	TrendGroups                       []UsageTrendGroup
	NavDate, Theme, Page, SeasonClass string
	// UsageWhen dates the page for the usage beacon (see HomeData.UsageWhen).
	UsageWhen                     string
	ShowBanner, ShowToday         bool
	Days, Max, Today, Yesterday   int
	BrowserDays, CompleteDays     int
	DailyAverage                  float64
	OfficeTotals                  []UsageOffice
	OrdoTotal, RemindersTotal     int
	Hours                         []string
	Rows                          []usage.Daily
	Chart                         []UsageBar
	FirstDate, LastDate, PeakDate string
}

// UsageOffice compares office browser-days on a common scale, without
// implying that overlapping office counts partition the site's visitors.
type UsageOffice struct {
	Name  string
	Count int
	Width float64
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
	d := UsageData{Page: "usage", Days: days, Rows: rows, Hours: usage.Hours, TrendGroups: usageTrendGroups(rows)}
	for h, name := range usage.Hours {
		office := UsageOffice{Name: name}
		for _, row := range rows {
			office.Count += row.Hours[h]
		}
		d.OfficeTotals = append(d.OfficeTotals, office)
	}
	peakOffice := 0
	for _, office := range d.OfficeTotals {
		peakOffice = max(peakOffice, office.Count)
	}
	if peakOffice > 0 {
		for i := range d.OfficeTotals {
			d.OfficeTotals[i].Width = 100 * float64(d.OfficeTotals[i].Count) / float64(peakOffice)
		}
	}
	for i, row := range rows {
		d.BrowserDays += row.Users
		d.OrdoTotal += row.Ordo
		d.RemindersTotal += row.Reminders
		// Today is still in progress. Include recorded zero days in the
		// completed-day average so quiet days do not inflate it.
		if i > 0 {
			d.CompleteDays++
			d.DailyAverage += float64(row.Users)
		}
	}
	if d.CompleteDays > 0 {
		d.DailyAverage /= float64(d.CompleteDays)
	}
	if len(rows) == 0 {
		return d
	}
	// A year-long window can reach into last year, so include the year
	// on dates outside the current reporting year.
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
