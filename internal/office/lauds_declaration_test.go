package office

import (
	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDeclaredLaudsOfTheDead(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	composed := 0
	// Includes Saturday, Sunday transfer to Monday, and every ordinary weekday.
	for year := 2024; year < 2052; year++ {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		for i := range days {
			day := &days[i]
			if day.Celebration == nil || day.Celebration.ID != "all-souls" {
				continue
			}
			for _, form := range models.PrayerForms {
				h, err := engine.ComposeHourWithOptions("lauds", day, calendar.ComputeMoveableDates(year), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				composed++
				var refs []string
				doxologies := 0
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						if e.Type == models.Psalm || e.Type == models.Canticle {
							refs = append(refs, e.SourceRef)
						}
						if e.Type == models.PsalmDoxology {
							doxologies++
						}
						if e.Type == models.PsalmDoxology && e.SourceRef != doxologyRestEternal {
							t.Errorf("%d %s: unexpected doxology %s", year, form, e.SourceRef)
						}
					}
				}
				if doxologies != 6 {
					t.Errorf("%d %s: %d doxologies, want six", year, form, doxologies)
				}
				want := []string{"psalms/051", "psalms/065", "psalms/063", "canticles/isaiah-38", "psalms/150", "canticles/benedictus"}
				if !reflect.DeepEqual(refs, want) {
					t.Errorf("%d %s: psalm/canticle order %v", year, form, refs)
				}
			}
		}
	}
	if composed != 28*len(models.PrayerForms) {
		t.Fatalf("composed %d All Souls offices", composed)
	}
}

func TestLaudsPsalmodyDeclarationResolution(t *testing.T) {
	day := &models.CalendarDay{Date: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), Season: models.Easter, Celebration: &models.Feast{ID: "example", ProperID: "shared", Rank: models.Double, Category: models.CategoryConfessor}}
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/shared/lauds-psalmody":         "psalm-antiphon-1 = psalms/051",
		"proper/shared-paschal/lauds-psalmody": "psalm-antiphon-1 = canticles/isaiah-38",
		"proper/shared/lauds-laudate-psalmody": "psalm-antiphon-5 = psalms/150",
	})
	items, source, err := resolveHourPsalmody(day, "lauds", laudsPsalmodyRef, corpus)
	if err != nil || source != "proper/shared-paschal/lauds-psalmody" || len(items) != 1 || items[0].psalm != "canticles/isaiah-38" {
		t.Fatalf("items=%v source=%s err=%v", items, source, err)
	}
	if !usesDeclaredLaudsPsalmody(day, corpus) || usesWeekdayLaudsPsalmody(day, corpus) {
		t.Fatal("explicit declaration should suppress weekday psalmody")
	}
	day.Season = models.Pentecost
	items, source, err = resolveHourPsalmody(day, "lauds", laudsPsalmodyRef, corpus)
	if err != nil || source != "proper/shared/lauds-psalmody" || len(items) != 1 || items[0].psalm != "psalms/051" {
		t.Fatalf("items=%v source=%s err=%v", items, source, err)
	}
	for _, body := range []string{"", "festal"} {
		c := texts.NewTestCorpus(map[string]string{"proper/shared/lauds-psalmody": body})
		if usesDeclaredLaudsPsalmody(day, c) {
			t.Errorf("%q should retain the existing psalmody sections", body)
		}
	}
	if usesDeclaredLaudsPsalmody(nil, corpus) || usesDeclaredLaudsPsalmody(day, nil) {
		t.Fatal("nil input has a declaration")
	}
	for _, tc := range []struct{ hour, ref, body string }{
		{"prime", laudsPsalmodyRef, "psalm-antiphon-1 = psalms/051"},
		{"lauds", vespersPsalmodyRef, "psalm-antiphon-1 = psalms/051"},
		{"lauds", laudsPsalmodyRef, ""},
		{"lauds", laudsPsalmodyRef, "ferial"},
		{"lauds", laudsPsalmodyRef, "psalm-antiphon-1 = psalms/051 dates=01-01"},
	} {
		c := texts.NewTestCorpus(map[string]string{"proper/shared/lauds-psalmody": tc.body})
		if _, _, err := resolveHourPsalmody(day, tc.hour, tc.ref, c); err == nil {
			t.Errorf("accepted %+v", tc)
		}
	}
}

func TestValidateDeclaredLaudsPsalmody(t *testing.T) {
	for _, tc := range []struct {
		name, body, laudate string
		wantError           bool
	}{
		{"valid", "psalm-antiphon-1 = canticles/example", "psalm-antiphon-5 = psalms/150", false},
		{"missing Laudate", "psalm-antiphon-1 = canticles/example", "", true},
		{"ferial unsupported", "ferial", "psalm-antiphon-5 = psalms/150", true},
		{"unknown psalm", "psalm-antiphon-1 = psalms/missing", "psalm-antiphon-5 = psalms/150", true},
		{"unknown antiphon", "missing = psalms/150", "psalm-antiphon-5 = psalms/150", true},
		{"wrong text type", "psalm-antiphon-1 = proper/shared/psalm-antiphon-1", "psalm-antiphon-5 = psalms/150", true},
		{"bad Laudate", "psalm-antiphon-1 = canticles/example", "festal", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := map[string]string{"proper/shared/lauds-psalmody": tc.body, "canticles/example": "Canticle", "psalms/150": "Psalm", "proper/shared/psalm-antiphon-1": "Antiphon", "proper/shared/psalm-antiphon-5": "Final antiphon"}
			if tc.laudate != "" {
				entries["proper/shared/lauds-laudate-psalmody"] = tc.laudate
			}
			errs := validateLaudsPsalmodyDeclarations(texts.NewTestCorpus(entries))
			if (len(errs) > 0) != tc.wantError {
				t.Errorf("errors=%v", errs)
			}
		})
	}
}

func TestDeclaredPsalmodyUsesCanticleAndOfficeDoxology(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{"canticles/example": "Canticle", "ordinary/lauds/psalm-antiphon-1": "Antiphon", doxologyGloriaPatri: "Gloria", doxologyRestEternal: "Rest eternal"})
	for _, id := range []string{"example", "all-souls"} {
		day := &models.CalendarDay{Celebration: &models.Feast{ID: id}}
		elems := composeResolvedPsalmody(day, "lauds", []psalmodyItem{{antiphon: "psalm-antiphon-1", psalm: "canticles/example"}}, corpus)
		if len(elems) != 4 || elems[1].Type != models.Canticle {
			t.Fatalf("elements=%v", elems)
		}
		want := "Gloria"
		if id == "all-souls" {
			want = "Rest eternal"
		}
		if !strings.Contains(elems[2].Text, want) {
			t.Errorf("%s doxology=%q", id, elems[2].Text)
		}
	}
}
