package output

import (
	"fmt"
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// FormatOfficeHour formats a composed office hour as plain text for CLI output.
// Announced antiphons (the opening half of a psalm/canticle frame when the
// office does not double them) print only through the mediant asterisk.
func FormatOfficeHour(hour *models.OfficeHour) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "%s\n", strings.ToUpper(hour.Hour))
	fmt.Fprintf(&b, "%s\n", hour.Date.Format("Monday, January 2, 2006"))
	if hour.Feast != "" {
		fmt.Fprintf(&b, "%s\n", hour.Feast)
	}
	fmt.Fprintf(&b, "Season: %s | Color: %s\n", hour.Season, hour.Color)
	b.WriteString(strings.Repeat("=", 60))
	b.WriteString("\n\n")

	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.Label != "" {
				// The Latin incipit rides on the label line rather than taking
				// a line of its own: this format is a flat stream of labelled
				// blocks, and a second header line would read as an element.
				label := elem.Label
				if elem.Incipit != "" {
					label += " · " + elem.Incipit
				}
				fmt.Fprintf(&b, "--- %s ---\n", label)
			}
			text := elem.DisplayText()
			if elem.Type == models.CorporateLordPrayer {
				text = formatCorporateLordPrayerText(elem)
			}
			if text != "" {
				b.WriteString(text)
				b.WriteString("\n\n")
			}
		}
	}

	return b.String()
}

// formatCorporateLordPrayerText adds only the response sigil required by the
// corporate form. The underlying shared text stays suitable for private and
// secret prayer elsewhere in the office.
func formatCorporateLordPrayerText(elem models.OfficeElement) string {
	var officiant, response strings.Builder
	for _, span := range elem.Voice {
		switch span.Role {
		case models.VoiceOfficiant:
			officiant.WriteString(span.Text)
		case models.VoiceResponse:
			response.WriteString(span.Text)
		}
	}
	if officiant.Len() == 0 || response.Len() == 0 {
		return elem.Text
	}
	return strings.TrimRight(officiant.String(), " \t\n") + "\nR. " + strings.TrimSpace(response.String())
}
