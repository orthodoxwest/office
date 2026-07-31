package models

import "testing"

func TestAntiphonAnnouncement(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{
			name: "asterisk mediant",
			in:   "Do away, O Lord, * mine offenses.",
			want: "Do away, O Lord.",
		},
		{
			name: "dagger mediant",
			in:   "O Lord, open thou my lips † and my mouth shall shew thy praise.",
			want: "O Lord, open thou my lips.",
		},
		{
			name: "double-dagger mediant",
			in:   "O praise the Lord ‡ all ye nations.",
			want: "O praise the Lord.",
		},
		{
			name: "earliest of several marks wins",
			// A dagger before an asterisk must not lose to the later mark.
			in:   "First half † then * a later asterisk.",
			want: "First half.",
		},
		{
			name: "asterisk before dagger still wins when earlier",
			in:   "First * then † later.",
			want: "First.",
		},
		{
			name: "existing period is preserved",
			in:   "I have yet many things to say unto you, but ye cannot bear them now. * Howbeit, when He is come.",
			want: "I have yet many things to say unto you, but ye cannot bear them now.",
		},
		{
			name: "existing question mark is preserved",
			in:   "Who is like unto thee? * Among the gods.",
			want: "Who is like unto thee?",
		},
		{
			name: "existing exclamation mark is preserved",
			in:   "O praise the Lord! * All ye nations.",
			want: "O praise the Lord!",
		},
		{
			name: "mark at the start of the text",
			// A leading mark has no words to abbreviate, so retain the full text.
			in:   "* Alleluia, alleluia.",
			want: "* Alleluia, alleluia.",
		},
		{
			name: "leading mark still beats a later mark",
			// The leading mark means the later mark cannot create an incipit.
			in:   "* first half † rest of antiphon.",
			want: "* first half † rest of antiphon.",
		},
		{
			name: "mark after one-character incipit",
			// The first safe boundary is index one; only a leading mark is whole.
			in:   "A* later words.",
			want: "A.",
		},
		{
			name: "no mark keeps full text",
			in:   "Alleluia",
			want: "Alleluia",
		},
		{
			name: "trims surrounding space",
			in:   "  Behold an high priest, * who in his days pleased God.  ",
			want: "Behold an high priest.",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "whitespace only",
			in:   "   ",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AntiphonAnnouncement(tt.in); got != tt.want {
				t.Errorf("AntiphonAnnouncement(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestOfficeElementDisplayText(t *testing.T) {
	full := "Do away, O Lord, * mine offenses."
	el := OfficeElement{Type: Antiphon, Text: full}
	if got := el.DisplayText(); got != full {
		t.Fatalf("unannounced DisplayText = %q, want full text", got)
	}
	el.Announce = true
	want := "Do away, O Lord."
	if got := el.DisplayText(); got != want {
		t.Fatalf("announced DisplayText = %q, want %q", got, want)
	}
	// Non-antiphons ignore Announce.
	psalm := OfficeElement{Type: Psalm, Text: "GOD be merciful", Announce: true}
	if got := psalm.DisplayText(); got != "GOD be merciful" {
		t.Fatalf("psalm DisplayText = %q", got)
	}
}
