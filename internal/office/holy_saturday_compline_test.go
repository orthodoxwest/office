package office

import (
	"fmt"
	"slices"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// Diurnal pp. 361–362, appointed by the 2026 ordo p. 53: the Nunc dimittis
// has the same antiphon as Vigil Vespers (p. 360), followed immediately by
// the greeting and collect, without a Kyrie. Compline retains Easter's
// evening ownership and its own opening, psalms and Marian conclusion.
func TestHolySaturdayComplineAppointments(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	const antiphon = "In the end of the sabbath, * as it began to dawn toward the first day of the week, came Mary Magdalene and the other Mary to see the sepulchre, alleluia."
	const source = "proper/holy-saturday/nunc-dimittis-antiphon"
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		moveable := calendar.ComputeMoveableDates(year)
		found := 0
		for i := range days {
			day := &days[i]
			if day.Celebration == nil || day.Celebration.ID != "holy-saturday" {
				continue
			}
			found++
			for _, form := range models.PrayerForms {
				t.Run(fmt.Sprintf("%d/%s", year, form), func(t *testing.T) {
					hour, err := engine.ComposeHourWithOptions("compline", day, moveable, ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					if hour.Season != models.Easter || hour.Color != models.White || hour.Feast != day.Vespers.Feast.Name || !hour.Date.Equal(day.Date) {
						t.Fatalf("Compline lost Easter ownership or civil date: %+v", hour)
					}
					var refs, psalms []string
					antiphons, doxologies, marian := 0, 0, 0
					for _, section := range hour.Sections {
						for _, e := range section.Elements {
							refs = append(refs, e.SourceRef)
							if e.SourceRef == source {
								antiphons++
								if e.Text != antiphon || !slices.Equal(e.SourceRefs, []string{"proper/holy-saturday/vigil-magnificat-antiphon"}) {
									t.Errorf("wrong Nunc antiphon or canonical source: %+v", e)
								}
							}
							if e.Type == models.Psalm {
								psalms = append(psalms, e.SourceRef)
							}
							if e.Type == models.PsalmDoxology {
								doxologies++
							}
							if e.SourceRef == "ordinary/marian/regina-caeli" {
								marian++
							}
							if e.SourceRef == "ordinary/compline/conclusion" && e.Text != "V. May the divine help remain with us always.\nR. And with our absent brethren. Amen." {
								t.Errorf("wrong inherited Marian conclusion: %q", e.Text)
							}
							if e.SourceRef == "ordinary/shared/kyrie" || e.Type == models.Hymn || e.SlotRef == "chapter" || e.SlotRef == "short-responsory" {
								t.Errorf("unexpected ordinary middle section: %s", e.SourceRef)
							}
						}
					}
					if antiphons != 2 || doxologies != 4 || marian != 1 || !slices.Equal(psalms, []string{"psalms/004", "psalms/091", "psalms/134"}) {
						t.Errorf("antiphons=%d doxologies=%d Marian=%d psalms=%v", antiphons, doxologies, marian, psalms)
					}
					greeting := "shared/leader/greeting-clergy"
					if form == models.PrayerPrivate {
						greeting = "shared/leader/greeting-lay"
					}
					want := []string{source, "canticles/nunc-dimittis", "ordinary/shared/gloria-patri", source, greeting, "shared/leader/let-us-pray", "ordinary/compline/collect"}
					start := slices.Index(refs, source)
					if start < 0 || start+len(want) > len(refs) || !slices.Equal(refs[start:start+len(want)], want) {
						t.Errorf("want uninterrupted canticle-to-collect sequence %v; got %v", want, refs)
					}
					// The Marian reference includes its psalter conclusion (p. 156).
					marianIndex := slices.Index(refs, "ordinary/marian/regina-caeli")
					if marianIndex < 0 || marianIndex+1 >= len(refs) || refs[marianIndex+1] != "ordinary/compline/conclusion" {
						t.Errorf("Marian antiphon lost its following conclusion: %v", refs)
					}
				})
				for _, neighbor := range []*models.CalendarDay{&days[i-1], &days[i+1]} {
					hour, err := engine.ComposeHourWithOptions("compline", neighbor, moveable, ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					for _, section := range hour.Sections {
						for _, e := range section.Elements {
							if e.SourceRef == source {
								t.Errorf("%s %s: Holy Saturday antiphon leaked into adjacent Compline", neighbor.Date, form)
							}
						}
					}
				}
			}
		}
		if found != 1 {
			t.Errorf("%d: got %d Holy Saturdays, want one", year, found)
		}
	}
}
