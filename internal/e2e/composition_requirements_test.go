package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// These expectations are read from sources, not regenerated from engine output.
// Keep citations and boundary cases with each requirement. A passing case verifies
// that assertion only; it is not a signoff for the entire hour or source page.
type compositionRequirement struct {
	ID     string            `json:"id"`
	Source string            `json:"source"`
	Cases  []compositionCase `json:"cases"`
}
type compositionCase struct {
	Date     string `json:"date"`
	Hour     string `json:"hour"`
	Owner    string `json:"commemoration_owner,omitempty"`
	Slot     string `json:"slot"`
	Ref      string `json:"ref,omitempty"`
	Contains string `json:"contains,omitempty"`
	Absent   bool   `json:"absent,omitempty"`
	Before   string `json:"before_slot,omitempty"`
}

func TestCompositionRequirements(t *testing.T) {
	raw, err := os.ReadFile("../../data/review/composition-requirements.json")
	if err != nil {
		t.Fatal(err)
	}
	var requirements []compositionRequirement
	if err = json.Unmarshal(raw, &requirements); err != nil {
		t.Fatal(err)
	}
	if len(requirements) == 0 {
		t.Fatal("no source requirements")
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	years := map[int]*yearData{}
	ids := map[string]bool{}
	for _, rule := range requirements {
		if rule.ID == "" || rule.Source == "" || len(rule.Cases) == 0 || ids[rule.ID] {
			t.Fatalf("invalid or duplicate requirement: %+v", rule)
		}
		ids[rule.ID] = true
		t.Run(rule.ID, func(t *testing.T) {
			for i, c := range rule.Cases {
				t.Run(fmt.Sprintf("%s/%s/%d", c.Date, c.Hour, i), func(t *testing.T) {
					date, err := time.Parse("2006-01-02", c.Date)
					if err != nil {
						t.Fatal(err)
					}
					if years[date.Year()] == nil {
						years[date.Year()] = buildYear(t, date.Year())
					}
					yd := years[date.Year()]
					hour, err := eng.ComposeHour(c.Hour, dayFor(t, yd, c.Date), yd.moveable)
					if err != nil {
						t.Fatal(err)
					}
					if c.Slot == "" || (!c.Absent && c.Ref == "" && c.Contains == "") {
						t.Fatal("case requires a slot and expectation")
					}
					var found []models.OfficeElement
					position, before, index := -1, -1, 0
					for _, section := range hour.Sections {
						for _, elem := range section.Elements {
							if elem.CommemorationOwnerID == c.Owner && elem.IsCommemoration == (c.Owner != "") {
								if elem.SlotRef == c.Slot {
									found = append(found, elem)
									if position < 0 {
										position = index
									}
								}
								if c.Before != "" && elem.SlotRef == c.Before && before < 0 {
									before = index
								}
							}
							index++
						}
					}
					if c.Absent {
						if len(found) > 0 {
							t.Errorf("%s: unexpected %s", rule.Source, c.Slot)
						}
						return
					}
					if len(found) == 0 {
						t.Fatalf("%s: missing %s for owner %q", rule.Source, c.Slot, c.Owner)
					}
					for _, elem := range found {
						if c.Ref != "" && elem.SourceRef != c.Ref {
							t.Errorf("%s: %s = %s, want %s", rule.Source, c.Slot, elem.SourceRef, c.Ref)
						}
						if c.Contains != "" && !strings.Contains(elem.Text, c.Contains) {
							t.Errorf("%s: %s lacks %q", rule.Source, c.Slot, c.Contains)
						}
					}
					if c.Before != "" && (before < 0 || position >= before) {
						t.Errorf("%s: %s must precede %s", rule.Source, c.Slot, c.Before)
					}
				})
			}
		})
	}
}

// Examine all dates, including unnamed ferias. Additional years exercise
// calendar collisions under the reviewed rubric, not future-ordo agreement.
func TestSundayCommemorationVersicleAcrossCalendars(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2026, 2027, 2032} {
		yd := buildYear(t, year)
		checked := 0
		for i := range yd.days {
			day := &yd.days[i]
			if day.Season != models.Pentecost {
				continue
			}
			for _, comm := range day.Commemorations {
				// The first two Sundays retain Trinity/Corpus Christi octave appointments.
				if comm.Category != models.CategorySunday || comm.ID == "pentecost-sunday-1" || comm.ID == "pentecost-sunday-2" {
					continue
				}
				hour, err := eng.ComposeHour("lauds", day, yd.moveable)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, sec := range hour.Sections {
					for _, elem := range sec.Elements {
						if elem.CommemorationOwnerID == comm.ID && elem.SlotRef == "commemoration-versicle" {
							found = true
							checked++
							if elem.SourceRef != "ordinary/lauds/versicle-sunday" {
								t.Errorf("Diurnal pp. xxix, 41: %s %s uses %s", day.Date.Format("2006-01-02"), comm.ID, elem.SourceRef)
							}
						}
					}
				}
				if !found {
					t.Errorf("missing appointed Sunday commemoration %s on %s", comm.ID, day.Date)
				}
			}
		}
		if checked == 0 {
			t.Errorf("%d: no Sunday commemoration cases exercised", year)
		}
		t.Logf("%d: %d Sunday commemoration versicles checked", year, checked)
	}
}
