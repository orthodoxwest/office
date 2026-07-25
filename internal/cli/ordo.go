package cli

import (
	"fmt"
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/output"
)

// cmdOrdo prints the text ordo for a year.
func cmdOrdo(e env, args []string) error {
	year, err := parseYear(args, "ordo")
	if err != nil {
		return err
	}

	days, moveable, engine, err := e.buildYear(year)
	if err != nil {
		return err
	}

	fmt.Fprint(e.out, output.FormatCalendar(days, engine, moveable))
	return nil
}

// cmdRubrics prints a per-day TSV of composed rubric flags (preces, suffrage,
// commemorations) for cross-checking against a printed ordo. scripts/
// ordo-compare.py reads these columns, so their order and spelling are a
// contract.
func cmdRubrics(e env, args []string) error {
	year, err := parseYear(args, "rubrics")
	if err != nil {
		return err
	}

	days, moveable, engine, err := e.buildYear(year)
	if err != nil {
		return err
	}

	commNames := func(comms []office.CommSummary) string {
		names := make([]string, 0, len(comms))
		for _, c := range comms {
			names = append(names, c.Name)
		}
		return strings.Join(names, "; ")
	}

	fmt.Fprintln(e.out, "date\tcelebration\tlauds_preces\tlauds_suffrage\tlauds_comms\thours_preces\tvespers_preces\tvespers_suffrage\tvespers_comms\tbenedictus_ant\tmagnificat_ant")
	for i := range days {
		day := &days[i]
		celebration := "Feria"
		if day.Celebration != nil {
			celebration = day.Celebration.Name
		}
		slug := day.Date.Format("2006-01-02")

		summarize := func(hourName string) (office.HourSummary, error) {
			hour, err := engine.ComposeHour(hourName, day, moveable)
			if err != nil {
				return office.HourSummary{}, fmt.Errorf("composing %s for %s: %w", hourName, slug, err)
			}
			return office.SummarizeHour(hour), nil
		}

		lauds, err := summarize("lauds")
		if err != nil {
			return err
		}
		// The minor hours share one preces disposition; Prime is representative.
		prime, err := summarize("prime")
		if err != nil {
			return err
		}
		vespers, err := summarize("vespers")
		if err != nil {
			return err
		}

		fmt.Fprintln(e.out, strings.Join([]string{
			slug, celebration,
			fmt.Sprintf("%t", lauds.Preces), fmt.Sprintf("%t", lauds.Suffrage), commNames(lauds.Comms),
			fmt.Sprintf("%t", prime.Preces),
			fmt.Sprintf("%t", vespers.Preces), fmt.Sprintf("%t", vespers.Suffrage), commNames(vespers.Comms),
			lauds.GospelAntFull, vespers.GospelAntFull,
		}, "\t"))
	}

	return nil
}

// buildYear builds the calendar, moveable dates, and office engine that the
// year-wide commands all need.
func (e env) buildYear(year int) ([]models.CalendarDay, *calendar.MoveableDates, *office.Engine, error) {
	days, err := calendar.BuildCalendar(year, e.dataDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("building calendar: %w", err)
	}

	engine, err := office.NewEngine(e.dataDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating office engine: %w", err)
	}

	return days, calendar.ComputeMoveableDates(year), engine, nil
}
