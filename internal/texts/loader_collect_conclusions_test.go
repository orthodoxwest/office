package texts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCorpusFixture builds a minimal data dir: data/texts/ with the conclusion
// formulas and one collect, plus whatever collect-conclusions.txt content is given.
func writeCorpusFixture(t *testing.T, conclusions string) string {
	t.Helper()
	dir := t.TempDir()
	textsDir := filepath.Join(dir, "texts", "shared")
	if err := os.MkdirAll(textsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	formulas := "[collect-conclusion-through]\nThrough Jesus Christ.\nR. Amen.\n\n" +
		"[collect-conclusion-who-liveth]\nWho with thee liveth.\nR. Amen.\n\n" +
		"[a-collect]\nO God, who didst.\n"
	if err := os.WriteFile(filepath.Join(textsDir, "formulas.txt"), []byte(formulas), 0o644); err != nil {
		t.Fatal(err)
	}
	if conclusions != "" {
		if err := os.WriteFile(filepath.Join(dir, "collect-conclusions.txt"), []byte(conclusions), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadCollectConclusionsRejectsBadData(t *testing.T) {
	// wantLine is the 1-based line the error must point at, so a reader can find
	// the offending entry. Leading comments and blanks make this non-trivial.
	tests := []struct {
		name        string
		content     string
		wantErrPart string
		wantLine    int
	}{
		{
			name:        "malformed line",
			content:     "# comment\n\nshared/formulas/a-collect\n",
			wantErrPart: "expected",
			wantLine:    3,
		},
		{
			name:        "three fields",
			content:     "shared/formulas/a-collect who-liveth extra\n",
			wantErrPart: "expected",
			wantLine:    1,
		},
		{
			name:        "unknown conclusion form",
			content:     "# c\nshared/formulas/a-collect no-such-form\n",
			wantErrPart: "not in the corpus",
			wantLine:    2,
		},
		{
			name:        "unknown collect key",
			content:     "\n\n\nproper/nonexistent/collect who-liveth\n",
			wantErrPart: "not in the corpus",
			wantLine:    4,
		},
		{
			name:        "duplicate key",
			content:     "shared/formulas/a-collect who-liveth\nshared/formulas/a-collect through\n",
			wantErrPart: "listed twice",
			wantLine:    2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadTexts(writeCorpusFixture(t, tt.content))
			if err == nil {
				t.Fatalf("LoadTexts accepted %q, want an error", tt.content)
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErrPart)
			}
			if want := fmt.Sprintf("collect-conclusions.txt:%d:", tt.wantLine); !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want it to point at %q", err, want)
			}
		})
	}
}

func TestLoadCollectConclusionsAcceptsGoodData(t *testing.T) {
	// Comments, blank lines, trailing whitespace and CRLF must all be tolerated.
	content := "# a comment\r\n\r\n   \n" +
		"shared/formulas/a-collect who-liveth   \r\n" +
		"# trailing comment\n"
	corpus, err := LoadTexts(writeCorpusFixture(t, content))
	if err != nil {
		t.Fatalf("LoadTexts rejected valid data: %v", err)
	}
	form, ok := corpus.CollectConclusionForm("shared/formulas/a-collect")
	if !ok || form != "who-liveth" {
		t.Errorf("CollectConclusionForm = (%q, %v), want (\"who-liveth\", true)", form, ok)
	}
	if _, ok := corpus.CollectConclusionForm("shared/formulas/unlisted"); ok {
		t.Error("CollectConclusionForm reported a form for an unlisted key")
	}
}

func TestLoadCollectConclusionsMissingFileIsNotAnError(t *testing.T) {
	corpus, err := LoadTexts(writeCorpusFixture(t, ""))
	if err != nil {
		t.Fatalf("LoadTexts failed with no collect-conclusions.txt: %v", err)
	}
	if _, ok := corpus.CollectConclusionForm("shared/formulas/a-collect"); ok {
		t.Error("expected no recorded form when the file is absent")
	}
}

// A corpus built for tests must not report forms it was never given.
func TestNewTestCorpusHasEmptyConclusionMap(t *testing.T) {
	corpus := NewTestCorpus(map[string]string{"proper/x/collect": "body"})
	if _, ok := corpus.CollectConclusionForm("proper/x/collect"); ok {
		t.Error("NewTestCorpus reported a conclusion form that was never set")
	}
	corpus.SetCollectConclusionForm("proper/x/collect", "who-liveth")
	if form, ok := corpus.CollectConclusionForm("proper/x/collect"); !ok || form != "who-liveth" {
		t.Errorf("after Set, got (%q, %v), want (\"who-liveth\", true)", form, ok)
	}
}
