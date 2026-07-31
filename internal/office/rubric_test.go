package office

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestBuildRubricSpansUsesReviewedCorpusRefsOnly(t *testing.T) {
	text := "Our Father is said secretly."
	got := buildRubricSpans("shared/formulas/closing-our-father", text)
	want := []models.RubricSpan{{Text: "Our Father", Prayed: true}, {Text: " is said secretly."}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans = %#v, want %#v", got, want)
	}
	if got := buildRubricSpans("ordinary/example/unreviewed-rubric", "Our Father's instruction is read."); got != nil {
		t.Fatalf("unreviewed prose must not be parsed as quoted prayer: %#v", got)
	}
	if got := buildRubricSpans("shared/formulas/closing-our-father", "The Our Father's prayer is read."); got != nil {
		t.Fatalf("changed reviewed wording must fail closed: %#v", got)
	}
}

func TestReviewedRubricPrayerAnnotationsMatchCorpus(t *testing.T) {
	corpus, err := texts.LoadTexts(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	for ref := range rubricPrayerPhrases {
		if spans := buildRubricSpans(ref, corpus.Get(ref)); len(spans) == 0 {
			t.Errorf("reviewed rubric %q no longer matches its annotation", ref)
		}
	}
}
