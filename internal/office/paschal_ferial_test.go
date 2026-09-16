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

func TestPaschalFerialCanticleAntiphons(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	// Diurnal pp.46,52,57,63,69,75: the canticle's own antiphon separates
	// the first psalm group and Laudate, each framed by the shared Alleluia.
	want := []string{"Thine anger", "Be thou ready", "The Lord shall judge", "I will sing", "O Lord,", "Ascribe ye greatness"}
	canticles := []string{"isaiah-12", "isaiah-38", "hannah", "exodus-15", "habakkuk-3", "deuteronomy-32"}
	for i := 0; i < 6; i++ {
		for _, form := range models.PrayerForms {
			for _, simple := range []bool{false, true} {
				day := &models.CalendarDay{Date: time.Date(2026, 5, 4+i, 0, 0, 0, 0, time.UTC), Season: models.Easter}
				if simple {
					day.Celebration = &models.Feast{ID: "test-simple", Rank: models.Simple, Category: models.CategoryConfessor}
				}
				h, err := engine.ComposeHourWithOptions("lauds", day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var ants []models.OfficeElement
				foundCanticle := false
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						if e.Type == models.Antiphon && strings.HasPrefix(e.SlotRef, "psalm-antiphon-") {
							ants = append(ants, e)
						}
						foundCanticle = foundCanticle || e.SourceRef == "canticles/"+canticles[i]
					}
				}
				if len(ants) != 6 || !foundCanticle {
					t.Fatalf("%d/%s/simple=%v: %d ants, canticle %v", i, form, simple, len(ants), foundCanticle)
				}
				slot := "psalm-antiphon-4"
				if i == 5 {
					slot = "psalm-antiphon-3"
				}
				key := "seasonal/easter/" + slot + "-lauds-" + strings.ToLower(day.Date.Weekday().String())
				for j, e := range ants {
					if j == 2 || j == 3 {
						if !strings.HasPrefix(e.Text, want[i]) || !strings.Contains(strings.ToLower(e.Text), "alleluia") || e.SourceRef != key || !slices.Equal(e.SourceRefs, []string{key}) {
							t.Errorf("%d/%s ant %d: %+v", i, form, j, e)
						}
						trace := traceProperResolution(day, "lauds", e.SlotRef, e.SourceRef, engine.corpus)
						if trace.SelectedTier != "seasonal" || trace.SelectedRef != key {
							t.Errorf("trace %+v", trace)
						}
					} else if e.Text != engine.corpus.Get("shared/formulas/paschal-psalm-antiphon") {
						t.Errorf("first/last group lost Alleluia: %+v", e)
					}
				}
			}
		}
	}
}

func TestSeasonalWeekdayAppointmentPrecedence(t *testing.T) {
	day := &models.CalendarDay{Date: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), Season: models.Easter, Celebration: &models.Feast{ID: "test", Rank: models.GreaterDouble, Category: models.CategoryConfessor}}
	for _, tc := range []struct{ name, key, body, want string }{
		{"weekday", "", "", "Weekday"},
		{"proper", "proper/test/psalm-antiphon-4", "Proper", "Proper"},
		{"common", "commons/confessor/psalm-antiphon-4", "Common", "Common"},
		{"proper omission", "proper/test/psalm-antiphon-4", texts.OmitMarker, texts.OmitMarker},
		{"weekday omission", "seasonal/easter/psalm-antiphon-4-lauds-monday", texts.OmitMarker, texts.OmitMarker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := map[string]string{"seasonal/easter/psalm-antiphon-4-lauds-monday": "Weekday", "seasonal/easter/psalm-antiphon-4-lauds": "Hour", "seasonal/easter/psalm-antiphon-4": "Generic"}
			if tc.key != "" {
				entries[tc.key] = tc.body
			}
			corpus := texts.NewTestCorpus(entries)
			if got, _ := resolveProperText(day, "lauds", "psalm-antiphon-4", corpus); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
			for _, hour := range []string{"prime", "terce", "sext", "none", "vespers"} {
				if got, _ := resolveProperText(day, hour, "psalm-antiphon-4", texts.NewTestCorpus(map[string]string{"seasonal/easter/psalm-antiphon-4-lauds-monday": "Weekday", "seasonal/easter/psalm-antiphon-4": "Generic"})); got != "Generic" {
					t.Errorf("Lauds leaked into %s: %q", hour, got)
				}
			}
		})
	}
	corpus := texts.NewTestCorpus(map[string]string{"seasonal/easter/psalm-antiphon-4-lauds-monday": "Weekday", "seasonal/easter/psalm-antiphon-4-lauds": "Hour", "ordinary/lauds/psalm-antiphon-4-monday": "Ordinary"})
	day.Date = day.Date.AddDate(0, 0, 1)
	if got, _ := resolveProperText(day, "lauds", "psalm-antiphon-4", corpus); got != "Hour" {
		t.Errorf("wrong weekday: %q", got)
	}
	day.Date = day.Date.AddDate(0, 0, -1)
	day.Season = models.Pentecost
	if got, _ := resolveProperText(day, "lauds", "psalm-antiphon-4", corpus); got != "Ordinary" {
		t.Errorf("wrong season: %q", got)
	}
}

func TestPaschalSaturdayBVMRetainsSaturdayPsalmody(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	// Diurnal pp.68*,71*,79 and current ordo p.63 (May16).
	for _, date := range []time.Time{time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC), time.Date(2027, 5, 15, 0, 0, 0, 0, time.UTC)} {
		days, err := calendar.BuildCalendar(date.Year(), "../../data")
		if err != nil {
			t.Fatal(err)
		}
		var actual *models.CalendarDay
		for i := range days {
			if days[i].Date.Equal(date) {
				actual = &days[i]
				break
			}
		}
		if actual == nil || actual.Celebration == nil || actual.Celebration.ID != saturdayOfficeBVMID {
			t.Fatalf("expected actual Saturday BVM on %s", date)
		}
		for _, season := range []models.Season{models.Easter, models.Pentecost} {
			for _, form := range models.PrayerForms {
				day := &models.CalendarDay{Date: date, Season: season, Celebration: &models.Feast{ID: saturdayOfficeBVMID, Rank: models.Simple, Category: models.CategoryBlessedVirgin}}
				if season == models.Easter {
					day = actual
				}
				h, err := engine.ComposeHourWithOptions("lauds", day, calendar.ComputeMoveableDates(date.Year()), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var psalms []string
				cant := 0
				glorias := 0
				var ants []models.OfficeElement
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						if e.Type == models.Psalm {
							psalms = append(psalms, e.SourceRef)
						}
						if e.SourceRef == "canticles/sirach-36" {
							cant++
						}
						if e.SourceRef == "ordinary/shared/gloria-patri" {
							glorias++
						}
						if strings.HasPrefix(e.SlotRef, "saturday-psalm-antiphon-") {
							ants = append(ants, e)
						}
					}
				}
				if !slices.Equal(psalms, []string{"psalms/067", "psalms/051", "psalms/143a", "psalms/143b", "psalms/148", "psalms/149", "psalms/150"}) || cant != 1 || glorias != 7 {
					t.Fatalf("%s/%s: %v cant %d glorias %d", season, form, psalms, cant, glorias)
				}
				if season == models.Easter {
					if len(ants) != 6 {
						t.Fatalf("Paschal antiphon groups: %+v", ants)
					}
					for _, i := range []int{2, 3} {
						if ants[i].SourceRef != "proper/saturday-office-bvm/saturday-psalm-antiphon-3-easter" || !slices.Equal(ants[i].SourceRefs, []string{"ordinary/lauds/festal-canticle-antiphon-saturday-easter"}) {
							t.Errorf("canticle antiphon: %+v", ants[i])
						}
					}
				} else {
					if len(ants) != 8 {
						t.Errorf("per-annum antiphon count %d", len(ants))
					}
					for _, e := range ants {
						if e.SlotRef == "saturday-psalm-antiphon-3" || e.SlotRef == "saturday-psalm-antiphon-4" {
							key := "proper/saturday-office-bvm/" + e.SlotRef
							if e.SourceRef != key || e.Text != engine.corpus.Get(key) {
								t.Errorf("non-Paschal antiphon changed: %+v", e)
							}
						}
					}
				}
			}
		}
	}
}
