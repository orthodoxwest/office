package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestResolveProperTextCommons(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"commons/martyr/antiphon":       "Martyr common antiphon",
		"commons/martyr/antiphon-lauds": "Martyr common lauds antiphon",
		"ordinary/lauds/antiphon":       "Ordinary antiphon",
	})

	t.Run("commons hour-qualified beats generic", func(t *testing.T) {
		day := &models.CalendarDay{
			Date:        time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			Season:      models.Lent,
			Celebration: &models.Feast{ID: "unknown-feast", Category: models.CategoryMartyr},
		}
		got, _ := resolveProperText(day, "lauds", "antiphon", corpus)
		if got != "Martyr common lauds antiphon" {
			t.Errorf("got %q, want hour-qualified commons text", got)
		}
	})

	t.Run("commons generic when no hour-qualified", func(t *testing.T) {
		corpus2 := texts.NewTestCorpus(map[string]string{
			"commons/martyr/antiphon": "Martyr common antiphon",
			"ordinary/lauds/antiphon": "Ordinary antiphon",
		})
		day := &models.CalendarDay{
			Date:        time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			Season:      models.Lent,
			Celebration: &models.Feast{ID: "unknown-feast", Category: models.CategoryMartyr},
		}
		got, _ := resolveProperText(day, "lauds", "antiphon", corpus2)
		if got != "Martyr common antiphon" {
			t.Errorf("got %q, want generic commons text", got)
		}
	})
}

func TestResolveProperTextHourQualified(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/christmas/antiphon-lauds": "Christmas lauds antiphon",
		"proper/christmas/antiphon":       "Christmas generic antiphon",
		"ordinary/lauds/antiphon":         "Ordinary antiphon",
	})

	day := &models.CalendarDay{
		Date:        time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC),
		Season:      models.Christmas,
		Celebration: &models.Feast{ID: "christmas"},
	}

	got, _ := resolveProperText(day, "lauds", "antiphon", corpus)
	if got != "Christmas lauds antiphon" {
		t.Errorf("got %q, want hour-qualified proper", got)
	}
}

func TestResolveProperTextUsesProperIDAlias(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/pentecost-sunday-23/collect": "XXIII Pentecost collect",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC),
		Season: models.Epiphany,
		Celebration: &models.Feast{
			ID:       "epiphany-sunday-7",
			ProperID: "pentecost-sunday-23",
			Category: models.CategorySunday,
		},
	}

	got, ref := resolveProperText(day, "lauds", "collect", corpus)
	if got != "XXIII Pentecost collect" {
		t.Fatalf("got %q, want ProperID-backed proper text", got)
	}
	if ref != "proper/pentecost-sunday-23/collect" {
		t.Fatalf("ref = %q, want proper/pentecost-sunday-23/collect", ref)
	}
}

func TestResolveProperTextWeekdayOrdinary(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/hymn-monday": "Monday lauds hymn",
		"ordinary/lauds/hymn":        "Generic lauds hymn",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), // Monday
		Season: models.Lent,
	}

	got, _ := resolveProperText(day, "lauds", "hymn", corpus)
	if got != "Monday lauds hymn" {
		t.Errorf("got %q, want weekday ordinary text", got)
	}
}

func TestResolveProperTextSharedFallback(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/shared/collect": "Shared collect",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
	}

	got, _ := resolveProperText(day, "lauds", "collect", corpus)
	if got != "Shared collect" {
		t.Errorf("got %q, want shared ordinary fallback", got)
	}
}

func TestResolveProperTextNotFound(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
	}

	got, ref := resolveProperText(day, "lauds", "missing-ref", corpus)
	if got != "[Proper text not found: missing-ref]" {
		t.Errorf("got %q, want not-found placeholder", got)
	}
	if ref != "missing-ref" {
		t.Errorf("ref = %q, want %q", ref, "missing-ref")
	}
}

func TestSubstituteProperName(t *testing.T) {
	tests := []struct {
		name, text, propName, want string
	}{
		{"empty name is no-op", "feast of N. the saint", "", "feast of N. the saint"},
		{"replaces N.", "feast of N. the saint", "Ambrose", "feast of Ambrose the saint"},
		{"multiple occurrences", "N. said to N. pray", "Benedict", "Benedict said to Benedict pray"},
		{"no placeholder", "no placeholder here", "Ambrose", "no placeholder here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteProperName(tt.text, tt.propName)
			if got != tt.want {
				t.Errorf("substituteProperName(%q, %q) = %q, want %q", tt.text, tt.propName, got, tt.want)
			}
		})
	}
}

func TestResolveProperTextNSubstitution(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"commons/confessor/collect": "O God, bless N. thy confessor",
		"ordinary/lauds/antiphon":   "O holy N., pray for us",
	})

	t.Run("substitutes N. from commons", func(t *testing.T) {
		day := &models.CalendarDay{
			Date:        time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
			Season:      models.Lent,
			Celebration: &models.Feast{ID: "st-benedict", Category: models.CategoryConfessor, ProperName: "Benedict"},
		}
		got, _ := resolveProperText(day, "lauds", "collect", corpus)
		if got != "O God, bless Benedict thy confessor" {
			t.Errorf("got %q, want N. replaced with Benedict", got)
		}
	})

	t.Run("no substitution when ProperName empty", func(t *testing.T) {
		day := &models.CalendarDay{
			Date:        time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
			Season:      models.Lent,
			Celebration: &models.Feast{ID: "st-benedict", Category: models.CategoryConfessor},
		}
		got, _ := resolveProperText(day, "lauds", "collect", corpus)
		if got != "O God, bless N. thy confessor" {
			t.Errorf("got %q, want N. preserved", got)
		}
	})

	t.Run("substitutes N. from ordinary fallback", func(t *testing.T) {
		day := &models.CalendarDay{
			Date:        time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
			Season:      models.Pentecost,
			Celebration: &models.Feast{ID: "st-unknown", ProperName: "Patrick"},
		}
		got, _ := resolveProperText(day, "lauds", "antiphon", corpus)
		if got != "O holy Patrick, pray for us" {
			t.Errorf("got %q, want N. replaced in ordinary", got)
		}
	})
}

func TestResolveProperTextPaschalCommons(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"commons/apostle-paschal/antiphon": "Paschal apostle antiphon, alleluia",
		"commons/apostle/antiphon":         "Regular apostle antiphon",
		"commons/apostle/collect":          "Regular apostle collect",
	})

	t.Run("paschal commons preferred during Easter", func(t *testing.T) {
		day := &models.CalendarDay{
			Date:        time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			Season:      models.Easter,
			Celebration: &models.Feast{ID: "st-mark", Category: models.CategoryApostle},
		}
		got, _ := resolveProperText(day, "lauds", "antiphon", corpus)
		if got != "Paschal apostle antiphon, alleluia" {
			t.Errorf("got %q, want paschal commons", got)
		}
	})

	t.Run("falls through to regular commons when no paschal entry", func(t *testing.T) {
		day := &models.CalendarDay{
			Date:        time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			Season:      models.Easter,
			Celebration: &models.Feast{ID: "st-mark", Category: models.CategoryApostle},
		}
		got, _ := resolveProperText(day, "lauds", "collect", corpus)
		if got != "Regular apostle collect" {
			t.Errorf("got %q, want regular commons fallthrough", got)
		}
	})

	t.Run("ignores paschal commons outside Easter", func(t *testing.T) {
		day := &models.CalendarDay{
			Date:        time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			Season:      models.Lent,
			Celebration: &models.Feast{ID: "st-mark", Category: models.CategoryApostle},
		}
		got, _ := resolveProperText(day, "lauds", "antiphon", corpus)
		if got != "Regular apostle antiphon" {
			t.Errorf("got %q, want regular commons (not paschal)", got)
		}
	})
}

func TestResolveProperTextSeasonalHourQualified(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"seasonal/lent/antiphon-lauds": "Lent lauds antiphon",
		"seasonal/lent/antiphon":       "Lent generic antiphon",
		"ordinary/lauds/antiphon":      "Ordinary antiphon",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
	}

	got, _ := resolveProperText(day, "lauds", "antiphon", corpus)
	if got != "Lent lauds antiphon" {
		t.Errorf("got %q, want hour-qualified seasonal text", got)
	}
}

func TestResolveProperTextIndexedRefFallsBackToBaseRef(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-mark/psalm-antiphon":           "Generic proper antiphon",
		"commons/apostle/psalm-antiphon":          "Generic common antiphon",
		"ordinary/lauds/psalm-antiphon":           "Generic ordinary antiphon",
		"ordinary/lauds/psalm-antiphon-2-sunday":  "Weekday-specific indexed antiphon",
		"ordinary/vespers/psalm-antiphon":         "Generic vespers antiphon",
		"seasonal/pentecost/psalm-antiphon":       "Generic seasonal antiphon",
		"seasonal/pentecost/psalm-antiphon-lauds": "Hour-qualified seasonal antiphon",
	})

	t.Run("proper generic backs indexed proper ref", func(t *testing.T) {
		day := &models.CalendarDay{
			Season:      models.Pentecost,
			Celebration: &models.Feast{ID: "st-mark"},
		}
		got, ref := resolveProperText(day, "lauds", "psalm-antiphon-2", corpus)
		if got != "Generic proper antiphon" {
			t.Fatalf("got %q, want proper generic fallback", got)
		}
		if ref != "proper/st-mark/psalm-antiphon" {
			t.Fatalf("ref = %q, want proper/st-mark/psalm-antiphon", ref)
		}
	})

	t.Run("commons generic backs indexed common ref", func(t *testing.T) {
		day := &models.CalendarDay{
			Season:      models.Lent,
			Celebration: &models.Feast{ID: "st-john", Category: models.CategoryApostle},
		}
		got, ref := resolveProperText(day, "lauds", "psalm-antiphon-2", corpus)
		if got != "Generic common antiphon" {
			t.Fatalf("got %q, want commons generic fallback", got)
		}
		if ref != "commons/apostle/psalm-antiphon" {
			t.Fatalf("ref = %q, want commons/apostle/psalm-antiphon", ref)
		}
	})

	t.Run("seasonal generic backs indexed seasonal ref", func(t *testing.T) {
		day := &models.CalendarDay{Season: models.Pentecost}
		got, ref := resolveProperText(day, "vespers", "psalm-antiphon-2", corpus)
		if got != "Generic seasonal antiphon" {
			t.Fatalf("got %q, want seasonal generic fallback", got)
		}
		if ref != "seasonal/pentecost/psalm-antiphon" {
			t.Fatalf("ref = %q, want seasonal/pentecost/psalm-antiphon", ref)
		}
	})
}

func TestResolveProperCollectTextMinorHoursReuseLauds(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/collect": "Lauds collect",
		"ordinary/terce/collect": "Terce collect",
		"ordinary/sext/collect":  "Sext collect",
		"ordinary/none/collect":  "None collect",
		"ordinary/prime/collect": "Prime collect",
	})

	day := &models.CalendarDay{Season: models.Lent}

	for _, hour := range []string{"terce", "sext", "none"} {
		t.Run(hour, func(t *testing.T) {
			got, ref := resolveProperCollectText(day, hour, corpus)
			if got != "Lauds collect" {
				t.Fatalf("got %q, want Lauds collect", got)
			}
			if ref != "ordinary/lauds/collect" {
				t.Fatalf("ref = %q, want ordinary/lauds/collect", ref)
			}
		})
	}

	got, ref := resolveProperCollectText(day, "prime", corpus)
	if got != "Prime collect" {
		t.Fatalf("prime got %q, want Prime collect", got)
	}
	if ref != "ordinary/prime/collect" {
		t.Fatalf("prime ref = %q, want ordinary/prime/collect", ref)
	}
}

func TestResolveProperTextEasterHourQualifiedOverrides(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"seasonal/easter/short-responsory-lauds":   "Paschal lauds short responsory",
		"seasonal/easter/short-responsory-vespers": "Paschal vespers short responsory",
		"seasonal/easter/versicle-lauds":           "Paschal lauds versicle",
		"seasonal/easter/versicle-vespers":         "Paschal vespers versicle",
		"ordinary/lauds/short-responsory":          "Ordinary lauds short responsory",
		"ordinary/vespers/short-responsory":        "Ordinary vespers short responsory",
		"ordinary/lauds/versicle":                  "Ordinary lauds versicle",
		"ordinary/vespers/versicle":                "Ordinary vespers versicle",
	})

	day := &models.CalendarDay{Season: models.Easter}

	got, ref := resolveProperText(day, "lauds", "short-responsory", corpus)
	if got != "Paschal lauds short responsory" || ref != "seasonal/easter/short-responsory-lauds" {
		t.Fatalf("lauds short-responsory = %q (%s), want seasonal hour-qualified override", got, ref)
	}

	got, ref = resolveProperText(day, "vespers", "short-responsory", corpus)
	if got != "Paschal vespers short responsory" || ref != "seasonal/easter/short-responsory-vespers" {
		t.Fatalf("vespers short-responsory = %q (%s), want seasonal hour-qualified override", got, ref)
	}

	got, ref = resolveProperText(day, "lauds", "versicle", corpus)
	if got != "Paschal lauds versicle" || ref != "seasonal/easter/versicle-lauds" {
		t.Fatalf("lauds versicle = %q (%s), want seasonal hour-qualified override", got, ref)
	}

	got, ref = resolveProperText(day, "vespers", "versicle", corpus)
	if got != "Paschal vespers versicle" || ref != "seasonal/easter/versicle-vespers" {
		t.Fatalf("vespers versicle = %q (%s), want seasonal hour-qualified override", got, ref)
	}
}
