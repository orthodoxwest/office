package audit

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestNotFoundMarkerRE(t *testing.T) {
	cases := []struct {
		text  string
		match bool
	}{
		{"[Text not found: ordinary/lauds/hymn]", true},
		{"[Proper text not found: chapter]", true},
		{"[commemoration-antiphon: st-anthony]", true},
		{"[commemoration-collect: octave-st-john-baptist]", true},
		// canticle section markup must not match
		{"[section: Ignis succensus est]", false},
		{"Blessed art thou, O Lord [Benedictus es]", false},
		{"plain antiphon text", false},
	}
	for _, c := range cases {
		if got := notFoundMarkerRE.MatchString(c.text); got != c.match {
			t.Errorf("notFoundMarkerRE(%q) = %v, want %v", c.text, got, c.match)
		}
	}
}

func TestTrimIndexSuffix(t *testing.T) {
	cases := map[string]string{
		"psalm-antiphon-3":    "psalm-antiphon",
		"psalm-antiphon":      "psalm-antiphon",
		"hymn":                "hymn",
		"easter-antiphon-1":   "easter-antiphon",
		"benedictus-antiphon": "benedictus-antiphon",
		"psalms/067":          "psalms/067", // digits preceded by "/", not "-"
	}
	for in, want := range cases {
		if got := trimIndexSuffix(in); got != want {
			t.Errorf("trimIndexSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSweepFeastVespersOwner(t *testing.T) {
	celebration := &models.Feast{ID: "today"}
	following := &models.Feast{ID: "tomorrow"}
	day := &models.CalendarDay{
		Celebration: celebration,
		Vespers: models.VespersDesignation{
			Owner: models.VespersIOfFollowing,
			Feast: following,
		},
	}
	if got := sweepFeast(day, "vespers"); got != following {
		t.Errorf("sweepFeast(vespers) = %v, want following feast", got)
	}
	if got := sweepFeast(day, "lauds"); got != celebration {
		t.Errorf("sweepFeast(lauds) = %v, want celebration", got)
	}
}
