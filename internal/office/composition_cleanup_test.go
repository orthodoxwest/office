package office

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// Diurnal pp. 475–479 and 2026 ordo p. 36 appoint the full Scholastica
// proper, with II Vespers repeating I except for the Magnificat antiphon.
func TestScholasticaAppointments(t *testing.T) {
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
		for _, form := range models.PrayerForms {
			for _, tc := range []struct{ date, hour string }{{"02-09", "vespers"}, {"02-10", "lauds"}, {"02-10", "terce"}, {"02-10", "vespers"}} {
				t.Run(fmt.Sprintf("%d/%s/%s/%s", year, tc.date, tc.hour, form), func(t *testing.T) {
					var day *models.CalendarDay
					for i := range days {
						if days[i].Date.Format("01-02") == tc.date {
							day = &days[i]
							break
						}
					}
					if day == nil {
						t.Fatal("missing date")
					}
					h, err := engine.ComposeHourWithOptions(tc.hour, day, md, ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					chapter := officeElementBySlot(h, "chapter")
					if chapter == nil || (chapter.Label != "Cant 2:10,14" || !strings.HasPrefix(chapter.Text, "Rise up, my love")) {
						t.Fatalf("wrong chapter: %+v", chapter)
					}
					if !slices.Contains(chapter.SourceRefs, "proper/st-scholastica/chapter-lauds") {
						t.Errorf("chapter source: %v", chapter.SourceRefs)
					}
					if tc.hour == "terce" {
						return
					}
					hymn := officeElementBySlot(h, "hymn")
					wantTitle, wantEnd := "Te beata sponsa Christi", "All the ages' course is run. Amen."
					if tc.hour == "lauds" {
						wantTitle = "Jam noctis umbrae concidunt"
						wantEnd = "While endless ages roll away. Amen."
					}
					if hymn == nil || hymn.Label != wantTitle || !strings.HasSuffix(hymn.Text, wantEnd) || len(strings.Split(hymn.Text, "\n\n")) != 6 {
						t.Fatalf("incomplete hymn: %+v", hymn)
					}
					if tc.hour == "vespers" && !strings.Contains(hymn.Text, "Shining still through all our days,") {
						t.Error("lost word in page continuation")
					}
					responsory := officeElementBySlot(h, "short-responsory")
					if responsory == nil {
						t.Fatal("missing responsory")
					}
					if tc.hour == "lauds" {
						if !strings.HasPrefix(responsory.Text, "R. In thy grace") {
							t.Fatalf("Lauds Common responsory changed: %s", responsory.Text)
						}
						return
					}
					if !strings.HasPrefix(responsory.Text, "R. In the likeness of a dove * The soul of Scholastica") || !strings.Contains(responsory.Text, "V. The heart of her brother rejoiced.") {
						t.Errorf("wrong Vespers responsory: %s", responsory.Text)
					}
					var psalms []string
					antiphons := 0
					for _, s := range h.Sections {
						for _, e := range s.Elements {
							if e.Type == models.Psalm {
								psalms = append(psalms, e.SourceRef)
							}
							if e.SlotRef == "psalm-antiphon-4" {
								antiphons++
								if !strings.HasPrefix(e.Text, "When holy Benedict") || !slices.Equal(e.SourceRefs, []string{"proper/st-scholastica/psalm-antiphon-5"}) {
									t.Errorf("fourth Vespers antiphon: %+v", e)
								}
							}
						}
					}
					if antiphons != 2 || !slices.Equal(psalms, []string{"psalms/110", "psalms/113", "psalms/122", "psalms/127"}) {
						t.Errorf("wrong Vespers psalmody: %v; antiphon count %d", psalms, antiphons)
					}
				})
			}
		}
	}
}

// The proper Lauds/Vespers endings do not suppress the seasonal doxology
// of the ordinary hymns at the other hours.
func TestScholasticaSmallHourHymnEndings(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		for i := range days {
			day := &days[i]
			if day.Date.Format("01-02") != "02-10" {
				continue
			}
			for _, hour := range []string{"prime", "terce", "sext", "none", "compline"} {
				for _, form := range models.PrayerForms {
					h, err := engine.ComposeHourWithOptions(hour, day, calendar.ComputeMoveableDates(year), ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					hymn := officeElementBySlot(h, "hymn")
					if hymn == nil || !slices.Contains(hymn.SourceRefs, "seasonal/epiphany/hymn-doxology") {
						t.Errorf("%d %s %s lost seasonal ending: %+v", year, hour, form, hymn)
					}
				}
			}
		}
	}
}

// The octave antiphons are printed on p. 558; p. 559 retains them on the
// octave day. Ordo pp. 77–78 also appoint them in commemorations.
func TestPeterPaulOctaveGospelAntiphons(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string]string{
		"lauds":   "Glorious princes * over all the earth, as in life they loved one another, so even in death they were not divided.",
		"vespers": "Peter the Apostle * and Paul the Doctor of the Gentiles: these have taught us thy law, O Lord.",
	}
	principal, commemorated := 0, 0
	seen := map[string]bool{}
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		md := calendar.ComputeMoveableDates(year)
		for i := range days {
			day := &days[i]
			date := day.Date.Format("01-02")
			if date < "06-28" || date > "07-08" {
				continue
			}
			for _, hour := range []string{"lauds", "vespers"} {
				for _, form := range models.PrayerForms {
					h, err := engine.ComposeHourWithOptions(hour, day, md, ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					owner := day.Celebration
					if hour == "vespers" && day.Vespers.Feast != nil {
						owner = day.Vespers.Feast
					}
					slot := "benedictus-antiphon"
					if hour == "vespers" {
						slot = "magnificat-antiphon"
					}
					for _, s := range h.Sections {
						for _, e := range s.Elements {
							expected := false
							if owner != nil && strings.HasPrefix(owner.ID, "ss-peter-paul-octave-day") && e.SlotRef == slot && !e.IsCommemoration {
								expected = true
								principal++
							}
							if strings.HasPrefix(e.CommemorationOwnerID, "ss-peter-paul-octave-day") && e.SlotRef == "commemoration-antiphon" {
								expected = true
								commemorated++
							}
							if expected {
								ownerID := e.CommemorationOwnerID
								if !e.IsCommemoration {
									ownerID = owner.ID
								}
								seen[ownerID+"/"+hour] = true
							}
							if expected && (e.Text != texts[hour] || !slices.Equal(e.SourceRefs, []string{"proper/ss-peter-paul-octave-day/" + slot})) {
								t.Errorf("%s %s %s: wrong octave antiphon %+v", day.Date, hour, form, e)
							}
							if !expected && e.Text == texts[hour] {
								t.Errorf("%s %s: octave text leaked into another appointment: %+v", day.Date, hour, e)
							}
						}
					}
				}
			}
		}
	}
	for _, id := range []string{"ss-peter-paul-octave-day", "ss-peter-paul-octave-day-2", "ss-peter-paul-octave-day-3", "ss-peter-paul-octave-day-4", "ss-peter-paul-octave-day-5", "ss-peter-paul-octave-day-6", "ss-peter-paul-octave-day-7"} {
		for _, hour := range []string{"lauds", "vespers"} {
			// June 30 Vespers belongs to St Paul or the following feast in
			// this sample; its octave text is still checked directly below.
			if id == "ss-peter-paul-octave-day-2" && hour == "vespers" {
				day := &models.CalendarDay{Celebration: &models.Feast{ID: id, ProperID: "ss-peter-paul-octave-set-1"}, Season: models.Pentecost}
				for _, ref := range []string{"magnificat-antiphon", "commemoration-antiphon"} {
					got, _ := resolveProperText(day, hour, ref, engine.corpus)
					if got != texts[hour] {
						t.Errorf("June 30 %s: %q", ref, got)
					}
				}
				continue
			}
			if !seen[id+"/"+hour] {
				t.Errorf("octave boundary not exercised: %s %s", id, hour)
			}
		}
	}
	if principal == 0 || commemorated == 0 {
		t.Fatalf("missing coverage: principal=%d commemorations=%d", principal, commemorated)
	}
}

func TestReviewedSundayAntiphonWording(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"02-15": "Unto you it is given * to know the mysteries of the kingdom of heaven: but to others in parables, said Jesus to his disciples.",
		"05-31": "To-day * are fulfilled the days of Pentecost, alleluia: to-day the Holy Spirit appeared in fire to the disciples, and bestowed upon them his manifold graces: sending them into all the world, to preach the gospel, and to testify: He that believeth and is baptized shall be saved, alleluia.",
	}
	for i := range days {
		day := &days[i]
		text, ok := want[day.Date.Format("01-02")]
		if !ok {
			continue
		}
		for _, form := range models.PrayerForms {
			h, err := engine.ComposeHourWithOptions("vespers", day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
			if err != nil {
				t.Fatal(err)
			}
			e := officeElementBySlot(h, "magnificat-antiphon")
			if e == nil || e.Text != text {
				t.Errorf("%s %s: %+v", day.Date, form, e)
			}
		}
	}
}

// The psalter p.151 appoints a simple verse with Paschaltide alleluias,
// not an expanded Into thy hands responsory.
func TestPaschalComplineVerse(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	// Pentecost uses a different season key, but Paschaltide continues
	// through the octave. Trinity I Vespers ends the Compline appointment.
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		md := calendar.ComputeMoveableDates(year)
		for i := range days {
			day := &days[i]
			if day.Date.Before(md.Easter) || day.Date.After(md.Pentecost.AddDate(0, 0, 8)) {
				continue
			}
			paschal := day.Date.Before(md.Pentecost.AddDate(0, 0, 6))
			want := "V. Keep us, O Lord, as the apple of an eye.\nR. Hide us under the shadow of thy wings."
			source := "ordinary/compline/short-responsory"
			if paschal {
				want = "V. Keep us, O Lord, as the apple of an eye. Alleluia.\nR. Hide us under the shadow of thy wings. Alleluia."
				source = "seasonal/easter/short-responsory-compline"
			}
			for _, form := range models.PrayerForms {
				h, err := engine.ComposeHourWithOptions("compline", day, md, ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				e := officeElementBySlot(h, "short-responsory")
				if e == nil || e.Text != want || !slices.Equal(e.SourceRefs, []string{source}) {
					t.Errorf("%s %s: wrong seasonal verse %+v", day.Date, form, e)
				}
			}
		}
	}
}
