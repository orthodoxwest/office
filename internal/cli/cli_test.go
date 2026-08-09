package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// testEnv runs commands against the repository's real data directory.
func testEnv(t *testing.T) (env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return env{dataDir: filepath.Join("..", "..", "data"), out: &out, err: &errOut}, &out, &errOut
}

func TestRubricsTSVContract(t *testing.T) {
	// scripts/ordo-compare.py parses these columns positionally, so the header
	// and the column order are a contract with the ordo cross-check tooling.
	e, out, _ := testEnv(t)
	if err := cmdRubrics(e, []string{"2026"}); err != nil {
		t.Fatalf("cmdRubrics: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	wantHeader := "date\tcelebration\tlauds_preces\tlauds_suffrage\tlauds_comms\thours_preces\t" +
		"vespers_preces\tvespers_suffrage\tvespers_comms\tbenedictus_ant\tmagnificat_ant"
	if lines[0] != wantHeader {
		t.Fatalf("header = %q, want %q", lines[0], wantHeader)
	}
	if len(lines) != 366 { // header + 365 days
		t.Fatalf("got %d lines, want 366", len(lines))
	}

	// Lent II Tuesday: no Lauds preces, Suffrage said, the Forty Martyrs
	// commemorated, and both gospel antiphons present in full.
	var row []string
	for _, line := range lines[1:] {
		if fields := strings.Split(line, "\t"); fields[0] == "2026-03-10" {
			row = fields
			break
		}
	}
	if row == nil {
		t.Fatal("2026-03-10 missing from the TSV")
	}
	if len(row) != 11 {
		t.Fatalf("row has %d columns, want 11: %q", len(row), row)
	}
	if row[1] != "Tuesday after Lent II" {
		t.Errorf("celebration = %q", row[1])
	}
	if row[2] != "false" || row[3] != "true" {
		t.Errorf("lauds preces/suffrage = %q/%q, want false/true", row[2], row[3])
	}
	if row[4] != "The Forty Holy Martyrs" {
		t.Errorf("lauds commemorations = %q", row[4])
	}
	if row[5] != "true" {
		t.Errorf("hours preces = %q, want true", row[5])
	}
	for i, name := range map[int]string{9: "benedictus", 10: "magnificat"} {
		if row[i] == "" {
			t.Errorf("%s antiphon is empty", name)
		}
		if strings.Contains(row[i], "\n") {
			t.Errorf("%s antiphon contains a newline, which would break the TSV", name)
		}
	}
}

func TestOrdoPrintsTabulaTemporaria(t *testing.T) {
	e, out, _ := testEnv(t)
	if err := cmdOrdo(e, []string{"2026"}); err != nil {
		t.Fatalf("cmdOrdo: %v", err)
	}

	body := out.String()
	for _, want := range []string{
		"TABULA TEMPORARIA — ANNO DOMINI 2026",
		"Golden Number",
		"Easter (Pascha) Day      April 12",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ordo missing %q", want)
		}
	}
}

func TestHourCommandComposesRequestedDay(t *testing.T) {
	e, out, _ := testEnv(t)
	if err := cmdHour(e, "lauds", []string{"2026-06-07"}); err != nil {
		t.Fatalf("cmdHour: %v", err)
	}
	if body := out.String(); !strings.Contains(body, "LAUDS") {
		t.Errorf("lauds output does not name the hour: %.200s", body)
	}
}

func TestReviewExplainEmitsJSON(t *testing.T) {
	e, out, _ := testEnv(t)
	if err := cmdReview(e, []string{"explain", "lauds", "2026-06-07"}); err != nil {
		t.Fatalf("cmdReview explain: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("explain output is not JSON: %v", err)
	}
}

func TestReviewResolutionInventoryEmitsJSONSummaryAndFallbacks(t *testing.T) {
	e, out, _ := testEnv(t)
	if err := cmdReview(e, []string{"resolution-inventory", "-start", "2026", "-years", "1", "-json"}); err != nil {
		t.Fatalf("resolution inventory JSON: %v", err)
	}
	var payload struct {
		Rows []struct {
			RequestedSlot  string   `json:"requested_slot"`
			CanonicalOwner string   `json:"canonical_owner"`
			SelectedRef    string   `json:"selected_ref"`
			DirectExisting []string `json:"direct_existing"`
		}
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("resolution inventory is not JSON: %v", err)
	}
	if len(payload.Rows) == 0 || payload.Rows[0].CanonicalOwner == "" || payload.Rows[0].RequestedSlot == "" || payload.Rows[0].SelectedRef == "" {
		t.Fatalf("inventory rows lack resolution metadata: %#v", payload.Rows)
	}

	out.Reset()
	if err := cmdReview(e, []string{"resolution-inventory", "-start", "2026", "-years", "1", "-fallback-only", "-summary"}); err != nil {
		t.Fatalf("resolution inventory fallback summary: %v", err)
	}
	if !strings.Contains(out.String(), "resolution inventory:") {
		t.Fatalf("summary = %q", out.String())
	}
}

func TestValidatePassesOnRepositoryData(t *testing.T) {
	e, out, _ := testEnv(t)
	if err := cmdValidate(e, nil); err != nil {
		t.Fatalf("cmdValidate: %v (output: %s)", err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "All data files valid." {
		t.Errorf("validate output = %q", got)
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(env) error
		want string
	}{
		{"ordo without year", func(e env) error { return cmdOrdo(e, nil) }, "usage: office ordo YEAR"},
		{"ordo with bad year", func(e env) error { return cmdOrdo(e, []string{"MMXXVI"}) }, "invalid year"},
		{"hour without date", func(e env) error { return cmdHour(e, "lauds", nil) }, "usage: office lauds"},
		{"hour with bad date", func(e env) error { return cmdHour(e, "lauds", []string{"7 June"}) }, "invalid date"},
		{"tex without hour", func(e env) error { return cmdTeX(e, []string{"--chant"}) }, "usage: office tex"},
		{"review without subcommand", func(e env) error { return cmdReview(e, nil) }, "Subcommands:"},
		{"review with unknown subcommand", func(e env) error { return cmdReview(e, []string{"frobnicate"}) }, "Subcommands:"},
		{"review sign without reviewer", func(e env) error { return cmdReview(e, []string{"sign", "lauds", "2026-06-07"}) }, "usage: office review sign"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, out, _ := testEnv(t)
			err := tt.run(e)
			if err == nil {
				t.Fatalf("expected an error, got output %q", out.String())
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
			if out.Len() > 0 {
				t.Errorf("usage errors should not write to stdout, got %q", out.String())
			}
		})
	}
}

func TestRunReportsUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"frobnicate"}, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Unknown command: frobnicate") {
		t.Errorf("stderr = %q", errOut.String())
	}
	if out.Len() > 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestRunWithoutArgsPrintsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Usage: office <command>") {
		t.Errorf("stderr = %q", errOut.String())
	}
}
