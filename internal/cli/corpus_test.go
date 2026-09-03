package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func corpusTestEnv(t *testing.T, file, content string) (env, string, *bytes.Buffer) {
	t.Helper()
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "texts", filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	return env{dataDir: dataDir, out: &out, err: &out}, path, &out
}

func TestCorpusShowStripsSectionComments(t *testing.T) {
	e, _, out := corpusTestEnv(t, "proper/st-test.txt", "preamble\n[collect]\n# SOURCE: old\nLine one.\n\nLine two.\n[next]\nOther\n")
	if err := cmdCorpus(e, []string{"show", "proper/st-test/collect"}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "Line one.\n\nLine two.\n"; got != want {
		t.Fatalf("show = %q, want %q", got, want)
	}
}

func TestCorpusPutReplacesOnlyLiveSection(t *testing.T) {
	original := "# header\n[collect]\n# SOURCE: old book p. 2\nOld body.\n\n[next]\nUntouched.\n"
	e, path, _ := corpusTestEnv(t, "proper/st-test.txt", original)
	bodyPath := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(bodyPath, []byte("New body.\nSecond line.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdCorpus(e, []string{"put", "proper/st-test/collect", "--file", bodyPath, "--source", "diurnal p. 595"}); err != nil {
		t.Fatal(err)
	}
	gotBytes, _ := os.ReadFile(path)
	got := string(gotBytes)
	if !strings.Contains(got, "[collect]\n# SOURCE: diurnal p. 595\nNew body.\nSecond line.\n") {
		t.Fatalf("updated section missing:\n%s", got)
	}
	if !strings.HasPrefix(got, "# header\n[collect]\n") || !strings.HasSuffix(got, "[next]\nUntouched.\n") {
		t.Fatalf("content outside target changed:\n%s", got)
	}
	if strings.Contains(got, "old book") || strings.Contains(got, "Old body") {
		t.Fatalf("old section content remains:\n%s", got)
	}
}

func TestCorpusPutActivatesCommentedScaffold(t *testing.T) {
	original := "# Proper scaffold\n# [collect]\n# The collect.\n#\n# [benedictus-antiphon]\n# Antiphon at Lauds.\n"
	e, path, _ := corpusTestEnv(t, "proper/st-test.txt", original)
	bodyPath := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(bodyPath, []byte("O God, hear us."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdCorpus(e, []string{"put", "proper/st-test/collect", "--file", bodyPath, "--source", "diurnal p. 7*"}); err != nil {
		t.Fatal(err)
	}
	gotBytes, _ := os.ReadFile(path)
	got := string(gotBytes)
	want := "# Proper scaffold\n[collect]\n# SOURCE: diurnal p. 7*\nO God, hear us.\n\n# [benedictus-antiphon]\n# Antiphon at Lauds.\n"
	if got != want {
		t.Fatalf("scaffold update:\n%q\nwant:\n%q", got, want)
	}
}

func TestCorpusShowPlainPsalmUsesLoaderCommentRule(t *testing.T) {
	e, _, out := corpusTestEnv(t, "psalms/001.txt", "Psalm 1\n# SOURCE: witness\n\n1. Blessed * is the man.\n")
	if err := cmdCorpus(e, []string{"show", "psalms/001"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "SOURCE") || !strings.Contains(out.String(), "Blessed") {
		t.Fatalf("show output = %q", out.String())
	}
}

func TestCorpusRejectsTraversalAndEmptyBodies(t *testing.T) {
	e, _, _ := corpusTestEnv(t, "proper/st-test.txt", "[collect]\nOld\n")
	if _, err := readCorpusBody(e.dataDir, "proper/../secret"); err == nil {
		t.Fatal("traversal key accepted")
	}
	if err := putCorpusBody(e.dataDir, "proper/st-test/collect", " \n", "diurnal p. 1"); err == nil {
		t.Fatal("empty replacement accepted")
	}
}
