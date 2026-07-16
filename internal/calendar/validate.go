package calendar

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// ValidateAll runs the full three-layer validation pipeline on all data files.
func ValidateAll(dataDir string) []string {
	var allErrors []string
	var allFeasts []*models.Feast

	// Feast files
	feastDir := filepath.Join(dataDir, "feasts")
	for _, fname := range FeastFiles {
		path := filepath.Join(feastDir, fname)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			allErrors = append(allErrors, fmt.Sprintf("Missing data file: %s", path))
			continue
		}

		// Layer 1: Syntax
		sections, err := parseINISections(path)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("Syntax error in %s: %v", fname, err))
			continue
		}

		// Layer 2: Schema
		var schemaErrors []string
		var feasts []*models.Feast
		for _, section := range sections {
			feast, err := sectionToFeast(section, fname)
			if err != nil {
				schemaErrors = append(schemaErrors, err.Error())
			} else {
				feasts = append(feasts, feast)
			}
		}
		allErrors = append(allErrors, schemaErrors...)
		if len(schemaErrors) == 0 {
			allFeasts = append(allFeasts, feasts...)
		}
	}

	// Layer 3: Semantic validation
	if len(allFeasts) > 0 {
		allErrors = append(allErrors, validateSemantics(allFeasts)...)
	}

	return allErrors
}

// validateSemantics performs cross-entry checks on parsed feasts.
func validateSemantics(feasts []*models.Feast) []string {
	var errs []string

	// Duplicate IDs
	seenIDs := make(map[string]int)
	for _, f := range feasts {
		seenIDs[f.ID]++
		if seenIDs[f.ID] > 1 {
			errs = append(errs, fmt.Sprintf("Duplicate feast ID: '%s'", f.ID))
		}
	}

	// only-with target IDs must exist.
	for _, f := range feasts {
		if f.OnlyWith == "" {
			continue
		}
		if seenIDs[f.OnlyWith] == 0 {
			errs = append(errs, fmt.Sprintf(
				"Feast '%s' has OnlyWith target '%s' which does not exist",
				f.ID, f.OnlyWith,
			))
		}
	}

	// Conflicting fixed dates at the same rank
	type dateRankKey struct {
		month int
		day   int
		rank  models.Rank
	}
	fixedDates := make(map[dateRankKey][]string)
	for _, f := range feasts {
		if f.IsFixed() {
			key := dateRankKey{f.Month, f.Day, f.Rank}
			fixedDates[key] = append(fixedDates[key], f.ID)
		}
	}
	for key, ids := range fixedDates {
		if key.rank == models.Commemoration {
			continue
		}
		if len(ids) > 1 {
			errs = append(errs, fmt.Sprintf(
				"Multiple feasts on %d/%d at rank %s: %s",
				key.month, key.day, key.rank, strings.Join(ids, ", "),
			))
		}
	}

	// Octave validation
	allowedOctaveRanks := map[models.Rank]bool{
		models.Double1stClass: true,
		models.Double2ndClass: true,
	}
	for _, f := range feasts {
		if f.HasOctave && !allowedOctaveRanks[f.Rank] {
			errs = append(errs, fmt.Sprintf(
				"Feast '%s' has an octave but is only ranked %s",
				f.ID, f.Rank,
			))
		}
	}

	// DateRule format validation
	for _, f := range feasts {
		if f.DateRule != "" && !isValidDateRule(f.DateRule) {
			errs = append(errs, fmt.Sprintf(
				"Feast '%s' has unrecognized DateRule: %q (expected easter±N, epiphany-sunday-N, advent-sunday-N, pentecost-sunday-N, holy-name, or last-sunday-october)",
				f.ID, f.DateRule,
			))
		}
	}

	return errs
}

// isValidDateRule reports whether rule is a recognized DateRule pattern.
// The three supported patterns mirror the logic in resolveFeastDate.
func isValidDateRule(rule string) bool {
	if easterOffsetRE.MatchString(rule) {
		return true
	}
	if suffix, ok := strings.CutPrefix(rule, "epiphany-sunday-"); ok {
		n, err := strconv.Atoi(suffix)
		return err == nil && n >= 1
	}
	if suffix, ok := strings.CutPrefix(rule, "advent-sunday-"); ok {
		n, err := strconv.Atoi(suffix)
		return err == nil && n >= 1 && n <= 4
	}
	if suffix, ok := strings.CutPrefix(rule, "pentecost-sunday-"); ok {
		n, err := strconv.Atoi(suffix)
		return err == nil && n >= 1
	}
	if rule == "holy-name" {
		return true
	}
	if rule == "last-sunday-october" {
		return true
	}
	return false
}
