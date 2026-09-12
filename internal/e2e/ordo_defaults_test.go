package e2e

import (
	"slices"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// These checks protect the source-backed defaults discussed in #15, #62,
// #93 and #248. The corresponding ordo discrepancies remain provisional;
// these tests do not establish personal clergy endorsement of an issue comment.
func Test2026SourceBackedDefaults(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	compose := func(t *testing.T, name, date string) *models.OfficeHour {
		t.Helper()
		hour, err := eng.ComposeHour(name, dayFor(t, yd, date), yd.moveable)
		if err != nil {
			t.Fatal(err)
		}
		return hour
	}
	elements := func(hour *models.OfficeHour) []models.OfficeElement {
		var result []models.OfficeElement
		for _, section := range hour.Sections {
			result = append(result, section.Elements...)
		}
		return result
	}
	sourceIndex := func(elems []models.OfficeElement, ref string) int {
		return slices.IndexFunc(elems, func(elem models.OfficeElement) bool {
			return elem.SourceRef == ref || slices.Contains(elem.SourceRefs, ref)
		})
	}

	// Diurnal X, p. xxx: the preceding Sunday survives at a D2 feast's I
	// Vespers. A plain Double displaced at Lauds does not accompany it. The
	// 2026 ordo's Sept 20 appointment supplies the parallel for Feb 1 (#93).
	for _, tc := range []struct{ date, feast, displaced string }{
		{"2026-02-01", "purification-bvm", "st-ignatius-antioch"},
		{"2026-09-20", "st-matthew", "st-eustace-companions"},
	} {
		t.Run("Sunday-commemoration/"+tc.date, func(t *testing.T) {
			day := dayFor(t, yd, tc.date)
			if day.Celebration == nil || day.Celebration.Category != models.CategorySunday {
				t.Fatal("case must start with an occurring Sunday")
			}
			if !slices.ContainsFunc(day.Commemorations, func(f *models.Feast) bool { return f.ID == tc.displaced }) {
				t.Fatalf("Lauds must retain the displaced %s", tc.displaced)
			}
			if day.Vespers.Owner != models.VespersIOfFollowing || day.Vespers.Feast == nil || day.Vespers.Feast.ID != tc.feast {
				t.Fatalf("want I Vespers of %s, got %+v", tc.feast, day.Vespers)
			}
			comms := day.Vespers.Commemorations
			if len(comms) != 1 || comms[0].ID != day.Celebration.ID {
				t.Errorf("Vespers must commemorate only the preceding Sunday: %+v", comms)
			}
			rendered := office.SummarizeHour(compose(t, "vespers", tc.date)).Comms
			if len(rendered) != 1 || rendered[0].Name != day.Celebration.Name {
				t.Errorf("rendered Vespers commemorations = %+v, want Sunday only", rendered)
			}
		})
	}

	// Diurnal V, p. xxvii, and the Saturday BVM office, p. 68*: Friday's
	// office ends before the chapter. Psalms 142, 144:1-8, 144:9-15 and
	// 145:1-13 (pp. 136-139) precede the Marian chapter, hymn and collect.
	// The two ferial appointments in the 2026 ordo are suspected errata (#248).
	for _, date := range []string{"2026-06-19", "2026-07-10"} {
		t.Run("BVM-from-chapter/"+date, func(t *testing.T) {
			day := dayFor(t, yd, date)
			if day.Vespers.Owner != models.VespersIOfFollowing || day.Vespers.Feast == nil ||
				day.Vespers.Feast.ID != "saturday-office-bvm" || !day.Vespers.PsalmodyFromPreceding {
				t.Fatalf("want BVM I Vespers from chapter, got %+v", day.Vespers)
			}
			hour := compose(t, "vespers", date)
			elems := elements(hour)
			var psalms []string
			lastPsalm := -1
			for i, elem := range elems {
				if elem.Type == models.Psalm {
					psalms = append(psalms, elem.SourceRef)
					lastPsalm = i
				}
			}
			wantPsalms := []string{"psalms/142", "psalms/144a", "psalms/144b", "psalms/145a"}
			if !slices.Equal(psalms, wantPsalms) {
				t.Errorf("psalms = %v, want Friday sequence %v", psalms, wantPsalms)
			}
			for _, n := range []string{"1", "2", "4"} {
				ref := "ordinary/vespers/psalm-antiphon-" + n + "-friday"
				if i := sourceIndex(elems, ref); i < 0 || i > lastPsalm {
					t.Errorf("Friday antiphon missing from psalmody: %s", ref)
				}
			}
			previous := lastPsalm
			for _, ref := range []string{
				"commons/blessed-virgin/chapter-lauds",
				"commons/blessed-virgin/short-responsory-lauds",
				"commons/blessed-virgin/hymn-vespers",
				"commons/blessed-virgin/versicle-vespers",
				"proper/saturday-office-bvm/magnificat-antiphon",
				"commons/blessed-virgin/collect",
			} {
				i := sourceIndex(elems, ref)
				if i < 0 || i <= previous {
					t.Errorf("missing or misplaced Marian source after psalmody: %s", ref)
				}
				previous = i
			}
			summary := office.SummarizeHour(hour)
			if summary.Color != models.White || !summary.Suffrage {
				t.Errorf("BVM Vespers must be white with suffrage: %+v", summary)
			}
			if sourceIndex(elems, "ordinary/shared/suffrage-antiphon-bvm") < 0 {
				t.Error("BVM Vespers must use the Marian suffrage form")
			}
		})
	}

	// Diurnal IV (p. xxvi), VII (p. xxviii), and General Rubrics XXXVII.2:
	// suppress preces for a Double commemoration or within an octave, while
	// an otherwise unimpeded Sunday or feria keeps them. March 16's reverse
	// discrepancy and the Einsiedeln office choice remain separate questions.
	// Compline uses the evening office: March 11 and June 28 have already
	// entered the Double offices of Gregory and Peter & Paul respectively.
	for _, tc := range []struct {
		date, primeReason, complineReason string
	}{
		{"2026-01-18", office.PrecesSuppressedDoubleCommemoration, office.PrecesSuppressedDoubleCommemoration},
		{"2026-02-27", office.PrecesSuppressedDoubleCommemoration, office.PrecesSuppressedDoubleCommemoration},
		{"2026-06-28", office.PrecesSuppressedWithinOctave, office.PrecesSuppressedDoubleOffice},
		{"2026-11-08", office.PrecesSuppressedWithinOctave, office.PrecesSuppressedWithinOctave},
		{"2026-02-08", office.PrecesSaid, office.PrecesSaid},
		{"2026-03-11", office.PrecesSaid, office.PrecesSuppressedDoubleOffice},
	} {
		for _, name := range []string{"prime", "compline"} {
			t.Run("preces/"+tc.date+"/"+name, func(t *testing.T) {
				reason := tc.primeReason
				if name == "compline" {
					reason = tc.complineReason
				}
				said := reason == office.PrecesSaid
				hour := compose(t, name, tc.date)
				if got := office.SummarizeHour(hour).Preces; got != said {
					t.Errorf("rendered preces = %v, want %v", got, said)
				}
				if !slices.ContainsFunc(hour.Decisions, func(d models.CompositionDecision) bool {
					return d.Rule == "preces" && d.Outcome == reason
				}) {
					t.Errorf("missing preces decision %s", reason)
				}
			})
		}
	}

	// The 2026 ordo's texts, collect and commemorations support these owners
	// despite its I/II labels (#62). Only ownership is protected on Aug 23;
	// its separate Magnificat appointment remains unresolved.
	for _, tc := range []struct {
		date, feast string
		owner       models.VespersOwner
		color       models.Color
		comms       []string
	}{
		{"2026-01-04", "holy-name-jesus", models.VespersIIOfPreceding, models.White, []string{"The Vigil of the Epiphany", "St. Telesphorus of Rome, Bishop & Martyr"}},
		{"2026-05-01", "ss-philip-james", models.VespersIIOfPreceding, models.Red, []string{"St. Athanasius"}},
		{"2026-08-23", "st-bartholomew", models.VespersIOfFollowing, models.Red, []string{"XII Sunday after Pentecost"}},
	} {
		t.Run("ownership-label/"+tc.date, func(t *testing.T) {
			day := dayFor(t, yd, tc.date)
			if day.Vespers.Owner != tc.owner || day.Vespers.Feast == nil || day.Vespers.Feast.ID != tc.feast {
				t.Fatalf("want owner %v of %s, got %+v", tc.owner, tc.feast, day.Vespers)
			}
			hour := compose(t, "vespers", tc.date)
			summary := office.SummarizeHour(hour)
			if summary.Color != tc.color {
				t.Errorf("Vespers color = %s, want %s", summary.Color, tc.color)
			}
			var comms []string
			for _, c := range summary.Comms {
				comms = append(comms, c.Name)
			}
			if !slices.Equal(comms, tc.comms) {
				t.Errorf("rendered commemorations = %v, want %v", comms, tc.comms)
			}
			// Check the office's collect, not a collect in a commemoration.
			for _, elem := range elements(hour) {
				if elem.Type == models.Collect && !elem.IsCommemoration {
					if elem.SourceRef != "proper/"+tc.feast+"/collect" {
						t.Errorf("office collect = %s, want proper/%s/collect", elem.SourceRef, tc.feast)
					}
					return
				}
			}
			t.Error("missing office collect")
		})
	}
}
