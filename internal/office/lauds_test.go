package office

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestLaudsWeekdayCondition(t *testing.T) {
	tests := []struct {
		name      string
		date      time.Time
		condition string
		want      bool
	}{
		{"Sunday matches weekday-sunday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-sunday", true},
		{"Sunday does not match weekday-monday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-monday", false},
		{"Monday matches weekday-monday", time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), "weekday-monday", true},
		{"Saturday matches weekday-saturday", time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC), "weekday-saturday", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date}
			got := evaluateCondition(tt.condition, day, nil)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestLaudsPreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "ferial day — preces",
			day:  makeDay(2026, 3, 16, nil, nil, ""),
			want: true,
		},
		{
			name: "double feast — no preces",
			day:  makeDay(2026, 3, 15, &models.Feast{ID: "test", Rank: models.Double}, nil, ""),
			want: false,
		},
		{
			name: "within octave — no preces",
			day:  makeDay(2026, 4, 13, nil, nil, "easter-sunday"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition("if-preces", tt.day, moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(if-preces) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveProperText(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/psalm-antiphon":      "Ordinary antiphon",
		"seasonal/lent/psalm-antiphon":       "Lenten antiphon",
		"proper/christmas/psalm-antiphon":    "Christmas antiphon",
		"ordinary/lauds/benedictus-antiphon": "Ordinary Benedictus",
	})

	tests := []struct {
		name    string
		day     *models.CalendarDay
		ref     string
		wantTxt string
	}{
		{
			name:    "feast-specific proper wins",
			day:     &models.CalendarDay{Season: models.Lent, Celebration: &models.Feast{ID: "christmas"}},
			ref:     "psalm-antiphon",
			wantTxt: "Christmas antiphon",
		},
		{
			name:    "seasonal fallback when no feast proper",
			day:     &models.CalendarDay{Season: models.Lent, Celebration: &models.Feast{ID: "unknown-feast"}},
			ref:     "psalm-antiphon",
			wantTxt: "Lenten antiphon",
		},
		{
			name:    "ordinary fallback when no seasonal",
			day:     &models.CalendarDay{Season: models.Easter, Celebration: &models.Feast{ID: "unknown-feast"}},
			ref:     "psalm-antiphon",
			wantTxt: "Ordinary antiphon",
		},
		{
			name:    "ordinary fallback with no celebration",
			day:     &models.CalendarDay{Season: models.Pentecost},
			ref:     "benedictus-antiphon",
			wantTxt: "Ordinary Benedictus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTxt, _ := resolveProperText(tt.day, "lauds", tt.ref, corpus)
			if gotTxt != tt.wantTxt {
				t.Errorf("resolveProperText() text = %q, want %q", gotTxt, tt.wantTxt)
			}
		})
	}
}

func TestLaudsCommemorations(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/commemoration-antiphon":   "Default antiphon",
		"ordinary/lauds/commemoration-versicle":   "Default versicle",
		"ordinary/lauds/commemoration-collect":    "Default collect",
		"proper/st-andrew/commemoration-antiphon": "Andrew antiphon",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Commemorations: []*models.Feast{
			{ID: "st-andrew", Name: "St. Andrew"},
		},
	}

	elems := addCommemorations(day, "lauds", corpus)

	if len(elems) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(elems))
	}

	// Heading
	if elems[0].Type != models.Heading {
		t.Errorf("element 0: want Heading, got %s", elems[0].Type)
	}
	if elems[0].Text != "Commemoration of St. Andrew" {
		t.Errorf("element 0 text = %q", elems[0].Text)
	}

	// Antiphon — should use feast-specific
	if elems[1].Text != "Andrew antiphon" {
		t.Errorf("element 1 text = %q, want feast-specific antiphon", elems[1].Text)
	}

	// Versicle — should use default
	if elems[2].Text != "Default versicle" {
		t.Errorf("element 2 text = %q, want default versicle", elems[2].Text)
	}

	// Collect — should use default
	if elems[3].Text != "Default collect" {
		t.Errorf("element 3 text = %q, want default collect", elems[3].Text)
	}
}

func TestLaudsFeriaCommemoration(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/benedictus-antiphon":    "Ferial Benedictus antiphon",
		"ordinary/lauds/versicle":               "Ferial versicle",
		"ordinary/vespers/magnificat-antiphon":  "Ferial Magnificat antiphon",
		"ordinary/vespers/versicle":             "Ferial vespers versicle",
		"proper/lent-sunday-3/collect":          "Collect of the preceding Sunday",
		"ordinary/lauds/commemoration-antiphon": "Saint antiphon N.",
		"ordinary/lauds/commemoration-versicle": "Saint versicle",
		"ordinary/lauds/commemoration-collect":  "Saint collect",
	})

	day := &models.CalendarDay{
		Date:        time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
		Season:      models.Lent,
		Celebration: &models.Feast{ID: "st-benedict", Name: "St. Benedict", Rank: models.Double2ndClass},
		FeriaCommemoration: &models.Feast{
			ID:       "penitential-feria",
			Name:     "Saturday after Lent III",
			Rank:     models.Commemoration,
			Category: models.CategoryFeria,
			ProperID: "lent-sunday-3",
		},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if len(elems) != 4 {
		t.Fatalf("expected 4 elements (feria commemoration), got %d", len(elems))
	}
	if elems[0].Text != "Commemoration of Saturday after Lent III" {
		t.Errorf("heading = %q", elems[0].Text)
	}
	if elems[1].Text != "Ferial Benedictus antiphon" {
		t.Errorf("antiphon = %q, want ferial antiphon from the Psalter", elems[1].Text)
	}
	if elems[2].Text != "Ferial versicle" {
		t.Errorf("versicle = %q, want ferial versicle", elems[2].Text)
	}
	if elems[3].Text != "Collect of the preceding Sunday" {
		t.Errorf("collect = %q, want preceding Sunday collect", elems[3].Text)
	}

	// The feria commemoration is a Lauds-only concern; Vespers must not surface it.
	if got := addCommemorations(day, "vespers", corpus); len(got) != 0 {
		t.Fatalf("vespers should not include the feria commemoration, got %d elements", len(got))
	}
}

func TestLaudsPrivilegedFeriaCommemorationUsesFerialTexts(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/benedictus-antiphon": "Ferial Benedictus antiphon",
		"ordinary/lauds/versicle":            "Ferial versicle",
		"proper/lent-sunday-2/collect":       "Governing Sunday collect",
	})
	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Commemorations: []*models.Feast{{
			ID:       "privileged-lenten-feria",
			Name:     "Thursday after Lent II",
			Rank:     models.PrivilegedFeria,
			Category: models.CategoryFeria,
			ProperID: "lent-sunday-2",
		}},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if len(elems) != 4 {
		t.Fatalf("expected 4 commemoration elements, got %d", len(elems))
	}
	if got := elems[1].Text; got != "Ferial Benedictus antiphon" {
		t.Errorf("antiphon = %q, want ferial Benedictus antiphon", got)
	}
	if got := elems[2].Text; got != "Ferial versicle" {
		t.Errorf("versicle = %q, want ferial versicle", got)
	}
	if got := elems[3].Text; got != "Governing Sunday collect" {
		t.Errorf("collect = %q, want governing Sunday collect", got)
	}
}

func TestLaudsNamedPrivilegedFeriaCommemorationUsesOwnProper(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/lent-ember-wednesday/benedictus-antiphon": "Ember Benedictus antiphon",
		"ordinary/lauds/benedictus-antiphon":              "Ferial Benedictus antiphon",
		"ordinary/lauds/versicle":                         "Ferial versicle",
		"proper/lent-ember-wednesday/collect":             "Ember collect",
	})
	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Commemorations: []*models.Feast{{
			ID:       "lent-ember-wednesday",
			Name:     "Lent Ember Wednesday",
			Rank:     models.PrivilegedFeria,
			Category: models.CategoryFeria,
		}},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if len(elems) != 4 {
		t.Fatalf("expected 4 commemoration elements, got %d", len(elems))
	}
	if got := elems[1].Text; got != "Ember Benedictus antiphon" {
		t.Errorf("antiphon = %q, want named Ember proper", got)
	}
	if got := elems[2].Text; got != "Ferial versicle" {
		t.Errorf("versicle = %q, want ferial versicle", got)
	}
	if got := elems[3].Text; got != "Ember collect" {
		t.Errorf("collect = %q, want named Ember proper", got)
	}
}

func TestAddCommemorationsStripsRedundantPrefix(t *testing.T) {
	// A feast whose proper title already begins with "Commemoration of" (e.g. the
	// June 30 / Jan 18 commemoration of St Paul) must not double the word when the
	// composer prefixes its own "Commemoration of".
	corpus := texts.NewTestCorpus(map[string]string{
		"commons/apostle/commemoration-antiphon": "This is my commandment",
		"ordinary/lauds/commemoration-versicle":  "Default versicle",
		"ordinary/lauds/commemoration-collect":   "Default collect",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC),
		Season: models.Pentecost,
		Commemorations: []*models.Feast{
			{ID: "commemoration-st-paul-apostle", Name: "Commemoration of St Paul, Apostle", Category: models.CategoryApostle},
		},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if elems[0].Text != "Commemoration of St Paul, Apostle" {
		t.Errorf("heading = %q, want the prefix applied once", elems[0].Text)
	}
}

func TestLaudsTemporalCommemorationUsesCanticleAntiphon(t *testing.T) {
	// A commemorated Sunday with no dedicated commemoration antiphon takes its
	// own gospel-canticle antiphon and the hour's little versicle — never the
	// saint-shaped "O holy N." fallback, which would leave an unfilled name.
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/pentecost-sunday-20/benedictus-antiphon": "So the father knew",
		"ordinary/lauds/versicle":                        "Ferial versicle",
		"ordinary/lauds/commemoration-antiphon":          "Pray for us, O holy N.",
		"ordinary/lauds/commemoration-versicle":          "The Lord hath chosen him for himself.",
		"ordinary/lauds/commemoration-collect":           "Saint collect",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC),
		Season: models.Pentecost,
		Commemorations: []*models.Feast{
			{ID: "pentecost-sunday-20", Name: "XX Sunday after Pentecost", Category: models.CategorySunday},
		},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if len(elems) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(elems))
	}
	if elems[1].Text != "So the father knew" {
		t.Errorf("antiphon = %q, want the Sunday's own Benedictus antiphon", elems[1].Text)
	}
	if elems[2].Text != "Ferial versicle" {
		t.Errorf("versicle = %q, want the hour's little versicle, not the saint versicle", elems[2].Text)
	}
}

func TestLaudsTemporalVigilFallsToPsalterAntiphon(t *testing.T) {
	// A vigil with no proper of its own falls to the Psalter canticle antiphon,
	// not the "O holy N." saint fallback.
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/benedictus-antiphon":    "Blessed be the Lord God of Israel",
		"ordinary/lauds/versicle":               "Ferial versicle",
		"ordinary/lauds/commemoration-antiphon": "Pray for us, O holy N.",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		Season: models.Pentecost,
		Commemorations: []*models.Feast{
			{ID: "vigil-of-st-james", Name: "Vigil of St. James", Category: models.CategoryFeria},
		},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if elems[1].Text != "Blessed be the Lord God of Israel" {
		t.Errorf("antiphon = %q, want the Psalter Benedictus antiphon", elems[1].Text)
	}
}

func TestComposeLaudsSundayPsalmodyOmitsFestalPsalms(t *testing.T) {
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
		Season: models.Pentecost,
		Color:  models.Green,
	}

	hour, err := engine.ComposeHour("lauds", day, calendar.ComputeMoveableDates(2026))
	if err != nil {
		t.Fatalf("ComposeHour(lauds): %v", err)
	}

	var psalmLabels []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.Type == models.Psalm {
				psalmLabels = append(psalmLabels, elem.Label)
			}
		}
	}

	if len(psalmLabels) < 7 {
		t.Fatalf("got %d psalms, want at least 7: %v", len(psalmLabels), psalmLabels)
	}

	wantPrefix := []string{"Psalm 67", "Psalm 51", "Psalm 118", "Psalm 63"}
	for i, want := range wantPrefix {
		if psalmLabels[i] != want {
			t.Fatalf("psalm %d = %q, want %q (all psalms: %v)", i, psalmLabels[i], want, psalmLabels)
		}
	}

	for _, got := range psalmLabels {
		if got == "Psalm 93" || got == "Psalm 100" {
			t.Fatalf("Sunday Lauds still contains festal psalm %q: %v", got, psalmLabels)
		}
	}
}

func TestComposeLaudsEpiphanyOctaveSundayUsesFestalPsalmody(t *testing.T) {
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC),
		Season: models.Epiphany,
		Color:  models.White,
		Celebration: &models.Feast{
			ID:       "epiphany-sunday-1",
			ProperID: "epiphany-sunday-within-octave",
			Category: models.CategorySunday,
		},
	}

	hour, err := engine.ComposeHour("lauds", day, calendar.ComputeMoveableDates(2026))
	if err != nil {
		t.Fatalf("ComposeHour(lauds): %v", err)
	}

	var psalmLabels []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.Type == models.Psalm {
				psalmLabels = append(psalmLabels, elem.Label)
			}
		}
	}

	wantPrefix := []string{"Psalm 67", "Psalm 93", "Psalm 100", "Psalm 63"}
	if len(psalmLabels) < len(wantPrefix) {
		t.Fatalf("got %d psalms, want at least %d: %v", len(psalmLabels), len(wantPrefix), psalmLabels)
	}
	for i, want := range wantPrefix {
		if psalmLabels[i] != want {
			t.Fatalf("psalm %d = %q, want %q (all psalms: %v)", i, psalmLabels[i], want, psalmLabels)
		}
	}
}

func TestComposeHourKeepsResolutionKeyButCanonicalizesAliasDependency(t *testing.T) {
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	day := &models.CalendarDay{
		Date:   time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
		Season: models.Pentecost,
		Celebration: &models.Feast{
			ID: "nativity-bvm", Category: models.CategoryBlessedVirgin,
		},
	}

	hour, err := engine.ComposeHour("lauds", day, calendar.ComputeMoveableDates(2026))
	if err != nil {
		t.Fatalf("ComposeHour(lauds): %v", err)
	}
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.SlotRef != "hymn" {
				continue
			}
			if elem.SourceRef != "proper/nativity-bvm/hymn-lauds" {
				t.Fatalf("SourceRef = %q, want proper resolution key", elem.SourceRef)
			}
			if len(elem.SourceRefs) != 1 || elem.SourceRefs[0] != "shared/blessed-virgin/hymn-lauds" {
				t.Fatalf("SourceRefs = %v, want canonical shared dependency", elem.SourceRefs)
			}
			return
		}
	}
	t.Fatal("composed Lauds has no hymn slot")
}

func TestAddCommemorationsUsesProperIDAlias(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/pentecost-sunday-23/commemoration-antiphon": "XXIII Pentecost antiphon",
		"ordinary/lauds/commemoration-versicle":             "Default versicle",
		"ordinary/lauds/commemoration-collect":              "Default collect",
	})

	day := &models.CalendarDay{
		Date:   time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC),
		Season: models.Epiphany,
		Commemorations: []*models.Feast{
			{
				ID:       "epiphany-sunday-7",
				Name:     "VII Sunday after Epiphany",
				ProperID: "pentecost-sunday-23",
			},
		},
	}

	elems := addCommemorations(day, "lauds", corpus)
	if len(elems) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(elems))
	}
	if elems[1].Text != "XXIII Pentecost antiphon" {
		t.Fatalf("element 1 text = %q, want ProperID-backed antiphon", elems[1].Text)
	}
}
