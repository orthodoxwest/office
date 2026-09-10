package cli

import (
	"strings"
	"testing"
)

func TestPrayerFormCommands(t *testing.T) {
	for _, form := range []string{"private", "deacon", "priest"} {
		for _, format := range []string{"text", "tex"} {
			t.Run(form+"/"+format, func(t *testing.T) {
				e, out, _ := testEnv(t)
				var err error
				if format == "text" {
					err = cmdHour(e, "compline", []string{"2026-03-11", "--form", form})
				} else {
					err = cmdTeX(e, []string{"--form=" + form, "compline", "2026-03-11"})
				}
				if err != nil {
					t.Fatal(err)
				}
				want := 2
				if form != "priest" {
					want = 1
				}
				if got := strings.Count(strings.ToLower(out.String()), "i confess to god almighty"); got != want {
					t.Fatalf("confession count = %d, want %d", got, want)
				}
			})
		}
	}
}

func TestPrayerFormOptionRejectsAmbiguity(t *testing.T) {
	for _, args := range [][]string{{"--form"}, {"--form="}, {"--form", "lay"}, {"--form=priest", "--form=deacon"}} {
		if _, _, err := takePrayerForm(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}
