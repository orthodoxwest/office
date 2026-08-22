package review

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

func TestDynamicResolutionRef(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{"proper/feast/collect", true},
		{"commons/martyr/collect", true},
		{"seasonal/lent/collect", true},
		{"ordinary/lauds/collect", true},
		{"proper", false},
		{"psalms/1", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			if got := isDynamicResolutionRef(tt.ref); got != tt.want {
				t.Fatalf("dynamic = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolutionInventorySummaryAndFilterFallbacks(t *testing.T) {
	inventory := &ResolutionInventory{Rows: []ResolutionInventoryRow{
		{SelectedTier: "proper", Occurrences: 3},
		{SelectedTier: "common", Occurrences: 2},
		{SelectedTier: "ordinary", Occurrences: 4},
		{SelectedTier: "not-found", Occurrences: 1},
	}}
	summary := ResolutionInventorySummary(inventory)
	if summary["proper"] != 3 || summary["common"] != 2 || summary["ordinary"] != 4 || summary["not-found"] != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	inventory.FilterFallbacks()
	if len(inventory.Rows) != 3 {
		t.Fatalf("fallback rows = %#v", inventory.Rows)
	}
	for _, row := range inventory.Rows {
		if row.SelectedTier == "proper" {
			t.Fatalf("direct proper survived fallback filter: %#v", inventory.Rows)
		}
	}
}

func TestBuildResolutionInventoryRejectsNonpositiveYears(t *testing.T) {
	for _, years := range []int{0, -1} {
		if _, err := BuildResolutionInventory("../../data", 2026, years); err == nil {
			t.Fatalf("years=%d unexpectedly succeeded", years)
		}
	}
}

func TestTraceInventoryElementEmptyCommemorationOwnerFailsClosed(t *testing.T) {
	engine, err := office.NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	// The ID-less commemoration is the trap: an empty owner would otherwise
	// match it and be reported as found.
	day := &models.CalendarDay{
		Celebration:    &models.Feast{ID: "principal-feast"},
		Commemorations: []*models.Feast{{Name: "Commemoration with no ID"}},
	}
	element := models.OfficeElement{IsCommemoration: true, SlotRef: "commemoration-collect", SourceRef: "ordinary/lauds/collect"}
	trace := traceInventoryElement(engine, day, "lauds", element)
	if trace.OwnerID != "" || trace.CanonicalOwner != "" {
		t.Fatalf("empty commemoration owner attributed to principal: %#v", trace)
	}
	if trace.Reason != office.UnknownCommemorationOwnerReason {
		t.Fatalf("reason = %q, want %q", trace.Reason, office.UnknownCommemorationOwnerReason)
	}
}

func TestBuildResolutionInventoryTracesAndDeduplicates(t *testing.T) {
	inventory, err := BuildResolutionInventory("../../data", 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.StartYear != 2026 || inventory.Years != 1 || len(inventory.Rows) == 0 {
		t.Fatalf("inventory metadata/rows = %#v", inventory)
	}

	tiers := make(map[string]bool)
	keys := make(map[string]bool)
	totalOccurrences := 0
	seenRepeated, seenMinorCollect, seenPrimeLauds := false, false, false
	seenPriscaCollect, seenPriscaVespers := false, false
	for _, row := range inventory.Rows {
		if !strings.HasPrefix(row.Date, "2026-") {
			t.Fatalf("row escaped requested year: %#v", row)
		}
		if row.OwnerID == "" || row.CanonicalOwner == "" || row.RequestedSlot == "" || row.ResolverHour == "" || row.ResolverSlot == "" || row.SelectedRef == "" || row.SelectedTier == "" {
			t.Fatalf("incomplete row: %#v", row)
		}
		if row.Occurrences < 1 {
			t.Fatalf("nonpositive occurrence count: %#v", row)
		}
		if row.Occurrences > 1 {
			seenRepeated = true
		}
		if (row.Hour == "terce" || row.Hour == "sext" || row.Hour == "none") && row.RequestedSlot == "collect" {
			if row.ResolverHour != "lauds" || row.ResolverSlot != "collect" {
				t.Fatalf("minor-hour collect coordinates = %#v", row)
			}
			seenMinorCollect = true
		}
		if row.Hour == "prime" && row.ResolverHour == "lauds" {
			seenPrimeLauds = true
		}
		if row.OwnerID == "comm-01-18-st-prisca-of-rome-virgin-martyr" {
			if row.Hour == "lauds" && row.RequestedSlot == "commemoration-collect" && row.Date == "2026-01-18" {
				if row.SelectedRef != "commons/virgin-martyr/commemoration-collect" || row.SelectedTier != "common" {
					t.Fatalf("Prisca Lauds collect = %#v", row)
				}
				seenPriscaCollect = true
			}
			if row.Hour == "vespers" && strings.HasPrefix(row.RequestedSlot, "commemoration-") {
				seenPriscaVespers = true
			}
		}
		key := fmt.Sprintf("%s\x1f%s\x1f%t\x1f%s\x1f%s\x1f%s", row.OwnerID, row.Hour, row.FirstVespers, row.SlotRef, row.SelectedRef, row.Reason)
		if keys[key] {
			t.Fatalf("duplicate inventory row key: %q", key)
		}
		keys[key] = true
		tiers[row.SelectedTier] = true
		totalOccurrences += row.Occurrences
	}
	if !seenRepeated || totalOccurrences <= len(inventory.Rows) {
		t.Fatalf("deduplication not observed: rows=%d occurrences=%d", len(inventory.Rows), totalOccurrences)
	}
	if !seenMinorCollect || !seenPrimeLauds {
		t.Fatalf("exceptional resolver coordinates absent: minor=%v prime=%v", seenMinorCollect, seenPrimeLauds)
	}
	if !seenPriscaCollect || !seenPriscaVespers {
		t.Fatalf("Prisca commemoration inventory rows absent: collect=%v vespers=%v", seenPriscaCollect, seenPriscaVespers)
	}
	for _, tier := range []string{"proper", "common", "seasonal", "ordinary", "shared", "temporal-week", "special"} {
		if !tiers[tier] {
			t.Fatalf("expected tier %q in %#v", tier, tiers)
		}
	}
	if !sort.SliceIsSorted(inventory.Rows, func(i, j int) bool {
		a, b := inventory.Rows[i], inventory.Rows[j]
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
	}) {
		t.Fatal("inventory rows are not deterministically sorted")
	}
}
