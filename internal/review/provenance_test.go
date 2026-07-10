package review

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanProvenance(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "proper", "example.txt"), `[collect]
# SOURCE: local-book.pdf (Proper of Example) p. 12 — parish source
Collect text.

[antiphon]
# SOURCE: divinum-officium Sancti/01-01 [Ant 2] — check against diurnal
Antiphon text.

[verified]
Verified text.

[unknown]
Unknown text.
`)
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), `key,content_hash,source,locator,page,status,reviewer,reviewed_on,notes
proper/example/verified,`+contentHash("Verified text.")+`,Printed Diurnal,Proper of Example,44,verified,alice,2026-07-09,word-for-word
`)

	inv, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	byKey := inv.ByKey()
	if got := byKey["proper/example/collect"]; got.Status != ProvenanceDocumented || got.Sources[0].Page != "12" {
		t.Fatalf("collect provenance = %#v", got)
	}
	if got := byKey["proper/example/antiphon"].Status; got != ProvenanceNeedsReview {
		t.Fatalf("antiphon status = %q", got)
	}
	if got := byKey["proper/example/verified"]; got.Status != ProvenanceVerified || got.Reviewer != "alice" {
		t.Fatalf("verified provenance = %#v", got)
	}
	if got := byKey["proper/example/unknown"].Status; got != ProvenanceUndocumented {
		t.Fatalf("unknown status = %q", got)
	}

	var out bytes.Buffer
	if err := WriteProvenanceCSV(inv, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "proper/example/collect") || !strings.Contains(out.String(), "local-book.pdf") {
		t.Fatalf("CSV missing provenance row:\n%s", out.String())
	}
}

func TestStaleAttestationNeedsReview(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), `key,content_hash,source,locator,page,status,reviewer,reviewed_on,notes
example,oldhash,Printed Diurnal,Ordinary,10,verified,alice,2026-07-09,word-for-word
`)
	inv, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := inv.ByKey()["example"]
	if got.Status != ProvenanceNeedsReview || !strings.Contains(got.Notes, "stale attestation") {
		t.Fatalf("stale provenance = %#v", got)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
