package review

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

func sampleHour(date time.Time, collectText string) *models.OfficeHour {
	return &models.OfficeHour{
		Date:   date,
		Hour:   "lauds",
		Title:  "Lauds",
		Season: models.Pentecost,
		Feast:  "Trinity Sunday",
		Color:  models.White,
		Sections: []models.OfficeSection{
			{
				Label: "The Collect",
				Elements: []models.OfficeElement{
					{Type: models.Collect, Text: collectText},
				},
			},
		},
	}
}

func TestHashHourExcludesDate(t *testing.T) {
	a := sampleHour(time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), "Almighty and everlasting God...")
	b := sampleHour(time.Date(2027, 6, 27, 0, 0, 0, 0, time.UTC), "Almighty and everlasting God...")
	if HashHour(a) != HashHour(b) {
		t.Error("identical compositions on different dates should hash the same")
	}
}

func TestHashHourIncludesContent(t *testing.T) {
	date := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	a := sampleHour(date, "Almighty and everlasting God...")
	b := sampleHour(date, "O God, whose never-failing providence...")
	if HashHour(a) == HashHour(b) {
		t.Error("compositions with different texts should hash differently")
	}

	c := sampleHour(date, "Almighty and everlasting God...")
	c.Sections[0].Elements[0].Rubric = "Said kneeling."
	if HashHour(a) == HashHour(c) {
		t.Error("compositions with different rubrics should hash differently")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Trinity Sunday":              "trinity-sunday",
		"XXII Sunday after Pentecost": "xxii-sunday-after-pentecost",
		"St. Mary's  Feast":           "st-mary-s-feast",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildManifestSweep(t *testing.T) {
	m, err := BuildManifest("../../data", 2026, 1)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	dayHours := 365 * len(HourNames)
	if len(m.Units) == 0 || len(m.Units) >= dayHours {
		t.Fatalf("expected dedup: got %d units for %d day-hours", len(m.Units), dayHours)
	}

	hashes := make(map[string]bool, len(m.Units))
	occurrences := 0
	foundTrinity := false
	for i := range m.Units {
		u := &m.Units[i]
		if hashes[u.Hash] {
			t.Fatalf("duplicate hash %s in manifest", u.Hash)
		}
		hashes[u.Hash] = true
		occurrences += u.Occurrences
		if u.UnitKey == "trinity-sunday" && u.Hour == "lauds" {
			foundTrinity = true
			if u.Priority() != "A" {
				t.Errorf("Trinity Sunday lauds priority = %q, want A", u.Priority())
			}
			if !strings.HasPrefix(u.URL(), "/lauds/2026-") {
				t.Errorf("unexpected representative URL %q", u.URL())
			}
		}
	}
	if occurrences != dayHours {
		t.Errorf("occurrences sum = %d, want %d", occurrences, dayHours)
	}
	if !foundTrinity {
		t.Error("expected a trinity-sunday lauds unit in the 2026 sweep")
	}
}
