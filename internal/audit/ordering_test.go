package audit

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// repoData is the repository's own data directory: these reports are only
// interesting on a corpus large enough to produce tied sort keys.
func repoData() string { return filepath.Join("..", "..", "data") }

// Both reports are assembled by ranging over maps, so any pair of rows the
// comparator calls equal comes out in a different order on each run. That made
// `make audit` and `make lint-texts` impossible to diff between runs, and the
// golden tests never caught it because they cover audit.Run, not the sweep.

func TestSweepOrderingIsStableAcrossRuns(t *testing.T) {
	first, err := SweepYear(repoData(), 2026)
	if err != nil {
		t.Fatalf("SweepYear: %v", err)
	}
	second, err := SweepYear(repoData(), 2026)
	if err != nil {
		t.Fatalf("SweepYear: %v", err)
	}

	if !reflect.DeepEqual(first.NotFound, second.NotFound) {
		t.Errorf("not-found rows are ordered differently between runs")
	}
	if !reflect.DeepEqual(first.OrdinaryFallbacks, second.OrdinaryFallbacks) {
		for i := range first.OrdinaryFallbacks {
			if i < len(second.OrdinaryFallbacks) && first.OrdinaryFallbacks[i] != second.OrdinaryFallbacks[i] {
				t.Fatalf("fallback rows diverge at %d:\n  %+v\n  %+v",
					i, first.OrdinaryFallbacks[i], second.OrdinaryFallbacks[i])
			}
		}
		t.Error("fallback rows are ordered differently between runs")
	}
}

// The run-to-run check above cannot see an ordering bug the repo's own data
// does not happen to trigger. It used to assert the sweep held at least one
// fallback, on the assumption that it always would; once the propers were
// completed it held none and the assertion failed rather than the check
// quietly going vacuous. The comparators are exercised directly instead, on
// rows built to tie on every earlier key.

func TestOrdinaryFallbackOrderingIsTotal(t *testing.T) {
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	rows := []OrdinaryFallback{
		{FeastID: "b-feast", Rank: models.Double, Hour: "vespers", Slot: "hymn", SourceRef: "ordinary/vespers/hymn-sunday", FirstDate: day},
		{FeastID: "b-feast", Rank: models.Double, Hour: "vespers", Slot: "hymn", SourceRef: "ordinary/vespers/hymn-saturday", FirstDate: day},
		{FeastID: "b-feast", Rank: models.Double, Hour: "vespers", Slot: "chapter", SourceRef: "ordinary/vespers/chapter", FirstDate: day},
		{FeastID: "b-feast", Rank: models.Double, Hour: "lauds", Slot: "hymn", SourceRef: "ordinary/lauds/hymn-sunday", FirstDate: day},
		{FeastID: "a-feast", Rank: models.Double, Hour: "lauds", Slot: "hymn", SourceRef: "ordinary/lauds/hymn-sunday", FirstDate: day},
		{FeastID: "a-feast", Rank: models.GreaterDouble, Hour: "lauds", Slot: "hymn", SourceRef: "ordinary/lauds/hymn-sunday", FirstDate: day},
		// Tie on every key but the date.
		{FeastID: "a-feast", Rank: models.GreaterDouble, Hour: "lauds", Slot: "chapter", SourceRef: "ordinary/lauds/chapter", FirstDate: day.AddDate(0, 0, 1)},
		{FeastID: "a-feast", Rank: models.GreaterDouble, Hour: "lauds", Slot: "chapter", SourceRef: "ordinary/lauds/chapter", FirstDate: day},
	}
	assertTotalOrder(t, len(rows), func(i, j int) bool {
		return lessOrdinaryFallback(&rows[i], &rows[j])
	})
}

func TestNotFoundOrderingIsTotal(t *testing.T) {
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	rows := []NotFoundText{
		{Marker: "[Proper text not found: hymn]", Hour: "vespers", FirstDate: day},
		{Marker: "[Proper text not found: hymn]", Hour: "lauds", FirstDate: day},
		{Marker: "[Proper text not found: chapter]", Hour: "lauds", FirstDate: day},
		// Tie on every key but the date.
		{Marker: "[Proper text not found: collect]", Hour: "lauds", FirstDate: day.AddDate(0, 0, 1)},
		{Marker: "[Proper text not found: collect]", Hour: "lauds", FirstDate: day},
	}
	assertTotalOrder(t, len(rows), func(i, j int) bool {
		return lessNotFound(&rows[i], &rows[j])
	})
}

// assertTotalOrder reports any pair the comparator leaves unordered. A total
// comparator answers true in exactly one direction for every distinct pair;
// one that answers false both ways lets sort.Slice return either arrangement.
func assertTotalOrder(t *testing.T, n int, less func(i, j int) bool) {
	t.Helper()
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if !less(i, j) && !less(j, i) {
				t.Errorf("rows %d and %d compare equal, so their order is arbitrary", i, j)
			}
			if less(i, j) && less(j, i) {
				t.Errorf("rows %d and %d each sort before the other", i, j)
			}
		}
	}
}

func TestLintOrderingIsStableAcrossRuns(t *testing.T) {
	first, err := Lint(repoData())
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	second, err := Lint(repoData())
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}

	if !reflect.DeepEqual(first.Advisory, second.Advisory) {
		for i := range first.Advisory {
			if i < len(second.Advisory) && first.Advisory[i] != second.Advisory[i] {
				t.Fatalf("advisory findings diverge at %d:\n  %+v\n  %+v",
					i, first.Advisory[i], second.Advisory[i])
			}
		}
		t.Error("advisory findings are ordered differently between runs")
	}
	if len(first.Advisory) == 0 {
		t.Fatal("no advisory findings — the ordering is not being exercised")
	}
}
