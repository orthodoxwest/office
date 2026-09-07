package office

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

// The 2024-2026 ordos prescribe Quicumque vult at Trinity Sunday Prime,
// after the last psalm's Gloria and before the repeated antiphon.
func TestAthanasianCreedAtPrime(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	engine, err := NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	const key = "proper/trinity-sunday/athanasian-creed"
	for _, year := range []int{2024, 2025, 2026} {
		days, err := calendar.BuildCalendar(year, dataDir)
		if err != nil {
			t.Fatal(err)
		}
		moveable := calendar.ComputeMoveableDates(year)
		count := 0
		for i := range days {
			day := &days[i]
			hour, err := engine.ComposeHour("prime", day, moveable)
			if err != nil {
				t.Fatal(err)
			}
			var elements []models.OfficeElement
			for _, section := range hour.Sections {
				elements = append(elements, section.Elements...)
			}
			found := 0
			for j, element := range elements {
				if strings.Contains(element.Text, "On Sundays and feasts outside of Eastertide") {
					t.Fatalf("%s: obsolete rubric remains", day.Date)
				}
				if element.SourceRef != key {
					continue
				}
				found++
				if j < 2 || j+2 >= len(elements) {
					t.Fatalf("%s: creed misplaced", day.Date)
				}
				if elements[j-2].SourceRef != "psalms/119-iv" || elements[j-1].Type != models.PsalmDoxology || elements[j+1].Type != models.PsalmDoxology || elements[j+2].Type != models.Antiphon || elements[j+2].Announce {
					t.Fatalf("%s: want last psalm, Gloria, creed, Gloria, full antiphon", day.Date)
				}
				if !strings.Contains(element.Text, "The Holy Ghost is of the Father through the Son:") || strings.Contains(element.Text, "The Holy Ghost is of the Father and of the Son:") {
					t.Fatal("creed lacks ordo's verse 23 correction")
				}
				lines := strings.Split(strings.TrimSpace(element.Text), "\n")
				if len(lines) != 42 {
					t.Fatalf("creed has %d verses, want 42", len(lines))
				}
				for n, line := range lines {
					if !strings.HasPrefix(line, fmt.Sprintf("%d. ", n+1)) {
						t.Fatalf("verse %d missing or out of order", n+1)
					}
				}
			}
			want := 0
			if day.Celebration != nil && day.Celebration.ID == "trinity-sunday" {
				want = 1
			}
			if found != want {
				t.Fatalf("%s: %d creeds, want %d", day.Date, found, want)
			}
			count += found
		}
		if count != 1 {
			t.Fatalf("%d: creed appears %d times, want once", year, count)
		}
	}
}
