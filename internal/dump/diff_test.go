package dump

import (
	"bytes"
	"strings"
	"testing"
)

const (
	metaLine  = `{"format":"office-dump/2","kind":"meta"}`
	dayLine   = `{"date":"2026-01-01","kind":"calendar_day","season":"christmas"}`
	hourLine  = `{"date":"2026-01-01","form":"private","hour":"lauds","kind":"hour","sections":[{"elements":[{"text":"O God, make speed to save me."}]}]}`
	hourLine2 = `{"date":"2026-01-01","form":"private","hour":"lauds","kind":"hour","sections":[{"elements":[{"text":"O God, make haste to save me."}]}]}`
)

func runDiff(t *testing.T, left, right []string) (DiffSummary, string) {
	t.Helper()
	var out bytes.Buffer
	join := func(lines []string) *strings.Reader { return strings.NewReader(strings.Join(lines, "\n") + "\n") }
	summary, err := Diff(join(left), join(right), &out, DiffOptions{})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	return summary, out.String()
}

func TestDiffIdenticalDumpsAreClean(t *testing.T) {
	summary, out := runDiff(t, []string{metaLine, dayLine, hourLine}, []string{metaLine, dayLine, hourLine})
	if !summary.Clean() || summary.Records != 3 {
		t.Fatalf("summary = %+v\n%s", summary, out)
	}
}

func TestDiffReportsTheDifferingPointerWithContext(t *testing.T) {
	summary, out := runDiff(t, []string{metaLine, dayLine, hourLine}, []string{metaLine, dayLine, hourLine2})
	if summary.Clean() || summary.Differing != 1 || summary.ByKind[KindHour] != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	for _, want := range []string{
		"line 3: hour 2026-01-01 lauds private",
		"/sections/0/elements/0/text",
		`"O God, make speed to save me."`,
		`"O God, make haste to save me."`,
		"3 records compared, 1 differ, hour 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestDiffStopsWhereTheSequenceDiverges(t *testing.T) {
	summary, out := runDiff(t, []string{metaLine, dayLine, hourLine}, []string{metaLine, hourLine})
	if !summary.Diverged || !strings.Contains(out, "left has calendar_day 2026-01-01, right has hour 2026-01-01 lauds private") {
		t.Fatalf("summary = %+v\n%s", summary, out)
	}

	summary, out = runDiff(t, []string{metaLine, dayLine}, []string{metaLine})
	if !summary.Diverged || !strings.Contains(out, "right dump ended; the other continues with calendar_day 2026-01-01") {
		t.Fatalf("summary = %+v\n%s", summary, out)
	}
}

func TestDiffNamesTheNonCanonicalSide(t *testing.T) {
	loose := `{"date":"2026-01-01","kind":"calendar_day","season": "christmas"}`
	summary, out := runDiff(t, []string{metaLine, dayLine}, []string{metaLine, loose})
	if summary.Clean() || !strings.Contains(out, "same values, different encoding (right is not canonical)") {
		t.Fatalf("summary = %+v\n%s", summary, out)
	}
}

func TestDiffListsMissingKeysAndLengthChanges(t *testing.T) {
	left := `{"date":"2026-01-01","kind":"calendar_day","list":[1,2],"only_left":true}`
	right := `{"date":"2026-01-01","kind":"calendar_day","list":[1],"only_right":null}`
	_, out := runDiff(t, []string{left}, []string{right})
	for _, want := range []string{"/list\n    left:  2 items\n    right: 1 items", "/only_left\n    left:  true\n    right: (absent)", "/only_right\n    left:  (absent)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestExcerptCentersOnTheFirstDifferenceWithoutSplittingRunes(t *testing.T) {
	prefix := strings.Repeat("℣ ", 30)
	l, r := excerpt(prefix+"alpha", prefix+"omega")
	if !strings.HasPrefix(l, "…") || !strings.HasSuffix(l, `alpha"`) || !strings.HasSuffix(r, `omega"`) {
		t.Fatalf("excerpt = %s | %s", l, r)
	}
	if strings.ContainsRune(l, '\uFFFD') {
		t.Fatalf("excerpt split a rune: %s", l)
	}
}
