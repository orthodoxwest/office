package office

import (
	"path/filepath"
	"strings"
	"testing"

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

// TestTriduumMajorHoursOpeningsEndings covers issue #276: Lauds and Vespers
// omit the opening versicles and close with the shared Christus factus est
// ending (p. 313), and Maundy Thursday/Good Friday Compline begins with the
// Confession and takes the Nunc dimittis and ending (p. 316). Holy Saturday
// Vespers (Easter) and Compline (Easter-Eve) are preserved.
func TestTriduumMajorHoursOpeningsEndings(t *testing.T) {
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
	wantAntiphon := map[string]string{
		"2026-04-09": "Christ, ✠ for our sake, became obedient unto death.",
		"2026-04-10": "Christ, ✠ for our sake, became obedient unto death, even the death of the cross.",
		"2026-04-11": "Christ, ✠ for our sake, became obedient unto death, even the death of the cross. Wherefore God also hath highly exalted him, and given him a Name which is above every name.",
	}

	for _, date := range []string{"2026-04-09", "2026-04-10"} {
		for _, hourName := range []string{"lauds", "vespers"} {
			t.Run(date+"/"+hourName, func(t *testing.T) {
				hour, err := engine.ComposeHour(hourName, byDate[date], moveable)
				if err != nil {
					t.Fatalf("ComposeHour: %v", err)
				}
				var sawOpening, sawHymn, sawChapter, sawMarian bool
				var sawTriduum, sawPs51 bool
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						for _, ref := range elem.SourceRefs {
							if strings.HasSuffix(ref, "/opening-versicle") {
								sawOpening = true
							}
							if strings.HasPrefix(ref, "ordinary/marian/") {
								sawMarian = true
							}
						}
						switch elem.Type {
						case models.Hymn:
							sawHymn = true
						case models.Chapter:
							sawChapter = true
						case models.Antiphon:
							if elem.Text == wantAntiphon[date] {
								sawTriduum = true
							}
						case models.Psalm:
							if elem.SourceRef == "psalms/051" {
								sawPs51 = true
							}
						}
					}
				}
				if sawOpening {
					t.Error("opening versicles rendered; Triduum begins with the first antiphon")
				}
				if sawHymn || sawChapter {
					t.Errorf("hymn=%v chapter=%v; the Triduum omits them", sawHymn, sawChapter)
				}
				if !sawTriduum {
					t.Errorf("missing Triduum antiphon %q", wantAntiphon[date])
				}
				if !sawPs51 {
					t.Error("missing Psalm 51 of the Triduum ending")
				}
				if sawMarian {
					t.Error("Marian antiphon rendered; the Triduum ends at the collect")
				}
			})
		}
		t.Run(date+"/compline", func(t *testing.T) {
			hour, err := engine.ComposeHour("compline", byDate[date], moveable)
			if err != nil {
				t.Fatalf("ComposeHour: %v", err)
			}
			var sawBlessing, sawConfession, sawNunc, sawTriduum, sawHymn bool
			var psalms []string
			for _, section := range hour.Sections {
				for _, elem := range section.Elements {
					for _, ref := range elem.SourceRefs {
						if ref == "ordinary/compline/opening-versicle" {
							sawBlessing = true
						}
						if ref == "ordinary/shared/confiteor" {
							sawConfession = true
						}
					}
					switch elem.Type {
					case models.Hymn:
						sawHymn = true
					case models.Canticle:
						if elem.SourceRef == "canticles/nunc-dimittis" {
							sawNunc = true
						}
					case models.Antiphon:
						if elem.Text == wantAntiphon[date] {
							sawTriduum = true
						}
					case models.Psalm:
						psalms = append(psalms, elem.SourceRef)
					}
				}
			}
			if sawBlessing {
				t.Error("Compline blessing rendered; p. 316 omits it")
			}
			if !sawConfession {
				t.Error("missing Confession at Triduum Compline")
			}
			for _, want := range []string{"psalms/004", "psalms/091", "psalms/134", "psalms/051"} {
				found := false
				for _, got := range psalms {
					if got == want {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %s at Triduum Compline", want)
				}
			}
			if !sawNunc {
				t.Error("missing Nunc dimittis at Triduum Compline")
			}
			if !sawTriduum {
				t.Errorf("missing Triduum antiphon %q", wantAntiphon[date])
			}
			if sawHymn {
				t.Error("hymn rendered; the Triduum omits it")
			}
		})
	}

	t.Run("2026-04-11/lauds keeps the ending", func(t *testing.T) {
		hour, err := engine.ComposeHour("lauds", byDate["2026-04-11"], moveable)
		if err != nil {
			t.Fatalf("ComposeHour: %v", err)
		}
		var sawTriduum bool
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.Type == models.Antiphon && elem.Text == wantAntiphon["2026-04-11"] {
					sawTriduum = true
				}
			}
		}
		if !sawTriduum {
			t.Error("Holy Saturday Lauds missing the Triduum ending")
		}
	})

	t.Run("2026-04-11/vespers stays Easter", func(t *testing.T) {
		hour, err := engine.ComposeHour("vespers", byDate["2026-04-11"], moveable)
		if err != nil {
			t.Fatalf("ComposeHour: %v", err)
		}
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.Type == models.Antiphon && elem.Text == wantAntiphon["2026-04-11"] {
					t.Error("Holy Saturday Vespers (Easter) renders the Triduum antiphon")
				}
			}
		}
	})

	t.Run("2026-04-11/compline stays Easter-Eve", func(t *testing.T) {
		hour, err := engine.ComposeHour("compline", byDate["2026-04-11"], moveable)
		if err != nil {
			t.Fatalf("ComposeHour: %v", err)
		}
		var sawTriduum, sawNuncAntiphon bool
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.Type == models.Antiphon && elem.Text == wantAntiphon["2026-04-11"] {
					sawTriduum = true
				}
				for _, ref := range elem.SourceRefs {
					if ref == "proper/holy-saturday/nunc-dimittis-antiphon" {
						sawNuncAntiphon = true
					}
				}
			}
		}
		if sawTriduum {
			t.Error("Holy Saturday Compline renders the Triduum antiphon")
		}
		if !sawNuncAntiphon {
			t.Error("Holy Saturday Compline lost its Nunc dimittis antiphon")
		}
	})
}
