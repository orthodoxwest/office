package output

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// incipitHour is one psalm with a Latin incipit and one without, so each test
// sees both the decorated and the bare label form.
func incipitHour() *models.OfficeHour {
	return &models.OfficeHour{
		Date:   time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC),
		Hour:   "Lauds",
		Title:  "Lauds",
		Season: models.Season("Christmastide"),
		Color:  models.Color("white"),
		Sections: []models.OfficeSection{{
			Label: "Psalmody",
			Elements: []models.OfficeElement{
				{
					Type:    models.Psalm,
					Label:   "Psalm 67",
					Incipit: "Deus misereatur nostri",
					Text:    "Psalm 67\n\nGOD be merciful * and bless us.",
				},
				{
					Type:  models.Psalm,
					Label: "Psalm 100",
					Text:  "Psalm 100\n\nO BE joyful * all ye lands.",
				},
			},
		}},
	}
}

func TestFormatOfficeHourShowsIncipitOnTheLabelLine(t *testing.T) {
	got := FormatOfficeHour(incipitHour())

	if !strings.Contains(got, "--- Psalm 67 · Deus misereatur nostri ---") {
		t.Errorf("expected the incipit on the psalm's label line:\n%s", got)
	}
	// A psalm with no recorded incipit keeps the plain label, with no
	// dangling separator.
	if !strings.Contains(got, "--- Psalm 100 ---") {
		t.Errorf("expected a bare label for the psalm without an incipit:\n%s", got)
	}
}

func TestFormatOfficeHourTeXSetsLabelAndIncipitApart(t *testing.T) {
	got := FormatOfficeHourTeX(incipitHour(), "", false)

	if !strings.Contains(got, `\newcommand{\psalmlabel}`) {
		t.Error("expected the psalmlabel macro in the preamble")
	}
	if !strings.Contains(got, `\psalmlabel{Psalm 67}{Deus misereatur nostri}`) {
		t.Errorf("expected the incipit as the macro's second argument:\n%s", got)
	}
	// The macro's empty-argument branch keeps an unlabelled psalm from
	// printing a stray separator.
	if !strings.Contains(got, `\psalmlabel{Psalm 100}{}`) {
		t.Errorf("expected an empty second argument when no incipit exists:\n%s", got)
	}
}
