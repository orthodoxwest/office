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

func TestWeekdayFestalLaudsAppointments(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	// Diurnal pp.44–79. The order includes Psalm67 and the joined Laudate group.
	psalms := [][]string{{"067", "051", "005", "036"}, {"067", "051", "043", "057"}, {"067", "051", "064", "065"}, {"067", "051", "088", "090"}, {"067", "051", "076", "092"}, {"067", "051", "143a", "143b"}}
	canticles := []string{"1-chronicles-29", "tobit-13", "judith-16", "jeremiah-31", "isaiah-45", "sirach-36"}
	ants := []string{"We praise", "Extol", "O Lord, thou art great", "My people", "In the Lord", "Shew us"}
	for i := 0; i < 6; i++ {
		for _, season := range []models.Season{models.Pentecost, models.Easter} {
			for _, form := range models.PrayerForms {
				day := &models.CalendarDay{Date: time.Date(2026, 9, 14+i, 0, 0, 0, 0, time.UTC), Season: season, Celebration: &models.Feast{ID: "test-lesser-double", Rank: models.Double, Category: models.CategoryConfessor}}
				h, err := engine.ComposeHourWithOptions("lauds", day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				cantCount := 0
				antCount := 0
				gloriaCount := 0
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						if e.Type == models.Psalm {
							got = append(got, strings.TrimPrefix(e.SourceRef, "psalms/"))
						}
						if e.SourceRef == "canticles/"+canticles[i] {
							cantCount++
						}
						if e.Type == models.Antiphon && strings.HasPrefix(e.Text, ants[i]) {
							wantRef := "ordinary/lauds/festal-canticle-antiphon-" + strings.ToLower(day.Date.Weekday().String())
							if season == models.Easter {
								wantRef += "-easter"
							}
							trace := traceProperResolution(day, "lauds", e.SlotRef, e.SourceRef, engine.corpus)
							if e.SourceRef != wantRef || trace.Reason != "festal-weekday-canticle" {
								t.Errorf("canticle source/trace: %s %+v", e.SourceRef, trace)
							}
							antCount++
							if strings.Contains(e.Text, "Alleluia") != (season == models.Easter) {
								t.Errorf("canticle antiphon season: %+v", e)
							}
						}
						if e.SourceRef == "ordinary/shared/gloria-patri" {
							gloriaCount++
						}
					}
				}
				want := append(slices.Clone(psalms[i]), "148", "149", "150")
				if !slices.Equal(got, want) || cantCount != 1 || antCount != 2 || gloriaCount != 7 {
					t.Errorf("%d/%s/%s: psalms %v, canticle %d, antiphons %d, Glorias %d", i, season, form, got, cantCount, antCount, gloriaCount)
				}
				// The Common still supplies everything from the chapter onward.
				if e := officeElementBySlot(h, "chapter"); e == nil || !strings.HasPrefix(e.SourceRef, "commons/confessor") {
					t.Errorf("chapter lost Common: %+v", e)
				}
				// Prime and the other little hours keep their Feast/Common antiphons.
				for _, hour := range []string{"prime", "terce", "sext", "none"} {
					minor, err := engine.ComposeHour(hour, day, calendar.ComputeMoveableDates(2026))
					if err != nil {
						t.Fatal(err)
					}
					for _, s := range minor.Sections {
						for _, e := range s.Elements {
							if strings.HasPrefix(e.SlotRef, "psalm-antiphon-") && !strings.HasPrefix(e.SourceRef, "commons/confessor") {
								t.Errorf("%s lost Common antiphon: %+v", hour, e)
							}
						}
					}
				}
			}
		}
	}
}

func TestWeekdayLaudsPsalmodyBoundaries(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{"proper/has-ants/psalm-antiphon-1": "Proper antiphon", "proper/omitted/psalm-antiphon-1": texts.OmitMarker})
	for _, tc := range []struct {
		name, id             string
		rank                 models.Rank
		category             models.FeastCategory
		weekday              time.Weekday
		weekdays, festalCant bool
	}{
		{"lesser Double", "plain", models.Double, models.CategoryConfessor, time.Monday, true, true},
		{"proper antiphons", "has-ants", models.Double, models.CategoryConfessor, time.Monday, false, false},
		{"omitted is absent", "omitted", models.Double, models.CategoryConfessor, time.Monday, true, true},
		{"Greater Double", "plain", models.GreaterDouble, models.CategoryConfessor, time.Monday, false, false},
		{"within octave office", "octave-day-2", models.SemiDouble, models.CategoryApostle, time.Monday, false, false},
		{"simple octave day", "octave-day-example", models.Simple, models.CategoryApostle, time.Monday, true, true},
		{"ordinary simple", "plain", models.Simple, models.CategoryConfessor, time.Monday, true, false},
		{"ferial vigil", "vigil", models.Simple, models.CategoryFeria, time.Monday, false, false},
		{"Saturday BVM separate", "saturday-office-bvm", models.Simple, models.CategoryBlessedVirgin, time.Saturday, false, false},
		{"anticipated Sunday", "anticipated-sunday", models.SemiDouble, models.CategorySunday, time.Saturday, true, true},
		{"ordinary Sunday", "sunday", models.SemiDouble, models.CategorySunday, time.Sunday, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: time.Date(2026, 9, 13+int(tc.weekday), 0, 0, 0, 0, time.UTC), Celebration: &models.Feast{ID: tc.id, Rank: tc.rank, Category: tc.category}}
			for _, octave := range []string{"", "epiphany"} {
				day.WithinOctaveOf = octave
				if got := usesWeekdayLaudsPsalmody(day, corpus); got != tc.weekdays {
					t.Errorf("weekday=%v, want %v (octave=%s)", got, tc.weekdays, octave)
				}
				if got := usesFestalWeekdayLaudsCanticle(day, corpus); got != tc.festalCant {
					t.Errorf("canticle=%v, want %v", got, tc.festalCant)
				}
			}
		})
	}
}

func TestActualWeekdayLaudsAppointments(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		for i := range days {
			day := &days[i]
			if !usesFestalWeekdayLaudsCanticle(day, engine.corpus) {
				continue
			}
			for _, form := range models.PrayerForms {
				h, err := engine.ComposeHourWithOptions("lauds", day, calendar.ComputeMoveableDates(year), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				want := map[time.Weekday]string{time.Monday: "1-chronicles-29", time.Tuesday: "tobit-13", time.Wednesday: "judith-16", time.Thursday: "jeremiah-31", time.Friday: "isaiah-45", time.Saturday: "sirach-36"}[day.Date.Weekday()]
				found := false
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						found = found || e.SourceRef == "canticles/"+want
						if e.SourceRef == "canticles/benedicite" {
							t.Errorf("%s/%s retained Sunday canticle", day.Date, form)
						}
					}
				}
				if !found {
					t.Errorf("%s/%s missing %s", day.Date, form, want)
				}
			}
		}
	}

}

// These appointments are asserted independently of the selection helper:
// 2026 ordo pp.25,29–30,74,89,129; held conflicts pp.59 and121.
func Test2026SourceBackedWeekdayCanticles(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"2026-01-02": "isaiah-45", "2026-01-03": "sirach-36",
		"2026-01-14": "judith-16", "2026-01-17": "sirach-36",
		"2026-01-20": "tobit-13", "2026-06-22": "1-chronicles-29",
		"2026-08-13": "jeremiah-31", "2026-12-31": "jeremiah-31",
		"2026-05-02": "benedicite", "2026-12-02": "benedicite",
	}
	for _, day := range days {
		want, ok := cases[day.Date.Format("2006-01-02")]
		if !ok {
			continue
		}
		h, err := engine.ComposeHour("lauds", &day, calendar.ComputeMoveableDates(2026))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, section := range h.Sections {
			for _, e := range section.Elements {
				if e.Type == models.Canticle && e.SourceRef != "canticles/benedictus" {
					got = append(got, e.SourceRef)
				}
			}
		}
		if !slices.Equal(got, []string{"canticles/" + want}) {
			t.Errorf("%s: %v, want %s", day.Date, got, want)
		}
	}
}

func TestWeekdayLaudsProperBoundaries(t *testing.T) {
	day := &models.CalendarDay{Date: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), Season: models.Easter, Celebration: &models.Feast{ID: "test", ProperID: "shared", Rank: models.Double, Category: models.CategoryConfessor}}
	for _, key := range []string{
		"proper/shared/psalm-antiphon-1", "proper/shared/psalm-antiphon-1-lauds",
		"proper/test-paschal/psalm-antiphon-1", "proper/test/psalm-antiphon-1-lauds-easter",
		"proper/test/psalm-antiphon", "proper/shared/lauds-psalmody",
	} {
		body := "Proper antiphon"
		if strings.HasSuffix(key, "lauds-psalmody") {
			body = "festal"
		}
		corpus := texts.NewTestCorpus(map[string]string{key: body})
		if usesWeekdayLaudsPsalmody(day, corpus) {
			t.Errorf("ignored %s", key)
		}
	}
	corpus := texts.NewTestCorpus(map[string]string{"proper/test/psalm-antiphon-1-vespers": "Evening only", "commons/confessor/psalm-antiphon-1": "Common"})
	if !usesWeekdayLaudsPsalmody(day, corpus) {
		t.Error("Vespers/Common manufactured proper Lauds antiphons")
	}
	if usesWeekdayLaudsPsalmody(day, nil) {
		t.Error("nil corpus")
	}
}

func TestValidateLaudsPsalmodyDeclarations(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{"proper/test/lauds-psalmody": "festal", "psalmody/lauds/festal": "festal"})
	if errs := validateLaudsPsalmodyDeclarations(corpus); len(errs) != 0 {
		t.Fatal(errs)
	}
	corpus = texts.NewTestCorpus(map[string]string{"proper/test/lauds-psalmody": "festel", "psalmody/lauds/festal": ""})
	if errs := validateLaudsPsalmodyDeclarations(corpus); len(errs) != 2 {
		t.Fatalf("invalid declarations accepted: %v", errs)
	}
}

// The vigil's ProperID gives festal Ascension psalmody although its category
// is feria: do not pick the ordinary Saturday Laudate slot4. Diurnal pp.392,394;
// 2026 ordo p.67. This used to conflate is-feast with festal psalmody.
func TestPentecostVigilLaudateAntiphon(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		for _, day := range days {
			if day.Celebration == nil || day.Celebration.ID != "vigil-pentecost" {
				continue
			}
			h, err := engine.ComposeHour("lauds", &day, calendar.ComputeMoveableDates(year))
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, s := range h.Sections {
				for _, e := range s.Elements {
					if e.SlotRef == "psalm-antiphon-5" {
						count++
						if e.Text != engine.corpus.Get("proper/ascension/psalm-antiphon-5") {
							t.Errorf("%d: Laudate antiphon = %q", year, e.Text)
						}
					}
				}
			}
			if count != 2 {
				t.Errorf("%d: Laudate antiphon count %d", year, count)
			}
		}
	}
}
