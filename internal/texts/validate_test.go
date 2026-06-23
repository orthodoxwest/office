package texts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateAllFlagsUnexpectedDirectivesInSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "texts", "proper", "good-friday.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	content := `[collect]
@Tempora/Quad6-4::s/unto death/unto death, even to the death of the cross/
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := ValidateAll(dir)
	want := `texts/proper/good-friday.txt:2 [collect]: unexpected Divinum Officium directive: "@Tempora/Quad6-4::s/unto death/unto death, even to the death of the cross/"`
	if len(got) != 1 {
		t.Fatalf("ValidateAll() len = %d, want 1 (%v)", len(got), got)
	}
	if got[0] != want {
		t.Fatalf("ValidateAll()[0] = %q, want %q", got[0], want)
	}
}

func TestValidateAllFlagsUnexpectedDirectivesInPlainTextFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "texts", "rubrics", "example.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("ex Tempora/Common/example\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := ValidateAll(dir)
	want := `texts/rubrics/example.txt:1: unexpected Divinum Officium extract directive: "ex Tempora/Common/example"`
	if len(got) != 1 {
		t.Fatalf("ValidateAll() len = %d, want 1 (%v)", len(got), got)
	}
	if got[0] != want {
		t.Fatalf("ValidateAll()[0] = %q, want %q", got[0], want)
	}
}

func TestValidateAllAcceptsNormalizedCorpusText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "texts", "proper", "good-friday.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	content := `[collect]
Look down, we beseech thee, O Lord, on this thy family.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if got := ValidateAll(dir); len(got) != 0 {
		t.Fatalf("ValidateAll() = %v, want no errors", got)
	}
}

func TestValidateAllFlagsPlaceholderCorpusEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "texts", "ordinary", "lauds.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	content := `[hymn]
Placeholder
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := ValidateAll(dir)
	want := `placeholder text remains in corpus: ordinary/lauds/hymn`
	if len(got) != 1 {
		t.Fatalf("ValidateAll() len = %d, want 1 (%v)", len(got), got)
	}
	if got[0] != want {
		t.Fatalf("ValidateAll()[0] = %q, want %q", got[0], want)
	}
}

func TestValidateAllFlagsUnexpectedControlCharacters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "texts", "ordinary", "lauds.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	content := "[opening-versicle]\nV.\x07 O God, make speed to save us.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := ValidateAll(dir)
	want := `texts/ordinary/lauds.txt:2:3: unexpected control character U+0007`
	if len(got) != 1 {
		t.Fatalf("ValidateAll() len = %d, want 1 (%v)", len(got), got)
	}
	if got[0] != want {
		t.Fatalf("ValidateAll()[0] = %q, want %q", got[0], want)
	}
}
