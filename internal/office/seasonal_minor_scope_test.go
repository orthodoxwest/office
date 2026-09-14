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
	corpus := scopedTestCorpus(t, map[string]string{
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

// Use the production scope file with controlled corpus candidates, so missing
// propers/commons really reach the seasonal tier under test.
func scopedTestCorpus(t *testing.T, entries map[string]string) *texts.TextCorpus {
	t.Helper()
	for _, season := range []string{"lent", "passiontide", "easter"} {
		for _, hour := range []string{"terce", "sext", "none"} {
			for _, slot := range []string{"chapter", "versicle", "psalm-antiphon"} {
				key := "seasonal/" + season + "/" + slot + "-" + hour
				if _, ok := entries[key]; !ok {
					entries[key] = "Seasonal test text"
				}
			}
		}
	}
	corpus := texts.NewTestCorpus(entries)
	if err := corpus.LoadAppointmentScopes("../../data/appointment-scopes.json"); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestSeasonalScopePreservesPrecedenceAndOmission(t *testing.T) {
	for _, tc := range []struct {
		name, date, proper, common, seasonal, want string
		feast                                      *models.Feast
	}{
		{"unnamed feria", "2026-03-30", "", "", "Seasonal", "Seasonal", nil},
		{"named feria", "2026-03-30", "", "", "Seasonal", "Seasonal", &models.Feast{ID: "holy-monday", Category: models.CategoryFeria}},
		{"synthesized feria", "2026-03-30", "", "", "Seasonal", "Seasonal", &models.Feast{ID: "privileged-lenten-feria", Category: models.CategoryFeria}},
		{"inactive falls through", "2026-03-29", "", "", "Seasonal", "Ordinary", nil},
		{"proper wins while inactive", "2026-03-29", "Proper", "", "Seasonal", "Proper", &models.Feast{ID: "example", Category: models.CategorySunday}},
		{"common wins while inactive", "2026-03-30", "", "Common", "Seasonal", "Common", &models.Feast{ID: "example", Category: models.CategoryMartyr}},
		{"proper omission wins", "2026-03-30", "@omit", "", "Seasonal", "@omit", &models.Feast{ID: "example", Category: models.CategoryFeria}},
		{"active seasonal omission", "2026-03-30", "", "", "@omit", "@omit", nil},
		{"inactive seasonal omission falls through", "2026-03-29", "", "", "@omit", "Ordinary", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			corpus := scopedTestCorpus(t, map[string]string{
				"proper/example/chapter-terce":       tc.proper,
				"commons/martyr/chapter-terce":       tc.common,
				"seasonal/passiontide/chapter-terce": tc.seasonal,
				"ordinary/terce/chapter":             "Ordinary",
			})
			date, err := time.Parse("2006-01-02", tc.date)
			if err != nil {
				t.Fatal(err)
			}
			day := &models.CalendarDay{Date: date, Season: models.Passiontide, Celebration: tc.feast}
			got, _ := resolveProperText(day, "terce", "chapter", corpus)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
