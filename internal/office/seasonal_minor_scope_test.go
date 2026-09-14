package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// Missing Sunday and feast propers must remain visible as fallbacks, rather
// than being concealed by a chapter appointed only to the ferial office.
func TestPassiontideFerialTextsDoNotFillMissingFestalPropers(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"seasonal/passiontide/chapter-terce":        "Ferial chapter",
		"seasonal/passiontide/psalm-antiphon-terce": "Passion week antiphon",
		"ordinary/terce/chapter":                    "Ordinary chapter",
		"ordinary/terce/psalm-antiphon":             "Ordinary antiphon",
	})
	for _, tc := range []struct {
		name, date, slot string
		category         models.FeastCategory
	}{
		{"Sunday chapter", "2026-03-29", "chapter", models.CategorySunday},
		{"feast chapter", "2026-03-31", "chapter", models.CategoryMartyr},
		{"Holy Week antiphon", "2026-04-06", "psalm-antiphon-2", models.CategoryFeria},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tc.date)
			if err != nil {
				t.Fatal(err)
			}
			day := &models.CalendarDay{Date: date, Season: models.Passiontide, Celebration: &models.Feast{ID: "missing-proper", Category: tc.category}}
			_, ref := resolveProperText(day, "terce", tc.slot, corpus)
			if ref == "seasonal/passiontide/chapter-terce" || ref == "seasonal/passiontide/psalm-antiphon-terce" {
				t.Fatalf("unappointed ferial text concealed a missing proper: %s", ref)
			}
		})
	}
}
