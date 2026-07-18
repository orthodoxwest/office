package review

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadZeroClassifications(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	inventory, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash := inventory.ByKey()["example"].ContentHash
	path := filepath.Join(dir, "review", zeroOccurrenceFile)
	writeTestFile(t, path, strings.Join(zeroOccurrenceHeader, ",")+"\n"+
		"example,"+hash+",dormant-policy,retained for an alternate local policy,\n")

	classifications, err := LoadZeroClassifications(dir, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if got := classifications["example"]; !got.Classified() || got.Stale {
		t.Fatalf("classification = %#v", got)
	}

	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Edited text.\n")
	inventory, err = ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	classifications, err = LoadZeroClassifications(dir, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if got := classifications["example"]; !got.Stale || got.Classified() {
		t.Fatalf("edited classification = %#v", got)
	}
}

func TestLoadZeroClassificationsValidates(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	inventory, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "review", zeroOccurrenceFile)
	header := strings.Join(zeroOccurrenceHeader, ",") + "\n"

	writeTestFile(t, path, header+"example,abc,defect,suspected resolver bug,\n")
	if _, err := LoadZeroClassifications(dir, inventory); err == nil || !strings.Contains(err.Error(), "issue") {
		t.Fatalf("missing issue error = %v", err)
	}

	writeTestFile(t, path, header+"example,abc,guess,reason,\n")
	if _, err := LoadZeroClassifications(dir, inventory); err == nil || !strings.Contains(err.Error(), "disposition") {
		t.Fatalf("invalid disposition error = %v", err)
	}

	writeTestFile(t, path, header+"missing,abc,dead,reason,\n")
	if _, err := LoadZeroClassifications(dir, inventory); err == nil || !strings.Contains(err.Error(), "unknown corpus key") {
		t.Fatalf("unknown key error = %v", err)
	}

	writeTestFile(t, path, header+
		"example,abc,dead,reason,\n"+
		"example,abc,dead,other reason,\n")
	if _, err := LoadZeroClassifications(dir, inventory); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestDetectZeroHeuristic(t *testing.T) {
	tests := map[string]ZeroHeuristic{
		"ordinary/lauds/psalm-antiphon":                       ZeroGenericPsalmAntiphon,
		"proper/example/psalm-antiphon-4":                     ZeroIndexedPsalmAntiphon,
		"proper/example/commemoration-versicle":               ZeroCommemorationSlot,
		"proper/example/benedictus-antiphon-wednesday":        ZeroWeekdayVariant,
		"proper/example/magnificat-antiphon-first":            ZeroFirstVespersVariant,
		"proper/example/short-responsory-vespers":             ZeroOther,
		"proper/example/psalm-antiphon-lauds":                 ZeroOther,
		"proper/example/commemoration-antiphon-first-vespers": ZeroCommemorationSlot,
	}
	for key, want := range tests {
		if got := DetectZeroHeuristic(key); got != want {
			t.Errorf("DetectZeroHeuristic(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestZeroOccurrenceReportFiltersAndGroups(t *testing.T) {
	classified := ZeroClassification{Disposition: ZeroSuppressed}
	queue := &ProvenanceQueue{StartYear: 2026, Years: 30, Entries: []ProvenanceQueueEntry{
		{Key: "rendered", Occurrences: 1},
		{Key: "proper/x/commemoration-antiphon", ContentHash: "a"},
		{Key: "ordinary/lauds/psalm-antiphon", ContentHash: "b", ZeroClassification: &classified},
	}}
	report := zeroOccurrenceReportFromQueue(queue)
	if len(report.Entries) != 2 {
		t.Fatalf("zero entries = %d", len(report.Entries))
	}
	if got := report.Entries[0].Heuristic; got != ZeroGenericPsalmAntiphon {
		t.Fatalf("first heuristic = %q", got)
	}
	var out bytes.Buffer
	if err := WriteZeroOccurrenceCSV(report, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"content_hash", "generic-psalm-antiphon", "suppressed", "commemoration-slot"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("zero report missing %q:\n%s", want, out.String())
		}
	}
}
