package usage

import (
	"context"
	"path/filepath"
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
	for _, event := range []struct{ id, scope string }{{"browser-a", "vespers"}, {"browser-b", "lauds"}, {"browser-c", "site"}} {
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
	if rows[0].Users != 3 || rows[0].Hours[0] != 2 || rows[0].Hours[5] != 1 || rows[1].Users != 0 {
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
	if rows[0].Day != "2026-09-05" || rows[0].Users != 1 || rows[1].Users != 3 {
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
	if remaining != 0 || rows[4].Users != 3 {
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
