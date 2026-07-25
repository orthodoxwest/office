package office

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestSummarizeHourExtractsRubricFlags(t *testing.T) {
	hour := &models.OfficeHour{
		Color: models.Red,
		Sections: []models.OfficeSection{
			{
				Label: "Benedictus",
				Elements: []models.OfficeElement{
					{
						Type:    models.Antiphon,
						SlotRef: "benedictus-antiphon",
						Text:    "Blessed be the Lord God of Israel,\nfor he hath visited * and redeemed his people.",
					},
				},
			},
			{
				Label: "Preces",
				Elements: []models.OfficeElement{
					{Type: models.Preces, Text: "V. Lord, have mercy."},
				},
			},
			{
				Label: "Suffrage of the Saints",
				Elements: []models.OfficeElement{
					{Type: models.Heading, Text: "Commemoration of St. Barnabas"},
					{Type: models.Antiphon, Text: "He was a good man * and full of the Holy Ghost."},
					{Type: models.Heading, Text: "Commemoration of the Cross"},
					// No antiphon before the section ends: the incipit stays empty.
				},
			},
		},
	}

	got := SummarizeHour(hour)

	if got.Color != models.Red {
		t.Errorf("Color = %q, want red", got.Color)
	}
	if !got.Preces {
		t.Error("Preces = false, want true")
	}
	if !got.Suffrage {
		t.Error("Suffrage = false, want true (section labelled Suffrage)")
	}
	// Cut at the mediant, then capped at nine words.
	if want := "Blessed be the Lord God of Israel, for he…"; got.GospelAnt != want {
		t.Errorf("GospelAnt = %q, want %q", got.GospelAnt, want)
	}
	wantFull := "Blessed be the Lord God of Israel, for he hath visited * and redeemed his people."
	if got.GospelAntFull != wantFull {
		t.Errorf("GospelAntFull = %q, want %q", got.GospelAntFull, wantFull)
	}
	if len(got.Comms) != 2 {
		t.Fatalf("Comms = %#v, want 2", got.Comms)
	}
	if got.Comms[0].Name != "St. Barnabas" || got.Comms[0].Incipit != "He was a good man" {
		t.Errorf("first commemoration = %#v", got.Comms[0])
	}
	if got.Comms[1].Name != "the Cross" || got.Comms[1].Incipit != "" {
		t.Errorf("second commemoration = %#v", got.Comms[1])
	}
}

func TestSummarizeHourQuietDay(t *testing.T) {
	hour := &models.OfficeHour{
		Color: models.Green,
		Sections: []models.OfficeSection{{
			Label:    "Psalmody",
			Elements: []models.OfficeElement{{Type: models.Psalm, Text: "1. Blessed is the man"}},
		}},
	}

	got := SummarizeHour(hour)
	if got.Preces || got.Suffrage || got.GospelAnt != "" || got.GospelAntFull != "" || len(got.Comms) != 0 {
		t.Errorf("quiet day summary = %#v", got)
	}
}

func TestIncipitTruncatesAtMediantOrNineWords(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"mediant wins", "Seek ye first the kingdom of God * and his righteousness.", "Seek ye first the kingdom of God"},
		{"nine words then ellipsis", "One two three four five six seven eight nine ten eleven", "One two three four five six seven eight nine…"},
		{"trailing punctuation stripped", "Alleluia, alleluia, alleluia.", "Alleluia, alleluia, alleluia"},
		{"newlines flattened", "O Lord,\nhear my prayer.", "O Lord, hear my prayer"},
		{"empty", "   ", ""},
		{"leading mediant is not a cut point", "* and his righteousness", "* and his righteousness"},
	}
	for _, tt := range tests {
		if got := incipit(tt.text); got != tt.want {
			t.Errorf("%s: incipit(%q) = %q, want %q", tt.name, tt.text, got, tt.want)
		}
	}
}
