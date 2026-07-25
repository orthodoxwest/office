package office

import (
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// CommSummary is one commemoration extracted from a composed hour: its name and
// the incipit of its gospel-canticle antiphon, mirroring the ordo's
// `Comm. Name ("incipit")` form.
type CommSummary struct {
	Name    string
	Incipit string
}

// HourSummary is the ordo-relevant digest of a single composed hour. It lives
// beside the composers rather than in a formatter package because deciding
// whether preces are said, whether the Suffrage occurs, and which
// commemorations a day carries is rubric logic, not presentation. The text
// ordo (output.FormatDay), the web calendar view, and the `rubrics` CLI all
// read it, so none of them re-walks a composed hour on its own.
type HourSummary struct {
	Color models.Color
	// GospelAnt is the Benedictus/Magnificat antiphon reduced to an incipit
	// for side-by-side reading; GospelAntFull is the whole antiphon on one
	// line, which the rubrics TSV emits for machine diffing against an ordo.
	GospelAnt     string
	GospelAntFull string
	Preces        bool
	Suffrage      bool
	Comms         []CommSummary
}

// SummarizeHour walks a composed hour and pulls out the fields the ordo prints:
// the Benedictus/Magnificat antiphon incipit, whether preces and the Suffrage
// are said, and each commemoration with its antiphon incipit.
func SummarizeHour(hour *models.OfficeHour) HourSummary {
	s := HourSummary{Color: hour.Color}
	for _, sec := range hour.Sections {
		if strings.Contains(sec.Label, "Suffrage") {
			s.Suffrage = true
		}
		for i, el := range sec.Elements {
			switch {
			case el.Type == models.Preces:
				s.Preces = true
			case el.Type == models.Heading && strings.HasPrefix(el.Text, "Commemoration of "):
				c := CommSummary{Name: strings.TrimPrefix(el.Text, "Commemoration of ")}
				// The commemoration antiphon is the next Antiphon element.
				for _, next := range sec.Elements[i+1:] {
					if next.Type == models.Antiphon {
						c.Incipit = incipit(next.Text)
						break
					}
					if next.Type == models.Heading {
						break
					}
				}
				s.Comms = append(s.Comms, c)
			case s.GospelAnt == "" && (el.SlotRef == "benedictus-antiphon" || el.SlotRef == "magnificat-antiphon"):
				s.GospelAnt = incipit(el.Text)
				s.GospelAntFull = strings.ReplaceAll(el.Text, "\n", " ")
			}
		}
	}
	return s
}

// incipit reduces an antiphon text to a short opening phrase for cross-checking
// against the ordo: it takes the text up to the mediant asterisk (or the first
// nine words) and strips trailing punctuation.
func incipit(text string) string {
	s := strings.ReplaceAll(text, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "*"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	words := strings.Fields(s)
	truncated := false
	if len(words) > 9 {
		words = words[:9]
		truncated = true
	}
	s = strings.TrimRight(strings.Join(words, " "), " ,;:.")
	if truncated {
		s += "…"
	}
	return s
}
