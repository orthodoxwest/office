package office

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// Diurnal pp.7,151: the preces Creed is secret until its final articles.
// At Compline the opening Pater is entirely secret (p.147), unlike its
// closing Pater and Creed, whose incipits are spoken (p.152).
func TestOrdinaryPrayerVoicesAndMartyrologyBoundary(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	day := &models.CalendarDay{Date: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), Season: models.Pentecost}
	for _, form := range models.PrayerForms {
		for _, hour := range []string{"prime", "compline"} {
			for _, preview := range []bool{false, true} {
				h, err := engine.ComposeHourWithOptions(hour, day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form, MartyrologyPreview: preview})
				if err != nil {
					t.Fatal(err)
				}
				preces, closing, martyrology, position := false, -1, -1, 0
				paterCount := 0
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						position++
						if e.SourceRef == "shared/formulas/faithful-departed" {
							closing = position
						}
						if e.SourceRef == "ordinary/prime/martyrology-rubric" || strings.HasPrefix(e.SourceRef, "ordinary/martyrology/") {
							if martyrology < 0 {
								martyrology = position
							}
						}
						if e.SourceRef == "ordinary/shared/our-father" {
							paterCount++
							if hour == "compline" && paterCount == 1 {
								assertVoice(t, e.Voice, []models.VoiceSpan{{Text: e.Text, Spoken: false}})
							}
						}
						if e.SourceRef != "ordinary/shared/apostles-creed" {
							continue
						}
						if s.Label == "Preces" {
							preces = true
							if len(e.Voice) != 3 || e.Voice[0].Text != "I believe" || !e.Voice[0].Spoken || e.Voice[1].Spoken || !e.Voice[2].Spoken || !strings.HasPrefix(e.Voice[2].Text, "The Resurrection of the body") || !strings.HasSuffix(e.Voice[2].Text, "Amen.") {
								t.Errorf("%s/%s preces Creed: %+v", hour, form, e)
							}
						} else if len(e.Voice) != 2 || e.Voice[1].Spoken {
							t.Errorf("ordinary secret Creed changed: %+v", e)
						}
					}
				}
				if !preces {
					t.Errorf("%s/%s missing ferial preces", hour, form)
				}
				if hour == "prime" && (closing < 0 || martyrology <= closing) {
					t.Errorf("Prime closes at %d, Martyrology at %d", closing, martyrology)
				}
				if hour == "compline" && closing >= 0 {
					t.Error("Compline gained Faithful departed")
				}
			}
		}
	}
}

func TestPrimeSundayAntiphonPrecedence(t *testing.T) {
	date := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, key, category string
		season              models.Season
	}{
		{"ordinary", "ordinary/prime/psalm-antiphon-1-sunday", string(models.CategorySunday), models.Pentecost},
		{"proper", "proper/test-feast/psalm-antiphon-1-lauds", string(models.CategorySunday), models.Pentecost},
		{"common", "commons/apostle/psalm-antiphon-1", string(models.CategoryApostle), models.Pentecost},
		{"seasonal", "seasonal/easter/psalm-antiphon-1-lauds", string(models.CategorySunday), models.Easter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := map[string]string{"ordinary/lauds/psalm-antiphon-1-sunday": "Two alleluias", "ordinary/prime/psalm-antiphon-1-sunday": "Three alleluias", tc.key: "Expected antiphon"}
			corpus := texts.NewTestCorpus(entries)
			day := &models.CalendarDay{Date: date, Season: tc.season, Celebration: &models.Feast{ID: "test-feast", Category: models.FeastCategory(tc.category)}}
			e := resolvePrimePsalmAntiphon(day, corpus, nil)
			if e.Text != "Expected antiphon" || e.SourceRef != tc.key {
				t.Fatalf("antiphon: %+v", e)
			}
			trace := traceProperResolution(day, "prime", e.SlotRef, e.SourceRef, corpus)
			want := "lauds"
			if tc.name == "ordinary" {
				want = "prime"
			}
			if trace.ResolverHour != want {
				t.Errorf("trace hour %s, want %s", trace.ResolverHour, want)
			}
		})
	}
}

// The p.6 Paschal verse lasts through Pentecost octave Saturday None.
func TestPaschalPrimeVerse(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		md := calendar.ComputeMoveableDates(year)
		for i := range days {
			day := &days[i]
			if day.Date.Before(md.Easter) || day.Date.After(md.TrinitySunday) {
				continue
			}
			for _, form := range models.PrayerForms {
				h, err := engine.ComposeHourWithOptions("prime", day, md, ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				e := officeElementBySlot(h, "pre-collect-versicle")
				want := 0
				if day.Date.Before(md.TrinitySunday) {
					want = 2
				}
				if e == nil || strings.Count(strings.ToLower(e.Text), "alleluia") != want {
					t.Errorf("%s/%s: %+v", day.Date, form, e)
				}
			}
		}
	}
}

// Psalter structure: pp.4–29,81–84 and 148–150. Wednesday joins 9b and 10
// before the first Gloria; ordinary Compline has no psalm antiphons or Nunc.
func TestOrdinaryPrimeAndComplinePsalmody(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"119-i", "119-ii", "119-iii", "119-iv"}, {"001", "002", "006"}, {"007", "008", "009a"}, {"009b", "010", "011", "012"}, {"013", "014", "015"}, {"016", "017", "018a"}, {"018b", "019", "020"}}
	for weekday := 0; weekday < 7; weekday++ {
		for _, hour := range []string{"prime", "compline"} {
			day := &models.CalendarDay{Date: time.Date(2026, 9, 13+weekday, 0, 0, 0, 0, time.UTC), Season: models.Pentecost}
			if weekday == 0 {
				day.Celebration = &models.Feast{ID: "ordinary-sunday", Category: models.CategorySunday}
			}
			h, err := engine.ComposeHour(hour, day, calendar.ComputeMoveableDates(2026))
			if err != nil {
				t.Fatal(err)
			}
			var psalms []string
			glorias := 0
			for _, s := range h.Sections {
				for _, e := range s.Elements {
					if e.Type == models.Psalm {
						psalms = append(psalms, strings.TrimPrefix(e.SourceRef, "psalms/"))
					}
					if e.SourceRef == "ordinary/shared/gloria-patri" {
						glorias++
						if hour == "prime" && weekday == 3 && len(psalms) == 1 {
							t.Error("Gloria splits Wednesday 9b/10")
						}
					}
					if hour == "compline" && (strings.HasPrefix(e.SourceRef, "canticles/nunc") || strings.HasPrefix(e.SlotRef, "psalm-antiphon")) {
						t.Errorf("ordinary Compline gained %+v", e)
					}
				}
			}
			expected := want[weekday]
			if hour == "compline" {
				expected = []string{"004", "091", "134"}
			}
			if !slices.Equal(psalms, expected) {
				t.Errorf("%s weekday %d: %v", hour, weekday, psalms)
			}
			count := 3
			if hour == "prime" && weekday == 0 {
				count = 4
			}
			if glorias != count {
				t.Errorf("%s weekday %d: %d psalm Glorias, want %d", hour, weekday, glorias, count)
			}
			if hour == "prime" && weekday == 0 {
				e := officeElementBySlot(h, "psalm-antiphon-1")
				if e == nil || strings.Count(strings.ToLower(e.Text), "alleluia") != 3 {
					t.Errorf("Sunday antiphon: %+v", e)
				}
			}
		}
	}
}
