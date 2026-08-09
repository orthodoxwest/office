// Package review provides resolution inventory reports for diurnal ingestion.
package review

import (
	"fmt"
	"sort"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/office"
)

// ResolutionInventoryRow is one representative dynamic proper resolution.
// It intentionally carries source keys and calendar metadata only, never text.
type ResolutionInventoryRow struct {
	OwnerID          string   `json:"owner_id,omitempty"`
	CanonicalOwner   string   `json:"canonical_owner,omitempty"`
	ProperIDs        []string `json:"proper_ids,omitempty"`
	Hour             string   `json:"hour"`
	FirstVespers     bool     `json:"first_vespers,omitempty"`
	RequestedSlot    string   `json:"requested_slot"`
	SlotRef          string   `json:"slot_ref"`
	ResolverHour     string   `json:"resolver_hour"`
	ResolverSlot     string   `json:"resolver_slot"`
	DirectCandidates []string `json:"direct_candidates,omitempty"`
	DirectExisting   []string `json:"direct_existing,omitempty"`
	SelectedRef      string   `json:"selected_ref"`
	SelectedTier     string   `json:"selected_tier"`
	Reason           string   `json:"reason"`
	Date             string   `json:"date"`
	Occurrences      int      `json:"occurrences"`
}

// ResolutionInventory is a deduplicated calendar sweep of dynamic proper
// resolutions. The owner is the evening owner for Vespers.
type ResolutionInventory struct {
	StartYear int                      `json:"start_year"`
	Years     int                      `json:"years"`
	Rows      []ResolutionInventoryRow `json:"rows"`
}

func isDynamicResolutionRef(ref string) bool {
	for _, prefix := range []string{"proper/", "commons/", "seasonal/", "ordinary/"} {
		if len(ref) >= len(prefix) && ref[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// BuildResolutionInventory sweeps composed hours and records the selected
// fallback tier for each dynamic slot. It uses the actual composed source key,
// so special composer paths (collects and hymn doxologies) stay truthful.
func BuildResolutionInventory(dataDir string, startYear, years int) (*ResolutionInventory, error) {
	if years < 1 {
		return nil, fmt.Errorf("years must be positive")
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating office engine: %w", err)
	}
	byKey := make(map[string]*ResolutionInventoryRow)
	for y := startYear; y < startYear+years; y++ {
		moveable := calendar.ComputeMoveableDates(y)
		days, err := calendar.BuildCalendar(y, dataDir)
		if err != nil {
			return nil, fmt.Errorf("building calendar for %d: %w", y, err)
		}
		for i := range days {
			day := &days[i]
			for _, hourName := range HourNames {
				hour, err := eng.ComposeHour(hourName, day, moveable)
				if err != nil {
					return nil, fmt.Errorf("composing %s for %s: %w", hourName, day.Date.Format("2006-01-02"), err)
				}
				for _, section := range hour.Sections {
					for _, element := range section.Elements {
						if element.SlotRef == "" {
							continue
						}
						// SourceRef is the source actually selected by the composer.
						// Tracing it as metadata (rather than resolving again) keeps
						// special composition paths truthful.
						trace := eng.TraceProperResolution(day, hourName, element.SlotRef, element.SourceRef)
						if !isDynamicResolutionRef(element.SourceRef) && trace.SelectedTier != "not-found" {
							continue
						}
						// Rows without a canonical office owner cannot support a
						// feast-specific proper proposal. They add substantial ferial
						// noise (especially Compline) without a safe target namespace.
						if trace.OwnerID == "" {
							continue
						}
						first := trace.FirstVespers
						key := trace.OwnerID + "\x1f" + hourName + "\x1f" + fmt.Sprint(first) + "\x1f" + trace.SlotRef + "\x1f" + trace.SelectedRef + "\x1f" + trace.Reason
						if old := byKey[key]; old != nil {
							old.Occurrences++
							continue
						}
						byKey[key] = &ResolutionInventoryRow{OwnerID: trace.OwnerID, CanonicalOwner: trace.CanonicalOwner, ProperIDs: trace.ProperIDs, Hour: hourName, FirstVespers: first, RequestedSlot: trace.RequestedSlot, SlotRef: trace.SlotRef, ResolverHour: trace.ResolverHour, ResolverSlot: trace.ResolverSlot, DirectCandidates: trace.DirectCandidates, DirectExisting: trace.DirectExisting, SelectedRef: trace.SelectedRef, SelectedTier: trace.SelectedTier, Reason: trace.Reason, Date: day.Date.Format("2006-01-02"), Occurrences: 1}
					}
				}
			}
		}
	}
	rows := make([]ResolutionInventoryRow, 0, len(byKey))
	for _, row := range byKey {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.OwnerID != b.OwnerID {
			return a.OwnerID < b.OwnerID
		}
		if a.Hour != b.Hour {
			return a.Hour < b.Hour
		}
		if a.SlotRef != b.SlotRef {
			return a.SlotRef < b.SlotRef
		}
		if a.SelectedRef != b.SelectedRef {
			return a.SelectedRef < b.SelectedRef
		}
		return a.Date < b.Date
	})
	return &ResolutionInventory{StartYear: startYear, Years: years, Rows: rows}, nil
}

// FilterFallbacks retains rows whose selected key is not a direct proper.
func (r *ResolutionInventory) FilterFallbacks() {
	rows := r.Rows[:0]
	for _, row := range r.Rows {
		if row.SelectedTier != "proper" {
			rows = append(rows, row)
		}
	}
	r.Rows = rows
}

// ResolutionInventorySummary counts rows and occurrences by selected tier.
func ResolutionInventorySummary(r *ResolutionInventory) map[string]int {
	out := make(map[string]int)
	for _, row := range r.Rows {
		out[row.SelectedTier] += row.Occurrences
	}
	return out
}
