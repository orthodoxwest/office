package web

import (
	"reflect"
	"sync"
	"testing"

	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/render"
)

func TestCalendarCacheSharesComposedYear(t *testing.T) {
	eng, err := office.NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	cache := newYearCache("../../data")
	const readers = 6
	results := make([][]render.MonthData, readers)
	var wg sync.WaitGroup
	for i := range readers {
		wg.Go(func() {
			var err error
			results[i], err = cache.getMonths(2026, eng)
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for i, months := range results {
		if len(months) != 12 {
			t.Fatalf("reader %d: got %d months", i, len(months))
		}
		if &months[0] != &results[0][0] {
			t.Fatal("concurrent requests did not reuse composed summaries")
		}
	}
	days, moveable, err := cache.get(2026)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(results[0], buildMonthData(days, eng, moveable)) {
		t.Fatal("cached year differs from directly composed calendar")
	}
}

func TestYearCacheEvictionPreservesActiveReaders(t *testing.T) {
	cache := newYearCache("../../data")
	first, err := cache.entry(2026)
	if err != nil {
		t.Fatal(err)
	}
	for year := 2027; year <= 2026+maxCachedYears; year++ {
		if _, err := cache.entry(year); err != nil {
			t.Fatal(err)
		}
	}
	if len(cache.entries) != maxCachedYears {
		t.Fatalf("cache holds %d years", len(cache.entries))
	}
	if _, ok := cache.entries[2026]; ok {
		t.Fatal("oldest year was not evicted")
	}
	if len(first.days) != 365 || first.days[0].Date.Year() != 2026 {
		t.Fatal("eviction invalidated an active reader")
	}
	again, err := cache.entry(2026)
	if err != nil {
		t.Fatal(err)
	}
	if again == first || !reflect.DeepEqual(again.days, first.days) {
		t.Fatal("evicted year was not rebuilt consistently")
	}
}

func TestYearCacheDoesNotCacheBuildFailures(t *testing.T) {
	cache := newYearCache(t.TempDir())
	if _, err := cache.getMonths(2026, nil); err == nil {
		t.Fatal("expected missing data error")
	}
	if len(cache.entries) != 0 || len(cache.order) != 0 {
		t.Fatal("failed year consumed a cache entry")
	}
}
