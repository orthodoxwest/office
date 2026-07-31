package output

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestFormatOfficeHourAnnouncedAntiphon(t *testing.T) {
	hour := &models.OfficeHour{
		Hour: "Lauds",
		Sections: []models.OfficeSection{{
			Elements: []models.OfficeElement{
				{Type: models.Antiphon, Text: "Do away, O Lord, * mine offenses.", Announce: true},
				{Type: models.Psalm, Label: "Psalm 51", Text: "HAVE mercy upon me."},
				{Type: models.Antiphon, Text: "Do away, O Lord, * mine offenses."},
			},
		}},
	}
	got := FormatOfficeHour(hour)
	if !strings.Contains(got, "Do away, O Lord.\n") {
		t.Fatalf("expected announced opening antiphon:\n%s", got)
	}
	// The full form must still appear after the psalm.
	if strings.Count(got, "mine offenses") != 1 {
		t.Fatalf("full antiphon should appear once (after the psalm):\n%s", got)
	}
}

func TestFormatOfficeHourCorporateLordPrayerAddsOnlyResponseSigil(t *testing.T) {
	hour := &models.OfficeHour{Hour: "Lauds", Sections: []models.OfficeSection{{Elements: []models.OfficeElement{{
		Type: models.CorporateLordPrayer,
		Text: "Our Father.\nAnd lead us not into temptation,\nBut deliver us from evil. Amen.",
		Voice: []models.VoiceSpan{
			{Text: "Our Father.\nAnd lead us not into temptation,\n", Spoken: true, Role: models.VoiceOfficiant},
			{Text: "But deliver us from evil. Amen.", Spoken: true, Role: models.VoiceResponse},
		},
	}}}}}
	got := FormatOfficeHour(hour)
	if !strings.Contains(got, "And lead us not into temptation,\nR. But deliver us from evil. Amen.") {
		t.Fatalf("expected corporate response sigil and Amen:\n%s", got)
	}
}
