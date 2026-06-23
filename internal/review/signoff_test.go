package review

import (
	"strings"
	"testing"
	"time"
)

func TestParseSignoffs(t *testing.T) {
	input := `# comment line

abc123def456 lauds trinity-sunday mary.k 2026-06-08 checked against diurnal + supplement
fff000fff000 vespers all-saints john.d 2026-06-09
`
	signoffs, err := ParseSignoffs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseSignoffs: %v", err)
	}
	if len(signoffs) != 2 {
		t.Fatalf("got %d signoffs, want 2", len(signoffs))
	}
	want := Signoff{
		Hash: "abc123def456", Hour: "lauds", UnitKey: "trinity-sunday",
		Reviewer: "mary.k", Date: "2026-06-08", Note: "checked against diurnal + supplement",
	}
	if signoffs[0] != want {
		t.Errorf("signoffs[0] = %+v, want %+v", signoffs[0], want)
	}
	if signoffs[1].Note != "" {
		t.Errorf("signoffs[1].Note = %q, want empty", signoffs[1].Note)
	}
}

func TestParseSignoffsRejectsShortLines(t *testing.T) {
	_, err := ParseSignoffs(strings.NewReader("abc123 lauds trinity-sunday\n"))
	if err == nil {
		t.Error("expected error for line with too few fields")
	}
}

func TestClassify(t *testing.T) {
	date := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	m := &Manifest{Units: []Unit{
		{Hash: "aaa", Hour: "lauds", UnitKey: "trinity-sunday", Date: date},
		{Hash: "bbb", Hour: "lauds", UnitKey: "trinity-sunday", Date: date.AddDate(1, 0, 20)}, // sibling variant
		{Hash: "ccc", Hour: "vespers", UnitKey: "all-saints", Date: date},
		{Hash: "ddd", Hour: "lauds", UnitKey: "pentecost", Date: date},
	}}
	signoffs := []Signoff{
		// Exact match for unit aaa.
		{Hash: "aaa", Hour: "lauds", UnitKey: "trinity-sunday", Reviewer: "mary.k", Date: "2026-06-08"},
		// Orphaned: hash no longer in manifest, same hour+key as ccc → ccc is stale.
		{Hash: "old", Hour: "vespers", UnitKey: "all-saints", Reviewer: "john.d", Date: "2026-05-01"},
	}

	statuses := Classify(m, signoffs)
	got := map[string]ReviewState{}
	for _, st := range statuses {
		got[st.Unit.Hash] = st.State
	}

	if got["aaa"] != Current {
		t.Errorf("aaa = %v, want Current", got["aaa"])
	}
	// Sibling of a reviewed unit whose sign-off hash is still live: not stale.
	if got["bbb"] != Unreviewed {
		t.Errorf("bbb = %v, want Unreviewed (sibling variant, not stale)", got["bbb"])
	}
	if got["ccc"] != Stale {
		t.Errorf("ccc = %v, want Stale (orphaned sign-off for same hour+key)", got["ccc"])
	}
	if got["ddd"] != Unreviewed {
		t.Errorf("ddd = %v, want Unreviewed", got["ddd"])
	}

	// The stale unit should carry the orphaned sign-off for attribution.
	for _, st := range statuses {
		if st.Unit.Hash == "ccc" && (st.Signoff == nil || st.Signoff.Reviewer != "john.d") {
			t.Error("stale unit should reference the orphaned sign-off")
		}
	}
}
