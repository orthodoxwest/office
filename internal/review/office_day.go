package review

import (
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// officeContextDay keeps review ownership aligned with the office that was
// actually composed while retaining the civil date used to locate the page.
func officeContextDay(day *models.CalendarDay, hourName string) *models.CalendarDay {
	contextDay := office.OfficeDayForHour(day, hourName)
	if contextDay == nil || day == nil || contextDay.Date.Equal(day.Date) {
		return contextDay
	}
	civilContext := *contextDay
	civilContext.Date = day.Date
	return &civilContext
}
