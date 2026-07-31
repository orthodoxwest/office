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

func TestRubricPhraseIndexRespectsQuotedPhraseBoundaries(t *testing.T) {
	const phrase = "Our Father"
	for _, tc := range []struct {
		name  string
		text  string
		start int
		want  int
	}{
		{"at start", "Our Father is said.", 0, 0},
		{"whole text", "Our Father", 0, 0},
		{"after punctuation", "Say (Our Father).", 0, 5},
		{"after earlier embedded text", "Our Father's rule; Our Father is said.", 0, 19},
		{"start skips first quoted phrase", "Our Father; Our Father.", len("Our Father; "), len("Our Father; ")},
		{"rejects word prefix", "Our Fatherly devotion.", 0, -1},
		{"rejects word suffix", "The Our FatherX speaks.", 0, -1},
		{"rejects possessive", "The Our Father's prayer.", 0, -1},
		{"rejects apostrophe prefix", "The 'Our Father' is named, not said.", 0, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := rubricPhraseIndex(tc.text, phrase, tc.start); got != tc.want {
				t.Fatalf("rubricPhraseIndex(%q, %q, %d) = %d, want %d", tc.text, phrase, tc.start, got, tc.want)
			}
		})
	}
}

func TestBuildRubricSpansDoesNotAppendAnEmptyTrailingInstruction(t *testing.T) {
	const ref = "test/exact-rubric-phrase"
	rubricPrayerPhrases[ref] = []string{"Our Father"}
	t.Cleanup(func() { delete(rubricPrayerPhrases, ref) })

	got := buildRubricSpans(ref, "Our Father")
	want := []models.RubricSpan{{Text: "Our Father", Prayed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildRubricSpans exact phrase = %+v, want %+v", got, want)
	}
}

func TestRubricWordByteRecognizesOnlyPhraseWordCharacters(t *testing.T) {
	for _, tc := range []struct {
		byte byte
		want bool
	}{
		{'a', true}, {'z', true}, {'A', true}, {'Z', true}, {'\'', true},
		{'0', false}, {'_', false}, {'-', false}, {' ', false}, {'.', false},
	} {
		if got := rubricWordByte(tc.byte); got != tc.want {
			t.Errorf("rubricWordByte(%q) = %v, want %v", tc.byte, got, tc.want)
		}
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
