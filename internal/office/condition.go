package office

import (
	"fmt"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

type conditionKind uint8

const (
	conditionPreces conditionKind = iota
	conditionSuffrage
	conditionCrossCommemoration
	conditionIsFeast
	conditionFestalLaudsPsalmody
	conditionFestalVespersPsalmody
	conditionIsFerial
	conditionWeekday
	conditionFeast
	conditionSeason
)

type conditionClause struct {
	kind    conditionKind
	negated bool
	value   string
	weekday time.Weekday
	season  models.Season
}

// parsedCondition is the deliberately small condition language used by hour
// definitions: comma-separated conjunctions of optionally negated atoms.
type parsedCondition struct {
	clauses []conditionClause
}

func parseCondition(raw string) (parsedCondition, error) {
	if strings.TrimSpace(raw) == "" {
		return parsedCondition{}, fmt.Errorf("condition is empty")
	}

	parts := strings.Split(raw, ",")
	parsed := parsedCondition{clauses: make([]conditionClause, 0, len(parts))}
	for _, part := range parts {
		atom := strings.TrimSpace(part)
		if atom == "" {
			return parsedCondition{}, fmt.Errorf("condition contains an empty clause")
		}
		negated := false
		for strings.HasPrefix(atom, "not-") {
			negated = !negated
			atom = strings.TrimPrefix(atom, "not-")
		}
		clause, err := parseConditionAtom(atom)
		if err != nil {
			return parsedCondition{}, err
		}
		clause.negated = negated
		parsed.clauses = append(parsed.clauses, clause)
	}
	return parsed, nil
}

func parseConditionAtom(atom string) (conditionClause, error) {
	switch atom {
	case "if-preces":
		return conditionClause{kind: conditionPreces}, nil
	case "if-suffrage":
		return conditionClause{kind: conditionSuffrage}, nil
	case "if-cross-commemoration":
		return conditionClause{kind: conditionCrossCommemoration}, nil
	case "is-feast":
		return conditionClause{kind: conditionIsFeast}, nil
	case "festal-lauds-psalmody":
		return conditionClause{kind: conditionFestalLaudsPsalmody}, nil
	case "festal-vespers-psalmody":
		return conditionClause{kind: conditionFestalVespersPsalmody}, nil
	case "is-ferial":
		return conditionClause{kind: conditionIsFerial}, nil
	}
	if value, ok := strings.CutPrefix(atom, "weekday-"); ok {
		weekdays := map[string]time.Weekday{
			"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
			"wednesday": time.Wednesday, "thursday": time.Thursday,
			"friday": time.Friday, "saturday": time.Saturday,
		}
		weekday, valid := weekdays[value]
		if !valid {
			return conditionClause{}, fmt.Errorf("invalid weekday condition %q", atom)
		}
		return conditionClause{kind: conditionWeekday, weekday: weekday, value: value}, nil
	}
	if value, ok := strings.CutPrefix(atom, "feast-"); ok {
		if value == "" {
			return conditionClause{}, fmt.Errorf("feast condition has an empty ID")
		}
		return conditionClause{kind: conditionFeast, value: value}, nil
	}
	if value, ok := strings.CutPrefix(atom, "season-"); ok {
		season, err := models.ParseSeason(value)
		if err != nil {
			return conditionClause{}, fmt.Errorf("invalid season condition %q", atom)
		}
		return conditionClause{kind: conditionSeason, season: season, value: value}, nil
	}
	return conditionClause{}, fmt.Errorf("unknown condition %q", atom)
}

func (c parsedCondition) evaluate(day *models.CalendarDay, moveable *calendar.MoveableDates, corpus *texts.TextCorpus) bool {
	for _, clause := range c.clauses {
		matched := clause.evaluate(day, moveable, corpus)
		if clause.negated {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}

func (c conditionClause) evaluate(day *models.CalendarDay, moveable *calendar.MoveableDates, corpus *texts.TextCorpus) bool {
	switch c.kind {
	case conditionPreces:
		return shouldSayPreces(day, moveable)
	case conditionSuffrage:
		return shouldSaySuffrage(day, moveable)
	case conditionCrossCommemoration:
		return shouldSayCrossCommemoration(day, moveable)
	case conditionIsFeast:
		return day.Celebration != nil &&
			day.Celebration.Category != models.CategoryFeria &&
			day.Celebration.Category != models.CategorySunday
	case conditionFestalLaudsPsalmody:
		return usesFestalLaudsPsalmody(day)
	case conditionFestalVespersPsalmody:
		return usesFestalVespersPsalmody(day, corpus)
	case conditionIsFerial:
		return day.Celebration == nil || day.Celebration.Category == models.CategoryFeria
	case conditionWeekday:
		return civilWeekday(day) == c.weekday
	case conditionFeast:
		return day.Celebration != nil && day.Celebration.ID == c.value
	case conditionSeason:
		return day.Season == c.season
	default:
		return false
	}
}

// evaluateCondition is retained for focused rule tests. Runtime composition
// evaluates the condition already compiled into HourSection.
func evaluateCondition(condition string, day *models.CalendarDay, moveable *calendar.MoveableDates) bool {
	parsed, err := parseCondition(condition)
	return err == nil && parsed.evaluate(day, moveable, nil)
}

func evaluateHourSectionCondition(section HourSection, day *models.CalendarDay, moveable *calendar.MoveableDates, corpus *texts.TextCorpus) bool {
	if section.parsedCondition != nil {
		return section.parsedCondition.evaluate(day, moveable, corpus)
	}
	// Hand-built sections in unit tests do not pass through the file parser.
	parsed, err := parseCondition(section.Condition)
	return err == nil && parsed.evaluate(day, moveable, corpus)
}
