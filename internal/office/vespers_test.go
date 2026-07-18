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
	want := []string{"Psalm 110", "Psalm 111", "Psalm 112", "Psalm 130"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("Nativity II Vespers psalms = %v, want %v", got, want)
	}
}

func TestComposeVespersAffectedFestalPsalmodyPairs(t *testing.T) {
	days, err := calendar.BuildCalendar(2026, filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		date          string
		fourthPsalm   string
		fourthIncipit string
	}{
		{"2026-06-10", "Psalm 128", "May the children"},
		{"2026-06-11", "Psalm 147b", "He that maketh"},
		{"2026-06-12", "Psalm 128", "May the children"},
		{"2026-06-15", "Psalm 147b", "He that maketh"},
		{"2026-06-16", "Psalm 128", "May the children"},
		{"2026-06-17", "Psalm 147b", "He that maketh"},
		{"2026-06-18", "Psalm 128", "May the children"},
		{"2026-09-28", "Psalm 113", "Angels and Archangels"},
		{"2026-09-29", "Psalm 138", "Angels and Archangels"},
		{"2026-12-25", "Psalm 130", "With the Lord"},
	}

	byDate := make(map[string]*models.CalendarDay, len(days))
	for i := range days {
		byDate[days[i].Date.Format("2006-01-02")] = &days[i]
	}
	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			day := byDate[tt.date]
			if day == nil {
				t.Fatalf("calendar day not found")
			}
			hour, err := engine.ComposeHour("vespers", day, moveable)
			if err != nil {
				t.Fatalf("ComposeHour: %v", err)
			}
			type pair struct {
				psalm    string
				antiphon string
			}
			var pairs []pair
			for _, section := range hour.Sections {
				for i, elem := range section.Elements {
					if elem.Type != models.Psalm {
						continue
					}
					antiphon := ""
					if i > 0 && section.Elements[i-1].Type == models.Antiphon {
						antiphon = section.Elements[i-1].Text
					}
					pairs = append(pairs, pair{psalm: elem.Label, antiphon: antiphon})
				}
			}
			if len(pairs) != 4 {
				t.Fatalf("psalm pairs = %#v, want four", pairs)
			}
			fourth := pairs[3]
			if fourth.psalm != tt.fourthPsalm || !strings.HasPrefix(fourth.antiphon, tt.fourthIncipit) {
				t.Fatalf("fourth pair = %q / %q, want %q / %q", fourth.antiphon, fourth.psalm, tt.fourthIncipit, tt.fourthPsalm)
			}
		})
	}
}

func TestComposeVespersNativityOctaveDateSetsAndConcurrence(t *testing.T) {
	days, err := calendar.BuildCalendar(2026, filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)
	wantFourth := map[string]string{
		"2026-12-25": "Psalm 130",
		"2026-12-26": "Psalm 132",
		"2026-12-27": "Psalm 130",
		"2026-12-28": "Psalm 132",
		"2026-12-29": "Psalm 130",
		// Dec. 30 is I Vespers of St Sylvester, so his common supersedes
		// the Nativity octave's date-conditioned Psalm 132.
		"2026-12-30": "Psalm 113",
	}
	for i := range days {
		date := days[i].Date.Format("2006-01-02")
		want, ok := wantFourth[date]
		if !ok {
			continue
		}
		hour, err := engine.ComposeHour("vespers", &days[i], moveable)
		if err != nil {
			t.Fatalf("%s: ComposeHour: %v", date, err)
		}
		var psalms []string
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.Type == models.Psalm {
					psalms = append(psalms, elem.Label)
				}
			}
		}
		if len(psalms) != 4 || psalms[3] != want {
			t.Errorf("%s psalms = %v, want fourth %s", date, psalms, want)
		}
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

func TestResolveVespersPsalmodyRemainingCommonClasses(t *testing.T) {
	corpus, err := texts.LoadTexts(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	tests := []struct {
		name       string
		day        *models.CalendarDay
		wantSource string
		wantPsalms []string
	}{
		{
			name: "bishop doctor second Vespers",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: "st-athanasius", Category: models.CategoryConfessorDoctor},
			},
			wantSource: "commons/confessor-doctor/vespers-psalmody",
			wantPsalms: []string{"psalms/110", "psalms/112", "psalms/113", "psalms/132"},
		},
		{
			name: "generic Lord feast",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: "unlisted-lord-feast", Category: models.CategoryLord},
			},
			wantSource: "commons/lord/vespers-psalmody",
			wantPsalms: []string{"psalms/110", "psalms/111", "psalms/112", "psalms/113"},
		},
		{
			name: "generic angel first Vespers",
			day: &models.CalendarDay{
				Date:         time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
				FirstVespers: true,
				Celebration:  &models.Feast{ID: "guardian-angels", Category: models.CategoryAngel},
			},
			wantSource: "commons/angel/vespers-psalmody-first",
			wantPsalms: []string{"psalms/110", "psalms/111", "psalms/112", "psalms/113"},
		},
		{
			name: "generic angel second Vespers",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: "guardian-angels", Category: models.CategoryAngel},
			},
			wantSource: "commons/angel/vespers-psalmody",
			wantPsalms: []string{"psalms/110", "psalms/111", "psalms/112", "psalms/138"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			psalmody, source, err := resolveVespersPsalmody(tt.day, corpus)
			if err != nil {
				t.Fatalf("resolveVespersPsalmody: %v", err)
			}
			if source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
			var got []string
			for _, item := range psalmody {
				got = append(got, item.psalm)
			}
			if strings.Join(got, "|") != strings.Join(tt.wantPsalms, "|") {
				t.Fatalf("psalmody = %v, want %v", got, tt.wantPsalms)
			}
		})
	}
}

func TestComposeVespersNewFestalClassPsalmAntiphonPairs(t *testing.T) {
	tests := []struct {
		name          string
		day           *models.CalendarDay
		wantPsalms    []string
		wantAntiphons []string
		wantAlleluia  bool
	}{
		{
			name: "bishop doctor",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
				Season:      models.Easter,
				Celebration: &models.Feast{ID: "st-athanasius", Category: models.CategoryConfessorDoctor},
			},
			wantPsalms: []string{"Psalm 110", "Psalm 112", "Psalm 113", "Psalm 132"},
			wantAntiphons: []string{
				"Behold a great priest",
				"There was none",
				"The Lord, * therefore",
				"Good and faithful servant",
			},
			wantAlleluia: true,
		},
		{
			name: "doctor not a bishop",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
				Season:      models.Easter,
				Celebration: &models.Feast{ID: "st-bede-venerable", Category: models.CategoryConfessorDoctor},
			},
			wantPsalms: []string{"Psalm 110", "Psalm 111", "Psalm 112", "Psalm 113"},
			wantAntiphons: []string{
				"Lord, thou deliveredst",
				"Well done, thou good servant",
				"A wise and faithful servant",
				"Good and faithful servant",
			},
			wantAlleluia: true,
		},
		{
			name: "generic angel",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: "guardian-angels", Category: models.CategoryAngel},
			},
			wantPsalms: []string{"Psalm 110", "Psalm 111", "Psalm 112", "Psalm 138"},
			wantAntiphons: []string{
				"God hath given His Angels",
				"Let us praise the Lord",
				"In heaven their Angels",
				"Praise ye God",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(filepath.Join("..", "..", "data"))
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			hour, err := engine.ComposeHour("vespers", tt.day, calendar.ComputeMoveableDates(2026))
			if err != nil {
				t.Fatalf("ComposeHour: %v", err)
			}
			var psalms []string
			var antiphons []string
			for _, section := range hour.Sections {
				for i, elem := range section.Elements {
					if elem.Type != models.Psalm {
						continue
					}
					psalms = append(psalms, elem.Label)
					antiphon := ""
					if i > 0 && section.Elements[i-1].Type == models.Antiphon {
						antiphon = section.Elements[i-1].Text
					}
					antiphons = append(antiphons, antiphon)
				}
			}
			if strings.Join(psalms, "|") != strings.Join(tt.wantPsalms, "|") {
				t.Fatalf("psalms = %v, want %v", psalms, tt.wantPsalms)
			}
			for i, want := range tt.wantAntiphons {
				if !strings.HasPrefix(antiphons[i], want) {
					t.Errorf("pair %d antiphon = %q, want incipit %q with %s", i+1, antiphons[i], want, psalms[i])
				}
				if tt.wantAlleluia && !strings.HasSuffix(strings.ToLower(antiphons[i]), "alleluia.") {
					t.Errorf("pair %d antiphon = %q, want Paschaltide alleluia", i+1, antiphons[i])
				}
			}
		})
	}
}

func TestUnprintedSecondVespersSidesRemainFerial(t *testing.T) {
	corpus, err := texts.LoadTexts(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}
	for _, feastID := range []string{"pentecost", "trinity-sunday", "exaltation-holy-cross", "christ-the-king"} {
		t.Run(feastID, func(t *testing.T) {
			day := &models.CalendarDay{
				Date:        time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
				Celebration: &models.Feast{ID: feastID, Category: models.CategoryLord},
			}
			psalmody, source, err := resolveVespersPsalmody(day, corpus)
			if err != nil {
				t.Fatalf("resolveVespersPsalmody: %v", err)
			}
			if len(psalmody) != 0 {
				t.Fatalf("psalmody from %q = %#v, want ferial", source, psalmody)
			}
			wantSource := "proper/" + feastID + "/vespers-psalmody"
			if source != wantSource {
				t.Fatalf("source = %q, want %q", source, wantSource)
			}
		})
	}
}

func TestFestalVespersPsalmodyFullYearHasFourPsalms(t *testing.T) {
	days, err := calendar.BuildCalendar(2026, filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	engine, err := NewEngine(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)
	festalEvenings := 0
	for i := range days {
		day := &days[i]
		hour, err := engine.ComposeHour("vespers", day, moveable)
		if err != nil {
			t.Fatalf("%s: ComposeHour: %v", day.Date.Format("2006-01-02"), err)
		}
		if !usesFestalVespersPsalmody(vespersOfficeDay(day), engine.corpus) {
			continue
		}
		festalEvenings++
		var psalms []string
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.Type == models.Psalm {
					psalms = append(psalms, elem.Label)
				}
			}
		}
		if len(psalms) != 4 {
			t.Errorf("%s: festal Vespers has %d psalms (%v), want four", day.Date.Format("2006-01-02"), len(psalms), psalms)
		}
	}
	t.Logf("composed all %d days of 2026; %d festal evenings each had four psalms", len(days), festalEvenings)
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
		"psalm-antiphon-1 = psalms/110 dates=12-25,12-25",
		"psalm-antiphon-1 = psalms/110 dates=12-25\npsalm-antiphon-1 = psalms/111 dates=12-25",
		"psalm-antiphon-1 = psalms/110 dates=2-30",
		"psalm-antiphon-1 = psalms/110 weekday=friday",
	}
	for _, declaration := range tests {
		if _, _, err := parsePsalmodyDeclaration(declaration); err == nil {
			t.Errorf("parsePsalmodyDeclaration(%q) succeeded, want error", declaration)
		}
	}
}

func TestSelectPsalmodyItemsByFixedDate(t *testing.T) {
	declaration := `psalm-antiphon-1 = psalms/110
psalm-antiphon-4 = psalms/130 dates=12-25,12-27,12-29
psalm-antiphon-4 = psalms/132 dates=12-26,12-28,12-30 antiphon=psalm-antiphon-4-alternate`
	items, ferial, err := parsePsalmodyDeclaration(declaration)
	if err != nil || ferial {
		t.Fatalf("parsePsalmodyDeclaration() = (%v, %t, %v)", items, ferial, err)
	}

	for day := 25; day <= 30; day++ {
		selected, err := selectPsalmodyItems(items, time.Date(2026, 12, day, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("December %d: %v", day, err)
		}
		if len(selected) != 2 {
			t.Fatalf("December %d selected %d items, want 2", day, len(selected))
		}
		wantPsalm := "psalms/130"
		wantAntiphon := "psalm-antiphon-4"
		if day%2 == 0 {
			wantPsalm = "psalms/132"
			wantAntiphon = "psalm-antiphon-4-alternate"
		}
		if selected[1].psalm != wantPsalm || selected[1].antiphon != wantAntiphon {
			t.Errorf("December %d fourth = %s/%s, want %s/%s", day, selected[1].antiphon, selected[1].psalm, wantAntiphon, wantPsalm)
		}
	}
	if _, err := selectPsalmodyItems(items, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("December 31 unexpectedly matched a conditional fourth psalm")
	}
}

func TestResolvePsalmodyFixedDateUsesCivilEveningAtFirstVespers(t *testing.T) {
	day := &models.CalendarDay{
		Date:         time.Date(2028, 12, 31, 0, 0, 0, 0, time.UTC),
		FirstVespers: true,
		Celebration:  &models.Feast{ID: "test-feast", Category: models.CategoryLord},
	}
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/test-feast/vespers-psalmody": `psalm-antiphon-4 = psalms/130 dates=12-29
psalm-antiphon-4 = psalms/132 dates=12-30`,
	})
	items, _, err := resolveVespersPsalmody(day, corpus)
	if err != nil {
		t.Fatalf("resolveVespersPsalmody: %v", err)
	}
	if len(items) != 1 || items[0].psalm != "psalms/132" {
		t.Fatalf("resolved items = %#v, want civil Dec. 30 Psalm 132", items)
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
