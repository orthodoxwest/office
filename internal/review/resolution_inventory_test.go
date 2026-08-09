package review

import (
	"fmt"
	"sort"
	"strings"
	"testing"
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
