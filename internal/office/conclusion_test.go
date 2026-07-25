package office

import (
	"os"
	"strings"
	"testing"

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
		if corpus.Get(key) == "" {
			t.Errorf("collect-conclusions names %q, which is not in the corpus", key)
		}
		if !strings.HasSuffix(key, "/collect") {
			t.Errorf("collect-conclusions names %q, which is not a collect", key)
		}
		if got, ok := corpus.CollectConclusionForm(key); !ok || got != form {
			t.Errorf("corpus did not load %q -> %q (got %q, %v)", key, form, got, ok)
		}
		if ref := conclusionFormRef + form; corpus.Get(ref) == "" {
			t.Errorf("%s uses form %q, but %s is not in the corpus", key, form, ref)
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
