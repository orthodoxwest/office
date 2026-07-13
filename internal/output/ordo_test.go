package output

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

func TestFormatDayUsesPrintedPrivilegedOctaveRank(t *testing.T) {
	day := &models.CalendarDay{
		Date:  time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		Color: models.White,
		Celebration: &models.Feast{
			ID:   "easter-sunday-octave-day-4",
			Name: "Day IV within the Octave of Easter",
			Rank: models.Double1stClass,
		},
	}

	got := FormatDay(day, nil, nil)
	if !strings.Contains(got, "Day IV within the Octave of Easter") || !strings.Contains(got, "[sd]") {
		t.Fatalf("FormatDay() = %q, want title-case name and [sd]", got)
	}
	if day.Celebration.Rank != models.Double1stClass {
		t.Fatalf("FormatDay changed internal rank to %q", day.Celebration.Rank)
	}
}

func TestFormatDayCorpusChristiOctaveSundayHeadline(t *testing.T) {
	day := &models.CalendarDay{
		Date:  time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC),
		Color: models.White,
		Celebration: &models.Feast{
			ID:   "pentecost-sunday-2",
			Name: "Sunday within the Octave of Corpus Christi",
			Rank: models.SemiDouble,
		},
	}

	lines := strings.Split(FormatDay(day, nil, nil), "\n")
	if len(lines) != 2 {
		t.Fatalf("FormatDay() lines = %d, want 2: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "Sunday within the Octave of Corpus Christi") {
		t.Fatalf("primary headline = %q", lines[0])
	}
	if !strings.Contains(lines[1], "II Sunday after Pentecost") {
		t.Fatalf("secondary headline = %q", lines[1])
	}
}
