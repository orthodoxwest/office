package audit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunReportsFlatIndexedPsalmAntiphons(t *testing.T) {
	dir := t.TempDir()

	feastsDir := filepath.Join(dir, "feasts")
	if err := os.MkdirAll(feastsDir, 0755); err != nil {
		t.Fatal(err)
	}
	temporal := `[easter-sunday]
Name = Easter Sunday
Rank = double-1st-class
Color = white
Category = apostle
DateRule = easter+0
`
	if err := os.WriteFile(filepath.Join(feastsDir, "temporal.txt"), []byte(temporal), 0644); err != nil {
		t.Fatal(err)
	}

	writeTextFile := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	writeTextFile("texts/proper/easter-sunday.txt", `[psalm-antiphon-1]
Same antiphon.

[psalm-antiphon-2]
Same antiphon.

[psalm-antiphon-3]
Same antiphon.

[psalm-antiphon-4]
Same antiphon.

[psalm-antiphon-5]
Same antiphon.
`)
	writeTextFile("texts/commons/apostle.txt", `[psalm-antiphon-1]
Same antiphon.

[psalm-antiphon-2]
Same antiphon.

[psalm-antiphon-3]
Same antiphon.

[psalm-antiphon-4]
Same antiphon.

[psalm-antiphon-5]
Same antiphon.
`)

	report, err := Run(dir)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(report.FlatProperAntiphons) != 1 {
		t.Fatalf("FlatProperAntiphons len = %d, want 1", len(report.FlatProperAntiphons))
	}
	if got := report.FlatProperAntiphons[0].ID; got != "easter-sunday" {
		t.Fatalf("FlatProperAntiphons[0].ID = %q, want easter-sunday", got)
	}

	if len(report.FlatCommonAntiphons) != 1 {
		t.Fatalf("FlatCommonAntiphons len = %d, want 1", len(report.FlatCommonAntiphons))
	}
	if got := report.FlatCommonAntiphons[0]; got != "apostle" {
		t.Fatalf("FlatCommonAntiphons[0] = %q, want apostle", got)
	}
}

func TestRunReportsTranslationReviewCandidates(t *testing.T) {
	dir := t.TempDir()

	feastsDir := filepath.Join(dir, "feasts")
	if err := os.MkdirAll(feastsDir, 0755); err != nil {
		t.Fatal(err)
	}
	temporal := `[trinity-sunday]
Name = Trinity Sunday
Rank = double-1st-class
Color = white
Category = sunday
DateRule = easter+56
`
	if err := os.WriteFile(filepath.Join(feastsDir, "temporal.txt"), []byte(temporal), 0644); err != nil {
		t.Fatal(err)
	}

	writeTextFile := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	writeTextFile("texts/proper/trinity-sunday.txt", `[collect]
Almighty and everlasting God, you have given thy servants grace to confess the true faith.

[benedictus-antiphon]
Blessed be the Holy Trinity.
`)
	writeTextFile("texts/commons/apostle.txt", `[collect]
Grant, we beseech thee, almighty God, that the intercession of thy blessed Apostle N. may help us.

[benedictus-antiphon]
You who left all and followed me shall receive a hundredfold.
`)

	report, err := Run(dir)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantModernCollects := []string{"proper/trinity-sunday/collect"}
	if !reflect.DeepEqual(report.ModernCollects, wantModernCollects) {
		t.Fatalf("ModernCollects = %v, want %v", report.ModernCollects, wantModernCollects)
	}

	wantMixedRegister := []string{"proper/trinity-sunday/collect"}
	if !reflect.DeepEqual(report.MixedRegister, wantMixedRegister) {
		t.Fatalf("MixedRegister = %v, want %v", report.MixedRegister, wantMixedRegister)
	}
}
