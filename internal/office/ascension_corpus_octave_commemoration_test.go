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

func TestAscensionAndCorpusSundayOctaveCommemorations(t *testing.T) {
	// Diurnal pp.391–393,418–421; 2026 ordo pp.65–67,71–72.
	// Reusing festal psalmody does not replace the Sunday's octave prayer.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		date, hour, owner, antiphon, verse, collect string
	}{
		{"05-25", "vespers", "ascension-octave-day-5", "O King of glory", "V. God is gone up with a merry noise", "Grant, we beseech thee, Almighty God"},
		{"05-26", "vespers", "ascension-octave-day-7", "O King of glory", "V. God is gone up with a merry noise", "Grant, we beseech thee, Almighty God"},
		{"05-23", "vespers", "ascension-octave-day-3", "O King of glory", "V. God is gone up with a merry noise", "Grant, we beseech thee, Almighty God"},
		{"06-13", "vespers", "corpus-christi-octave-day-3", "O sacred banquet", "V. Thou gavest them bread from heaven", "O God, who under a wonderful Sacrament"},
		{"06-14", "vespers", "corpus-christi-octave-day-4", "O how sweet", "V. Thou gavest them bread from heaven", "O God, who under a wonderful Sacrament"},
		{"05-24", "lauds", "ascension-octave-day-4", "I ascend", "V. The Lord hath prepared", "Grant, we beseech thee, Almighty God"},
		{"05-26", "lauds", "ascension-octave-day-6", "I ascend", "V. The Lord hath prepared", "Grant, we beseech thee, Almighty God"},
		{"05-27", "lauds", "ascension-octave-day-7", "I ascend", "V. The Lord hath prepared", "Grant, we beseech thee, Almighty God"},
		{"06-14", "lauds", "corpus-christi-octave-day-4", "I am * the living bread", "V. He maketh peace in thy borders", "O God, who in a wonderful Sacrament"},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				var day *models.CalendarDay
				for i := range days {
					if days[i].Date.Format("01-02") == tc.date {
						day = &days[i]
						break
					}
				}
				hour, err := engine.ComposeHourWithOptions(tc.hour, day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]string{"commemoration-antiphon": tc.antiphon, "commemoration-versicle": tc.verse, "commemoration-collect": tc.collect}
				seen := map[string]int{}
				for _, section := range hour.Sections {
					for _, element := range section.Elements {
						if element.CommemorationOwnerID != tc.owner {
							continue
						}
						if prefix, ok := want[element.SlotRef]; ok {
							seen[element.SlotRef]++
							if tc.hour == "vespers" && element.SlotRef != "commemoration-collect" {
								trace := engine.TraceCommemorationResolution(day, tc.hour, element.SlotRef, element.SourceRef, tc.owner)
								if trace.OwnerID != tc.owner || trace.SelectedTier != "proper" || trace.Reason != "octave-commemoration-context" || !slices.Contains(trace.DirectExisting, element.SourceRef) || trace.ResolverSlot == element.SlotRef {
									t.Errorf("context appointment trace = %#v", trace)
								}
							}
							if !strings.HasPrefix(element.Text, prefix) {
								t.Errorf("%s = %q, want %q", element.SlotRef, element.Text, prefix)
							}
						}
					}
				}
				for slot := range want {
					if seen[slot] != 1 {
						t.Errorf("%s rendered %d times, want once", slot, seen[slot])
					}
				}
			})
		}
	}
}

func TestAscensionSundayEveningAntiphonHold(t *testing.T) {
	// #398: ordo p.66 appoints O King of glory, Diurnal p.393 Father.
	// Preserve the existing selection pending a ruling; this is a hold,
	// not an assertion that the psalm-antiphon fallback is correct.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	feast := &models.Feast{ID: "ascension-octave-day-4", ProperID: "ascension", Category: models.CategoryLord}
	text, ref := lookupCommemoration(feast, models.Easter, "vespers", "commemoration-antiphon", engine.corpus)
	if !strings.HasPrefix(text, "Ye men of Galilee") || ref != "proper/ascension/commemoration-antiphon" {
		t.Fatalf("held selection changed to %q (%s)", text, ref)
	}
}

func TestSundayOctaveVespersContextAndFootnote(t *testing.T) {
	// Generic fixture: no feast IDs or octave-day ordinal prescribe a form.
	// Diurnal p.421 footnote repeats the I-Vespers form when tomorrow's
	// office is not of the octave, even if tomorrow falls inside its dates.
	const prefix = "proper/example/commemoration-antiphon"
	for _, tc := range []struct {
		name              string
		owner             models.VespersOwner
		following, within string
		category          models.FeastCategory
		missing           bool
		want              string
	}{
		{"Sunday I", models.VespersIOfFollowing, "", "example", models.CategorySunday, false, "Sunday I form"},
		{"Sunday II before octave office", models.VespersIIOfPreceding, "example", "example", models.CategorySunday, false, "Sunday II form"},
		{"Sunday II before another office", models.VespersIIOfPreceding, "", "example", models.CategorySunday, false, "Sunday I form"},
		{"another feast owns I Vespers", models.VespersIOfFollowing, "", "example", models.CategoryConfessor, false, "Generic fallback"},
		{"another feast owns II Vespers", models.VespersIIOfPreceding, "example", "example", models.CategoryConfessor, false, "Generic fallback"},
		{"different octave", models.VespersIIOfPreceding, "example", "other", models.CategorySunday, false, "Generic fallback"},
		{"outside any octave", models.VespersIIOfPreceding, "example", "", models.CategorySunday, false, "Generic fallback"},
		{"no evening designation", models.VespersNotApplicable, "example", "example", models.CategorySunday, false, "Generic fallback"},
		{"held II does not inherit I", models.VespersIIOfPreceding, "example", "example", models.CategorySunday, true, "Generic fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := map[string]string{
				prefix:                            "Generic fallback",
				prefix + "-sunday-first-vespers":  "Sunday I form",
				prefix + "-sunday-second-vespers": "Sunday II form",
				prefix + "-sunday-second-vespers-before-other-office": "Sunday I form",
			}
			if tc.missing {
				delete(entries, prefix+"-sunday-second-vespers")
			}
			corpus := texts.NewTestCorpus(entries)
			comm := &models.Feast{ID: "example-octave-day-4", ProperID: "example", Category: models.CategoryLord}
			owner := &models.Feast{ID: "office-owner", Category: tc.category}
			day := &models.CalendarDay{
				Date: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), Celebration: owner, WithinOctaveOf: tc.within,
				Vespers: models.VespersDesignation{Owner: tc.owner, Feast: owner, WithinOctaveOf: tc.within, FollowingOfficeOctaveOf: tc.following, Commemorations: []*models.Feast{comm}},
			}
			elems := addCommemorations(vespersOfficeDay(day), "vespers", corpus, false)
			if elems[1].Text != tc.want {
				t.Fatalf("antiphon = %q (%s), want %q", elems[1].Text, elems[1].SourceRef, tc.want)
			}
		})
	}
}

func TestOctaveSundayAppointmentsDoNotLeakToOtherOffices(t *testing.T) {
	// #398 remains held; Corpus appointments remain limited to Sundays,
	// not Peter–Paul or transferred Visitation. These are scope boundaries,
	// not source attestations of the existing generic fallback wording.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ date, office, comm, parent string }{
		{"2024-06-14", "st-basil-great", "ascension-octave-day-2", "ascension"},
		{"2026-05-24", "ascension-sunday-within-octave", "ascension-octave-day-4", "ascension"},
		{"2030-06-29", "ss-peter-paul", "corpus-christi-octave-day-3", "corpus-christi"},
		{"2027-07-04", "visitation-bvm", "corpus-christi-octave-day-4", "corpus-christi"},
		{"2032-07-04", "visitation-bvm", "corpus-christi-octave-day-4", "corpus-christi"},
		{"2043-07-05", "visitation-bvm", "corpus-christi-octave-day-4", "corpus-christi"},
	} {
		t.Run(tc.date, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tc.date)
			if err != nil {
				t.Fatal(err)
			}
			days, err := calendar.BuildCalendar(date.Year(), "../../data")
			if err != nil {
				t.Fatal(err)
			}
			day := &days[date.YearDay()-1]
			if day.Vespers.Feast == nil || day.Vespers.Feast.ID != tc.office {
				t.Fatalf("unexpected Vespers owner: %#v", day.Vespers.Feast)
			}
			for _, form := range models.PrayerForms {
				hour, err := engine.ComposeHourWithOptions("vespers", day, calendar.ComputeMoveableDates(date.Year()), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				seen := 0
				for _, s := range hour.Sections {
					for _, e := range s.Elements {
						if e.CommemorationOwnerID == tc.comm && (e.SlotRef == "commemoration-antiphon" || e.SlotRef == "commemoration-versicle") {
							seen++
							if want := "proper/" + tc.parent + "/" + e.SlotRef; e.SourceRef != want {
								t.Errorf("%s %s source = %s, want held fallback %s", form, e.SlotRef, e.SourceRef, want)
							}
						}
					}
				}
				if seen != 2 {
					t.Errorf("%s found %d octave antiphons/versicles, want 2", form, seen)
				}
			}
		})
	}
}

func TestOctaveCommemorationAtFirstVespersScope(t *testing.T) {
	// Parent appointments for days within an octave do not prescribe the
	// terminal day, another hour, Sunday, or a following-octave commemoration
	// at II Vespers (XIV.11). No date-window inference is needed at I Vespers.
	for _, tc := range []struct {
		name, id, hour string
		vespers        models.VespersOwner
		category       models.FeastCategory
		want           string
	}{
		{"I of another feast outside the octave window", "example-octave-day-7", "vespers", models.VespersIOfFollowing, models.CategoryConfessor, "Appointed antiphon"},
		{"terminal octave day", "example-octave-day", "vespers", models.VespersIOfFollowing, models.CategoryConfessor, "Existing fallback"},
		{"Sunday has its own appointments", "example-octave-day-3", "vespers", models.VespersIOfFollowing, models.CategorySunday, "Existing fallback"},
		{"II before following octave office", "example-octave-day-5", "vespers", models.VespersIIOfPreceding, models.CategoryConfessor, "Existing fallback"},
		{"Lauds", "example-octave-day-5", "lauds", models.VespersIOfFollowing, models.CategoryConfessor, "Existing fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			corpus := texts.NewTestCorpus(map[string]string{
				"proper/example/commemoration-antiphon-at-first-vespers": "Appointed antiphon",
				"proper/example/commemoration-antiphon":                  "Existing fallback",
			})
			comm := &models.Feast{ID: tc.id, ProperID: "example", Category: models.CategoryLord}
			feast := &models.Feast{ID: "other-feast", Category: tc.category}
			day := &models.CalendarDay{Celebration: feast, WithinOctaveOf: "example", Commemorations: []*models.Feast{comm},
				Vespers: models.VespersDesignation{Owner: tc.vespers, Feast: feast, FollowingOfficeOctaveOf: "example", Commemorations: []*models.Feast{comm}},
			}
			if tc.category == models.CategorySunday {
				day.Vespers.WithinOctaveOf = "example"
			}
			officeDay := day
			if tc.hour == "vespers" {
				officeDay = vespersOfficeDay(day)
			}
			elems := addCommemorations(officeDay, tc.hour, corpus, false)
			if elems[1].Text != tc.want {
				t.Fatalf("antiphon = %q, want %q", elems[1].Text, tc.want)
			}
			if elems[1].Text == "Appointed antiphon" {
				engine := &Engine{corpus: corpus}
				trace := engine.TraceCommemorationResolution(day, tc.hour, elems[1].SlotRef, elems[1].SourceRef, comm.ID)
				if trace.OwnerID != comm.ID || trace.ResolverSlot != "commemoration-antiphon-at-first-vespers" || !slices.Contains(trace.DirectExisting, elems[1].SourceRef) {
					t.Errorf("outside-window trace = %#v", trace)
				}
			}
		})
	}
}

func TestAscensionOctaveAtBedeFirstVespers(t *testing.T) {
	// The same ordinal belongs to Sunday I Vespers in 2026 and Bede I in
	// 2028; the appointment is established by the actual evening context.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2028, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	date := time.Date(2028, 5, 26, 0, 0, 0, 0, time.UTC)
	day := &days[date.YearDay()-1]
	if day.Vespers.Owner != models.VespersIOfFollowing || day.Vespers.Feast.ID != "st-bede-venerable" {
		t.Fatal("fixture must be Bede I Vespers")
	}
	for _, form := range models.PrayerForms {
		hour, err := engine.ComposeHourWithOptions("vespers", day, calendar.ComputeMoveableDates(2028), ComposeOptions{Form: form})
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for _, section := range hour.Sections {
			for _, e := range section.Elements {
				if e.CommemorationOwnerID != "ascension-octave-day-3" {
					continue
				}
				switch e.SlotRef {
				case "commemoration-antiphon":
					seen++
					if !strings.HasPrefix(e.Text, "O King of glory") || e.SourceRef != "proper/ascension/commemoration-antiphon-at-first-vespers" {
						t.Errorf("%s antiphon = %q (%s)", form, e.Text, e.SourceRef)
					}
				case "commemoration-versicle":
					seen++
					if !strings.HasPrefix(e.Text, "V. God is gone up with a merry noise") || e.SourceRef != "proper/ascension/commemoration-versicle-at-first-vespers" {
						t.Errorf("%s versicle = %q (%s)", form, e.Text, e.SourceRef)
					}
				}
			}
		}
		if seen != 2 {
			t.Errorf("%s found %d octave antiphons/versicles, want 2", form, seen)
		}
	}
}
