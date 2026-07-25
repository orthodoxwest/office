package cli

import (
	"fmt"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/output"
)

// composeHour builds the calendar and engine for the given date and composes
// the named hour.
func composeHour(dataDir, hourName string, date time.Time) (*models.OfficeHour, error) {
	year := date.Year()
	moveable := calendar.ComputeMoveableDates(year)

	days, err := calendar.BuildCalendar(year, dataDir)
	if err != nil {
		return nil, fmt.Errorf("building calendar: %w", err)
	}

	dayIndex := date.YearDay() - 1
	if dayIndex < 0 || dayIndex >= len(days) {
		return nil, fmt.Errorf("date out of range")
	}

	engine, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}

	return engine.ComposeHour(hourName, &days[dayIndex], moveable)
}

// cmdHour prints one composed hour as plain text.
func cmdHour(e env, hourName string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: office %s YYYY-MM-DD", hourName)
	}

	date, err := parseDate(args[0])
	if err != nil {
		return err
	}

	hour, err := composeHour(e.dataDir, hourName, date)
	if err != nil {
		return fmt.Errorf("composing %s: %w", hourName, err)
	}

	fmt.Fprint(e.out, output.FormatOfficeHour(hour))
	return nil
}

// cmdTeX emits one composed hour as a LuaLaTeX booklet source.
func cmdTeX(e env, args []string) error {
	const texUsage = "usage: office tex [--chant] HOUR [YYYY-MM-DD]\n" +
		"Example: office tex lauds 2026-03-11 > lauds.tex\n" +
		"         office tex --chant compline > compline.tex"

	// Parse the optional --chant flag; collect remaining positional args.
	chant := false
	var positional []string
	for _, a := range args {
		if a == "--chant" {
			chant = true
		} else {
			positional = append(positional, a)
		}
	}

	if len(positional) < 1 {
		return fmt.Errorf("%s", texUsage)
	}

	hourName := positional[0]
	date := time.Now()
	if len(positional) >= 2 {
		var err error
		if date, err = parseDate(positional[1]); err != nil {
			return err
		}
	}

	hour, err := composeHour(e.dataDir, hourName, date)
	if err != nil {
		return fmt.Errorf("composing %s: %w", hourName, err)
	}

	fmt.Fprint(e.out, output.FormatOfficeHourTeX(hour, e.dataDir, chant))
	return nil
}
