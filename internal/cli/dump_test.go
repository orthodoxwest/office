package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpDiffAndDigestRoundTrip(t *testing.T) {
	e, out, _ := testEnv(t)
	args := []string{"-dates", "2026-12-25,2026-11-02", "-hours", "compline,vespers", "-forms", "private", "-groups", "calendar,office,hours"}
	if err := cmdDump(e, args); err != nil {
		t.Fatalf("cmdDump: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	// meta, calendar_year, then per date calendar_day + office_day + 2 hours.
	if len(lines) != 2+2*4 {
		t.Fatalf("got %d lines, want 10", len(lines))
	}
	if !strings.Contains(lines[2], `"date":"2026-11-02"`) {
		t.Fatalf("dates not in calendar order: %s", lines[2][:80])
	}

	dir := t.TempDir()
	left := filepath.Join(dir, "left.jsonl")
	if err := os.WriteFile(left, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	right := filepath.Join(dir, "right.jsonl")
	changed := strings.Replace(out.String(), `"marian_antiphon":"alma-redemptoris`, `"marian_antiphon":"salve-regina`, 1)
	if changed == out.String() {
		t.Fatal("fixture edit did not apply")
	}
	if err := os.WriteFile(right, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}

	e, diffOut, _ := testEnv(t)
	if err := cmdDump(e, []string{"diff", left, left}); err != nil {
		t.Fatalf("identical dumps: %v\n%s", err, diffOut)
	}
	e, diffOut, _ = testEnv(t)
	if err := cmdDump(e, []string{"diff", left, right}); !errors.Is(err, errReported) {
		t.Fatalf("differing dumps: err = %v", err)
	}
	if !strings.Contains(diffOut.String(), "/marian_antiphon") {
		t.Fatalf("diff output lacks the field:\n%s", diffOut)
	}

	e, digestOut, _ := testEnv(t)
	if err := cmdDump(e, []string{"digest", left}); err != nil {
		t.Fatalf("digest: %v", err)
	}
	if !strings.Contains(digestOut.String(), `"date_hours": 4`) {
		t.Fatalf("digest output:\n%s", digestOut)
	}
}

func TestDumpRejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"-start", "2026", "extra"},
		{"-dates", "2026-13-01"},
		{"-dates", "2026-01-01", "-start", "2026"},
		{"-start", "2026", "-hours", "matins"},
		{"diff", "only-one"},
		{"digest", "a", "b"},
		{"digest", filepath.Join(t.TempDir(), "missing.jsonl")},
	} {
		e, _, _ := testEnv(t)
		if err := cmdDump(e, args); err == nil {
			t.Errorf("cmdDump(%q) succeeded, want an error", args)
		}
	}
}
