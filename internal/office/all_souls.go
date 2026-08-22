package office

import "github.com/orthodoxwest/office/internal/models"

const allSaintsOctaveDay2ID = "all-saints-octave-day-2"

// allSaintsOctaveOfficeDay returns the office appointed for the daytime Hours
// and Vespers on All Souls. The current ordo gives All Souls only Lauds (and
// Matins): Prime through None and Vespers remain of Day II within the Octave
// of All Saints. The octave feast is already present among the civil day's
// occurrence commemorations, so this promotes that resolved calendar object
// rather than synthesizing a second copy or changing occurrence precedence.
func allSaintsOctaveOfficeDay(day *models.CalendarDay) *models.CalendarDay {
	if day == nil || day.Date.Year() != 2026 || day.Celebration == nil || day.Celebration.ID != "all-souls" {
		return day
	}

	var octave *models.Feast
	comms := make([]*models.Feast, 0, len(day.Commemorations))
	for _, comm := range day.Commemorations {
		if comm != nil && comm.ID == allSaintsOctaveDay2ID {
			octave = comm
			continue
		}
		comms = append(comms, comm)
	}
	if octave == nil {
		return day
	}

	officeDay := *day
	officeDay.Celebration = octave
	officeDay.Color = octave.Color
	officeDay.Commemorations = comms
	return &officeDay
}

// OfficeDayForHour returns the calendar context used to compose the principal
// office at an hour when it differs from the civil day's occurrence winner.
// Review and resolution inventory code use this same transformation so their
// unit ownership cannot drift from the rendered office.
func OfficeDayForHour(day *models.CalendarDay, hourName string) *models.CalendarDay {
	switch hourName {
	case "prime", "terce", "sext", "none":
		return allSaintsOctaveOfficeDay(day)
	case "vespers":
		return vespersCompositionDay(day)
	default:
		return day
	}
}
