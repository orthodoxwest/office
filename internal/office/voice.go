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

// Seam where the corporate Lord's Prayer passes from the officiant to the
// people's response.
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

// buildCorporateLordPrayerVoice identifies the two participants in the
// corporate Lord's Prayer. The shared corpus text remains the complete prayer
// without speaker notation, so its private and secret uses stay unchanged.
//
// The officiant says through the appointed versicle; the congregation answers
// the final response, including Amen. Nil makes a malformed source visible as
// an ordinary prayer instead of guessing a boundary.
func buildCorporateLordPrayerVoice(ref, text string) []models.VoiceSpan {
	if ref != "ordinary/shared/our-father" {
		return nil
	}
	seam := strings.Index(text, ourFatherAloudSeam)
	if seam < 0 {
		return nil
	}
	end := seam + len(ourFatherAloudSeam)
	// Keep terminal punctuation with the officiant, rather than presenting it
	// as an orphan before the response.
	for end < len(text) {
		r := text[end]
		if r != ',' && r != '.' && r != ';' && r != ':' {
			break
		}
		end++
	}
	response := strings.TrimLeft(text[end:], " \t\n")
	if response == "" {
		return nil
	}
	// Preserve the exact corpus text by keeping its separator with the
	// officiant span. Renderers reflow it naturally before the response line.
	return []models.VoiceSpan{
		{Text: text[:len(text)-len(response)], Spoken: true, Role: models.VoiceOfficiant},
		{Text: response, Spoken: true, Role: models.VoiceResponse},
	}
}
