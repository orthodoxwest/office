package e2e

import (
	"slices"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal pp. 417–421 and the 2026 ordo pp. 71–72 appoint the Sunday
// within the Corpus Christi octave. The little hours borrow the Lauds
// antiphons, but have Sunday chapters; I and II Vespers use different fourth
// psalms. II Vespers repeats the responsory printed at I Vespers (pp. 417–418).
func TestCorpusSundayAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "proper/pentecost-sunday-2/"
	vespersAntiphons := map[string]string{
		"psalm-antiphon-1": "Christ the Lord, a Priest for ever after the order of Melchisedech, offered bread and wine.",
		"psalm-antiphon-2": "The merciful and gracious Lord hath given meat unto them that fear him, in remembrance of his marvellous doings.",
		"psalm-antiphon-3": "I will receive the cup of salvation, and offer the sacrifice of thanksgiving.",
	}
	for _, fixture := range []struct {
		year        int
		eve, sunday string
	}{
		{2026, "2026-06-13", "2026-06-14"},
		// A further occurrence exercises the rule, not future-ordo parity.
		{2028, "2028-06-17", "2028-06-18"},
	} {
		yd := buildYear(t, fixture.year)
		for _, tc := range []struct {
			date, hour, chapter, verse, antiphon, hymn string
			psalms                                     []string
		}{
			{fixture.sunday, "prime", "", "", "Wisdom hath builded her house", "", []string{"119-i", "119-ii", "119-iii", "119-iv"}},
			{fixture.sunday, "terce", "Marvel not, my brethren", "V. He gave them bread from heaven, alleluia.\nR. So man did eat angels' food, alleluia.", "Thou feddest", "", []string{"119-v", "119-vi", "119-vii"}},
			{fixture.sunday, "sext", "Hereby perceive we the love of God", "V. He fed them with the finest wheat flour, alleluia.\nR. And with honey out of the stony rock did he satisfy them, alleluia.", "Rich is the bread of Christ", "", []string{"119-viii", "119-ix", "119-x"}},
			{fixture.sunday, "none", "My little children", "V. Thou bringest bread out of the earth, alleluia.\nR. And wine that maketh glad the heart of man, alleluia.", "To him that overcometh", "", []string{"119-xi", "119-xii", "119-xiii"}},
			{fixture.eve, "vespers", "Marvel not, my brethren", "V. He fed them with the finest wheat flour, alleluia.\nR. And with honey out of the stony rock did he satisfy them, alleluia.", "", "Now, my tongue", []string{"110", "111", "116b", "147b"}},
			{fixture.sunday, "vespers", "Marvel not, my brethren", "V. He fed them with the finest wheat flour, alleluia.\nR. And with honey out of the stony rock did he satisfy them, alleluia.", "", "Now, my tongue", []string{"110", "111", "116b", "128"}},
		} {
			for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
				t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
					d := dayFor(t, yd, tc.date)
					v, err := engine.ComposeHourWithOptions(tc.hour, d, yd.moveable, office.ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					if v.Feast != "Sunday within the Octave of Corpus Christi" {
						t.Fatalf("fixture has wrong owner: %s", v.Feast)
					}
					var psalms, antiphons []string
					chapterPos, versePos, index := -1, -1, 0
					chapters, verses, hymns, responsories := 0, 0, 0, 0
					suffix := tc.hour
					if tc.date == fixture.eve {
						suffix = "first-vespers"
					}
					for _, s := range v.Sections {
						for _, e := range s.Elements {
							index++
							if e.IsCommemoration {
								continue
							}
							if e.Type == models.Psalm {
								psalms = append(psalms, strings.TrimPrefix(e.SourceRef, "psalms/"))
							}
							if strings.HasPrefix(e.SlotRef, "psalm-antiphon") {
								// Check full repetition as well as the pre-psalm incipit.
								if len(antiphons) == 0 || antiphons[len(antiphons)-1] != e.SourceRef {
									antiphons = append(antiphons, e.SourceRef)
								}
								if tc.antiphon != "" && !strings.Contains(strings.ReplaceAll(e.Text, " *", ""), tc.antiphon) {
									t.Errorf("wrong little-hour antiphon: %s", e.Text)
								}
								if tc.hour == "vespers" {
									want := vespersAntiphons[e.SlotRef]
									if e.SlotRef == "psalm-antiphon-4" {
										want = "May the children of the Church be like the olive branches, round about the table of the Lord."
										if tc.date == fixture.eve {
											want = "He that maketh peace in the Church's borders is the Lord, who filleth us with the flour of wheat."
										}
									}
									got := strings.Join(strings.Fields(strings.ReplaceAll(e.Text, "*", "")), " ")
									if got != want {
										t.Errorf("%s text = %q, want %q", e.SlotRef, got, want)
									}
								}
							}
							if e.SlotRef == "chapter" && tc.chapter != "" {
								chapters++
								chapterPos = index
								if e.SourceRef != prefix+"chapter-"+suffix || !strings.Contains(e.Text, tc.chapter) || !strings.HasSuffix(e.Text, "R. Thanks be to God.") {
									t.Errorf("wrong chapter: %s %q", e.SourceRef, e.Text)
								}
							}
							if e.SlotRef == "versicle" && tc.verse != "" {
								verses++
								versePos = index
								if e.Type != models.Versicle || e.SourceRef != prefix+"versicle-"+suffix || e.Text != tc.verse {
									t.Errorf("wrong verse: %s %q", e.SourceRef, e.Text)
								}
							}
							if e.SlotRef == "hymn" && tc.hymn != "" {
								hymns++
								if e.SourceRef != prefix+"hymn-"+suffix || !strings.Contains(e.Text, tc.hymn) {
									t.Errorf("wrong hymn: %s", e.SourceRef)
								}
							}
							if e.SlotRef == "short-responsory" && tc.hour == "vespers" {
								responsories++
								if e.SourceRef != prefix+"short-responsory-"+suffix || !strings.Contains(e.Text, "He gave them bread from heaven") || !strings.Contains(e.Text, "So man did eat angels' food.") {
									t.Errorf("wrong Vespers responsory: %s %q", e.SourceRef, e.Text)
								}
							}
						}
					}
					if !slices.Equal(psalms, tc.psalms) {
						t.Errorf("psalms = %v, want %v", psalms, tc.psalms)
					}
					if tc.hour == "vespers" {
						want := []string{prefix + "psalm-antiphon-1-" + suffix, prefix + "psalm-antiphon-2-" + suffix, prefix + "psalm-antiphon-3-" + suffix, prefix + "psalm-antiphon-4-" + suffix}
						if !slices.Equal(antiphons, want) || hymns != 1 || responsories != 1 {
							t.Errorf("Vespers antiphons = %v, hymns = %d, responsories = %d", antiphons, hymns, responsories)
						}
					} else if len(antiphons) != 1 {
						t.Errorf("want one repeated little-hour antiphon, got %v", antiphons)
					}
					if tc.chapter != "" && (chapters != 1 || verses != 1 || chapterPos >= versePos) {
						t.Errorf("want one chapter before one verse: counts %d/%d, positions %d/%d", chapters, verses, chapterPos, versePos)
					}
				})
			}
		}
	}
}

func TestCorpusSundayYieldsToTransferredVisitation(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2027, 2032} {
		yd := buildYear(t, year)
		for i := range yd.days {
			d := &yd.days[i]
			if d.Date.Month() != 7 || d.Date.Day() != 4 {
				continue
			}
			for _, name := range []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"} {
				v, err := engine.ComposeHour(name, d, yd.moveable)
				if err != nil {
					t.Fatal(err)
				}
				if v.Feast != "Visitation of the Blessed Virgin Mary" {
					t.Fatalf("fixture must exercise the transferred Visitation, got %s", v.Feast)
				}
				for _, s := range v.Sections {
					for _, e := range s.Elements {
						if !e.IsCommemoration && strings.HasPrefix(e.SourceRef, "proper/pentecost-sunday-2/") {
							t.Errorf("%s %s leaks the displaced Sunday's proper: %s", d.Date, name, e.SourceRef)
						}
					}
				}
			}
		}
	}
}

// General Rubrics X, p. xxix: a commemorated office supplies its own verse.
// Both Vespers of this Sunday use the same printed pair (pp. 418, 421).
func TestDisplacedCorpusSundayCommemorationVerse(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2027, 2032} {
		yd := buildYear(t, year)
		for i := range yd.days {
			d := &yd.days[i]
			if d.Date.Month() != 7 || (d.Date.Day() != 3 && d.Date.Day() != 4) {
				continue
			}
			for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
				v, err := engine.ComposeHourWithOptions("vespers", d, yd.moveable, office.ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				found := 0
				for _, s := range v.Sections {
					for _, e := range s.Elements {
						if e.CommemorationOwnerID != "pentecost-sunday-2" || e.SlotRef != "commemoration-versicle" {
							continue
						}
						found++
						if !e.IsCommemoration || e.SourceRef != "proper/pentecost-sunday-2/versicle-vespers" || e.Text != "V. He fed them with the finest wheat flour, alleluia.\nR. And with honey out of the stony rock did he satisfy them, alleluia." {
							t.Errorf("%s %s: wrong Sunday commemoration verse: %+v", d.Date, form, e)
						}
					}
				}
				if found != 1 {
					t.Errorf("%s %s: got %d Sunday commemoration verses", d.Date, form, found)
				}
			}
		}
	}
}
