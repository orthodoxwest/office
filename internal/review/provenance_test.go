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

[todo]
# TODO(diurnal): locate this text in the printed diurnal.
Text awaiting source research.
`)
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), `key,content_hash,source,locator,page,status,reviewer,reviewed_on,notes
proper/example/verified,`+contentHash("Verified text.")+`,Printed Diurnal,Proper of Example,44,verified,alice,2026-07-09,word-for-word
`)

	inv, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	byKey := inv.ByKey()
	if got := byKey["proper/example/collect"]; got.Status != ProvenanceNeedsReview || got.Sources[0].Page != "12" {
		t.Fatalf("collect provenance = %#v", got)
	}
	if got := byKey["proper/example/antiphon"].Status; got != ProvenanceNeedsReview {
		t.Fatalf("antiphon status = %q", got)
	}
	if got := byKey["proper/example/verified"]; got.Status != ProvenanceVerified || got.Reviewer != "alice" {
		t.Fatalf("verified provenance = %#v", got)
	}
	if got := byKey["proper/example/unknown"].Status; got != ProvenanceSourceUnknown {
		t.Fatalf("unknown status = %q", got)
	}
	if got := byKey["proper/example/todo"].Status; got != ProvenanceNeedsReview {
		t.Fatalf("TODO status = %q", got)
	}

	var out bytes.Buffer
	if err := WriteProvenanceCSV(inv, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "proper/example/collect") || !strings.Contains(out.String(), "local-book.pdf") {
		t.Fatalf("CSV missing provenance row:\n%s", out.String())
	}
	if strings.Contains(out.String(), "content_hash") || strings.Contains(out.String(), contentHash("Verified text.")) {
		t.Fatalf("reviewer inventory exposes implementation hash:\n%s", out.String())
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
	if got.Status != ProvenanceNeedsReview || !strings.Contains(got.Notes, "earlier text version") {
		t.Fatalf("stale provenance = %#v", got)
	}
}

func TestRecordAttestation(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), strings.Join(provenanceHeader, ",")+"\n")
	inv, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash := inv.ByKey()["example"].ContentHash
	got, err := RecordAttestation(dir, AttestOptions{
		Key: "example", Reviewer: "alice", Source: "Printed Diurnal",
		Page: "10", ReviewedOn: "2026-07-10", Notes: "word-for-word",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != ProvenanceVerified || got.ContentHash != hash {
		t.Fatalf("attestation result = %#v", got)
	}
	rescanned, err := ScanProvenance(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := rescanned.ByKey()["example"]; got.Status != ProvenanceVerified || got.Reviewer != "alice" {
		t.Fatalf("rescanned provenance = %#v", got)
	}
}

func TestRecordAttestationUnknownKeyDoesNotChangeFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	path := filepath.Join(dir, "review", "provenance.csv")
	initial := strings.Join(provenanceHeader, ",") + "\n"
	writeTestFile(t, path, initial)
	_, err := RecordAttestation(dir, AttestOptions{
		Key: "missing", Reviewer: "alice", Source: "Printed Diurnal", Page: "10",
	})
	if err == nil {
		t.Fatal("expected unknown key error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != initial {
		t.Fatalf("file changed after failure:\n%s", after)
	}
}

func TestRecordAttestationRequiresReplace(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "example.txt"), "Current text.\n")
	hash := contentHash("Current text.")
	path := filepath.Join(dir, "review", "provenance.csv")
	initial := strings.Join(provenanceHeader, ",") + "\n" +
		"example," + hash + ",Printed Diurnal,Ordinary,10,verified,alice,2026-07-10,first\n"
	writeTestFile(t, path, initial)
	_, err := RecordAttestation(dir, AttestOptions{
		Key: "example", Reviewer: "bob", Source: "Printed Diurnal", Page: "10",
	})
	if err == nil || !strings.Contains(err.Error(), "--replace") {
		t.Fatalf("error = %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != initial {
		t.Fatal("duplicate failure changed provenance file")
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
