package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/texts"
)

// brokenCorporaDir holds small data directories, each broken in a known way,
// with the validators' expected report in expected.txt. The Rust engine is
// checked against the same files (RUST-PORT.md, Phase 2), so the reports must
// be deterministic and must not depend on where the data directory lives.
const brokenCorporaDir = "testdata/broken-corpora"

// ValidationReport runs the data validators a case has input for: the
// calendar layer when it has feasts/, the texts layer when it has texts/.
// The data directory is written as $DATA.
func ValidationReport(dataDir string) string {
	var b strings.Builder
	layer := func(name string, errs []string) {
		b.WriteString("== " + name + "\n")
		for _, e := range errs {
			b.WriteString(strings.ReplaceAll(e, dataDir, "$DATA") + "\n")
		}
	}
	if isDir(filepath.Join(dataDir, "feasts")) {
		layer("calendar", calendar.ValidateAll(dataDir))
	}
	if isDir(filepath.Join(dataDir, "texts")) {
		layer("texts", texts.ValidateAll(dataDir))
	}
	return b.String()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func TestBrokenCorporaValidation(t *testing.T) {
	cases, err := os.ReadDir(brokenCorporaDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		t.Run(c.Name(), func(t *testing.T) {
			dir := filepath.Join(brokenCorporaDir, c.Name())
			got := ValidationReport(dir)
			if !strings.Contains(got, "== ") || strings.Count(got, "\n") < 2 {
				t.Fatalf("case reports nothing:\n%s", got)
			}
			path := filepath.Join(dir, "expected.txt")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v\n\trun: go test ./internal/e2e/ -update", err)
			}
			if got != string(want) {
				t.Errorf("validation report mismatch for %s\n--- got\n%s--- want\n%s", c.Name(), got, want)
			}
			// Twice more: map iteration must not reorder the report.
			for range 2 {
				if again := ValidationReport(dir); again != got {
					t.Fatalf("validation report is not deterministic:\n%s\nvs\n%s", got, again)
				}
			}
		})
	}
}
