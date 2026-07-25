package audit

import (
	"path/filepath"
	"reflect"
	"testing"
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
	if len(first.OrdinaryFallbacks) == 0 {
		t.Fatal("no fallbacks in the sweep — the ordering is not being exercised")
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
