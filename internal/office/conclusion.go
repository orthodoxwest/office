package office

import (
	"strings"

	"github.com/orthodoxwest/office/internal/texts"
)

// General Rubrics of the Monastic Breviary XXXIII.4 appoints a conclusion for
// every collect, inflected by whether and where the collect names the Son, and
// by whether it names the Holy Spirit. The choice is a property of each
// collect's own text rather than of the day, so it is recorded per corpus key
// in data/collect-conclusions.txt; "through" is the default and is not listed
// there.
//
// XXXIII.5 governs a run of collects: only the first and the last are
// concluded, and the ones between are not. Each is still preceded by Let us
// pray. See applyConclusion's callers for where that sequencing is enforced.

const (
	conclusionFormRef = "shared/formulas/collect-conclusion-"
	defaultConclusion = "through"
)

// conclusionRefFor returns the corpus ref of the conclusion formula appointed
// for the collect resolved from sourceRef, and whether one is available.
func conclusionRefFor(sourceRef string, corpus *texts.TextCorpus) (string, bool) {
	form := defaultConclusion
	if named, ok := corpus.CollectConclusionForm(sourceRef); ok {
		form = named
	}
	ref := conclusionFormRef + form
	if corpus.Get(ref) == "" {
		return "", false
	}
	return ref, true
}

// applyConclusion appends the appointed conclusion to a collect body, returning
// the extended text and the refs it drew on. A collect that is not to be
// concluded (XXXIII.5) simply is not passed through here.
func applyConclusion(text, sourceRef string, corpus *texts.TextCorpus) (string, []string) {
	// An unresolved slot renders as a "[...]" marker; concluding it would
	// disguise missing data as a complete prayer. An omitted collect is about
	// to be dropped, and must reach the composer as the bare marker.
	if text == "" || strings.HasPrefix(text, "[") || texts.IsOmitted(text) {
		return text, []string{sourceRef}
	}
	ref, ok := conclusionRefFor(sourceRef, corpus)
	if !ok {
		return text, []string{sourceRef}
	}
	return strings.TrimRight(text, "\n") + "\n" + corpus.Get(ref), []string{sourceRef, ref}
}
