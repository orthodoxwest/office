// Package calendar implements the liturgical calendar engine.
package calendar

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// FeastFiles lists the feast definition files to load, in order.
var FeastFiles = []string{"temporal.txt", "sanctoral.txt", "awrv.txt", "commemorations.txt"}

// parseINISections parses an INI-like text file into sections.
// Each [section] header starts a new section. Key = value pairs are collected
// into a map. Lines starting with # are comments. Blank lines are ignored.
func parseINISections(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sections []map[string]string
	var current map[string]string

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			id := trimmed[1 : len(trimmed)-1]
			current = map[string]string{"_id": id}
			sections = append(sections, current)
			continue
		}

		key, value, found := strings.Cut(trimmed, "=")
		if !found {
			return nil, fmt.Errorf("%s:%d: expected Key = value, got %q", path, lineNum, trimmed)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if current == nil {
			return nil, fmt.Errorf("%s:%d: key-value pair outside of section", path, lineNum)
		}
		current[key] = value
	}

	return sections, scanner.Err()
}

// sectionToFeast converts a parsed INI section into a Feast.
func sectionToFeast(m map[string]string, sourceFile string) (*models.Feast, error) {
	f := &models.Feast{
		ID:     m["_id"],
		Name:   m["Name"],
		Source: models.SourceBase,
	}

	if f.ID == "" {
		return nil, fmt.Errorf("%s: section missing ID", sourceFile)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("%s: feast %q missing Name", sourceFile, f.ID)
	}

	rank, err := models.ParseRank(m["Rank"])
	if err != nil {
		return nil, fmt.Errorf("%s: feast %q: %w", sourceFile, f.ID, err)
	}
	f.Rank = rank

	color, err := models.ParseColor(m["Color"])
	if err != nil {
		return nil, fmt.Errorf("%s: feast %q: %w", sourceFile, f.ID, err)
	}
	f.Color = color

	if cat, ok := m["Category"]; ok {
		c, err := models.ParseFeastCategory(cat)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: %w", sourceFile, f.ID, err)
		}
		f.Category = c
	}

	if dr, ok := m["DateRule"]; ok {
		f.DateRule = dr
	}

	if ms, ok := m["Month"]; ok {
		month, err := strconv.Atoi(ms)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: invalid Month: %w", sourceFile, f.ID, err)
		}
		f.Month = month
	}
	if ds, ok := m["Day"]; ok {
		day, err := strconv.Atoi(ds)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: invalid Day: %w", sourceFile, f.ID, err)
		}
		f.Day = day
	}

	if v, ok := m["HasOctave"]; ok {
		f.HasOctave, err = parseDataBool(v)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: HasOctave: %w", sourceFile, f.ID, err)
		}
	}
	if v, ok := m["HasVigil"]; ok {
		f.HasVigil, err = parseDataBool(v)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: HasVigil: %w", sourceFile, f.ID, err)
		}
	}
	if v, ok := m["IsVigil"]; ok {
		f.IsVigil, err = parseDataBool(v)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: IsVigil: %w", sourceFile, f.ID, err)
		}
	}
	if v, ok := m["IsApostolicCompanion"]; ok {
		f.IsApostolicCompanion, err = parseDataBool(v)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: IsApostolicCompanion: %w", sourceFile, f.ID, err)
		}
	}
	if v, ok := m["OnlyWith"]; ok {
		f.OnlyWith = v
	}
	if v, ok := m["SkipRomanLeapShift"]; ok {
		f.SkipRomanLeapShift, err = parseDataBool(v)
		if err != nil {
			return nil, fmt.Errorf("%s: feast %q: SkipRomanLeapShift: %w", sourceFile, f.ID, err)
		}
	}
	if v, ok := m["Source"]; ok {
		f.Source = models.FeastSource(v)
	}
	if v, ok := m["Notes"]; ok {
		f.Notes = v
	}
	if v, ok := m["ProperName"]; ok {
		f.ProperName = v
	}
	if v, ok := m["ProperID"]; ok {
		f.ProperID = v
	}

	// Validate: no unrecognized keys
	knownKeys := map[string]bool{
		"_id": true, "Name": true, "Rank": true, "Color": true,
		"Category": true, "ProperName": true, "ProperID": true, "DateRule": true,
		"Month": true, "Day": true, "HasOctave": true, "HasVigil": true, "IsVigil": true,
		"IsApostolicCompanion": true,
		"OnlyWith":             true, "SkipRomanLeapShift": true, "Source": true, "Notes": true,
	}
	for key := range m {
		if !knownKeys[key] {
			return nil, fmt.Errorf("%s: feast %q: unrecognized key %q", sourceFile, f.ID, key)
		}
	}

	// Validate: must have either a complete, valid month/day or a date rule.
	if (f.Month == 0) != (f.Day == 0) {
		return nil, fmt.Errorf("%s: feast %q must specify Month and Day together", sourceFile, f.ID)
	}
	hasFixed := f.Month != 0 && f.Day != 0
	hasRule := f.DateRule != ""
	if !hasFixed && !hasRule {
		return nil, fmt.Errorf("%s: feast %q must have either Month/Day or DateRule", sourceFile, f.ID)
	}
	if hasFixed && hasRule {
		return nil, fmt.Errorf("%s: feast %q must not have both Month/Day and DateRule", sourceFile, f.ID)
	}
	if hasFixed && !validFixedDate(f.Month, f.Day) {
		return nil, fmt.Errorf("%s: feast %q has invalid fixed date %d/%d", sourceFile, f.ID, f.Month, f.Day)
	}

	return f, nil
}

func parseDataBool(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false, got %q", value)
	}
}

func validFixedDate(month, day int) bool {
	if month < 1 || month > 12 || day < 1 {
		return false
	}
	// Leap year 2000 permits February 29 while still rejecting impossible dates.
	date := time.Date(2000, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int(date.Month()) == month && date.Day() == day
}

// LoadFeasts loads and merges all feast definition files from dataDir/feasts/.
func LoadFeasts(dataDir string) ([]*models.Feast, error) {
	feastDir := filepath.Join(dataDir, "feasts")
	var feasts []*models.Feast

	for _, fname := range FeastFiles {
		path := filepath.Join(feastDir, fname)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		sections, err := parseINISections(path)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", fname, err)
		}

		for _, section := range sections {
			feast, err := sectionToFeast(section, fname)
			if err != nil {
				return nil, err
			}
			feasts = append(feasts, feast)
		}
	}

	return feasts, nil
}
