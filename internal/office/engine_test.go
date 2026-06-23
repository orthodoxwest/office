package office

import (
	"strings"
	"testing"

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
