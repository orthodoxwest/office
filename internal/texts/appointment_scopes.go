package texts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// AppointmentScope limits a seasonal fallback lookup. Bounds are offsets
// from Easter in civil days: from is inclusive, until is exclusive. A nil
// bound is open. Inactive scopes skip the tier; they never suppress a slot.
// The loaded values are immutable and contain no transcribed wording.
type AppointmentScope struct {
	ID              string        `json:"id"`
	Source          string        `json:"source"`
	Season          models.Season `json:"season"`
	Hours           []string      `json:"hours"`
	Slots           []string      `json:"slots"`
	RequireFerial   bool          `json:"require_ferial,omitempty"`
	ExcludeWeekdays []string      `json:"exclude_weekdays,omitempty"`
	FromEaster      *int          `json:"from_easter,omitempty"`
	UntilEaster     *int          `json:"until_easter,omitempty"`

	excludedWeekdays [7]bool
}

type appointmentScopeKey struct {
	season models.Season
	hour   string
}

// HasAppointmentScopes reports that a sidecar was successfully validated,
// including an explicitly empty array. A missing file is distinct from [].
func (c *TextCorpus) HasAppointmentScopes() bool {
	return c.appointmentScopes != nil
}

// Allows uses the office date for bounds and the composer's civil weekday
// for weekday predicates, including the existing I Vespers convention.
func (s *AppointmentScope) Allows(date, easter time.Time, weekday time.Weekday, ferial bool) bool {
	return (!s.RequireFerial || ferial) && !s.excludedWeekdays[weekday] &&
		(s.FromEaster == nil || !date.Before(easter.AddDate(0, 0, *s.FromEaster))) &&
		(s.UntilEaster == nil || date.Before(easter.AddDate(0, 0, *s.UntilEaster)))
}

// SeasonalAppointmentScope returns the single scope for this lookup, or nil
// for an unrestricted lookup. Slot is the resolver's base slot (numeric
// antiphon suffixes removed); a trailing * selector matches a slot family.
func (c *TextCorpus) SeasonalAppointmentScope(season models.Season, hour, slot string) *AppointmentScope {
	for _, scope := range c.appointmentScopes[appointmentScopeKey{season, hour}] {
		for _, selector := range scope.Slots {
			if scopeSlotMatches(selector, slot) {
				return scope
			}
		}
	}
	return nil
}

func scopeSlotMatches(selector, slot string) bool {
	if prefix, ok := strings.CutSuffix(selector, "*"); ok {
		return strings.HasPrefix(slot, prefix)
	}
	return slot == selector
}

// LoadAppointmentScopes replaces scope metadata only after the complete file
// passes validation. LoadTexts calls it automatically; small test corpora can
// load a fixture explicitly. Call only during setup, before sharing the corpus.
// Any error, including a missing file, preserves the previous scope index.
func (c *TextCorpus) LoadAppointmentScopes(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var scopes []*AppointmentScope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&scopes); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: expected one JSON array", path)
	}
	if scopes == nil {
		return fmt.Errorf("%s: expected an array, not null", path)
	}
	index := make(map[appointmentScopeKey][]*AppointmentScope)
	ids := make(map[string]bool)
	for _, scope := range scopes {
		if scope == nil {
			return fmt.Errorf("%s: null appointment scope", path)
		}
		if err := scope.validate(c); err != nil {
			return fmt.Errorf("%s: scope %q: %w", path, scope.ID, err)
		}
		if ids[scope.ID] {
			return fmt.Errorf("%s: duplicate scope ID %q", path, scope.ID)
		}
		ids[scope.ID] = true
		for _, hour := range scope.Hours {
			key := appointmentScopeKey{scope.Season, hour}
			for _, prior := range index[key] {
				for _, a := range prior.Slots {
					for _, b := range scope.Slots {
						if scopeSelectorsOverlap(a, b) {
							return fmt.Errorf("%s: scopes %q and %q overlap at %s/%s (%s, %s)", path, prior.ID, scope.ID, scope.Season, hour, a, b)
						}
					}
				}
			}
			index[key] = append(index[key], scope)
		}
	}
	c.appointmentScopes = index
	return nil
}

func (c *TextCorpus) loadAppointmentScopes(dataDir string) error {
	err := c.LoadAppointmentScopes(filepath.Join(dataDir, "appointment-scopes.json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *AppointmentScope) validate(c *TextCorpus) error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Source) == "" || len(s.Hours) == 0 || len(s.Slots) == 0 {
		return fmt.Errorf("id, source, hours and slots are required")
	}
	if !s.Season.Valid() {
		return fmt.Errorf("unknown season %q", s.Season)
	}
	if s.FromEaster != nil && s.UntilEaster != nil && *s.FromEaster >= *s.UntilEaster {
		return fmt.Errorf("from_easter must precede until_easter")
	}
	for _, bound := range []*int{s.FromEaster, s.UntilEaster} {
		if bound != nil && (*bound < -366 || *bound > 366) {
			return fmt.Errorf("easter offset must be between -366 and 366")
		}
	}
	if !s.RequireFerial && len(s.ExcludeWeekdays) == 0 && s.FromEaster == nil && s.UntilEaster == nil {
		return fmt.Errorf("scope requires an applicability restriction")
	}
	for _, weekday := range s.ExcludeWeekdays {
		found := false
		for day := time.Sunday; day <= time.Saturday; day++ {
			if weekday == strings.ToLower(day.String()) {
				if s.excludedWeekdays[day] {
					return fmt.Errorf("duplicate weekday %q", weekday)
				}
				s.excludedWeekdays[day], found = true, true
			}
		}
		if !found {
			return fmt.Errorf("unknown weekday %q", weekday)
		}
	}
	seenHours := make(map[string]bool)
	for _, hour := range s.Hours {
		switch hour {
		case "lauds", "prime", "terce", "sext", "none", "vespers", "compline":
		default:
			return fmt.Errorf("unknown hour %q", hour)
		}
		if seenHours[hour] {
			return fmt.Errorf("duplicate hour %q", hour)
		}
		seenHours[hour] = true
		for i, slot := range s.Slots {
			// This first schema covers the three migrated slot families.
			// Reject hour-qualified/numeric keys: they would never match a
			// base-slot lookup and could silently leave a fallback unscoped.
			switch slot {
			case "chapter", "versicle", "psalm-antiphon*":
			default:
				return fmt.Errorf("invalid slot selector %q", slot)
			}
			for _, prior := range s.Slots[:i] {
				if scopeSelectorsOverlap(prior, slot) {
					return fmt.Errorf("overlapping slot selectors %q and %q", prior, slot)
				}
			}
			if !c.hasSeasonalScopeSlot(s.Season, hour, slot) {
				return fmt.Errorf("slot %q has no seasonal corpus candidate for %s/%s", slot, s.Season, hour)
			}
		}
	}
	return nil
}

func scopeSelectorsOverlap(a, b string) bool {
	return scopeSlotMatches(a, strings.TrimSuffix(b, "*")) || scopeSlotMatches(b, strings.TrimSuffix(a, "*"))
}

func (c *TextCorpus) hasSeasonalScopeSlot(season models.Season, hour, slot string) bool {
	prefix := "seasonal/" + string(season) + "/"
	if family, ok := strings.CutSuffix(slot, "*"); ok {
		for key := range c.texts {
			if strings.HasPrefix(key, prefix+family) {
				return true
			}
		}
		for key := range c.aliases {
			if strings.HasPrefix(key, prefix+family) {
				return true
			}
		}
		return false
	}
	return c.Has(prefix+slot+"-"+hour) || c.Has(prefix+slot)
}
