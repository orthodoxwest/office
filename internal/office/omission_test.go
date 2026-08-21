package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// The Triduum omits the chapter, short responsory and hymn outright (Monastic
// Diurnal p. 311), and Vespers omits the versicle too (p. 315). Without an
// omission idiom the resolver would walk on to the seasonal tier and always
// land on something, so the corpus could not say "not said here".
func TestOmittedElementIsDroppedInsteadOfFallingThrough(t *testing.T) {
	feast := &models.Feast{ID: "good-friday", Name: "Good Friday", Category: models.CategoryLord}
	day := makeDay(2026, time.April, 10, feast, nil, "")
	day.Season = models.Passiontide

	corpus := texts.NewTestCorpus(map[string]string{
		"proper/good-friday/chapter":   texts.OmitMarker,
		"seasonal/passiontide/chapter": "!Jer 11:20\nBut, O Lord of hosts…",
		"ordinary/vespers/chapter":     "The ordinary chapter.",
	})

	elem := HourElement{Type: "proper-chapter", Ref: "chapter"}

	got := appendHourElement(nil, day, "vespers", elem, corpus)
	if len(got) != 0 {
		t.Errorf("appendHourElement kept %d element(s), want 0: %+v", len(got), got)
	}

	// The same slot on a day with no omission still resolves normally.
	ordinary := makeDay(2026, time.April, 8, nil, nil, "")
	ordinary.Season = models.Passiontide
	got = appendHourElement(nil, ordinary, "vespers", elem, corpus)
	if len(got) != 1 {
		t.Fatalf("appendHourElement dropped an element that is said: %+v", got)
	}
	if got[0].Label != "Jer 11:20" {
		t.Errorf("chapter label = %q, want %q", got[0].Label, "Jer 11:20")
	}
}

// The Little Hours resolve their versicle from the short responsory outside the
// hour definitions, so they need their own drop: without it an omitted
// responsory reached shortResponsoryVersicle and rendered as a "[Little Hours
// versicle not found]" marker in the office.
func TestOmittedShortResponsoryDropsTheLittleHoursVersicle(t *testing.T) {
	feast := &models.Feast{ID: "good-friday", Name: "Good Friday", Category: models.CategoryLord}
	day := makeDay(2026, time.April, 10, feast, nil, "")
	day.Season = models.Passiontide

	corpus := texts.NewTestCorpus(map[string]string{
		"proper/good-friday/short-responsory-terce": texts.OmitMarker,
		"ordinary/terce/short-responsory":           "R. Ordinary. * Responsory.",
	})

	got := appendResolved(nil, resolveMinorHourVersicle(day, "terce", corpus))
	if len(got) != 0 {
		t.Errorf("Little Hours versicle survived an omitted responsory: %+v", got)
	}
}

// A section whose every element is omitted must not render as a bare heading.
func TestSectionWithOnlyOmittedElementsIsDropped(t *testing.T) {
	feast := &models.Feast{ID: "good-friday", Name: "Good Friday", Category: models.CategoryLord}
	day := makeDay(2026, time.April, 10, feast, nil, "")
	day.Season = models.Passiontide

	corpus := texts.NewTestCorpus(map[string]string{
		"proper/good-friday/chapter":             texts.OmitMarker,
		"proper/good-friday/short-responsory":    texts.OmitMarker,
		"proper/good-friday/magnificat-antiphon": "When he had received the vinegar…",
	})

	sections := []HourSection{
		{
			Name:  "Chapter",
			Label: "The Chapter",
			Elements: []HourElement{
				{Type: "proper-chapter", Ref: "chapter"},
				{Type: "proper-short-responsory", Ref: "short-responsory"},
			},
		},
		{
			Name:     "Magnificat",
			Label:    "The Gospel Canticle",
			Elements: []HourElement{{Type: "proper-antiphon", Ref: "magnificat-antiphon"}},
		},
	}

	hour, err := composeMajorHour(day, sections, corpus, nil, majorHourOptions{hourName: "vespers", title: "Vespers"})
	if err != nil {
		t.Fatalf("composeMajorHour: %v", err)
	}
	dropEmptySections(hour) // as ComposeHour does for every composer

	if len(hour.Sections) != 1 {
		t.Fatalf("got %d sections, want 1: %+v", len(hour.Sections), hour.Sections)
	}
	if hour.Sections[0].Label != "The Gospel Canticle" {
		t.Errorf("surviving section = %q, want %q", hour.Sections[0].Label, "The Gospel Canticle")
	}
}
