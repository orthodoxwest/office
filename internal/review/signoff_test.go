package review

import (
	"bytes"
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
		Reviewer: "mary.k", Date: "2026-06-08", Schema: 0,
		Note: "checked against diurnal + supplement",
	}
	if signoffs[0] != want {
		t.Errorf("signoffs[0] = %+v, want %+v", signoffs[0], want)
	}
	if signoffs[1].Note != "" {
		t.Errorf("signoffs[1].Note = %q, want empty", signoffs[1].Note)
	}
}

func TestSignoffForPageResolvesInternalIdentity(t *testing.T) {
	date := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	signoff, unit, err := SignoffForPage("../../data", "lauds", date, "mary.k", "checked")
	if err != nil {
		t.Fatal(err)
	}
	if signoff.Hash == "" || signoff.Hash != unit.Hash {
		t.Fatalf("signoff identity was not resolved: signoff=%#v unit=%#v", signoff, unit)
	}
	if signoff.Hour != "lauds" || signoff.UnitKey != "trinity-sunday" || signoff.Reviewer != "mary.k" {
		t.Fatalf("signoff = %#v", signoff)
	}
}

func TestStatusOutputUsesPageIdentity(t *testing.T) {
	date := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	statuses := []UnitStatus{{Unit: &Unit{
		Hash: "0123456789ab", Hour: "lauds", UnitKey: "trinity-sunday",
		Name: "Trinity Sunday", Rank: "double-1st-class", Date: date,
	}, State: Unreviewed}}
	var out bytes.Buffer
	PrintStatus(statuses, &out)
	if strings.Contains(out.String(), "0123456789ab") || strings.Contains(out.String(), "sign HASH") {
		t.Fatalf("status output exposes internal composition identity:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "review sign HOUR YYYY-MM-DD REVIEWER") {
		t.Fatalf("status output lacks page-based sign-off guidance:\n%s", out.String())
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
