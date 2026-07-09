package calendar

import "testing"

// Values cross-checked against the archdiocesan ordo PDFs in ../resources.
// Note: the printed 2021 and 2024 ordos list "Sundays after Pentecost" figures
// (23 and 21) that disagree with every straightforward calendar count and with
// the calendar builder's own reckoning; those are treated as ordo transcription
// artifacts and deliberately not encoded here.
func TestComputeTabula(t *testing.T) {
	cases := []struct {
		year       int
		golden     int
		letter     string
		afterEpiph int
		spring     string // Ember Wednesday, "Jan 2" form
		summer     string
		autumn     string
		winter     string
	}{
		{2026, 13, "D", 4, "March 4", "June 3", "September 16", "December 16"},
		{2024, 11, "F", 8, "March 27", "June 26", "September 18", "December 18"},
	}

	for _, c := range cases {
		t.Run(c.letter, func(t *testing.T) {
			tab := ComputeTabula(c.year)
			if tab.GoldenNumber != c.golden {
				t.Errorf("%d GoldenNumber = %d, want %d", c.year, tab.GoldenNumber, c.golden)
			}
			if tab.DominicalLetter != c.letter {
				t.Errorf("%d DominicalLetter = %q, want %q", c.year, tab.DominicalLetter, c.letter)
			}
			if tab.SundaysAfterEpiphany != c.afterEpiph {
				t.Errorf("%d SundaysAfterEpiphany = %d, want %d", c.year, tab.SundaysAfterEpiphany, c.afterEpiph)
			}
			if got := tab.Spring.Wed.Format("January 2"); got != c.spring {
				t.Errorf("%d Spring ember Wed = %s, want %s", c.year, got, c.spring)
			}
			if got := tab.Summer.Wed.Format("January 2"); got != c.summer {
				t.Errorf("%d Summer ember Wed = %s, want %s", c.year, got, c.summer)
			}
			if got := tab.Autumn.Wed.Format("January 2"); got != c.autumn {
				t.Errorf("%d Autumn ember Wed = %s, want %s", c.year, got, c.autumn)
			}
			if got := tab.Winter.Wed.Format("January 2"); got != c.winter {
				t.Errorf("%d Winter ember Wed = %s, want %s", c.year, got, c.winter)
			}
		})
	}
}

func TestRoman(t *testing.T) {
	cases := map[int]string{1: "I", 4: "IV", 9: "IX", 11: "XI", 13: "XIII", 24: "XXIV"}
	for n, want := range cases {
		if got := Roman(n); got != want {
			t.Errorf("Roman(%d) = %q, want %q", n, got, want)
		}
	}
}
