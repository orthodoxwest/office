package texts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

const testAppointmentScope = `[{"id":"sample","source":"test requirement","season":"lent","hours":["terce"],"slots":["chapter"],"require_ferial":true,"exclude_weekdays":["sunday"],"from_easter":-10,"until_easter":-2}]`

func scopeFixture(t *testing.T, raw string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "texts", "seasonal"), 0755); err != nil {
		t.Fatal(err)
	}
	body := "[chapter-terce]\nFerial chapter\n\n[versicle-terce]\n@omit\n\n[psalm-antiphon-terce]\n@use seasonal/lent/chapter-terce\n"
	if err := os.WriteFile(filepath.Join(dir, "texts", "seasonal", "lent.txt"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "appointment-scopes.json")
	if raw != "" {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, path
}

func TestAppointmentScopeBoundsAndOfficeKinds(t *testing.T) {
	dir, _ := scopeFixture(t, testAppointmentScope)
	c, err := LoadTexts(dir)
	if err != nil {
		t.Fatal(err)
	}
	scope := c.SeasonalAppointmentScope(models.Lent, "terce", "chapter")
	if scope == nil {
		t.Fatal("missing configured scope")
	}
	easter := time.Date(2032, 5, 2, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name         string
		offset       int
		weekday      time.Weekday
		ferial, want bool
	}{
		{"before", -11, time.Thursday, true, false},
		{"inclusive start", -10, time.Friday, true, true},
		{"last day", -3, time.Thursday, true, true},
		{"exclusive end", -2, time.Friday, true, false},
		{"feast", -5, time.Tuesday, false, false},
		{"Sunday even if mislabeled feria", -7, time.Sunday, true, false},
		{"civil weekday supplied by composer", -7, time.Saturday, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := scope.Allows(easter.AddDate(0, 0, tc.offset), easter, tc.weekday, tc.ferial); got != tc.want {
				t.Fatalf("Allows = %v, want %v", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		season     models.Season
		hour, slot string
	}{
		{models.Lent, "lauds", "chapter"}, {models.Easter, "terce", "chapter"}, {models.Lent, "terce", "versicle"},
	} {
		if c.SeasonalAppointmentScope(tc.season, tc.hour, tc.slot) != nil {
			t.Errorf("scope leaked into %+v", tc)
		}
	}
	if c.Get("seasonal/lent/chapter-terce") != "Ferial chapter" || !IsOmitted(c.Get("seasonal/lent/versicle-terce")) {
		t.Fatal("scope metadata mutated corpus wording or omission")
	}
	if len(c.texts) != 2 || len(c.aliases) != 1 {
		t.Fatal("metadata became corpus entries")
	}
}

func TestAppointmentScopeLoaderRejectsInvalidData(t *testing.T) {
	var rules []map[string]any
	if err := json.Unmarshal([]byte(testAppointmentScope), &rules); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, field string
		value       any
		want        string
	}{
		{"unknown field", "ferail", true, "unknown field"},
		{"missing source", "source", "", "required"},
		{"bad season", "season", "lnt", "unknown season"},
		{"bad hour", "hours", []string{"matins"}, "unknown hour"},
		{"duplicate hour", "hours", []string{"terce", "terce"}, "duplicate hour"},
		{"bad slot", "slots", []string{"chapetr"}, "invalid slot"},
		{"qualified slot", "slots", []string{"chapter-terce"}, "invalid slot"},
		{"numeric slot", "slots", []string{"psalm-antiphon-2"}, "invalid slot"},
		{"duplicate selector", "slots", []string{"chapter", "chapter"}, "overlapping slot"},
		{"unknown weekday", "exclude_weekdays", []string{"sun"}, "unknown weekday"},
		{"duplicate weekday", "exclude_weekdays", []string{"sunday", "sunday"}, "duplicate weekday"},
		{"empty interval", "until_easter", float64(-10), "must precede"},
		{"inverted interval", "until_easter", float64(-11), "must precede"},
		{"unbounded offset", "from_easter", float64(-999), "between -366"},
		{"missing candidate", "hours", []string{"sext"}, "no seasonal corpus candidate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clone := make(map[string]any)
			for k, v := range rules[0] {
				clone[k] = v
			}
			clone[tc.field] = tc.value
			b, err := json.Marshal([]map[string]any{clone})
			if err != nil {
				t.Fatal(err)
			}
			dir, _ := scopeFixture(t, string(b))
			_, err = LoadTexts(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
	for _, raw := range []string{"null", "[null]", testAppointmentScope + " []", strings.TrimSuffix(testAppointmentScope, "]") + "," + strings.TrimPrefix(testAppointmentScope, "["), strings.TrimSuffix(testAppointmentScope, "]") + "," + strings.Replace(strings.TrimPrefix(testAppointmentScope, "["), `"sample"`, `"other"`, 1)} {
		dir, _ := scopeFixture(t, raw)
		if _, err := LoadTexts(dir); err == nil {
			t.Fatalf("accepted invalid or overlapping scopes: %s", raw)
		}
	}
}

func TestAppointmentScopeOpenBoundsFamilyAndAtomicLoad(t *testing.T) {
	raw := `[{"id":"family","source":"test","season":"lent","hours":["terce"],"slots":["psalm-antiphon*"],"from_easter":0}]`
	dir, path := scopeFixture(t, raw)
	c, err := LoadTexts(dir)
	if err != nil {
		t.Fatal(err)
	}
	scope := c.SeasonalAppointmentScope(models.Lent, "terce", "psalm-antiphon")
	if scope == nil || c.SeasonalAppointmentScope(models.Lent, "terce", "psalm-antiphon-variant") != scope {
		t.Fatal("slot family did not match")
	}
	date := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	if !scope.Allows(date.AddDate(0, 0, 100), date, time.Monday, false) {
		t.Fatal("open upper bound restricted a date")
	}
	if err := os.WriteFile(path, []byte(`[{"id":"bad"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadAppointmentScopes(path); err == nil {
		t.Fatal("accepted invalid replacement")
	}
	if c.SeasonalAppointmentScope(models.Lent, "terce", "psalm-antiphon") != scope {
		t.Fatal("invalid load changed active scopes")
	}
	for _, raw := range []string{"", "[]"} {
		dir, _ := scopeFixture(t, raw)
		c, err := LoadTexts(dir)
		if err != nil {
			t.Fatal(err)
		}
		if c.SeasonalAppointmentScope(models.Lent, "terce", "chapter") != nil {
			t.Fatal("invented default scope")
		}
	}
}
