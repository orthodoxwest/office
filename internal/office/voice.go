package office

import (
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// Incipits spoken aloud before continuing secretly. Keys are corpus refs.
var prayerIncipits = map[string]string{
	"ordinary/shared/our-father":     "Our Father",
	"ordinary/shared/hail-mary":      "Hail, Mary",
	"ordinary/shared/apostles-creed": "I believe",
}

// Seam where the partly-secret Our Father resumes aloud.
const ourFatherAloudSeam = "And lead us not into temptation"

// buildPrayerVoice partitions a full prayer text into spoken/silent spans.
//
// secret (partly=false): spoken incipit + silent remainder.
// partly-secret: spoken incipit + silent middle + spoken tail from the seam.
//
// Returns nil when the ref has no known incipit or the text does not start with
// it — callers then render Text as ordinary spoken prose.
func buildPrayerVoice(ref, text string, partly bool) []models.VoiceSpan {
	incipit, ok := prayerIncipits[ref]
	if !ok || !strings.HasPrefix(text, incipit) {
		return nil
	}

	if !partly {
		rest := text[len(incipit):]
		if rest == "" {
			return []models.VoiceSpan{{Text: incipit, Spoken: true}}
		}
		return []models.VoiceSpan{
			{Text: incipit, Spoken: true},
			{Text: rest, Spoken: false},
		}
	}

	seamIdx := strings.Index(text, ourFatherAloudSeam)
	if seamIdx < len(incipit) {
		// Missing or degenerate seam: fall back to typical secret delivery.
		return buildPrayerVoice(ref, text, false)
	}

	spans := []models.VoiceSpan{{Text: incipit, Spoken: true}}
	if middle := text[len(incipit):seamIdx]; middle != "" {
		spans = append(spans, models.VoiceSpan{Text: middle, Spoken: false})
	}
	if tail := text[seamIdx:]; tail != "" {
		spans = append(spans, models.VoiceSpan{Text: tail, Spoken: true})
	}
	return spans
}
