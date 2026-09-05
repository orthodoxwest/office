package render

import "github.com/orthodoxwest/office/internal/usage"

type UsageData struct {
	NavDate, Theme, Page, SeasonClass string
	ShowBanner, ShowToday             bool
	Days, Max                         int
	Hours                             []string
	Rows                              []usage.Daily
}
