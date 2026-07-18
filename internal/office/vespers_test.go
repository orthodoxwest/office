package office

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func vespersPsalmLabels(t *testing.T, day *models.CalendarDay) []string {
	t.Helper()
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	hour, err := engine.ComposeHour("vespers", day, calendar.ComputeMoveableDates(day.Date.Year()))
	if err != nil {
		t.Fatalf("ComposeHour(vespers): %v", err)
	}

	var labels []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.Type == models.Psalm {
				labels = append(labels, elem.Label)
			}
		}
	}
	return labels
}

func TestComposeVespersNativityIIUsesProperFestalPsalmody(t *testing.T) {
	day := &models.CalendarDay{
		Date:   time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC),
		Season: models.Christmas,
		Color:  models.White,
		Celebration: &models.Feast{
			ID:       "christmas",
			Category: models.CategoryLord,
		},
	}

	got := vespersPsalmLabels(t, day)
	want := []string{"Psalm 110", "Psalm 111", "Psalm 112", "Psalm 130", "Psalm 132"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("Nativity II Vespers psalms = %v, want %v", got, want)
	}
}

func TestComposeVespersApostleUsesCommonFestalPsalmody(t *testing.T) {
	day := &models.CalendarDay{
		Date:   time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
		Season: models.Advent,
		Color:  models.Red,
		Celebration: &models.Feast{
			ID:       "st-andrew",
			Category: models.CategoryApostle,
		},
	}

	got := vespersPsalmLabels(t, day)
	want := []string{"Psalm 110", "Psalm 113", "Psalm 116b", "Psalm 139"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("apostle II Vespers psalms = %v, want %v", got, want)
	}
}

func TestComposeVespersFirstVespersUsesFollowingFeastFestalPsalmody(t *testing.T) {
	day := &models.CalendarDay{
		Date:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		Season: models.Easter,
		Color:  models.White,
		Vespers: models.VespersDesignation{
			Owner: models.VespersIOfFollowing,
			Feast: &models.Feast{
				ID:       "ss-philip-james",
				Category: models.CategoryApostle,
				Color:    models.Red,
			},
			Color:  models.Red,
			Season: models.Easter,
		},
	}

	got := vespersPsalmLabels(t, day)
	want := []string{"Psalm 110", "Psalm 111", "Psalm 112", "Psalm 113"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("apostle I Vespers psalms = %v, want %v", got, want)
	}
}

func TestComposeVespersFeriaRetainsWeekdayPsalmody(t *testing.T) {
	day := &models.CalendarDay{
		Date:   time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
		Season: models.Pentecost,
		Color:  models.Green,
	}

	got := vespersPsalmLabels(t, day)
	want := []string{"Psalm 130", "Psalm 131", "Psalm 132", "Psalm 133"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("ferial Tuesday Vespers psalms = %v, want %v", got, want)
	}
}

func TestFestalVespersPsalmodyLeavesUnattestedClassOnPsalter(t *testing.T) {
	day := &models.CalendarDay{
		Date: time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		Celebration: &models.Feast{
			ID:       "st-athanasius",
			Category: models.CategoryConfessorDoctor,
		},
	}
	corpus, err := texts.LoadTexts(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	psalmody, source, err := resolveVespersPsalmody(day, corpus)
	if err != nil {
		t.Fatalf("resolveVespersPsalmody: %v", err)
	}
	if len(psalmody) != 0 {
		t.Fatalf("unattested confessor-doctor class selected psalmody from %q: %#v", source, psalmody)
	}
}

func TestResolveVespersPsalmodyLayering(t *testing.T) {
	const standard = "psalm-antiphon-1 = psalms/110"
	const common = "psalm-antiphon-1 = psalms/112"
	const proper = "psalm-antiphon-1 = psalms/132"
	day := &models.CalendarDay{
		Celebration: &models.Feast{ID: "test-feast", Category: models.CategoryMartyr},
	}

	tests := []struct {
		name       string
		corpus     map[string]string
		wantPsalm  string
		wantSource string
	}{
		{
			name: "proper overrides common",
			corpus: map[string]string{
				defaultVespersPsalmodyKey:            standard,
				"commons/martyr/vespers-psalmody":    common,
				"proper/test-feast/vespers-psalmody": proper,
			},
			wantPsalm:  "psalms/132",
			wantSource: "proper/test-feast/vespers-psalmody",
		},
		{
			name: "common overrides default",
			corpus: map[string]string{
				defaultVespersPsalmodyKey:         standard,
				"commons/martyr/vespers-psalmody": common,
			},
			wantPsalm:  "psalms/112",
			wantSource: "commons/martyr/vespers-psalmody",
		},
		{
			name: "default applies after absent proper and common",
			corpus: map[string]string{
				defaultVespersPsalmodyKey: standard,
			},
			wantPsalm:  "psalms/110",
			wantSource: defaultVespersPsalmodyKey,
		},
		{
			name: "ferial declaration stops default",
			corpus: map[string]string{
				defaultVespersPsalmodyKey:         standard,
				"commons/martyr/vespers-psalmody": ferialPsalmodyDeclaration,
			},
			wantSource: "commons/martyr/vespers-psalmody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, source, err := resolveVespersPsalmody(day, texts.NewTestCorpus(tt.corpus))
			if err != nil {
				t.Fatalf("resolveVespersPsalmody: %v", err)
			}
			if source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
			if tt.wantPsalm == "" {
				if len(items) != 0 {
					t.Fatalf("psalmody = %#v, want ferial", items)
				}
				return
			}
			if len(items) != 1 || items[0].psalm != tt.wantPsalm {
				t.Fatalf("psalmody = %#v, want %s", items, tt.wantPsalm)
			}
		})
	}
}

func TestParsePsalmodyDeclarationRejectsMalformedData(t *testing.T) {
	tests := []string{
		"",
		"psalm-antiphon-1 psalms/110",
		"psalm-antiphon-1 =",
		"psalm-antiphon-1 = psalms/110\npsalm-antiphon-1 = psalms/111",
	}
	for _, declaration := range tests {
		if _, _, err := parsePsalmodyDeclaration(declaration); err == nil {
			t.Errorf("parsePsalmodyDeclaration(%q) succeeded, want error", declaration)
		}
	}
}

func TestValidateVespersPsalmodyDeclarationReferences(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		defaultVespersPsalmodyKey:           "psalm-antiphon-1 = psalms/missing",
		"ordinary/vespers/psalm-antiphon-1": "Antiphon text.",
	})
	errs := validateVespersPsalmodyDeclarations(corpus)
	if len(errs) != 1 || !strings.Contains(errs[0], "psalm ref not found in corpus: psalms/missing") {
		t.Fatalf("validation errors = %v, want missing psalm ref", errs)
	}
}

func TestVespersWeekdayCondition(t *testing.T) {
	tests := []struct {
		name      string
		date      time.Time
		first     bool
		condition string
		want      bool
	}{
		{"Sunday matches weekday-sunday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), false, "weekday-sunday", true},
		{"Sunday does not match weekday-monday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), false, "weekday-monday", false},
		{"Monday matches weekday-monday", time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), false, "weekday-monday", true},
		{"Saturday matches weekday-saturday", time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC), false, "weekday-saturday", true},
		{"Sunday first Vespers uses civil Saturday", time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC), true, "weekday-saturday", true},
		{"Sunday first Vespers is not civil Sunday", time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC), true, "weekday-sunday", false},
		{"Low Sunday first Vespers uses civil Saturday", time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC), true, "weekday-saturday", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date, FirstVespers: tt.first}
			if tt.first {
				day.Celebration = &models.Feast{Category: models.CategorySunday}
				if tt.name == "Low Sunday first Vespers uses civil Saturday" {
					day.Celebration.Category = models.CategoryLord
				}
			}
			got := evaluateCondition(tt.condition, day, nil)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestVespersPreces(t *testing.T) {
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

func TestVespersDecisionsExposeCivilAndOfficeContexts(t *testing.T) {
	day := &models.CalendarDay{
		Date:           time.Date(2026, 1, 17, 0, 0, 0, 0, time.UTC),
		Season:         models.Epiphany,
		Celebration:    &models.Feast{ID: "st-anthony-egypt", Rank: models.Double, Category: models.CategoryConfessor},
		ResolutionRule: "occurrence:general-precedence",
		Vespers: models.VespersDesignation{
			Owner:  models.VespersIOfFollowing,
			Feast:  &models.Feast{ID: "epiphany-sunday-2", Rank: models.SemiDouble, Category: models.CategorySunday},
			Season: models.Epiphany,
			Commemorations: []*models.Feast{
				{ID: "comm-1"}, {ID: "comm-2"}, {ID: "comm-3"}, {ID: "comm-4"},
			},
		},
	}
	hour := &models.OfficeHour{}
	appendContextDecisions(hour, day, "vespers")

	assertDecision(t, hour.Decisions, "context:weekday", "saturday", "")
	assertDecision(t, hour.Decisions, "context:office", "celebration", "st-anthony-egypt")
	assertDecision(t, hour.Decisions, "context:commemorations", "0", "")
	assertDecision(t, hour.Decisions, "office-context:weekday", "sunday", "")
	assertDecision(t, hour.Decisions, "office-context:office", "celebration", "epiphany-sunday-2")
	assertDecision(t, hour.Decisions, "office-context:commemorations", "4", "")
	assertDecision(t, hour.Decisions, "office-context:first-vespers", "yes", "")
}

func assertDecision(t *testing.T, decisions []models.CompositionDecision, rule, outcome, detail string) {
	t.Helper()
	for _, decision := range decisions {
		if decision.Rule == rule && decision.Outcome == outcome && decision.Detail == detail {
			return
		}
	}
	t.Errorf("missing decision %s=%s (%s) in %#v", rule, outcome, detail, decisions)
}

func TestVespersProperAntiphonFallback(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/vespers/psalm-antiphon":      "Vespers antiphon",
		"ordinary/lauds/psalm-antiphon":        "Lauds antiphon",
		"ordinary/vespers/magnificat-antiphon": "Vespers Magnificat",
	})

	day := &models.CalendarDay{Season: models.Pentecost}

	// Vespers should use ordinary/vespers/, not ordinary/lauds/
	gotTxt, _ := resolveProperText(day, "vespers", "psalm-antiphon", corpus)
	if gotTxt != "Vespers antiphon" {
		t.Errorf("resolveProperText(vespers) = %q, want %q", gotTxt, "Vespers antiphon")
	}

	// Lauds should still use ordinary/lauds/
	gotTxt, _ = resolveProperText(day, "lauds", "psalm-antiphon", corpus)
	if gotTxt != "Lauds antiphon" {
		t.Errorf("resolveProperText(lauds) = %q, want %q", gotTxt, "Lauds antiphon")
	}
}

func TestVespersUsesFollowingOfficeWhenConcurrenceSaysFirstVespers(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers": "Joseph psalm antiphon",
		"proper/st-cyril/psalm-antiphon-1-vespers":  "Cyril psalm antiphon",
		"proper/st-joseph/collect":                  "Joseph collect",
		"proper/st-cyril/collect":                   "Cyril collect",
		"ordinary/vespers/opening-versicle":         "Opening versicle",
	})
	sections := []HourSection{
		{
			Name:      "Psalmody-Thursday",
			Label:     "Psalmody",
			Condition: "weekday-thursday",
			Elements: []HourElement{
				{Type: "proper-antiphon", Ref: "psalm-antiphon-1"},
			},
		},
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Color:  models.Violet,
		Celebration: &models.Feast{
			ID:       "st-cyril",
			Name:     "St Cyril",
			Rank:     models.Double,
			Color:    models.White,
			Category: models.CategoryConfessorDoctor,
		},
		Commemorations: []*models.Feast{
			{ID: "st-edward", Name: "St Edward", Rank: models.Commemoration},
		},
		Vespers: models.VespersDesignation{
			Owner:  models.VespersIOfFollowing,
			Feast:  &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:  models.White,
			Season: models.Lent,
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	if hour.Date != day.Date {
		t.Fatalf("hour date = %s, want civil date %s", hour.Date, day.Date)
	}
	if hour.Feast != "St Joseph" {
		t.Fatalf("hour feast = %q, want St Joseph", hour.Feast)
	}
	if hour.Color != models.White {
		t.Fatalf("hour color = %s, want white", hour.Color)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Joseph psalm antiphon") || !strings.Contains(joined, "Joseph collect") {
		t.Fatalf("expected following feast texts, got:\n%s", joined)
	}
	if strings.Contains(joined, "Cyril") || strings.Contains(joined, "St Edward") {
		t.Fatalf("expected preceding office texts and commemorations to be omitted, got:\n%s", joined)
	}
}

func TestVespersCommemoratesOutgoingOfficeAtFirstVespersOfFollowing(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers": "Joseph psalm antiphon",
		"proper/st-joseph/collect":                  "Joseph collect",
		"commons/confessor/commemoration-antiphon":  "Cyril commemoration antiphon",
		"commons/confessor/commemoration-collect":   "Cyril commemoration collect",
	})
	sections := []HourSection{
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Celebration: &models.Feast{
			ID: "st-cyril", Name: "St Cyril", Rank: models.Double, Category: models.CategoryConfessor,
		},
		Vespers: models.VespersDesignation{
			Owner:          models.VespersIOfFollowing,
			Feast:          &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:          models.White,
			Season:         models.Lent,
			Commemorations: []*models.Feast{{ID: "st-cyril", Name: "St Cyril", Rank: models.Double, Category: models.CategoryConfessor}},
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Commemoration of St Cyril") {
		t.Fatalf("expected commemoration of the outgoing office, got:\n%s", joined)
	}
}

func TestVespersWithoutOwnerUsesFollowingDayCommemorations(t *testing.T) {
	current := &models.Feast{ID: "current", Name: "Current Memorial", Rank: models.Commemoration}
	incoming := &models.Feast{ID: "incoming", Name: "Incoming Memorial", Rank: models.Commemoration}
	day := &models.CalendarDay{
		Date:           time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC),
		Season:         models.Epiphany,
		Color:          models.Green,
		Commemorations: []*models.Feast{current},
		Vespers: models.VespersDesignation{
			Owner:          models.VespersNotApplicable,
			Commemorations: []*models.Feast{incoming},
			Rule:           "concurrence:neither-office-has-rights",
		},
	}

	officeDay := vespersOfficeDay(day)
	if officeDay == day {
		t.Fatal("expected a Vespers-specific day copy")
	}
	if len(officeDay.Commemorations) != 1 || officeDay.Commemorations[0] != incoming {
		t.Fatalf("commemorations = %#v, want incoming memorial only", officeDay.Commemorations)
	}
	if officeDay.Date != day.Date || officeDay.Color != day.Color {
		t.Fatalf("no-owner Vespers changed office context: %#v", officeDay)
	}
}

func TestVespersCommemoratesIncomingOfficeAtSecondVespersOfPreceding(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers": "Joseph psalm antiphon",
		"proper/st-joseph/collect":                  "Joseph collect",
		"commons/confessor/commemoration-antiphon":  "Turibius commemoration antiphon",
		"commons/confessor/commemoration-collect":   "Turibius commemoration collect",
	})
	sections := []HourSection{
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Celebration: &models.Feast{
			ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White,
		},
		Vespers: models.VespersDesignation{
			Owner:          models.VespersIIOfPreceding,
			Feast:          &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:          models.White,
			Season:         models.Lent,
			Commemorations: []*models.Feast{{ID: "st-turibius", Name: "St Turibius", Rank: models.Double, Category: models.CategoryConfessor}},
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Commemoration of St Turibius") {
		t.Fatalf("expected commemoration of the incoming office, got:\n%s", joined)
	}
}

func TestVespersUsesPrecedingOfficeWhenConcurrenceSaysSecondVespers(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/st-joseph/psalm-antiphon-1-vespers":   "Joseph psalm antiphon",
		"proper/st-joseph/collect":                    "Joseph collect",
		"proper/st-cuthbert/psalm-antiphon-1-vespers": "Cuthbert psalm antiphon",
		"proper/st-cuthbert/collect":                  "Cuthbert collect",
	})
	sections := []HourSection{
		{
			Name:      "Psalmody-Thursday",
			Label:     "Psalmody",
			Condition: "weekday-thursday",
			Elements: []HourElement{
				{Type: "proper-antiphon", Ref: "psalm-antiphon-1"},
			},
		},
		{
			Name:  "Collect",
			Label: "Collect",
			Elements: []HourElement{
				{Type: "proper-collect", Ref: "collect"},
				{Type: "commemorations"},
			},
		},
	}

	day := &models.CalendarDay{
		Date:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		Season: models.Lent,
		Color:  models.White,
		Celebration: &models.Feast{
			ID:    "st-joseph",
			Name:  "St Joseph",
			Rank:  models.Double2ndClass,
			Color: models.White,
		},
		Vespers: models.VespersDesignation{
			Owner:  models.VespersIIOfPreceding,
			Feast:  &models.Feast{ID: "st-joseph", Name: "St Joseph", Rank: models.Double2ndClass, Color: models.White},
			Color:  models.White,
			Season: models.Lent,
		},
	}

	hour, err := (&VespersComposer{}).Compose(day, sections, corpus, nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	if hour.Date != day.Date {
		t.Fatalf("hour date = %s, want civil date %s", hour.Date, day.Date)
	}
	if hour.Feast != "St Joseph" {
		t.Fatalf("hour feast = %q, want St Joseph", hour.Feast)
	}

	var texts []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			texts = append(texts, elem.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "Joseph psalm antiphon") || !strings.Contains(joined, "Joseph collect") {
		t.Fatalf("expected preceding feast texts, got:\n%s", joined)
	}
	if strings.Contains(joined, "Cuthbert") {
		t.Fatalf("expected following feast texts to be omitted, got:\n%s", joined)
	}
}
