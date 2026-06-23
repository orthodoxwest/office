package output

import (
	"fmt"
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// FormatOfficeHour formats a composed office hour as plain text for CLI output.
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
				fmt.Fprintf(&b, "--- %s ---\n", elem.Label)
			}
			if elem.Text != "" {
				b.WriteString(elem.Text)
				b.WriteString("\n\n")
			}
		}
	}

	return b.String()
}
