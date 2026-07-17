package review

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrescreenValidates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review", "prescreen.csv")
	header := strings.Join(prescreenHeader, ",") + "\n"

	writeTestFile(t, path, header+"proper/x/collect,abc123,severe,reason,2026-07,\n")
	if _, err := LoadPrescreen(dir); err == nil || !strings.Contains(err.Error(), "severity") {
		t.Fatalf("invalid severity error = %v", err)
	}

	writeTestFile(t, path, header+
		"proper/x/collect,abc123,high,reason,2026-07,\n"+
		"proper/x/collect,abc123,medium,other,2026-07,\n")
	if _, err := LoadPrescreen(dir); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate key error = %v", err)
	}

	writeTestFile(t, path, header+"proper/x/collect,,high,reason,2026-07,\n")
	if _, err := LoadPrescreen(dir); err == nil || !strings.Contains(err.Error(), "content_hash") {
		t.Fatalf("missing hash error = %v", err)
	}

	writeTestFile(t, path, header+"proper/x/collect,abc123,high,reason,2026-07,42\n")
	flags, err := LoadPrescreen(dir)
	if err != nil || len(flags) != 1 || flags[0].Issue != "42" {
		t.Fatalf("flags = %#v, err = %v", flags, err)
	}

	if flags, err := LoadPrescreen(t.TempDir()); err != nil || flags != nil {
		t.Fatalf("missing ledger should be empty: %#v, %v", flags, err)
	}
}

func TestSuspicionByKey(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "proper", "example.txt"), `[flagged]
Flagged text stands.

[edited]
Edited since the flag.

[cleared]
Verified since the flag.

[cutoff]
This entry has no terminal punctuation at all
`)
	inv, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	byKey := inv.ByKey()
	writeTestFile(t, filepath.Join(dir, "review", "prescreen.csv"), strings.Join(prescreenHeader, ",")+"\n"+
		"proper/example/flagged,"+byKey["proper/example/flagged"].ContentHash+",high,wording garbled,2026-07,\n"+
		"proper/example/edited,olderhash,medium,dropped word,2026-07,\n"+
		"proper/example/cleared,"+byKey["proper/example/cleared"].ContentHash+",high,suspect,2026-07,\n")
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), strings.Join(provenanceHeader, ",")+"\n"+
		"proper/example/cleared,"+byKey["proper/example/cleared"].ContentHash+",Printed Diurnal,Ordinary,10,verified,alice,2026-07-10,word-for-word\n")

	inv, err = ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	suspicions, err := SuspicionByKey(dir, inv)
	if err != nil {
		t.Fatal(err)
	}

	if got := suspicions["proper/example/flagged"]; len(got) != 1 || got[0].State != SuspicionOpen || got[0].Label != "prescreen:high" {
		t.Fatalf("flagged suspicions = %#v", got)
	}
	if got := suspicions["proper/example/edited"]; len(got) != 1 || got[0].State != SuspicionAddressed {
		t.Fatalf("edited suspicions = %#v", got)
	}
	if got := suspicions["proper/example/cleared"]; got != nil {
		t.Fatalf("verified entry should resolve its flag: %#v", got)
	}
	found := false
	for _, s := range suspicions["proper/example/cutoff"] {
		found = found || s.Label == "lint:truncated"
	}
	if !found {
		t.Fatalf("advisory lint not surfaced: %#v", suspicions["proper/example/cutoff"])
	}
}

func TestSuspicionByKeyRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	writeTestFile(t, filepath.Join(dir, "review", "prescreen.csv"), strings.Join(prescreenHeader, ",")+"\n"+
		"missing,abc123,high,reason,2026-07,\n")
	inv, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SuspicionByKey(dir, inv); err == nil || !strings.Contains(err.Error(), "unknown corpus key") {
		t.Fatalf("error = %v", err)
	}
}

func TestRecordPrescreenFlag(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")

	recorded, err := RecordPrescreenFlag(dir, PrescreenFlag{
		Key: "example", Severity: PrescreenHigh, Reason: "garbled", Flagged: "2026-07",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.ContentHash != contentHash("Current text.") {
		t.Fatalf("flag not bound to current content: %#v", recorded)
	}

	if _, err := RecordPrescreenFlag(dir, PrescreenFlag{
		Key: "example", Severity: PrescreenMedium, Reason: "other", Flagged: "2026-08",
	}, false); err == nil || !strings.Contains(err.Error(), "--replace") {
		t.Fatalf("duplicate error = %v", err)
	}

	if _, err := RecordPrescreenFlag(dir, PrescreenFlag{
		Key: "missing", Severity: PrescreenHigh, Reason: "x", Flagged: "2026-07",
	}, false); err == nil || !strings.Contains(err.Error(), "unknown corpus key") {
		t.Fatalf("unknown key error = %v", err)
	}

	flags, err := LoadPrescreen(dir)
	if err != nil || len(flags) != 1 || flags[0].Key != "example" {
		t.Fatalf("ledger after failures = %#v, err = %v", flags, err)
	}
}

func TestAttestPrunesPrescreenFlag(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	writeTestFile(t, filepath.Join(dir, "texts", "other.txt"), "Other text.\n")
	for _, key := range []string{"example", "other"} {
		if _, err := RecordPrescreenFlag(dir, PrescreenFlag{
			Key: key, Severity: PrescreenHigh, Reason: "suspect", Flagged: "2026-07",
		}, false); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := RecordAttestation(dir, AttestOptions{
		Key: "example", Reviewer: "alice", Source: "Printed Diurnal", Page: "10",
	}); err != nil {
		t.Fatal(err)
	}

	flags, err := LoadPrescreen(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 1 || flags[0].Key != "other" {
		t.Fatalf("attestation should prune only the verified key's flag: %#v", flags)
	}
}

func TestSuspicionString(t *testing.T) {
	s := Suspicion{Label: "prescreen:high", State: SuspicionAddressed, Reason: "dropped word"}
	if got, want := s.String(), "prescreen:high (addressed) — dropped word"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
