package usage

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDailyDeduplicationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 23, 59, 0, 0, eastern)
	ctx := context.Background()
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if err := s.Record(ctx, now, "browser-a", "lauds"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for _, event := range []struct{ id, scope string }{{"browser-a", "vespers"}, {"browser-b", "lauds"}, {"browser-c", "site"}, {"browser-d", "ordo"}, {"browser-e", "reminders"}} {
		if err := s.Record(ctx, now, event.id, event.scope); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.Daily(ctx, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Users != 5 || rows[0].Hours[0] != 2 || rows[0].Hours[5] != 1 || rows[0].Ordo != 1 || rows[0].Reminders != 1 || rows[1].Users != 0 {
		t.Fatalf("counts: %+v", rows)
	}
	tomorrow := now.Add(2 * time.Minute)
	if err := s.Record(ctx, tomorrow, "browser-a", "lauds"); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Daily(ctx, tomorrow, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Day != "2026-09-05" || rows[0].Users != 1 || rows[1].Users != 5 {
		t.Fatalf("midnight counts: %+v", rows)
	}
	var hashes int
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT browser) FROM seen WHERE scope='vespers' OR day='2026-09-05'").Scan(&hashes); err != nil {
		t.Fatal(err)
	}
	if hashes != 2 {
		t.Fatalf("daily hashes were reused: %d", hashes)
	}
	future := now.AddDate(0, 0, 4)
	rows, err = s.Daily(ctx, future, 7)
	if err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM seen").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 || rows[4].Users != 5 {
		t.Fatalf("retention lost totals or retained identifiers: %d %+v", remaining, rows)
	}
}

func TestReportingDayAcrossDST(t *testing.T) {
	for _, value := range []string{"2026-03-08T04:59:00Z", "2026-11-01T03:59:00Z"} {
		now, _ := time.Parse(time.RFC3339, value)
		if Day(now) == now.Format(time.DateOnly) {
			t.Fatalf("used UTC instead of Eastern: %s", value)
		}
	}
}

func TestDimensionVocabularyIsUnambiguous(t *testing.T) {
	// Counts live under "<key>:<value>" forever, so the vocabulary has to keep
	// those names distinct and colon-free: a duplicate key, a value shared
	// within a family, or a page scope that could be mistaken for a qualified
	// dimension would all misattribute rows written years apart.
	keys := map[string]bool{}
	scopes := map[string]bool{}
	for _, d := range Dimensions {
		if d.Key == "" || strings.Contains(d.Key, ":") || keys[d.Key] {
			t.Fatalf("dimension key %q is empty, qualified, or repeated", d.Key)
		}
		keys[d.Key] = true
		if d.Values[0] == d.Values[1] {
			t.Fatalf("dimension %q has one value twice", d.Key)
		}
		for _, value := range d.Values {
			if value == "" || strings.Contains(value, ":") {
				t.Fatalf("dimension %q has an empty or qualified value %q", d.Key, value)
			}
			scope := d.Scope(value)
			if scopes[scope] || ValidScope(scope) {
				t.Fatalf("dimension scope %q collides with another scope", scope)
			}
			scopes[scope] = true
			if key, ok := dimensionKey(scope); !ok || key != d.Key {
				t.Fatalf("scope %q does not resolve to its own family", scope)
			}
		}
	}
	for _, scope := range append([]string{"site", "ordo", "reminders"}, Hours...) {
		if strings.Contains(scope, ":") {
			t.Fatalf("page scope %q reaches into the dimension namespace", scope)
		}
	}
}

func TestBeaconDimensionsParseAndCount(t *testing.T) {
	for _, tc := range []struct {
		body  string
		scope string
		dims  []string
	}{
		// What the current app.js sends.
		{"lauds appearance:apse screen:mobile", "lauds", []string{"appearance:apse", "screen:mobile"}},
		{"site appearance:nave screen:desktop", "site", []string{"appearance:nave", "screen:desktop"}},
		// A browser still running app.js from before dimensions existed: the
		// page counts exactly as it always did.
		{"vespers", "vespers", nil},
		// Tokens this build cannot read are dropped, not fatal — a client
		// cached from a newer build must not lose its page count. A bare value
		// is one of those: it names no family, so it is not counted as one.
		{"ordo chant:gabc appearance:apse", "ordo", []string{"appearance:apse"}},
		{"ordo apse", "ordo", nil},
		// One value per family wins; the rest are noise.
		{"prime appearance:nave appearance:apse screen:mobile", "prime", []string{"appearance:nave", "screen:mobile"}},
	} {
		event, ok := ParseEvent(tc.body)
		if !ok || event.Scope != tc.scope || !slices.Equal(event.Dimensions, tc.dims) {
			t.Errorf("ParseEvent(%q) = %+v %v", tc.body, event, ok)
		}
	}
	for _, body := range []string{"", "matins", "matins appearance:nave", "appearance:nave", " lauds"} {
		if event, ok := ParseEvent(body); ok {
			t.Errorf("ParseEvent(%q) accepted a bad scope: %+v", body, event)
		}
	}

	s, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, eastern)
	// One reader praying two hours in the dark on a phone counts once in each
	// dimension, exactly as they count once in the daily total.
	for _, hour := range []string{"lauds", "vespers"} {
		if err := s.Record(ctx, now, "browser-a", hour, "appearance:apse", "screen:mobile"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Record(ctx, now, "browser-b", "lauds", "appearance:nave", "screen:desktop"); err != nil {
		t.Fatal(err)
	}
	// A browser that has not picked up the new app.js still counts overall.
	if err := s.Record(ctx, now, "browser-c", "lauds"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"narthex:vault", "apse"} {
		if err := s.Record(ctx, now, "browser-d", "lauds", bad); err == nil {
			t.Fatalf("Record accepted %q", bad)
		}
	}
	rows, err := s.Daily(ctx, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Users != 3 || rows[0].Hours[0] != 3 {
		t.Fatalf("totals: %+v", rows[0])
	}
	if rows[0].Dimensions["appearance:apse"] != 1 || rows[0].Dimensions["screen:mobile"] != 1 ||
		rows[0].Dimensions["appearance:nave"] != 1 || rows[0].Dimensions["screen:desktop"] != 1 {
		t.Fatalf("dimensions: %+v", rows[0].Dimensions)
	}
	// A day nobody reported reads as zero rather than panicking on a nil map.
	if rows[1].Dimensions["appearance:nave"] != 0 {
		t.Fatalf("quiet day: %+v", rows[1])
	}
	// The un-refreshed browser is in the total but in neither appearance, so
	// the family reports less than the day: the report says so rather than
	// silently attributing it.
	if rows[0].Dimensions["appearance:nave"]+rows[0].Dimensions["appearance:apse"] >= rows[0].Users {
		t.Fatalf("silent client counted in a dimension: %+v", rows[0])
	}
	// Switching appearance during the day counts on both sides rather than
	// moving the browser between them.
	if err := s.Record(ctx, now.Add(time.Hour), "browser-a", "compline", "appearance:nave", "screen:mobile"); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Daily(ctx, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Users != 3 || rows[0].Dimensions["appearance:nave"] != 2 ||
		rows[0].Dimensions["appearance:apse"] != 1 || rows[0].Dimensions["screen:mobile"] != 1 {
		t.Fatalf("appearance switch: %+v", rows[0])
	}
}
