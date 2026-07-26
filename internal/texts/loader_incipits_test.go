package texts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeIncipitFixture builds a minimal data dir holding two psalms and one
// canticle, plus whatever latin-incipits.txt content is given.
func writeIncipitFixture(t *testing.T, incipits string) string {
	t.Helper()
	dir := t.TempDir()
	psalmsDir := filepath.Join(dir, "texts", "psalms")
	canticlesDir := filepath.Join(dir, "texts", "canticles")
	for _, sub := range []string{psalmsDir, canticlesDir} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(psalmsDir, "067.txt"), "Psalm 67\n\nGOD be merciful unto us * and bless us.\n")
	write(filepath.Join(psalmsDir, "119-i.txt"), "Psalm 119:1-8\n\nBLESSED are those * that are undefiled.\n")
	write(filepath.Join(canticlesDir, "magnificat.txt"), "!Luke 1:46-55\n\nMy soul doth magnify the Lord * and my spirit.\n")
	if incipits != "" {
		write(filepath.Join(dir, "latin-incipits.txt"), incipits)
	}
	return dir
}

// full is the incipit file every psalm and canticle in the fixture needs, so
// tests that are not about coverage can start from a complete corpus.
const fullIncipits = "psalms/067 = Deus misereatur nostri\n" +
	"psalms/119-i = Beati immaculati in via\n" +
	"canticles/magnificat = Magnificat anima mea Dominum\n"

func TestLoadIncipitsRejectsBadData(t *testing.T) {
	// wantLine is the 1-based line the error must point at, so a reader can
	// find the offending entry past the file's long explanatory header.
	tests := []struct {
		name        string
		content     string
		wantErrPart string
		wantLine    int
	}{
		{
			name:        "missing separator",
			content:     "# comment\n\npsalms/067 Deus misereatur\n",
			wantErrPart: "expected",
			wantLine:    3,
		},
		{
			name:        "empty incipit",
			content:     "psalms/067 =   \n",
			wantErrPart: "empty incipit",
			wantLine:    1,
		},
		{
			name:        "key outside the psalter and canticles",
			content:     "# c\nordinary/lauds/opening-versicle = Deus in adjutorium\n",
			wantErrPart: "not a psalm or canticle key",
			wantLine:    2,
		},
		{
			name:        "unknown psalm key",
			content:     "\n\n\npsalms/999 = Nusquam est\n",
			wantErrPart: "not in the corpus",
			wantLine:    4,
		},
		{
			name:        "duplicate key",
			content:     "psalms/067 = Deus misereatur nostri\npsalms/067 = Jubilate Deo\n",
			wantErrPart: "listed twice",
			wantLine:    2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadTexts(writeIncipitFixture(t, tt.content))
			if err == nil {
				t.Fatalf("LoadTexts accepted %q, want an error", tt.content)
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErrPart)
			}
			if want := fmt.Sprintf("latin-incipits.txt:%d:", tt.wantLine); !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want it to point at %q", err, want)
			}
		})
	}
}

func TestLoadIncipitsAcceptsGoodData(t *testing.T) {
	// Comments, blank lines, padding around the "=", and CRLF must all be
	// tolerated; an incipit may itself contain no "=" surprises.
	content := "# a comment\r\n\r\n   \n" +
		"psalms/067    =    Deus misereatur nostri   \r\n" +
		"psalms/119-i = Beati immaculati in via\n" +
		"canticles/magnificat = Magnificat anima mea Dominum\n" +
		"# trailing comment\n"
	corpus, err := LoadTexts(writeIncipitFixture(t, content))
	if err != nil {
		t.Fatalf("LoadTexts rejected valid data: %v", err)
	}
	want := map[string]string{
		"psalms/067":           "Deus misereatur nostri",
		"psalms/119-i":         "Beati immaculati in via",
		"canticles/magnificat": "Magnificat anima mea Dominum",
	}
	for key, incipit := range want {
		if got := corpus.Incipit(key); got != incipit {
			t.Errorf("Incipit(%q) = %q, want %q", key, got, incipit)
		}
	}
	if got := corpus.Incipit("psalms/nonexistent"); got != "" {
		t.Errorf("Incipit of an unlisted key = %q, want empty", got)
	}
	if missing := corpus.MissingIncipits(); len(missing) != 0 {
		t.Errorf("MissingIncipits = %v, want none", missing)
	}
}

// A psalm added to the corpus without an incipit must be reported, because a
// silently unlabelled psalm is the failure mode this data guards against.
func TestMissingIncipitsReportsUncoveredEntries(t *testing.T) {
	corpus, err := LoadTexts(writeIncipitFixture(t, "psalms/067 = Deus misereatur nostri\n"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	got := corpus.MissingIncipits()
	want := []string{"canticles/magnificat", "psalms/119-i"}
	if len(got) != len(want) {
		t.Fatalf("MissingIncipits = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MissingIncipits[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadIncipitsMissingFileIsNotAnError(t *testing.T) {
	corpus, err := LoadTexts(writeIncipitFixture(t, ""))
	if err != nil {
		t.Fatalf("LoadTexts failed with no latin-incipits.txt: %v", err)
	}
	if got := corpus.Incipit("psalms/067"); got != "" {
		t.Errorf("Incipit = %q with no incipit file, want empty", got)
	}
}

// An @use alias must reach the incipit recorded against its target, so an
// aliased psalm is not silently unlabelled.
func TestIncipitResolvesThroughAlias(t *testing.T) {
	dir := writeIncipitFixture(t, fullIncipits)
	alias := filepath.Join(dir, "texts", "psalms", "sunday-lauds-2.txt")
	if err := os.WriteFile(alias, []byte("@use psalms/067\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	corpus, err := LoadTexts(dir)
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	if got := corpus.Incipit("psalms/sunday-lauds-2"); got != "Deus misereatur nostri" {
		t.Errorf("Incipit through alias = %q, want %q", got, "Deus misereatur nostri")
	}
}

// A corpus built for tests must not report incipits it was never given.
func TestNewTestCorpusHasEmptyIncipitMap(t *testing.T) {
	corpus := NewTestCorpus(map[string]string{"psalms/067": "body"})
	if got := corpus.Incipit("psalms/067"); got != "" {
		t.Errorf("NewTestCorpus reported incipit %q that was never set", got)
	}
	corpus.SetIncipit("psalms/067", "Deus misereatur nostri")
	if got := corpus.Incipit("psalms/067"); got != "Deus misereatur nostri" {
		t.Errorf("after Set, Incipit = %q, want %q", got, "Deus misereatur nostri")
	}
}
