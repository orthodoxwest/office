package render

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

func TestComposedPrecesCreedRendersSilentMiddleAndSpokenTail(t *testing.T) {
	engine, err := office.NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	day := &models.CalendarDay{Date: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), Season: models.Pentecost}
	for _, name := range []string{"prime", "compline"} {
		h, err := engine.ComposeHour(name, day, calendar.ComputeMoveableDates(2026))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, section := range h.Sections {
			if section.Label != "Preces" {
				continue
			}
			for _, e := range section.Elements {
				if e.SourceRef != "ordinary/shared/apostles-creed" {
					continue
				}
				found = true
				html := renderOfficeElement(e, "")
				for _, want := range []string{`class="spoken-text">I believe</span>`, `class="secret-text"> in God`, `class="spoken-text">The Resurrection of the body`, `And the Life everlasting. Amen.</span>`} {
					if !strings.Contains(html, want) {
						t.Errorf("%s missing %q: %s", name, want, html)
					}
				}
			}
		}
		if !found {
			t.Fatalf("%s missing Creed", name)
		}
	}
}
