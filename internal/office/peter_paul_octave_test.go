package office

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// Diurnal pp. 558–560 appoint the Apostle Common with the octave's own
// responsories, verses and collect. Synthetic offices exercise all seven
// generated IDs, including offices displaced in the real calendar below.
func TestPeterPaulOctaveAppointments(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	laudsAnts := []string{"This is my commandment", "Greater love hath no man", "Ye are my friends", "Blessed are the peacemakers", "In your patience"}
	vespersAnts := []string{"The Lord sware", "May the Lord set him", "Thou, O Lord, hast broken", "Firmly stablished"}
	for n := 2; n <= 8; n++ {
		id := fmt.Sprintf("ss-peter-paul-octave-day-%d", n)
		rank := models.SemiDouble
		if n == 8 {
			id = "ss-peter-paul-octave-day"
			rank = models.GreaterDouble
		}
		f := &models.Feast{ID: id, ProperID: fmt.Sprintf("ss-peter-paul-octave-set-%d", n-1), Category: models.CategoryApostle, Rank: rank}
		if n == 8 {
			f.ProperID = "ss-peter-paul"
		}
		day := &models.CalendarDay{Date: time.Date(2026, 6, 28+n, 0, 0, 0, 0, time.UTC), Season: models.Pentecost, Celebration: f, Color: models.Red, WithinOctaveOf: "ss-peter-paul"}
		for _, form := range models.PrayerForms {
			for _, hour := range []string{"lauds", "prime", "terce", "sext", "none", "vespers"} {
				t.Run(fmt.Sprintf("%s/%s/%s", id, hour, form), func(t *testing.T) {
					h, err := engine.ComposeHourWithOptions(hour, day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					check := func(slot, prefix, source string) {
						t.Helper()
						e := officeElementBySlot(h, slot)
						if e == nil || !strings.HasPrefix(e.Text, prefix) || (source != "" && !slices.Contains(e.SourceRefs, source)) {
							t.Fatalf("%s: %+v; want %q from %s", slot, e, prefix, source)
						}
					}
					if hour != "prime" {
						prefix, source := "Brethren: Now therefore", "commons/apostle/chapter-lauds"
						if hour == "sext" {
							prefix, source = "By the hands of the Apostles", "commons/apostle/chapter-sext"
						}
						if hour == "none" {
							prefix, source = "And they departed", "commons/apostle/chapter-none"
						}
						check("chapter", prefix, source)
						collect, source := "O God, who hast consecrated", "proper/ss-peter-paul/collect"
						if n == 8 {
							collect, source = "O God, whose right hand upheld", "proper/ss-peter-paul-octave-day/collect"
						}
						check("collect", collect, source)
						if n == 8 && !strings.Contains(officeElementBySlot(h, "collect").Text, "Who with God the Father") {
							t.Error("octave-day collect lost its second-person conclusion")
						}
					}
					if hour == "lauds" || hour == "vespers" {
						check("hymn", "Let heav'n's exultant praises ring", "commons/apostle/hymn-lauds")
						if hour == "lauds" {
							check("short-responsory", "R. Thou shalt make them princes", "commons/apostle/short-responsory-vespers")
							check("versicle", "V. Their sound is gone out", "commons/apostle/versicle-first-vespers")
						} else {
							check("short-responsory", "R. Their sound is gone out", "commons/apostle/short-responsory-lauds")
							check("versicle", "V. Thou shalt make them princes", "commons/apostle/versicle-sext")
						}
						ants := laudsAnts
						if hour == "vespers" {
							ants = vespersAnts
						}
						// Terminal II Vespers psalmody is held: ordo p.78 cites p.7*,
						// whereas the Diurnal p.559 directs II Vespers to p.12*.
						if hour != "vespers" || n != 8 {
							for i, prefix := range ants {
								check(fmt.Sprintf("psalm-antiphon-%d", i+1), prefix, "")
							}
						}
						if hour == "vespers" {
							var psalms []string
							for _, s := range h.Sections {
								for _, e := range s.Elements {
									if e.Type == models.Psalm {
										psalms = append(psalms, e.SourceRef)
									}
								}
							}
							if !slices.Equal(psalms, []string{"psalms/110", "psalms/113", "psalms/116b", "psalms/139"}) {
								t.Errorf("II Vespers psalms: %v", psalms)
							}
						}
					} else {
						slot, prefix := "psalm-antiphon-1", laudsAnts[0]
						switch hour {
						case "terce":
							slot, prefix = "psalm-antiphon-2", laudsAnts[1]
						case "sext":
							slot, prefix = "psalm-antiphon-3", laudsAnts[2]
						case "none":
							slot, prefix = "psalm-antiphon-5", laudsAnts[4]
						}
						check(slot, prefix, "")
						if hour != "prime" {
							verse := map[string]string{"terce": "V. Their sound is gone out", "sext": "V. Thou shalt make them princes", "none": "V. Exceedingly honoured"}[hour]
							check("versicle", verse, "")
							if officeElementBySlot(h, "short-responsory") != nil {
								t.Error("little hour gained a responsory instead of its simple verse")
							}
						}
					}
				})
			}
		}
	}
}

// Ordo pp.77–78 explicitly use the octave verse and the p.560 collect at
// the anticipated terminal commemoration. Corpus Christi coincidences in
// 2027/2032 keep this check independent of principal-office coverage.
func TestPeterPaulOctaveCommemorationVersesAndCollects(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		for i := range days {
			day := &days[i]
			date := day.Date.Format("01-02")
			if date < "06-28" || date > "07-08" {
				continue
			}
			for _, hour := range []string{"lauds", "vespers"} {
				for _, form := range models.PrayerForms {
					h, err := engine.ComposeHourWithOptions(hour, day, calendar.ComputeMoveableDates(year), ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					for _, s := range h.Sections {
						for _, e := range s.Elements {
							if !strings.HasPrefix(e.CommemorationOwnerID, "ss-peter-paul-octave-day") {
								continue
							}
							switch e.SlotRef {
							case "commemoration-versicle":
								want := "V. Their sound is gone out"
								if hour == "vespers" {
									want = "V. Thou shalt make them princes"
								}
								if !strings.HasPrefix(e.Text, want) {
									t.Errorf("%s/%s/%s verse: %s", day.Date, hour, form, e.Text)
								}
								seen[hour] = true
							case "commemoration-collect":
								want, source := "O God, who hast consecrated", "proper/ss-peter-paul/collect"
								if e.CommemorationOwnerID == "ss-peter-paul-octave-day" {
									want, source = "O God, whose right hand upheld", "proper/ss-peter-paul-octave-day/collect"
									seen["terminal"] = true
								}
								if !strings.HasPrefix(e.Text, want) || !slices.Contains(e.SourceRefs, source) {
									t.Errorf("%s/%s/%s collect: %+v", day.Date, hour, form, e)
								}
							}
						}
					}
				}
			}
		}
	}
	if !seen["lauds"] || !seen["vespers"] || !seen["terminal"] {
		t.Fatalf("missing calendar coverage: %v", seen)
	}
}

func TestPeterPaulTerminalFirstVespers(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	feast := &models.Feast{ID: "ss-peter-paul-octave-day", ProperID: "ss-peter-paul", Category: models.CategoryApostle, Rank: models.GreaterDouble}
	day := &models.CalendarDay{Date: time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC), Season: models.Pentecost, Vespers: models.VespersDesignation{Owner: models.VespersIOfFollowing, Feast: feast, Season: models.Pentecost}}
	for _, form := range models.PrayerForms {
		h, err := engine.ComposeHourWithOptions("vespers", day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
		if err != nil {
			t.Fatal(err)
		}
		for n, prefix := range []string{"This is my commandment", "Greater love hath no man", "Ye are my friends", "In your patience"} {
			e := officeElementBySlot(h, fmt.Sprintf("psalm-antiphon-%d", n+1))
			if e == nil || !strings.HasPrefix(e.Text, prefix) {
				t.Fatalf("I Vespers antiphon %d: %+v", n+1, e)
			}
		}
		verse := officeElementBySlot(h, "versicle")
		if verse == nil || !strings.HasPrefix(verse.Text, "V. Thou shalt make them princes") {
			t.Errorf("I Vespers verse: %+v", verse)
		}
		var psalms []string
		for _, s := range h.Sections {
			for _, e := range s.Elements {
				if e.Type == models.Psalm {
					psalms = append(psalms, e.SourceRef)
				}
			}
		}
		if !slices.Equal(psalms, []string{"psalms/110", "psalms/111", "psalms/112", "psalms/113"}) {
			t.Errorf("I Vespers psalms: %v", psalms)
		}
	}
}
