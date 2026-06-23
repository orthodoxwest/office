package calendar

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

const penitentialRulesFile = "penitential.txt"

type penitentialRule struct {
	ID         string
	From       string
	To         string
	Weekdays   map[time.Weekday]bool
	Fast       *bool
	Abstinence *bool
}

func loadPenitentialRules(dataDir string) ([]penitentialRule, error) {
	path := filepath.Join(dataDir, penitentialRulesFile)
	sections, err := parseINISections(path)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", penitentialRulesFile, err)
	}

	rules := make([]penitentialRule, 0, len(sections))
	for _, section := range sections {
		rule, err := sectionToPenitentialRule(section, penitentialRulesFile)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func sectionToPenitentialRule(m map[string]string, sourceFile string) (penitentialRule, error) {
	rule := penitentialRule{
		ID: m["_id"],
	}
	if rule.ID == "" {
		return rule, fmt.Errorf("%s: rule missing ID", sourceFile)
	}

	from := m["From"]
	to := m["To"]
	if from == "" || to == "" {
		return rule, fmt.Errorf("%s: rule %q must have From and To", sourceFile, rule.ID)
	}
	rule.From = from
	rule.To = to

	if raw := m["Weekdays"]; raw != "" {
		weekdays, err := parseWeekdays(raw)
		if err != nil {
			return rule, fmt.Errorf("%s: rule %q: %w", sourceFile, rule.ID, err)
		}
		rule.Weekdays = weekdays
	}

	if raw, ok := m["Fast"]; ok {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return rule, fmt.Errorf("%s: rule %q: invalid Fast value %q", sourceFile, rule.ID, raw)
		}
		rule.Fast = &v
	}
	if raw, ok := m["Abstinence"]; ok {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return rule, fmt.Errorf("%s: rule %q: invalid Abstinence value %q", sourceFile, rule.ID, raw)
		}
		rule.Abstinence = &v
	}
	if rule.Fast == nil && rule.Abstinence == nil {
		return rule, fmt.Errorf("%s: rule %q must set Fast and/or Abstinence", sourceFile, rule.ID)
	}

	knownKeys := map[string]bool{
		"_id": true, "From": true, "To": true, "Weekdays": true,
		"Fast": true, "Abstinence": true,
	}
	for key := range m {
		if !knownKeys[key] {
			return rule, fmt.Errorf("%s: rule %q: unrecognized key %q", sourceFile, rule.ID, key)
		}
	}

	return rule, nil
}

func parseWeekdays(raw string) (map[time.Weekday]bool, error) {
	weekdays := make(map[time.Weekday]bool)
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(strings.ToLower(part))
		if name == "" {
			continue
		}
		weekday, ok := weekdayNames[name]
		if !ok {
			return nil, fmt.Errorf("invalid weekday %q", part)
		}
		weekdays[weekday] = true
	}
	if len(weekdays) == 0 {
		return nil, fmt.Errorf("no valid weekdays configured")
	}
	return weekdays, nil
}

var weekdayNames = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

var anchorOffsetRE = regexp.MustCompile(`^(.*)@([+-]\d+)$`)

func applyPenitentialRules(days []models.CalendarDay, rules []penitentialRule, feastDates map[string]time.Time) error {
	dateIndex := make(map[time.Time]int, len(days))
	for i := range days {
		day := &days[i]
		dateIndex[day.Date] = i
	}

	for _, rule := range rules {
		start, err := resolvePenitentialAnchor(rule.From, days[0].Date.Year(), feastDates)
		if err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}
		end, err := resolvePenitentialAnchor(rule.To, days[0].Date.Year(), feastDates)
		if err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}
		if end.Before(start) {
			return fmt.Errorf("rule %q: To precedes From", rule.ID)
		}

		for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
			if len(rule.Weekdays) > 0 && !rule.Weekdays[current.Weekday()] {
				continue
			}
			idx, ok := dateIndex[current]
			if !ok {
				continue
			}
			if rule.Fast != nil {
				days[idx].Penitential.Fast = *rule.Fast
			}
			if rule.Abstinence != nil {
				days[idx].Penitential.Abstinence = *rule.Abstinence
			}
		}
	}

	for i := range days {
		if days[i].Date.Weekday() == time.Sunday {
			days[i].Penitential.Fast = false
		}
		if days[i].Penitential.Fast {
			days[i].Penitential.Abstinence = true
		}
	}

	return nil
}

func resolvePenitentialAnchor(raw string, year int, feastDates map[string]time.Time) (time.Time, error) {
	anchor, offset, err := splitAnchorOffset(raw)
	if err != nil {
		return time.Time{}, err
	}

	var date time.Time
	switch {
	case strings.HasPrefix(anchor, "feast:"):
		id := strings.TrimPrefix(anchor, "feast:")
		resolved, ok := feastDates[id]
		if !ok {
			return time.Time{}, fmt.Errorf("unknown feast anchor %q", id)
		}
		date = resolved
	case strings.HasPrefix(anchor, "date:"):
		md := strings.TrimPrefix(anchor, "date:")
		parts := strings.Split(md, "-")
		if len(parts) != 2 {
			return time.Time{}, fmt.Errorf("invalid date anchor %q", raw)
		}
		month, err := strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid month in %q", raw)
		}
		day, err := strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid day in %q", raw)
		}
		date = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}, fmt.Errorf("invalid anchor %q", raw)
	}

	return date.AddDate(0, 0, offset), nil
}

func splitAnchorOffset(raw string) (string, int, error) {
	if strings.HasPrefix(raw, "date:") {
		return raw, 0, nil
	}
	m := anchorOffsetRE.FindStringSubmatch(raw)
	if m == nil {
		return raw, 0, nil
	}
	offset, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid offset in %q", raw)
	}
	return m[1], offset, nil
}
