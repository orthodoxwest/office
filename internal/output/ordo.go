// Package output provides formatters for calendar output.
package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

var dayAbbrevs = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

func dayAbbrev(wd time.Weekday) string {
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6
	// We want: Monday=0, ..., Sunday=6
	idx := (int(wd) + 6) % 7
	return dayAbbrevs[idx]
}

func formatName(name string, rank models.Rank) string {
	if rank == models.Double1stClass {
		return strings.ToUpper(name)
	}
	return name
}

// FormatDay formats a single CalendarDay as text line(s).
func FormatDay(day *models.CalendarDay) string {
	dow := dayAbbrev(day.Date.Weekday())
	dayNum := fmt.Sprintf("%3d", day.Date.Day())
	marker := fmt.Sprintf("%-2s", day.Penitential.Marker())
	color := day.Color.Abbrev()

	var line string
	if day.Celebration == nil {
		name := "Feria"
		if day.Tempora != "" {
			name = day.Tempora
		}
		line = fmt.Sprintf("%s  %s  %s %-42s       %s", dayNum, dow, marker, name, color)
	} else {
		feast := day.Celebration
		name := formatName(feast.Name, feast.Rank)
		rankStr := fmt.Sprintf("[%s]", feast.Rank.Abbrev())
		line = fmt.Sprintf("%s  %s  %s %-42s %s %s", dayNum, dow, marker, name, fmt.Sprintf("%-5s", rankStr), color)
	}

	lines := []string{line}
	if day.FeriaCommemoration != nil {
		lines = append(lines, fmt.Sprintf("           Com: %s (Lauds)", day.FeriaCommemoration.Name))
	}
	for _, comm := range day.Commemorations {
		lines = append(lines, fmt.Sprintf("           Com: %s", comm.Name))
	}

	if day.Vespers.Owner != models.VespersNotApplicable {
		var vLine string
		switch day.Vespers.Owner {
		case models.VespersIIOfPreceding:
			vLine = fmt.Sprintf("           Vespers: II prec. %s", day.Vespers.Color.Abbrev())
		case models.VespersIOfFollowing:
			vLine = fmt.Sprintf("           Vespers: I fol. %s (%s)", day.Vespers.Color.Abbrev(), day.Vespers.Feast.Name)
		}
		lines = append(lines, vLine)
	}

	return strings.Join(lines, "\n")
}

// FormatCalendar formats a full year's calendar as text ordo.
func FormatCalendar(days []models.CalendarDay) string {
	var lines []string
	currentMonth := 0
	separator := strings.Repeat("-", 60)

	for i := range days {
		day := &days[i]
		month := int(day.Date.Month())
		if month != currentMonth {
			currentMonth = month
			monthName := strings.ToUpper(day.Date.Month().String())
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, monthName, separator)
		}
		lines = append(lines, FormatDay(day))
	}

	return strings.Join(lines, "\n") + "\n"
}
