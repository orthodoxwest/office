package office

import (
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestSundayNearestOctober1(t *testing.T) {
	// 2026: Oct 1 is a Thursday → nearest Sunday is Oct 4.
	if got := sundayNearestOctober1(2026); !got.Equal(date("2026-10-04")) {
		t.Errorf("2026 = %s, want 2026-10-04", got.Format("2006-01-02"))
	}
	// 2024: Oct 1 is a Tuesday → preceding Sunday Sep 29 is nearer.
	if got := sundayNearestOctober1(2024); !got.Equal(date("2024-09-29")) {
		t.Errorf("2024 = %s, want 2024-09-29", got.Format("2006-01-02"))
	}
}

func TestSundayLaudsHymnIsSummer(t *testing.T) {
	// 2026 summer window: [II Sunday after Trinity 2026-06-21, Sunday nearest
	// Oct 1 2026-10-04).
	tests := []struct {
		date string
		want bool
		why  string
	}{
		{"2026-06-20", false, "day before the II Sunday after Trinity"},
		{"2026-06-21", true, "II Sunday after Trinity — window opens (inclusive)"},
		{"2026-06-28", true, "mid-summer green Sunday"},
		{"2026-09-27", true, "last Sunday before the autumn Sunday"},
		{"2026-10-04", false, "Sunday nearest Oct 1 — window closes (exclusive)"},
		{"2026-11-01", false, "autumn — winter hymn resumes"},
		{"2026-02-08", false, "Septuagesima — winter window"},
		{"2026-06-07", false, "Trinity Sunday — before the II Sunday"},
	}
	for _, tt := range tests {
		if got := sundayLaudsHymnIsSummer(date(tt.date)); got != tt.want {
			t.Errorf("sundayLaudsHymnIsSummer(%s) = %v, want %v (%s)", tt.date, got, tt.want, tt.why)
		}
	}
}
