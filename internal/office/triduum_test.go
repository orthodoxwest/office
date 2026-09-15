package office

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// TestTriduumLittleHours covers the Little Hours rubric printed at Monastic
// Diurnal p. 314: the Hour begins at once with a fixed psalmody, without the
// opening versicles, hymn, chapter or Gloria Patri, and ends at the antiphon
// Christ, for our sake with the Our Father, Psalm 51 and the collect of the day.
func TestTriduumLittleHours(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	days, err := calendar.BuildCalendar(2026, dataDir)
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	engine, err := NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)

	byDate := make(map[string]*models.CalendarDay, len(days))
	for i := range days {
		byDate[days[i].Date.Format("2006-01-02")] = &days[i]
	}

	// Easter 2026 falls on April 12 by the Julian paschalion.
	triduum := map[string]string{
		"2026-04-09": "Christ, ✠ for our sake, became obedient unto death.",
		"2026-04-10": "Christ, ✠ for our sake, became obedient unto death, even the death of the cross.",
		"2026-04-11": "Christ, ✠ for our sake, became obedient unto death, even the death of the cross. Wherefore God also hath highly exalted him, and given him a Name which is above every name.",
	}
	psalmody := map[string][]string{
		"prime": {"psalms/054", "psalms/119-i", "psalms/119-ii", "psalms/119-iii", "psalms/119-iv"},
		"terce": {"psalms/119-v", "psalms/119-vi", "psalms/119-vii", "psalms/119-viii", "psalms/119-ix", "psalms/119-x"},
		"sext":  {"psalms/119-xi", "psalms/119-xii", "psalms/119-xiii", "psalms/119-xiv", "psalms/119-xv", "psalms/119-xvi"},
		"none":  {"psalms/119-xvii", "psalms/119-xviii", "psalms/119-xix", "psalms/119-xx", "psalms/119-xxi", "psalms/119-xxii"},
	}

	for date, wantAntiphon := range triduum {
		for _, hourName := range []string{"prime", "terce", "sext", "none"} {
			t.Run(date+"/"+hourName, func(t *testing.T) {
				day := byDate[date]
				if day == nil {
					t.Fatalf("calendar day %s not found", date)
				}
				hour, err := engine.ComposeHour(hourName, day, moveable)
				if err != nil {
					t.Fatalf("ComposeHour(%s): %v", hourName, err)
				}

				var psalms []string
				var antiphons []string
				var sawHymn, sawChapter, sawDoxology bool
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						switch elem.Type {
						case models.Psalm:
							psalms = append(psalms, elem.SourceRef)
						case models.Antiphon:
							antiphons = append(antiphons, elem.Text)
						case models.Hymn:
							sawHymn = true
						case models.Chapter:
							sawChapter = true
						case models.PsalmDoxology:
							sawDoxology = true
						}
					}
				}

				want := append(append([]string{}, psalmody[hourName]...), "psalms/051")
				if strings.Join(psalms, " ") != strings.Join(want, " ") {
					t.Errorf("psalms = %v, want %v", psalms, want)
				}
				if len(antiphons) != 1 || antiphons[0] != wantAntiphon {
					t.Errorf("antiphons = %q, want the single antiphon %q", antiphons, wantAntiphon)
				}
				if sawHymn {
					t.Error("hymn rendered; the Triduum Hours omit it (p. 314)")
				}
				if sawChapter {
					t.Error("chapter rendered; the Triduum Hours omit it (p. 314)")
				}
				if sawDoxology {
					t.Error("Gloria Patri rendered; it is not said during the Triduum (p. 311)")
				}
			})
		}
	}

	t.Run("Lauds and Vespers say no psalm doxology", func(t *testing.T) {
		// p. 311 prints the Triduum psalms with the antiphon following straight
		// on from the last verse. Holy Saturday evening is excluded: its
		// Vespers belongs to Easter, which says the Gloria Patri again.
		for _, date := range []string{"2026-04-09", "2026-04-10"} {
			for _, hourName := range []string{"lauds", "vespers", "compline"} {
				hour, err := engine.ComposeHour(hourName, byDate[date], moveable)
				if err != nil {
					t.Fatalf("ComposeHour(%s, %s): %v", hourName, date, err)
				}
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						if elem.Type == models.PsalmDoxology {
							t.Errorf("%s %s: psalm doxology rendered, want none", date, hourName)
						}
					}
				}
			}
		}
	})

	t.Run("ordinary days keep their weekday form", func(t *testing.T) {
		day := byDate["2026-04-08"] // Wednesday in Holy Week, outside the Triduum
		hour, err := engine.ComposeHour("terce", day, moveable)
		if err != nil {
			t.Fatalf("ComposeHour(terce): %v", err)
		}
		var sawHymn, sawDoxology bool
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				switch elem.Type {
				case models.Hymn:
					sawHymn = true
				case models.PsalmDoxology:
					sawDoxology = true
				}
			}
		}
		if !sawHymn || !sawDoxology {
			t.Errorf("Wednesday in Holy Week: hymn=%v doxology=%v, want both", sawHymn, sawDoxology)
		}

		// Easter's I Vespers on Holy Saturday evening is outside the Triduum
		// office even though the civil day is in it.
		easterEve, err := engine.ComposeHour("vespers", byDate["2026-04-11"], moveable)
		if err != nil {
			t.Fatalf("ComposeHour(vespers, 2026-04-11): %v", err)
		}
		var sawEasterDoxology bool
		for _, section := range easterEve.Sections {
			for _, elem := range section.Elements {
				if elem.Type == models.PsalmDoxology {
					sawEasterDoxology = true
				}
			}
		}
		if !sawEasterDoxology {
			t.Error("Holy Saturday Vespers (Easter I Vespers) says no psalm doxology, want one")
		}
	})
}

// TestTriduumMajorHoursOpeningsEndings checks the actual sequence, rather than
// the presence of an antiphon or Psalm 51 (which also opens Lauds). Sources:
// Diurnal pp. 305, 313-316; 2026 ordo Sacred Triduum notes and April 8-11.
func TestTriduumMajorHoursOpeningsEndings(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	moveable := calendar.ComputeMoveableDates(2026)
	for _, date := range []string{"2026-04-09", "2026-04-10", "2026-04-11"} {
		d, err := time.Parse(time.DateOnly, date)
		if err != nil {
			t.Fatal(err)
		}
		day := &days[d.YearDay()-1]
		for _, name := range hourNames {
			if date == "2026-04-11" && (name == "vespers" || name == "compline") {
				continue
			}
			for _, form := range models.PrayerForms {
				t.Run(date+"/"+name+"/"+string(form), func(t *testing.T) {
					hour, err := engine.ComposeHourWithOptions(name, day, moveable, ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					var elements []models.OfficeElement
					for _, section := range hour.Sections {
						if !section.Collapsible {
							elements = append(elements, section.Elements...)
						}
					}
					antiphon := -1
					noise := 0
					for i, elem := range elements {
						switch elem.Type {
						case models.Hymn, models.Chapter, models.ShortResponsory, models.PsalmDoxology, models.OpeningAcclamation, models.CorporateLordPrayer, models.Preces:
							t.Errorf("unexpected %s: %s", elem.Type, elem.SourceRef)
						}
						if elem.IsCommemoration || elem.LeaderSlot == "greeting" || strings.HasPrefix(elem.SourceRef, "ordinary/marian/") || elem.SourceRef == "shared/leader/let-us-pray" {
							t.Errorf("unexpected ordinary ending: %s", elem.SourceRef)
						}
						if elem.SourceRef == "proper/"+day.Celebration.ID+"/triduum-antiphon" {
							if antiphon >= 0 {
								t.Error("duplicate Christus factus est")
							}
							antiphon = i
						}
						if elem.SourceRef == "shared/formulas/triduum-lauds-noise-rubric" {
							noise++
						}
					}
					if antiphon < 0 {
						t.Fatal("missing Christus factus est")
					}
					var ending []models.OfficeElement
					for _, elem := range elements[antiphon+1:] {
						if elem.Type != models.Rubric {
							ending = append(ending, elem)
						}
					}
					if len(ending) != 3 {
						t.Fatalf("ending has %d prayers, want Our Father, Miserere, collect", len(ending))
					}
					if ending[0].SourceRef != "ordinary/shared/our-father" || ending[1].SourceRef != "psalms/051" || ending[2].SourceRef != "proper/"+day.Celebration.ID+"/collect" {
						t.Fatalf("wrong ending sources: %s / %s / %s", ending[0].SourceRef, ending[1].SourceRef, ending[2].SourceRef)
					}
					assertVoice(t, ending[0].Voice, []models.VoiceSpan{{Text: ending[0].Text, Spoken: false}})
					collect := ending[2]
					if !strings.HasPrefix(collect.Text, "Almighty God, we beseech thee graciously to behold this thy family") {
						t.Errorf("wrong Triduum collect: %s", collect.Text)
					}
					if len(collect.Voice) != 2 {
						t.Fatalf("collect has no spoken/silent partition: %+v", collect.Voice)
					}
					if !collect.Voice[0].Spoken || collect.Voice[1].Spoken || !strings.HasPrefix(collect.Voice[1].Text, "Who with thee") || collect.Voice[0].Text+collect.Voice[1].Text != collect.Text {
						t.Errorf("incorrect collect delivery: %+v", collect.Voice)
					}
					if strings.Contains(collect.Text, "R. Amen") {
						t.Error("silent conclusion presents a congregational response")
					}
					if !slices.Contains(collect.SourceRefs, "shared/formulas/collect-conclusion-who-liveth") {
						t.Errorf("missing conclusion provenance: %v", collect.SourceRefs)
					}
					if name == "lauds" || name == "vespers" {
						first := elements[0]
						if first.Type != models.Antiphon {
							t.Errorf("first element is %s (%s), want first antiphon", first.Type, first.SourceRef)
						}
						if name == "lauds" && (elements[1].SourceRef != "psalms/051" || noise != 1) {
							t.Errorf("Lauds first psalm=%s, noise rubrics=%d", elements[1].SourceRef, noise)
						}
					}
					if name != "lauds" && noise != 0 {
						t.Error("Lauds candle/noise rubric at another hour")
					}
					if name == "compline" {
						var before []models.OfficeElement
						for _, elem := range elements[:antiphon] {
							if elem.Type != models.Rubric {
								before = append(before, elem)
							}
						}
						if len(before) < 5 || before[0].LeaderSlot != "confession" {
							t.Fatal("Compline does not begin with confession")
						}
						last := before[len(before)-4:]
						want := []string{"psalms/004", "psalms/091", "psalms/134", "canticles/nunc-dimittis"}
						for i, elem := range last {
							if elem.SourceRef != want[i] {
								t.Errorf("Compline psalm/canticle %d=%s, want %s", i, elem.SourceRef, want[i])
							}
						}
						for _, elem := range before[:len(before)-4] {
							if elem.LeaderSlot != "confession" {
								t.Errorf("extra Compline opening: %s", elem.SourceRef)
							}
						}
					}
				})
			}
		}
	}
	// Opening/ending omissions stop at None on Saturday and do not reach back
	// into Wednesday. Check both civil-day Compline and Easter-owned Vespers.
	for _, tc := range []struct{ date, name string }{
		{"2026-04-08", "lauds"}, {"2026-04-08", "vespers"}, {"2026-04-08", "compline"},
		{"2026-04-11", "vespers"}, {"2026-04-11", "compline"},
		{"2026-04-12", "lauds"}, {"2026-04-12", "compline"},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.name+"/"+string(form)+"/boundary", func(t *testing.T) {
				d, _ := time.Parse(time.DateOnly, tc.date)
				hour, err := engine.ComposeHourWithOptions(tc.name, &days[d.YearDay()-1], moveable, ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var opening, doxology, marian, nunc bool
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						if strings.Contains(elem.SourceRef, "triduum") || elem.SlotRef == "triduum-antiphon" {
							t.Errorf("Triduum element beyond boundary: %s", elem.SourceRef)
						}
						opening = opening || strings.HasSuffix(elem.SourceRef, "/opening-versicle") || elem.LeaderSlot == "opening"
						doxology = doxology || elem.Type == models.PsalmDoxology
						marian = marian || strings.HasPrefix(elem.SourceRef, "ordinary/marian/")
						nunc = nunc || elem.SourceRef == "proper/holy-saturday/nunc-dimittis-antiphon"
						if elem.Type == models.Collect && len(elem.Voice) > 0 {
							t.Error("ordinary collect acquired silent conclusion")
						}
					}
				}
				if tc.date == "2026-04-11" && tc.name == "vespers" {
					if opening || !doxology || marian {
						t.Errorf("Vigil opening=%v, doxology=%v, Marian=%v", opening, doxology, marian)
					}
				} else if !opening || !doxology || !marian {
					t.Errorf("ordinary opening=%v, doxology=%v, Marian=%v", opening, doxology, marian)
				}
				if tc.date == "2026-04-11" && tc.name == "compline" && !nunc {
					t.Error("Holy Saturday Compline lost proper Nunc dimittis antiphon")
				}
			})
		}
	}
}

// Diurnal pp. 305–311, 336–341 and 356–359 give the Lauds frames;
// pp. 315–316 give the same five Vespers frames on Thursday and Friday.
// Psalm 51 appears again only in the common ending (p. 313).
func TestTriduumPsalmodyAppointments(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		offset int
		hour   string
		refs   []string
	}{
		{-3, "lauds", []string{"psalms/051", "psalms/090", "psalms/036", "canticles/exodus-15", "psalms/147a", "canticles/benedictus", "psalms/051"}},
		{-2, "lauds", []string{"psalms/051", "psalms/143", "psalms/085", "canticles/habakkuk-3", "psalms/147b", "canticles/benedictus", "psalms/051"}},
		{-1, "lauds", []string{"psalms/051", "psalms/092", "psalms/064", "canticles/isaiah-38", "psalms/150", "canticles/benedictus", "psalms/051"}},
		{-3, "vespers", []string{"psalms/116b", "psalms/120", "psalms/140", "psalms/141", "psalms/142", "canticles/magnificat", "psalms/051"}},
		{-2, "vespers", []string{"psalms/116b", "psalms/120", "psalms/140", "psalms/141", "psalms/142", "canticles/magnificat", "psalms/051"}},
	}
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		dates := calendar.ComputeMoveableDates(year)
		for _, tc := range cases {
			date := dates.Easter.AddDate(0, 0, tc.offset)
			day := &days[date.YearDay()-1]
			t.Run(date.Format(time.DateOnly)+"/"+tc.hour, func(t *testing.T) {
				hour, err := engine.ComposeHour(tc.hour, day, dates)
				if err != nil {
					t.Fatal(err)
				}
				var refs []string
				var psalmody []models.OfficeElement
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						if elem.Type == models.Psalm || elem.Type == models.Canticle {
							refs = append(refs, elem.SourceRef)
						}
						if elem.Type != models.Rubric && !section.Collapsible {
							psalmody = append(psalmody, elem)
						}
					}
				}
				if !slices.Equal(refs, tc.refs) {
					t.Fatalf("psalms/canticles=%v, want %v", refs, tc.refs)
				}
				for i := 0; i < 5; i++ {
					before, after := psalmody[i*3], psalmody[i*3+2]
					if before.Type != models.Antiphon || after.Type != models.Antiphon || before.SourceRef != after.SourceRef {
						t.Fatalf("psalmody frame %d is not bounded by its own antiphon", i+1)
					}
					slot := fmt.Sprintf("psalm-antiphon-%d", i+1)
					expected := "proper/" + day.Celebration.ID + "/" + slot
					if tc.hour == "vespers" {
						expected += "-vespers"
					}
					if before.SourceRef != expected {
						t.Errorf("frame %d antiphon=%s, want %s", i+1, before.SourceRef, expected)
					}
				}
			})
		}
	}
}
