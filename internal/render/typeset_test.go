package render

import "testing"

func TestTypeset(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"even unto Aaron's beard", "even unto Aaron’s beard"},
		{"the angels' song", "the angels’ song"},
		{"Safe on th' eternal shore.", "Safe on th’ eternal shore."},
		{"Disperse th'oppressive shades", "Disperse th’oppressive shades"},
		{"'Mid the Twelve, his chosen band,", "’Mid the Twelve, his chosen band,"},
		{"'Tis the season", "’Tis the season"},
		{"his name of 'the Watchful'.", "his name of ‘the Watchful’."},
		{"Whose name 'God's might' doth signify:", "Whose name ‘God’s might’ doth signify:"},
		{"'The Lord is risen from the dead.'", "‘The Lord is risen from the dead.’"},
		{`He said, "Peace be unto you."`, "He said, “Peace be unto you.”"},
		{"Luke 1:46-55", "Luke 1:46–55"},
		{"2 Cor. 1:3-4", "2 Cor. 1:3–4"},
		{"eye-lids to slumber", "eye-lids to slumber"},
		{"resting-place", "resting-place"},
		{"No punctuation to change.", "No punctuation to change."},
	} {
		if got := Typeset(tc.in); got != tc.want {
			t.Errorf("Typeset(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEscTextTypesetsBeforeEscaping(t *testing.T) {
	if got, want := escText(`David's <b>`), "David’s &lt;b&gt;"; got != want {
		t.Errorf("escText = %q, want %q", got, want)
	}
}
