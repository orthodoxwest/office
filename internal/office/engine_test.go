package office

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// --- titleCase ---

func TestTitleCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"benedictus", "Benedictus"},
		{"magnificat antiphon", "Magnificat Antiphon"},
		{"", ""},
		{"a", "A"},
	}
	for _, tt := range tests {
		if got := titleCase(tt.in); got != tt.want {
			t.Errorf("titleCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- extractHymnTitle ---

func TestExtractHymnTitle(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "title then body",
			text:      "Aeterne rerum conditor\n\nFirst verse\n\nSecond verse",
			wantTitle: "Aeterne rerum conditor",
			wantBody:  "First verse\n\nSecond verse",
		},
		{
			name:      "no blank line — no title",
			text:      "Just one block of text",
			wantTitle: "",
			wantBody:  "Just one block of text",
		},
		{
			name:      "multi-line first block — not a title",
			text:      "Line one\nLine two\n\nRest of body",
			wantTitle: "",
			wantBody:  "Line one\nLine two\n\nRest of body",
		},
		{
			name:      "empty body after title",
			text:      "Title\n\n",
			wantTitle: "Title",
			wantBody:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotBody := extractHymnTitle(tt.text)
			if gotTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body = %q, want %q", gotBody, tt.wantBody)
			}
		})
	}
}

// --- extractChapterRef ---

func TestExtractChapterRef(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantRef  string
		wantBody string
	}{
		{
			name:     "with ref marker",
			text:     "!1 Cor 13:1\nCharity suffereth long.",
			wantRef:  "1 Cor 13:1",
			wantBody: "Charity suffereth long.",
		},
		{
			name:     "no ref marker",
			text:     "Ordinary chapter text.",
			wantRef:  "",
			wantBody: "Ordinary chapter text.",
		},
		{
			name:     "space after exclamation is preserved in ref",
			text:     "! Rom 8:1\nBody text.",
			wantRef:  " Rom 8:1",
			wantBody: "Body text.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRef, gotBody := extractChapterRef(tt.text)
			if gotRef != tt.wantRef {
				t.Errorf("ref = %q, want %q", gotRef, tt.wantRef)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body = %q, want %q", gotBody, tt.wantBody)
			}
		})
	}
}

// --- formatLabel ---

func TestFormatLabel(t *testing.T) {
	tests := []struct {
		elemType string
		ref      string
		want     string
	}{
		{"psalm", "psalms/004", "Psalm 4"},
		{"psalm", "psalms/119", "Psalm 119"},
		{"psalm", "psalms/000", "Psalm 0"},
		{"canticle", "canticles/benedictus", "Benedictus"},
		{"canticle", "canticles/nunc-dimittis", "Nunc Dimittis"},
		{"hymn", "hymns/aeterne-rerum-conditor", "Aeterne Rerum Conditor"},
		{"versicle", "ordinary/compline/versicle", ""},
		{"antiphon", "ordinary/lauds/antiphon", ""},
	}
	for _, tt := range tests {
		t.Run(tt.elemType+"/"+tt.ref, func(t *testing.T) {
			got := formatLabel(tt.elemType, tt.ref)
			if got != tt.want {
				t.Errorf("formatLabel(%q, %q) = %q, want %q", tt.elemType, tt.ref, got, tt.want)
			}
		})
	}
}

// --- mapElementType ---

func TestMapElementType(t *testing.T) {
	tests := []struct {
		in   string
		want models.ElementType
	}{
		{"psalm", models.Psalm},
		{"canticle", models.Canticle},
		{"hymn", models.Hymn},
		{"antiphon", models.Antiphon},
		{"versicle", models.Versicle},
		{"response", models.Response},
		{"prayer", models.Prayer},
		{"secret-prayer", models.Prayer},
		{"partly-secret-prayer", models.Prayer},
		{"preces", models.Preces},
		{"gloria-patri", models.Doxology},
		{"rubric", models.Rubric},
		{"chapter", models.Chapter},
		{"collect", models.Collect},
		{"blessing", models.Blessing},
		{"marian", models.Antiphon},
		{"proper-antiphon", models.Antiphon},
		{"proper-collect", models.Collect},
		{"proper-hymn", models.Hymn},
		{"proper-responsory", models.Response},
		{"proper-chapter", models.Chapter},
		{"commemorations", models.Rubric},
		{"unknown-type", models.Rubric}, // default fallthrough
	}
	for _, tt := range tests {
		got := mapElementType(tt.in)
		if got != tt.want {
			t.Errorf("mapElementType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- collapseUniformAntiphons ---

func TestCollapseUniformAntiphonsLengthBoundary(t *testing.T) {
	t.Run("two identical antiphons remain", func(t *testing.T) {
		hour := &models.OfficeHour{Sections: []models.OfficeSection{{
			Elements: []models.OfficeElement{
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Psalm, Text: "Psalm 1"},
				{Type: models.Antiphon, Text: "Alleluia"},
			},
		}}}

		collapseUniformAntiphons(hour)

		assertSectionElements(t, hour, 0,
			"antiphon:Alleluia",
			"psalm:Psalm 1",
			"antiphon:Alleluia",
		)
	})

	t.Run("three identical antiphons collapse to a framing pair", func(t *testing.T) {
		hour := &models.OfficeHour{Sections: []models.OfficeSection{{
			Elements: []models.OfficeElement{
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Psalm, Text: "Psalm 1"},
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Psalm, Text: "Psalm 2"},
				{Type: models.Antiphon, Text: "Alleluia"},
			},
		}}}

		collapseUniformAntiphons(hour)

		assertSectionElements(t, hour, 0,
			"antiphon:Alleluia",
			"psalm:Psalm 1",
			"psalm:Psalm 2",
			"antiphon:Alleluia",
		)
	})
}

func TestCollapseUniformAntiphonsAcrossAdjacentPsalmSections(t *testing.T) {
	hour := &models.OfficeHour{Sections: []models.OfficeSection{
		{
			Elements: []models.OfficeElement{
				{Type: models.Rubric, Text: "Before"},
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Psalm, Text: "Psalm 1"},
				{Type: models.Antiphon, Text: "Alleluia"},
			},
		},
		{
			Elements: []models.OfficeElement{
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Canticle, Text: "Canticle"},
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Rubric, Text: "After"},
			},
		},
	}}

	collapseUniformAntiphons(hour)

	assertSectionElements(t, hour, 0,
		"rubric:Before",
		"antiphon:Alleluia",
		"psalm:Psalm 1",
	)
	assertSectionElements(t, hour, 1,
		"canticle:Canticle",
		"antiphon:Alleluia",
		"rubric:After",
	)
}

func TestCollapseUniformAntiphonsStopsAtNonPsalmSection(t *testing.T) {
	hour := &models.OfficeHour{Sections: []models.OfficeSection{
		{
			Elements: []models.OfficeElement{
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Psalm, Text: "Psalm 1"},
				{Type: models.Antiphon, Text: "Alleluia"},
			},
		},
		{
			Elements: []models.OfficeElement{
				{Type: models.Rubric, Text: "A separate ceremony"},
			},
		},
		{
			Elements: []models.OfficeElement{
				{Type: models.Antiphon, Text: "Alleluia"},
				{Type: models.Psalm, Text: "Psalm 2"},
				{Type: models.Antiphon, Text: "Alleluia"},
			},
		},
	}}

	collapseUniformAntiphons(hour)

	assertSectionElements(t, hour, 0,
		"antiphon:Alleluia",
		"psalm:Psalm 1",
		"antiphon:Alleluia",
	)
	assertSectionElements(t, hour, 1, "rubric:A separate ceremony")
	assertSectionElements(t, hour, 2,
		"antiphon:Alleluia",
		"psalm:Psalm 2",
		"antiphon:Alleluia",
	)
}

func TestCollapseUniformAntiphonsTreatsTextsAsMaximalGroups(t *testing.T) {
	hour := &models.OfficeHour{Sections: []models.OfficeSection{{
		Elements: []models.OfficeElement{
			{Type: models.Antiphon, Text: "A"},
			{Type: models.Psalm, Text: "Psalm 1"},
			{Type: models.Antiphon, Text: "A"},
			{Type: models.Psalm, Text: "Psalm 2"},
			{Type: models.Antiphon, Text: "A"},
			{Type: models.Antiphon, Text: "B"},
			{Type: models.Psalm, Text: "Psalm 3"},
			{Type: models.Antiphon, Text: "B"},
			{Type: models.Psalm, Text: "Psalm 4"},
			{Type: models.Antiphon, Text: "B"},
			{Type: models.Antiphon, Text: "C"},
		},
	}}}

	collapseUniformAntiphons(hour)

	assertSectionElements(t, hour, 0,
		"antiphon:A",
		"psalm:Psalm 1",
		"psalm:Psalm 2",
		"antiphon:A",
		"antiphon:B",
		"psalm:Psalm 3",
		"psalm:Psalm 4",
		"antiphon:B",
		"antiphon:C",
	)
}

func TestCollapseUniformAntiphonsIgnoresNonAntiphonTextMatches(t *testing.T) {
	hour := &models.OfficeHour{Sections: []models.OfficeSection{{
		Elements: []models.OfficeElement{
			{Type: models.Antiphon, Text: "Same text"},
			{Type: models.Psalm, Text: "Same text"},
			{Type: models.Rubric, Text: "Same text"},
			{Type: models.Heading, Text: "Same text"},
			{Type: models.Antiphon, Text: "Same text"},
		},
	}}}

	collapseUniformAntiphons(hour)

	assertSectionElements(t, hour, 0,
		"antiphon:Same text",
		"psalm:Same text",
		"rubric:Same text",
		"heading:Same text",
		"antiphon:Same text",
	)
}

func TestSectionHasPsalmody(t *testing.T) {
	tests := []struct {
		name     string
		elements []models.OfficeElement
		want     bool
	}{
		{"psalm", []models.OfficeElement{{Type: models.Psalm}}, true},
		{"canticle", []models.OfficeElement{{Type: models.Canticle}}, true},
		{"non-psalm elements", []models.OfficeElement{{Type: models.Antiphon}, {Type: models.Rubric}}, false},
		{"empty section", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sectionHasPsalmody(models.OfficeSection{Elements: tt.elements})
			if got != tt.want {
				t.Fatalf("sectionHasPsalmody() = %t, want %t", got, tt.want)
			}
		})
	}
}

func assertSectionElements(t *testing.T, hour *models.OfficeHour, section int, want ...string) {
	t.Helper()
	got := make([]string, 0, len(hour.Sections[section].Elements))
	for _, element := range hour.Sections[section].Elements {
		got = append(got, string(element.Type)+":"+element.Text)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("section %d elements = %v, want %v", section, got, want)
	}
}

// --- markPsalmDoxologies ---

func TestMarkPsalmDoxologies(t *testing.T) {
	hour := &models.OfficeHour{
		Sections: []models.OfficeSection{
			{
				Elements: []models.OfficeElement{
					{Type: models.Psalm},
					{Type: models.Doxology}, // should become PsalmDoxology
					{Type: models.Canticle},
					{Type: models.Doxology}, // should become PsalmDoxology
					{Type: models.Versicle},
					{Type: models.Doxology}, // NOT after psalm/canticle — stays Doxology
				},
			},
		},
	}

	markPsalmDoxologies(hour)

	elems := hour.Sections[0].Elements
	if elems[1].Type != models.PsalmDoxology {
		t.Errorf("elems[1].Type = %q, want PsalmDoxology (after Psalm)", elems[1].Type)
	}
	if elems[3].Type != models.PsalmDoxology {
		t.Errorf("elems[3].Type = %q, want PsalmDoxology (after Canticle)", elems[3].Type)
	}
	if elems[5].Type != models.Doxology {
		t.Errorf("elems[5].Type = %q, want Doxology (after Versicle, not promoted)", elems[5].Type)
	}
}

func TestMarkPsalmDoxologiesFirstElement(t *testing.T) {
	// Doxology at index 0 must not panic (no previous element)
	hour := &models.OfficeHour{
		Sections: []models.OfficeSection{
			{
				Elements: []models.OfficeElement{
					{Type: models.Doxology},
				},
			},
		},
	}
	markPsalmDoxologies(hour)
	if hour.Sections[0].Elements[0].Type != models.Doxology {
		t.Error("first-element doxology should stay Doxology")
	}
}

// --- resolveElement ---

func TestResolveElement(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/compline/versicle": "O God, make speed.",
		"psalms/004":                 "Psalm 4 text",
		"ordinary/chapter":           "!Rom 8:1\nChapter body",
		"ordinary/preces":            "Lord have mercy.",
		"ordinary/our-father":        "Our Father, who art in heaven,\nBut deliver us from evil. Amen.",
	})

	tests := []struct {
		name      string
		elem      HourElement
		wantType  models.ElementType
		wantLabel string
		wantText  string
	}{
		{
			name:      "versicle found",
			elem:      HourElement{Type: "versicle", Ref: "ordinary/compline/versicle"},
			wantType:  models.Versicle,
			wantLabel: "",
			wantText:  "O God, make speed.",
		},
		{
			name:      "psalm with label",
			elem:      HourElement{Type: "psalm", Ref: "psalms/004"},
			wantType:  models.Psalm,
			wantLabel: "Psalm 4",
			wantText:  "Psalm 4 text",
		},
		{
			name:      "chapter extracts ref",
			elem:      HourElement{Type: "chapter", Ref: "ordinary/chapter"},
			wantType:  models.Chapter,
			wantLabel: "Rom 8:1",
			wantText:  "Chapter body",
		},
		{
			name:      "preces type",
			elem:      HourElement{Type: "preces", Ref: "ordinary/preces"},
			wantType:  models.Preces,
			wantLabel: "Preces",
			wantText:  "Lord have mercy.",
		},
		{
			name:     "partly secret prayer omits final amen",
			elem:     HourElement{Type: "partly-secret-prayer", Ref: "ordinary/our-father"},
			wantType: models.Prayer,
			wantText: "Our Father, who art in heaven,\nBut deliver us from evil.",
		},
		{
			name:     "secret prayer keeps full text",
			elem:     HourElement{Type: "secret-prayer", Ref: "ordinary/our-father"},
			wantType: models.Prayer,
			wantText: "Our Father, who art in heaven,\nBut deliver us from evil. Amen.",
		},
		{
			name:     "missing ref produces placeholder",
			elem:     HourElement{Type: "versicle", Ref: "missing/ref"},
			wantType: models.Versicle,
			wantText: "[Text not found: missing/ref]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveElement(tt.elem, corpus)
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if tt.wantLabel != "" && got.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", got.Label, tt.wantLabel)
			}
			if got.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", got.Text, tt.wantText)
			}
		})
	}
}

// --- resolveHourElement ---

// --- substituteHymnDoxology ---

func TestSubstituteHymnDoxology(t *testing.T) {
	standardHymn := "First verse\n\nSecond verse\n\nAll laud to God the Father be;\nAll praise, Eternal Son, to thee;\nAll glory, as is ever meet,\nTo God the holy Paraclete. Amen."
	christmasDox := "All honour, laud, and glory be,\nO Jesu, Virgin-born to thee;\nAll glory, as is ever meet,\nTo Father, Son, and Paraclete. Amen."

	tests := []struct {
		name          string
		hymnBody      string
		doxology      string
		wantSuffix    string
		wantUnchanged bool
	}{
		{
			name:       "replaces last stanza with seasonal doxology",
			hymnBody:   standardHymn,
			doxology:   christmasDox,
			wantSuffix: christmasDox,
		},
		{
			name:          "does not substitute hymn without Amen.",
			hymnBody:      "First verse\n\nSecond verse\n\nNot a proper ending",
			doxology:      christmasDox,
			wantUnchanged: true,
		},
		{
			name:          "does not substitute single-stanza hymn (no paragraph break)",
			hymnBody:      "Only stanza. Amen.",
			doxology:      christmasDox,
			wantUnchanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteHymnDoxology(tt.hymnBody, tt.doxology)
			if tt.wantUnchanged {
				if got != tt.hymnBody {
					t.Errorf("expected unchanged, got %q", got)
				}
				return
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("result does not end with expected doxology\ngot: %q\nwant suffix: %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestUsesAscensionHymnDoxology(t *testing.T) {
	tests := []struct {
		name   string
		date   time.Time
		season models.Season
		want   bool
	}{
		{"Eastertide before Ascension", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), models.Easter, false},
		{"Ascension Day", time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC), models.Easter, true},
		{"Eastertide after Ascension", time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC), models.Easter, true},
		{"Pentecost", time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC), models.Pentecost, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date, Season: tt.season}
			if got := usesAscensionHymnDoxology(day); got != tt.want {
				t.Fatalf("usesAscensionHymnDoxology() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveHourElementUsesAscensionHymnDoxology(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"commons/martyr-paschal/hymn-lauds":       "Martyr hymn\n\nFirst stanza.\n\nEaster ending.\nAmen.",
		"seasonal/easter/hymn-doxology-ascension": "Ascension ending.\nAmen.",
	})
	day := &models.CalendarDay{
		Date:        time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC),
		Season:      models.Easter,
		Celebration: &models.Feast{ID: "test-martyr", Category: models.CategoryMartyr},
	}

	got := resolveHourElement(day, "lauds", HourElement{Type: "proper-hymn", Ref: "hymn"}, corpus)
	if !strings.HasSuffix(got.Text, "Ascension ending.\nAmen.") {
		t.Fatalf("hymn text = %q, want Ascension ending", got.Text)
	}
	if len(got.SourceRefs) != 2 || got.SourceRefs[1] != "seasonal/easter/hymn-doxology-ascension" {
		t.Fatalf("source refs = %v, want seasonal Ascension doxology dependency", got.SourceRefs)
	}
}

func TestResolveHourElement(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/lauds/antiphon":    "Ordinary antiphon",
		"ordinary/lauds/collect":     "Ordinary collect",
		"ordinary/terce/collect":     "Terce collect",
		"ordinary/lauds/hymn":        "Hymn Title\n\nHymn body",
		"ordinary/lauds/responsory":  "Ordinary responsory",
		"ordinary/lauds/chapter":     "!Isa 1:1\nChapter body",
		"ordinary/compline/versicle": "Versicle text",
	})

	day := &models.CalendarDay{Season: models.Lent}

	tests := []struct {
		name      string
		elem      HourElement
		wantType  models.ElementType
		wantLabel string
	}{
		{"proper-antiphon", HourElement{Type: "proper-antiphon", Ref: "antiphon"}, models.Antiphon, ""},
		{"proper-collect", HourElement{Type: "proper-collect", Ref: "collect"}, models.Collect, ""},
		{"proper-hymn extracts title", HourElement{Type: "proper-hymn", Ref: "hymn"}, models.Hymn, "Hymn Title"},
		{"proper-responsory", HourElement{Type: "proper-responsory", Ref: "responsory"}, models.Response, ""},
		{"proper-chapter extracts ref", HourElement{Type: "proper-chapter", Ref: "chapter"}, models.Chapter, "Isa 1:1"},
		{"fallthrough versicle", HourElement{Type: "versicle", Ref: "ordinary/compline/versicle"}, models.Versicle, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveHourElement(day, "lauds", tt.elem, corpus)
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if tt.wantLabel != "" && got.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", got.Label, tt.wantLabel)
			}
		})
	}

	t.Run("proper-collect for minor hour reuses lauds collect", func(t *testing.T) {
		got := resolveHourElement(day, "terce", HourElement{Type: "proper-collect", Ref: "collect"}, corpus)
		if got.Type != models.Collect {
			t.Fatalf("Type = %q, want %q", got.Type, models.Collect)
		}
		if got.Text != "Ordinary collect" {
			t.Fatalf("Text = %q, want %q", got.Text, "Ordinary collect")
		}
	})
}

func TestResolveMarianElementUsesFebruarySecondOfficeBoundary(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"ordinary/marian/alma-redemptoris-christmas": "Alma",
		"ordinary/marian/ave-regina-caelorum":        "Ave",
	})
	day := &models.CalendarDay{
		Date:           time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC),
		MarianAntiphon: "ave-regina-caelorum",
	}

	vespers := resolveMarianElement(day, "vespers", corpus)
	if vespers.Text != "Alma" || vespers.Label != "Alma Redemptoris Mater" {
		t.Fatalf("Vespers Marian antiphon = (%q, %q), want Alma Redemptoris", vespers.Text, vespers.Label)
	}
	if vespers.SlotRef != "marian-antiphon" {
		t.Fatalf("Vespers Marian SlotRef = %q, want marian-antiphon", vespers.SlotRef)
	}
	vKey, vBoundary := marianAntiphonSelection(day, "vespers")
	if vKey != "alma-redemptoris-christmas" || vBoundary != MarianBoundaryPurificationVespersOverride {
		t.Fatalf("vespers selection = (%q, %q)", vKey, vBoundary)
	}

	compline := resolveMarianElement(day, "compline", corpus)
	if compline.Text != "Ave" || compline.Label != "Ave Regina Caelorum" {
		t.Fatalf("Compline Marian antiphon = (%q, %q), want Ave Regina", compline.Text, compline.Label)
	}
	if compline.SlotRef != "marian-antiphon" || !strings.HasPrefix(compline.SourceRef, "ordinary/marian/") {
		t.Fatalf("Compline Marian metadata = slot %q ref %q", compline.SlotRef, compline.SourceRef)
	}
	cKey, cBoundary := marianAntiphonSelection(day, "compline")
	if cKey != "ave-regina-caelorum" || cBoundary != MarianBoundaryCivilDay {
		t.Fatalf("compline selection = (%q, %q)", cKey, cBoundary)
	}

	// Decision traces must diverge for the Feb 2 boundary.
	vHour := &models.OfficeHour{}
	appendContextDecisions(vHour, day, "vespers", nil)
	assertDecisionPresent(t, vHour.Decisions, "marian:selection", "alma-redemptoris-christmas")
	assertDecisionPresent(t, vHour.Decisions, "marian:boundary", MarianBoundaryPurificationVespersOverride)

	cHour := &models.OfficeHour{}
	appendContextDecisions(cHour, day, "compline", nil)
	assertDecisionPresent(t, cHour.Decisions, "marian:selection", "ave-regina-caelorum")
	assertDecisionPresent(t, cHour.Decisions, "marian:boundary", MarianBoundaryCivilDay)
}

func assertDecisionPresent(t *testing.T, decisions []models.CompositionDecision, rule, outcome string) {
	t.Helper()
	for _, d := range decisions {
		if d.Rule == rule && d.Outcome == outcome {
			return
		}
	}
	t.Errorf("missing decision %s=%s in %#v", rule, outcome, decisions)
}

func TestAntiphonsDoubled(t *testing.T) {
	double := &models.CalendarDay{
		Celebration: &models.Feast{ID: "st-joseph", Rank: models.Double1stClass},
	}
	semi := &models.CalendarDay{
		Celebration: &models.Feast{ID: "epiphany-sunday-3", Rank: models.SemiDouble},
	}
	feria := &models.CalendarDay{}

	if !antiphonsDoubled(double, "lauds") {
		t.Error("Double feast Lauds should double antiphons")
	}
	if !antiphonsDoubled(double, "vespers") {
		t.Error("Double feast Vespers should double antiphons")
	}
	// Even on a Double feast, the Little Hours and Compline only announce.
	for _, hour := range []string{"prime", "terce", "sext", "none", "compline"} {
		if antiphonsDoubled(double, hour) {
			t.Errorf("%s should never double antiphons", hour)
		}
	}
	if antiphonsDoubled(semi, "lauds") {
		t.Error("Semi-double Lauds should only announce")
	}
	if antiphonsDoubled(feria, "lauds") {
		t.Error("ferial Lauds should only announce")
	}

	// I Vespers of a following Double on a ferial civil day: the Vespers
	// office (not the civil celebration) decides doubling. Negating the
	// hourName == "vespers" branch would consult today's nil celebration and
	// wrongly refuse to double.
	iVespersOfDouble := &models.CalendarDay{
		Celebration: nil,
		Vespers: models.VespersDesignation{
			Owner: models.VespersIOfFollowing,
			Feast: &models.Feast{ID: "st-joseph", Rank: models.Double1stClass},
		},
	}
	if !antiphonsDoubled(iVespersOfDouble, "vespers") {
		t.Error("I Vespers of a Double must double antiphons even when today is a feria")
	}
	if antiphonsDoubled(iVespersOfDouble, "lauds") {
		t.Error("ferial Lauds on the civil day must not double just because Vespers is Double")
	}

	// Symmetrically: II Vespers stays with today's Double, not tomorrow.
	iiVespers := &models.CalendarDay{
		Celebration: &models.Feast{ID: "st-joseph", Rank: models.Double},
		Vespers: models.VespersDesignation{
			Owner: models.VespersIIOfPreceding,
			Feast: &models.Feast{ID: "st-joseph", Rank: models.Double},
		},
	}
	if !antiphonsDoubled(iiVespers, "vespers") {
		t.Error("II Vespers of a Double should double")
	}
}

func TestMarkAnnouncedAntiphons(t *testing.T) {
	full := "Do away, O Lord, * mine offenses."
	hour := &models.OfficeHour{Sections: []models.OfficeSection{{
		Elements: []models.OfficeElement{
			{Type: models.Antiphon, Text: "Alleluia"}, // opening — not a psalm frame
			{Type: models.Antiphon, Text: full},
			{Type: models.Psalm, Text: "Psalm 51"},
			{Type: models.PsalmDoxology, Text: "Glory be"},
			{Type: models.Antiphon, Text: full},
			{Type: models.Heading, Text: "Commemoration of N."},
			{Type: models.Antiphon, Text: "Pray for us, O holy N. *"},
			{Type: models.Versicle, Text: "V. The Lord hath chosen him."},
		},
	}}}

	// Feria: announce before-psalmody only.
	markAnnouncedAntiphons(hour, &models.CalendarDay{}, "lauds")
	els := hour.Sections[0].Elements
	if els[0].Announce {
		t.Error("opening Alleluia must not be announced")
	}
	if !els[1].Announce {
		t.Error("opening psalm antiphon must be announced on a feria")
	}
	if els[4].Announce {
		t.Error("closing psalm antiphon must stay full")
	}
	if els[6].Announce {
		t.Error("commemoration antiphon must stay full")
	}

	// Reset and mark as Double Lauds: nothing announced.
	for i := range els {
		els[i].Announce = false
	}
	day := &models.CalendarDay{Celebration: &models.Feast{Rank: models.Double}}
	markAnnouncedAntiphons(hour, day, "lauds")
	for i, el := range hour.Sections[0].Elements {
		if el.Announce {
			t.Errorf("element %d should not be announced on Double Lauds", i)
		}
	}

	// Double feast at Prime still announces.
	for i := range els {
		els[i].Announce = false
	}
	markAnnouncedAntiphons(hour, day, "prime")
	if !hour.Sections[0].Elements[1].Announce {
		t.Error("Prime on a Double feast still only announces the opening antiphon")
	}
}

func TestIsBeforePsalmodyAntiphon(t *testing.T) {
	elems := []models.OfficeElement{
		{Type: models.Antiphon, Text: "A"},
		{Type: models.Psalm, Text: "p"},
		{Type: models.PsalmDoxology, Text: "g"},
		{Type: models.Antiphon, Text: "A"},
		{Type: models.Antiphon, Text: "B"},
		{Type: models.Canticle, Text: "c"},
		{Type: models.Antiphon, Text: "B"},
	}
	want := map[int]bool{0: true, 3: false, 4: true, 6: false}
	for i, w := range want {
		if got := isBeforePsalmodyAntiphon(elems, i); got != w {
			t.Errorf("index %d: got %v, want %v", i, got, w)
		}
	}
	// Out-of-range and non-antiphon indices must be false (boundary on i >= len).
	for _, i := range []int{-1, len(elems), len(elems) + 1} {
		if isBeforePsalmodyAntiphon(elems, i) {
			t.Errorf("index %d should be false", i)
		}
	}
	if isBeforePsalmodyAntiphon(elems, 1) { // psalm, not antiphon
		t.Error("psalm index must not count as a before-antiphon")
	}
	// Trailing antiphon with nothing after is a closer, not an opener.
	trailing := []models.OfficeElement{{Type: models.Antiphon, Text: "only"}}
	if isBeforePsalmodyAntiphon(trailing, 0) {
		t.Error("sole antiphon with no following psalm is not a before-antiphon")
	}
}
