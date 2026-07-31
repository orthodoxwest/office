package office

import (
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// rubricPrayerPhrases records the small, reviewed set of prayer names and
// words quoted for recitation inside instructional rubrics. It is deliberately
// keyed by corpus ref: interpreting arbitrary occurrences of "Our Father" in
// prose would be both brittle and liable to colour ordinary instructional text
// as though it were spoken.
var rubricPrayerPhrases = map[string][]string{
	"ordinary/session/our-father-hail-mary-rubric":         {"Our Father", "Hail Mary"},
	"ordinary/session/prime-opening-secret-prayers-rubric": {"Our Father", "Hail Mary", "Apostles' Creed"},
	"ordinary/session/little-hours-opening-rubric":         {"Our Father", "Hail Mary"},
	"ordinary/session/closing-rubric":                      {"Our Father", "Hail Mary"},
	"shared/formulas/our-father-partly-secret-rubric": {
		"Our Father", "And lead us not into temptation", "But deliver us from evil. Amen.",
	},
	"shared/formulas/closing-our-father": {"Our Father"},
	"ordinary/compline/confiteor-rubric": {"Our Father"},
}

// buildRubricSpans partitions a known rubric into instruction and quoted
// prayer words. A changed corpus phrase fails closed (nil), keeping the whole
// rubric red until its annotation is consciously reviewed.
func buildRubricSpans(ref, text string) []models.RubricSpan {
	phrases := rubricPrayerPhrases[ref]
	if len(phrases) == 0 {
		return nil
	}

	spans := make([]models.RubricSpan, 0, len(phrases)*2+1)
	pos := 0
	for _, phrase := range phrases {
		i := rubricPhraseIndex(text, phrase, pos)
		if i < 0 {
			return nil
		}
		if i > pos {
			spans = append(spans, models.RubricSpan{Text: text[pos:i]})
		}
		spans = append(spans, models.RubricSpan{Text: phrase, Prayed: true})
		pos = i + len(phrase)
	}
	if pos < len(text) {
		spans = append(spans, models.RubricSpan{Text: text[pos:]})
	}
	return spans
}

// rubricPhraseIndex accepts only a whole quoted phrase. In particular it
// rejects possessives such as "Our Father's", which are instruction prose,
// not words being prayed.
func rubricPhraseIndex(text, phrase string, start int) int {
	for from := start; from <= len(text)-len(phrase); {
		i := strings.Index(text[from:], phrase)
		if i < 0 {
			return -1
		}
		i += from
		beforeOK := i == 0 || !rubricWordByte(text[i-1])
		after := i + len(phrase)
		afterOK := after == len(text) || !rubricWordByte(text[after])
		if beforeOK && afterOK {
			return i
		}
		from = i + len(phrase)
	}
	return -1
}

func rubricWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '\''
}
