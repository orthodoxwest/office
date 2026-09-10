package render

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// LeaderForm carries the fully resolved composition and its matching review
// metadata. All forms travel in the same cacheable document.
type LeaderForm struct {
	Form       models.PrayerForm
	Hour       *models.OfficeHour
	Assurance  HourAssurance
	ReportURL  string
	ShowBanner bool
}

type LeaderSection struct {
	Label       string
	Collapsible bool
	HTML        template.HTML
}

// leaderSections shares unchanged elements and bundles only explicit ordinary
// slots. Each slot may contain a different number of elements (the choir
// confession expands to several prayers and rubrics, and a repeated private
// greeting disappears). A structural mismatch
// fails instead of attaching an alternative to the wrong prayer.
func leaderSections(forms []LeaderForm) ([]LeaderSection, error) {
	if len(forms) != len(models.PrayerForms) || forms[2].Hour == nil {
		return nil, fmt.Errorf("expected three leader forms")
	}
	// The priest form retains every greeting, making it the alignment
	// template even where the private substitution is omitted.
	const baseIndex = 2
	base := forms[baseIndex].Hour
	for i, form := range forms {
		if form.Form != models.PrayerForms[i] || form.Hour == nil || len(form.Hour.Sections) != len(base.Sections) {
			return nil, fmt.Errorf("inconsistent leader forms")
		}
	}
	var sections []LeaderSection
	for si, section := range base.Sections {
		positions := make([]int, len(forms))
		var html strings.Builder
		for positions[baseIndex] < len(section.Elements) {
			baseStart := positions[baseIndex]
			first := section.Elements[baseStart]
			baseEnd := baseStart + 1
			for baseEnd < len(section.Elements) && section.Elements[baseEnd].LeaderSlot == first.LeaderSlot {
				baseEnd++
			}
			var groups [][]models.OfficeElement
			for fi, form := range forms {
				s := form.Hour.Sections[si]
				if s.Label != section.Label || s.Collapsible != section.Collapsible {
					return nil, fmt.Errorf("leader section mismatch")
				}
				start := positions[fi]
				if fi == 0 && first.LeaderSlot == "greeting" && (start >= len(s.Elements) || s.Elements[start].LeaderSlot != first.LeaderSlot) {
					groups = append(groups, nil)
					continue
				}
				if start >= len(s.Elements) || s.Elements[start].LeaderSlot != first.LeaderSlot {
					return nil, fmt.Errorf("leader slot mismatch")
				}
				end := start + 1
				if first.LeaderSlot == "" {
					// Common runs can join across a missing private greeting.
					// Consume only the template's run, then compare it below.
					end = start + baseEnd - baseStart
					if end > len(s.Elements) {
						return nil, fmt.Errorf("incomplete common leader sequence")
					}
				} else {
					for end < len(s.Elements) && s.Elements[end].LeaderSlot == first.LeaderSlot {
						end++
					}
				}
				groups = append(groups, s.Elements[start:end])
				positions[fi] = end
			}
			if first.LeaderSlot == "" {
				for _, group := range groups[1:] {
					if !reflect.DeepEqual(groups[0], group) {
						return nil, fmt.Errorf("unmarked leader variation")
					}
				}
				html.WriteString(string(renderSectionElements(groups[0])))
				continue
			}
			// Deacon and priest currently share some complete sequences. Emit
			// identical alternatives once, with both applicable values.
			used := make([]bool, len(forms))
			var available []string
			for fi, group := range groups {
				if len(group) > 0 {
					available = append(available, string(forms[fi].Form))
				}
			}
			html.WriteString(`<div class="leader-slot" data-leader-slot="` + template.HTMLEscapeString(first.LeaderSlot) + `"`)
			if len(available) < len(forms) {
				// Hide the grid item itself so an omitted greeting leaves no
				// empty row or extra gap, on screen or in print.
				html.WriteString(` data-leaders="` + strings.Join(available, " ") + `"`)
			}
			html.WriteString(`>`)
			for fi, group := range groups {
				if used[fi] || len(group) == 0 {
					continue
				}
				leaders := []string{string(forms[fi].Form)}
				for next := fi + 1; next < len(forms); next++ {
					if reflect.DeepEqual(group, groups[next]) {
						leaders = append(leaders, string(forms[next].Form))
						used[next] = true
					}
				}
				html.WriteString(`<div data-leaders="` + strings.Join(leaders, " ") + `">`)
				html.WriteString(string(renderSectionElements(group)))
				html.WriteString(`</div>`)
			}
			html.WriteString(`</div>`)
		}
		for fi, form := range forms {
			if positions[fi] != len(form.Hour.Sections[si].Elements) {
				return nil, fmt.Errorf("unmatched leader elements")
			}
		}
		sections = append(sections, LeaderSection{Label: section.Label, Collapsible: section.Collapsible, HTML: template.HTML(html.String())})
	}
	return sections, nil
}
