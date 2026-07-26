package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
)

// bodyClasses returns the class attribute of the rendered page's <body>.
func bodyClasses(t *testing.T, s *Server, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleRoot(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d", path, rec.Code)
	}
	body := rec.Body.String()
	const open = `<body class="`
	i := strings.Index(body, open)
	if i < 0 {
		t.Fatalf("GET %s: no <body class=...> in response", path)
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatalf("GET %s: unterminated body class attribute", path)
	}
	return rest[:j]
}

func hasClass(classes, want string) bool {
	for _, c := range strings.Fields(classes) {
		if c == want {
			return true
		}
	}
	return false
}

// Dates are derived from the Julian paschalion rather than hardcoded: Julian
// Pascha differs from Gregorian Easter, so a hand-written "Easter Sunday" would
// silently test the wrong day.
func TestSeasonalOrnamentClassFollowsThePaschalion(t *testing.T) {
	s, err := New("../../data", ":0")
	if err != nil {
		t.Fatal(err)
	}

	const year = 2026
	m := calendar.ComputeMoveableDates(year)
	day := func(d time.Time) string { return d.Format("2006-01-02") }

	for _, tc := range []struct {
		what string
		date time.Time
		want string // "" means no season class at all
	}{
		// Veiled: Passion Sunday through Holy Saturday.
		{"Passion Sunday", m.PassionSunday, "season-passiontide"},
		{"mid-Passiontide", m.PassionSunday.AddDate(0, 0, 3), "season-passiontide"},
		{"Palm Sunday", m.PassionSunday.AddDate(0, 0, 7), "season-passiontide"},
		{"Good Friday", m.GoodFriday, "season-passiontide"},
		{"Holy Saturday", m.HolySaturday, "season-passiontide"},

		// Unveiled either side. Lent proper keeps the ordinary gold: the
		// veiling starts at Passion Sunday, not Ash Wednesday.
		{"Saturday before Passion Sunday", m.PassionSunday.AddDate(0, 0, -1), ""},
		{"Ash Wednesday", m.AshWednesday, ""},

		// Bright: Paschaltide, Easter Sunday through the eve of Pentecost.
		{"Easter Sunday", m.Easter, "season-eastertide"},
		{"Easter Monday", m.Easter.AddDate(0, 0, 1), "season-eastertide"},
		{"Low Sunday (octave day)", m.Easter.AddDate(0, 0, 7), "season-eastertide"},
		{"Ascension", m.Ascension, "season-eastertide"},
		{"eve of Pentecost", m.Pentecost.AddDate(0, 0, -1), "season-eastertide"},

		// Ordinary gold resumes after Paschaltide closes.
		{"Pentecost", m.Pentecost, ""},
		{"Trinity Sunday", m.Pentecost.AddDate(0, 0, 7), ""},
	} {
		t.Run(tc.what, func(t *testing.T) {
			slug := day(tc.date)
			for _, path := range []string{"/lauds/" + slug, "/?date=" + slug} {
				classes := bodyClasses(t, s, path)
				for _, class := range []string{"season-passiontide", "season-eastertide"} {
					want := class == tc.want
					if got := hasClass(classes, class); got != want {
						t.Errorf("%s (%s): %s has %q = %v, want %v (classes: %q)",
							tc.what, slug, path, class, got, want, classes)
					}
				}
			}
		})
	}
}

// Holy Saturday's Vespers is I Vespers of Easter, so the hour composer reports
// the Easter season for that office while the day itself is still Passiontide.
// The ornament follows the hour on hour pages and the day on the home page:
// the gold is unveiled for the paschal Vespers, and Lauds that morning is not.
func TestHolySaturdayVespersUnveilsAheadOfTheDay(t *testing.T) {
	s, err := New("../../data", ":0")
	if err != nil {
		t.Fatal(err)
	}
	slug := calendar.ComputeMoveableDates(2026).HolySaturday.Format("2006-01-02")

	if classes := bodyClasses(t, s, "/lauds/"+slug); !hasClass(classes, "season-passiontide") {
		t.Errorf("Holy Saturday Lauds classes = %q, want the veil", classes)
	}
	if classes := bodyClasses(t, s, "/vespers/"+slug); !hasClass(classes, "season-eastertide") {
		t.Errorf("Holy Saturday Vespers classes = %q, want paschal gold", classes)
	}
	if classes := bodyClasses(t, s, "/?date="+slug); !hasClass(classes, "season-passiontide") {
		t.Errorf("Holy Saturday home classes = %q, want the veil", classes)
	}
}

// The veil covers the gilding and nothing else. Functional uses of gold —
// focus rings, disclosure markers, today's ordo row — must keep reading --gold
// directly so no season can dim an affordance.
func TestOnlyOrnamentsAreSeasonallyRetinted(t *testing.T) {
	css, err := os.ReadFile("static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	style := string(css)

	for _, want := range []string{
		"--ornament: var(--gold);",
		"--ornament-line: var(--gold-line);",
		"body.season-passiontide {",
		"body.season-eastertide {",
		// The ornaments themselves.
		".cross {\n  color: var(--ornament);",
		"border-top: 3px double var(--ornament-line);",
	} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css is missing %q", want)
		}
	}

	// Functional gold, which the seasons must not reach.
	for _, want := range []string{
		"outline: 2px solid var(--gold);",                  // focus ring
		"color: var(--gold-line);\n  font-size: 0.85em",    // hour date-nav disclosure marker
		"color-mix(in srgb, var(--gold) 14%, transparent)", // today's ordo row
	} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css no longer keeps functional gold %q", want)
		}
	}

	// Both themes must define both seasons, or one appearance would be veiled
	// and the other not.
	for _, season := range []string{"season-passiontide", "season-eastertide"} {
		if n := strings.Count(style, "body."+season+" {"); n != 3 {
			t.Errorf("body.%s is declared %d times, want 3 (default, prefers-dark, data-theme=dark)", season, n)
		}
	}
}
