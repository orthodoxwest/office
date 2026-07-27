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
	if !strings.Contains(got, "Do away, O Lord, *\n") {
		t.Fatalf("expected announced opening antiphon:\n%s", got)
	}
	// The full form must still appear after the psalm.
	if strings.Count(got, "mine offenses") != 1 {
		t.Fatalf("full antiphon should appear once (after the psalm):\n%s", got)
	}
}
