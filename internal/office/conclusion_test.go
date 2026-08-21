package office

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func conclusionTestCorpus() *texts.TextCorpus {
	corpus := texts.NewTestCorpus(map[string]string{
		"shared/formulas/collect-conclusion-through":      "Through Jesus Christ thy Son, our Lord.\nR. Amen.",
		"shared/formulas/collect-conclusion-through-same": "Through the same Jesus Christ thy Son, our Lord.\nR. Amen.",
		"shared/formulas/collect-conclusion-who-liveth":   "Who with thee liveth and reigneth.\nR. Amen.",
		"shared/formulas/collect-conclusion-through-spirit": "Through Jesus Christ thy Son, our Lord, " +
			"in the unity of the same Holy Ghost.\nR. Amen.",
	})
	corpus.SetCollectConclusionForm("proper/a/collect", "through-same")
	corpus.SetCollectConclusionForm("proper/b/collect", "who-liveth")
	corpus.SetCollectConclusionForm("proper/c/collect", "through-spirit")
	return corpus
}

func TestConclusionRefForUsesRecordedForm(t *testing.T) {
	corpus := conclusionTestCorpus()
	tests := []struct {
		name      string
		sourceRef string
		want      string
	}{
		{"recorded through-same", "proper/a/collect", "shared/formulas/collect-conclusion-through-same"},
		{"recorded who-liveth", "proper/b/collect", "shared/formulas/collect-conclusion-who-liveth"},
		{"recorded spirit variant", "proper/c/collect", "shared/formulas/collect-conclusion-through-spirit"},
		{"unrecorded falls to default", "proper/z/collect", "shared/formulas/collect-conclusion-through"},
		{"commons default", "commons/martyr/collect", "shared/formulas/collect-conclusion-through"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := conclusionRefFor(tt.sourceRef, corpus)
			if !ok {
				t.Fatalf("conclusionRefFor(%q) reported no conclusion available", tt.sourceRef)
			}
			if got != tt.want {
				t.Errorf("conclusionRefFor(%q) = %q, want %q", tt.sourceRef, got, tt.want)
			}
		})
	}
}

func TestConclusionRefForMissingFormula(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{})
	corpus.SetCollectConclusionForm("proper/a/collect", "nonexistent-form")
	if ref, ok := conclusionRefFor("proper/a/collect", corpus); ok {
		t.Errorf("conclusionRefFor with missing formula = (%q, true), want unavailable", ref)
	}
	// The default formula is absent too, so an unrecorded collect is also unavailable.
	if ref, ok := conclusionRefFor("proper/z/collect", corpus); ok {
		t.Errorf("conclusionRefFor with no default formula = (%q, true), want unavailable", ref)
	}
}

func TestApplyConclusionAppendsAppointedForm(t *testing.T) {
	corpus := conclusionTestCorpus()

	text, refs := applyConclusion("O God, who didst.", "proper/a/collect", corpus)
	if !strings.Contains(text, "Through the same Jesus Christ") {
		t.Errorf("applyConclusion did not append the through-same form: %q", text)
	}
	if !strings.HasPrefix(text, "O God, who didst.") {
		t.Errorf("applyConclusion did not preserve the collect body: %q", text)
	}
	if len(refs) != 2 || refs[0] != "proper/a/collect" ||
		refs[1] != "shared/formulas/collect-conclusion-through-same" {
		t.Errorf("applyConclusion refs = %v, want [collect, conclusion]", refs)
	}
}

func TestApplyConclusionLeavesUnresolvedTextAlone(t *testing.T) {
	corpus := conclusionTestCorpus()

	// A not-found marker must not be given a conclusion — that would disguise
	// missing data as a complete prayer.
	text, refs := applyConclusion("[Proper text not found: collect]", "proper/a/collect", corpus)
	if strings.Contains(text, "Through") {
		t.Errorf("applyConclusion concluded a not-found marker: %q", text)
	}
	if len(refs) != 1 {
		t.Errorf("applyConclusion refs = %v, want just the source ref", refs)
	}

	if text, _ := applyConclusion("", "proper/a/collect", corpus); text != "" {
		t.Errorf("applyConclusion on empty text = %q, want empty", text)
	}
}

func TestApplyConclusionSeparatesBodyFromConclusion(t *testing.T) {
	corpus := conclusionTestCorpus()
	text, _ := applyConclusion("O God, who didst.\n\n", "proper/z/collect", corpus)
	if strings.Contains(text, "didst.\n\n\n") {
		t.Errorf("applyConclusion left blank lines between body and conclusion: %q", text)
	}
	if !strings.Contains(text, "didst.\nThrough Jesus Christ") {
		t.Errorf("applyConclusion did not join body to conclusion on one break: %q", text)
	}
}

// Every form named in the real data must resolve to a real formula, and every
// key must name a collect that exists. A typo here would silently drop the
// conclusion or attach it to nothing.
func TestCollectConclusionDataIsConsistent(t *testing.T) {
	corpus, err := texts.LoadTexts("../../data")
	if err != nil {
		t.Fatalf("loading corpus: %v", err)
	}
	// Read the file directly: the corpus exposes lookups, not the whole map,
	// and this test is about the recorded data being internally consistent.
	raw, err := os.ReadFile("../../data/collect-conclusions.txt")
	if err != nil {
		t.Fatalf("reading collect-conclusions.txt: %v", err)
	}
	recorded := 0
	for line := range strings.SplitSeq(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) != 2 {
			t.Errorf("malformed line %q", trimmed)
			continue
		}
		recorded++
		key, form := fields[0], fields[1]
		// Unknown keys, unknown forms and duplicates are rejected by the loader
		// itself, so LoadTexts above would already have failed. What is left to
		// check here is that the key really names a collect — "commemoration-collect"
		// is a collect too, so match on the segment, not a "/collect" suffix.
		section := key[strings.LastIndex(key, "/")+1:]
		if !strings.Contains(section, "collect") {
			t.Errorf("collect-conclusions names %q, whose section %q is not a collect", key, section)
		}
		if got, ok := corpus.CollectConclusionForm(key); !ok || got != form {
			t.Errorf("corpus did not load %q -> %q (got %q, %v)", key, form, got, ok)
		}
	}
	if recorded == 0 {
		t.Fatal("no collect conclusion forms recorded in data/collect-conclusions.txt")
	}
	if corpus.Get(conclusionFormRef+defaultConclusion) == "" {
		t.Errorf("the default conclusion %s%s is missing", conclusionFormRef, defaultConclusion)
	}
}

// No collect reachable through proper-collect may carry its conclusion inline:
// the engine appends one, and a collect that already had its own would print
// the ending twice.
//
// The exceptions are the collects named directly by an hour definition as a
// plain "collect" element. Those never pass through applyConclusion, so they
// keep their conclusions in the text. Any other /collect entry — including the
// ordinary-tier fallbacks behind proper-collect — must not.
func TestNoProperCollectCarriesInlineConclusion(t *testing.T) {
	directlyReferenced := map[string]bool{
		"ordinary/prime/collect":           true,
		"ordinary/compline/collect":        true,
		"ordinary/shared/suffrage-collect": true,
		"ordinary/shared/cross-collect":    true,
	}
	corpus, err := texts.LoadTexts("../../data")
	if err != nil {
		t.Fatalf("loading corpus: %v", err)
	}
	for key, body := range corpus.Entries() {
		if !strings.Contains(key, "collect") || directlyReferenced[key] ||
			strings.HasPrefix(key, conclusionFormRef) {
			continue
		}
		for _, marker := range []string{"liveth and reigneth", "livest and reignest"} {
			if strings.Contains(body, marker) {
				t.Errorf("%s carries an inline conclusion (%q); the engine appends one, "+
					"so this would print twice", key, marker)
			}
		}
	}
}

// --- XXXIII.5 sequencing ---
//
// "When several Collects are to be said, only the first is said with the
// conclusion. The other Collects are not concluded, except the last Collect."
// The collect of the day always takes the first conclusion (engine.go), so
// addCommemorations must conclude the last commemoration and no other.
//
// These cases previously had no unit coverage at all: every mutation of the
// `i == len(comms)-1` guard survived.

func sequencingTestCorpus() *texts.TextCorpus {
	// The commemoration lookup chain ends at the commons content fallback
	// (commemorationFallbackSlots -> "collect"), so the collect must be reachable
	// as commons/<category>/collect for these feasts to resolve.
	return texts.NewTestCorpus(map[string]string{
		"shared/formulas/collect-conclusion-through": "CONCLUSION.\nR. Amen.",
		"commons/confessor/benedictus-antiphon":      "Ant. N.",
		"commons/confessor/versicle-lauds":           "V. N.\nR. N.",
		"commons/confessor/collect":                  "Collect of N.",
	})
}

func commemorationFeasts(n int) []*models.Feast {
	feasts := make([]*models.Feast, n)
	for i := range feasts {
		feasts[i] = &models.Feast{
			ID:         fmt.Sprintf("comm-%d", i),
			Name:       fmt.Sprintf("Commemorated Saint %d", i),
			Rank:       models.Simple,
			Category:   models.CategoryConfessor,
			ProperName: fmt.Sprintf("Saint%d", i),
		}
	}
	return feasts
}

// collectTexts returns the text of each Collect element, in order.
func collectTexts(elems []models.OfficeElement) []string {
	var out []string
	for _, e := range elems {
		if e.Type == models.Collect {
			out = append(out, e.Text)
		}
	}
	return out
}

func TestCommemorationSequencingConcludesOnlyTheLast(t *testing.T) {
	corpus := sequencingTestCorpus()

	tests := []struct {
		name  string
		count int
		// want[i] reports whether collect i should carry a conclusion.
		want []bool
	}{
		{"single commemoration is the last, so concluded", 1, []bool{true}},
		{"of two, only the second", 2, []bool{false, true}},
		{"of three, only the third", 3, []bool{false, false, true}},
		{"of five, only the fifth", 5, []bool{false, false, false, false, true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := makeDay(2026, time.July, 26, nil, commemorationFeasts(tt.count), "")
			got := collectTexts(addCommemorations(day, "lauds", corpus, false))
			if len(got) != tt.count {
				t.Fatalf("got %d collects, want %d", len(got), tt.count)
			}
			for i, wantConcluded := range tt.want {
				concluded := strings.Contains(got[i], "CONCLUSION.")
				if concluded != wantConcluded {
					t.Errorf("collect %d of %d: concluded=%v, want %v (XXXIII.5)\n  text: %q",
						i, tt.count, concluded, wantConcluded, got[i])
				}
			}
		})
	}
}

func TestCommemorationSequencingWithNoCommemorations(t *testing.T) {
	day := makeDay(2026, time.July, 26, nil, nil, "")
	if elems := addCommemorations(day, "lauds", sequencingTestCorpus(), false); len(elems) != 0 {
		t.Errorf("addCommemorations with no commemorations returned %d elements, want 0", len(elems))
	}
}

// The conclusion ref must be recorded on the concluded collect and absent from
// the others, so review tooling attributes the text correctly.
func TestCommemorationSequencingRecordsConclusionRef(t *testing.T) {
	day := makeDay(2026, time.July, 26, nil, commemorationFeasts(3), "")
	elems := addCommemorations(day, "lauds", sequencingTestCorpus(), false)

	var collects []models.OfficeElement
	for _, e := range elems {
		if e.Type == models.Collect {
			collects = append(collects, e)
		}
	}
	if len(collects) != 3 {
		t.Fatalf("got %d collects, want 3", len(collects))
	}
	for i, c := range collects {
		hasRef := false
		for _, r := range c.SourceRefs {
			if strings.HasPrefix(r, conclusionFormRef) {
				hasRef = true
			}
		}
		if want := i == 2; hasRef != want {
			t.Errorf("collect %d: conclusion ref present=%v, want %v (refs=%v)", i, hasRef, want, c.SourceRefs)
		}
	}
}

// When a Suffrage or Commemoration of the Cross follows, it carries the run's
// last conclusion, so no commemoration collect is concluded (XXXIII.5; see
// collectRunContinues for the Diurnal's printed ordering at p. 144).
func TestCommemorationSequencingWithSuffrageFollowing(t *testing.T) {
	corpus := sequencingTestCorpus()

	for _, count := range []int{1, 2, 3} {
		day := makeDay(2026, time.July, 26, nil, commemorationFeasts(count), "")
		got := collectTexts(addCommemorations(day, "lauds", corpus, true))
		if len(got) != count {
			t.Fatalf("got %d collects, want %d", len(got), count)
		}
		for i, text := range got {
			if strings.Contains(text, "CONCLUSION.") {
				t.Errorf("count=%d collect %d is concluded, want unconcluded: %q",
					count, i, text)
			}
		}
	}
}

func TestCollectRunContinues(t *testing.T) {
	section := func(name string, elemTypes ...string) HourSection {
		s := HourSection{Name: name}
		for _, et := range elemTypes {
			s.Elements = append(s.Elements, HourElement{Type: et})
		}
		return s
	}
	// The shape of the Lauds and Vespers definitions: the day's collect, the
	// commemorations, the two conditional devotions, then the blessing that
	// XXXIII.3 uses to close the run, and the Marian antiphon beyond it.
	sections := []HourSection{
		section("Collect", "proper-collect"),
		section("Commemorations", "commemorations"),
		section("Suffrage", "antiphon", "versicle", "collect"),
		section("CrossCommemoration", "antiphon", "versicle", "collect"),
		section("Closing", "blessing", "versicle"),
		section("Marian", "marian", "collect"),
	}

	tests := []struct {
		name     string
		included []bool
		// want is indexed by section. Only the Commemorations answer is
		// consumed today; the whole vector is asserted to pin the scan.
		want []bool
	}{
		{
			name:     "suffrage said: commemorations are not the last collect",
			included: []bool{true, true, true, false, true, true},
			want:     []bool{true, true, false, false, false, false},
		},
		{
			name:     "cross said instead of suffrage",
			included: []bool{true, true, false, true, true, true},
			want:     []bool{true, true, true, false, false, false},
		},
		{
			name:     "neither said: the last commemoration closes the run",
			included: []bool{true, true, false, false, true, true},
			want:     []bool{false, false, false, false, false, false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectRunContinues(sections, tt.included)
			for i := range sections {
				if got[i] != tt.want[i] {
					t.Errorf("section %q: got %v, want %v (full=%v)",
						sections[i].Name, got[i], tt.want[i], got)
				}
			}
		})
	}
}
