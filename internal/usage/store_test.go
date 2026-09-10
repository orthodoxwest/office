package usage

import (
	"context"
	"path/filepath"
	"slices"
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

func TestBeaconDimensionsParseAndCount(t *testing.T) {
	for _, tc := range []struct {
		body  string
		scope string
		dims  []string
	}{
		// What the current app.js sends.
		{"lauds apse mobile", "lauds", []string{"apse", "mobile"}},
		{"site nave desktop", "site", []string{"nave", "desktop"}},
		// A browser still running app.js from before dimensions existed: the
		// page counts exactly as it always did.
		{"vespers", "vespers", nil},
		// Tokens this build cannot read are dropped, not fatal — a client
		// cached from a newer build must not lose its page count.
		{"ordo narthex apse", "ordo", []string{"apse"}},
		// One token per family wins; the rest are noise.
		{"prime nave apse mobile mobile", "prime", []string{"nave", "mobile"}},
	} {
		event, ok := ParseEvent(tc.body)
		if !ok || event.Scope != tc.scope || !slices.Equal(event.Dimensions, tc.dims) {
			t.Errorf("ParseEvent(%q) = %+v %v", tc.body, event, ok)
		}
	}
	for _, body := range []string{"", "matins", "matins nave", "nave", " lauds"} {
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
		if err := s.Record(ctx, now, "browser-a", hour, "apse", "mobile"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Record(ctx, now, "browser-b", "lauds", "nave", "desktop"); err != nil {
		t.Fatal(err)
	}
	// A browser that has not picked up the new app.js still counts overall.
	if err := s.Record(ctx, now, "browser-c", "lauds"); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, now, "browser-d", "lauds", "narthex"); err == nil {
		t.Fatal("Record accepted an unknown dimension")
	}
	rows, err := s.Daily(ctx, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Users != 3 || rows[0].Hours[0] != 3 {
		t.Fatalf("totals: %+v", rows[0])
	}
	if rows[0].Apse != 1 || rows[0].Mobile != 1 || rows[0].Nave != 1 || rows[0].Desktop != 1 {
		t.Fatalf("dimensions: %+v", rows[0])
	}
	// The un-refreshed browser is in the total but in neither appearance, so
	// the pair reports less than the day: the report says so rather than
	// silently attributing it.
	if rows[0].Nave+rows[0].Apse >= rows[0].Users {
		t.Fatalf("silent client counted in a dimension: %+v", rows[0])
	}
	// Switching appearance during the day counts on both sides rather than
	// moving the browser between them.
	if err := s.Record(ctx, now.Add(time.Hour), "browser-a", "compline", "nave", "mobile"); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Daily(ctx, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Users != 3 || rows[0].Nave != 2 || rows[0].Apse != 1 || rows[0].Mobile != 1 {
		t.Fatalf("appearance switch: %+v", rows[0])
	}
}
