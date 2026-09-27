package dump

import (
	"fmt"
	"slices"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Record groups a Selection can ask for.
const (
	GroupCalendar = "calendar" // calendar_year and calendar_day
	GroupOffice   = "office"   // office_day
	GroupHours    = "hours"    // hour
)

// Groups is the canonical group order.
var Groups = []string{GroupCalendar, GroupOffice, GroupHours}

// HourNames is the canonical order of hours within a date.
var HourNames = []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

// Selection chooses a slice of the record stream. Either set StartYear and
// Years for whole civil years, or set Dates for individual days. Empty Hours,
// Forms, or Groups mean all of them.
type Selection struct {
	StartYear int
	Years     int
	Dates     []time.Time
	Hours     []string
	Forms     []models.PrayerForm
	Groups    []string
}

// normalize validates the selection and puts every list in canonical order,
// so the record order depends only on what was selected, not how it was
// spelled on the command line.
func (s Selection) normalize() (Selection, error) {
	if len(s.Dates) > 0 {
		if s.StartYear != 0 || s.Years != 0 {
			return s, fmt.Errorf("select either dates or a year window, not both")
		}
		dates := make([]time.Time, 0, len(s.Dates))
		for _, d := range s.Dates {
			dates = append(dates, time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC))
		}
		slices.SortFunc(dates, func(a, b time.Time) int { return a.Compare(b) })
		s.Dates = slices.Compact(dates)
	} else if s.StartYear < 1 || s.Years < 1 {
		return s, fmt.Errorf("select dates or a year window (start year and at least one year)")
	}
	var err error
	if s.Hours, err = canonicalSubset("hour", s.Hours, HourNames); err != nil {
		return s, err
	}
	if s.Groups, err = canonicalSubset("record group", s.Groups, Groups); err != nil {
		return s, err
	}
	forms := make([]string, 0, len(s.Forms))
	for _, f := range s.Forms {
		forms = append(forms, string(f))
	}
	forms, err = canonicalSubset("prayer form", forms, stringList(models.PrayerForms))
	if err != nil {
		return s, err
	}
	s.Forms = s.Forms[:0:0]
	for _, f := range forms {
		s.Forms = append(s.Forms, models.PrayerForm(f))
	}
	return s, nil
}

// canonicalSubset returns the members of order that appear in chosen, in
// order's sequence, or all of order when chosen is empty.
func canonicalSubset(what string, chosen, order []string) ([]string, error) {
	if len(chosen) == 0 {
		return slices.Clone(order), nil
	}
	for _, c := range chosen {
		if !slices.Contains(order, c) {
			return nil, fmt.Errorf("unknown %s %q (want one of %v)", what, c, order)
		}
	}
	var out []string
	for _, o := range order {
		if slices.Contains(chosen, o) {
			out = append(out, o)
		}
	}
	return out, nil
}

func stringList[S ~string](values []S) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, string(v))
	}
	return out
}

func (s Selection) has(group string) bool { return slices.Contains(s.Groups, group) }

func (s Selection) meta() Record {
	sel := Record{
		"start_year": nil,
		"years":      nil,
		"dates":      nil,
		"hours":      anyList(s.Hours),
		"forms":      anyList(stringList(s.Forms)),
		"groups":     anyList(s.Groups),
	}
	if len(s.Dates) > 0 {
		dates := make([]string, 0, len(s.Dates))
		for _, d := range s.Dates {
			dates = append(dates, day(d))
		}
		sel["dates"] = anyList(dates)
	} else {
		sel["start_year"], sel["years"] = s.StartYear, s.Years
	}
	return Record{"kind": KindMeta, "format": Format, "selection": sel}
}

func anyList(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

// yearPlan is one civil year of the selection; nil dates means every day.
type yearPlan struct {
	year  int
	dates []time.Time
}

func (s Selection) plan() []yearPlan {
	if len(s.Dates) == 0 {
		plans := make([]yearPlan, 0, s.Years)
		for y := s.StartYear; y < s.StartYear+s.Years; y++ {
			plans = append(plans, yearPlan{year: y})
		}
		return plans
	}
	var plans []yearPlan
	for _, d := range s.Dates {
		if n := len(plans); n > 0 && plans[n-1].year == d.Year() {
			plans[n-1].dates = append(plans[n-1].dates, d)
			continue
		}
		plans = append(plans, yearPlan{year: d.Year(), dates: []time.Time{d}})
	}
	return plans
}

// Generator produces dump records from one data directory. It is safe for
// concurrent use: the office engine is immutable once built.
type Generator struct {
	dataDir string
	engine  *office.Engine
}

// NewGenerator loads the corpus and hour definitions under dataDir.
func NewGenerator(dataDir string) (*Generator, error) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}
	return &Generator{dataDir: dataDir, engine: engine}, nil
}

// Generate calls emit for each selected record in canonical order: the meta
// record; then per civil year, calendar_year followed by each date's
// calendar_day, office_day, and hour records (hours in HourNames order, each
// in models.PrayerForms order).
func (g *Generator) Generate(sel Selection, emit func(Record) error) error {
	sel, err := sel.normalize()
	if err != nil {
		return err
	}
	if err := emit(sel.meta()); err != nil {
		return err
	}
	for _, p := range sel.plan() {
		if err := g.generateYear(sel, p, emit); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) generateYear(sel Selection, p yearPlan, emit func(Record) error) error {
	days, err := calendar.BuildCalendar(p.year, g.dataDir)
	if err != nil {
		return fmt.Errorf("building calendar for %d: %w", p.year, err)
	}
	moveable := calendar.ComputeMoveableDates(p.year)
	if sel.has(GroupCalendar) {
		if err := emit(calendarYearRecord(p.year, calendar.ComputeTabula(p.year), moveable)); err != nil {
			return err
		}
	}
	indices := make([]int, 0, len(days))
	if p.dates == nil {
		for i := range days {
			indices = append(indices, i)
		}
	} else {
		for _, d := range p.dates {
			indices = append(indices, d.YearDay()-1)
		}
	}
	for n, i := range indices {
		if i < 0 || i >= len(days) {
			return fmt.Errorf("%s is outside the %d calendar", day(p.dates[n]), p.year)
		}
		d := &days[i]
		if p.dates != nil && day(d.Date) != day(p.dates[n]) {
			return fmt.Errorf("calendar for %d returned %s at %s", p.year, day(d.Date), day(p.dates[n]))
		}
		if err := g.generateDay(sel, d, moveable, emit); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) generateDay(sel Selection, d *models.CalendarDay, moveable *calendar.MoveableDates, emit func(Record) error) error {
	if sel.has(GroupCalendar) {
		rec, err := calendarDayRecord(d)
		if err != nil {
			return err
		}
		if err := emit(rec); err != nil {
			return err
		}
	}
	if sel.has(GroupOffice) {
		rec, err := officeDayRecord(d)
		if err != nil {
			return err
		}
		if err := emit(rec); err != nil {
			return err
		}
	}
	if !sel.has(GroupHours) {
		return nil
	}
	for _, hourName := range sel.Hours {
		for _, form := range sel.Forms {
			h, err := g.engine.ComposeHourWithOptions(hourName, d, moveable, office.ComposeOptions{Form: form})
			if err != nil {
				return fmt.Errorf("composing %s %s for %s: %w", hourName, form, day(d.Date), err)
			}
			rec, err := hourRecord(d.Date, hourName, form, h)
			if err != nil {
				return err
			}
			if err := emit(rec); err != nil {
				return err
			}
		}
	}
	return nil
}
