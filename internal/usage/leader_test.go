package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPrayerFormDimensionCountsFormsWithoutInflatingVisits(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	for _, body := range []string{"lauds prayer-form:private", "lauds prayer-form:private", "lauds prayer-form:priest", "vespers prayer-form:deacon", "site prayer-form:private", "lauds prayer-form:unknown"} {
		event, ok := ParseEvent(body)
		if !ok {
			t.Fatal(body)
		}
		if err := s.Record(context.Background(), now, "one-browser", event.Scope, event.Dimensions...); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.Daily(context.Background(), now, 1)
	if err != nil {
		t.Fatal(err)
	}
	row := rows[0]
	if row.Users != 1 || row.Hours[0] != 1 || row.Hours[5] != 1 {
		t.Fatalf("inflated visits: %+v", row)
	}
	for _, form := range []string{"private", "deacon", "priest"} {
		if row.Dimensions["prayer-form:"+form] != 1 {
			t.Errorf("wrong %s count: %+v", form, row)
		}
	}
	for _, scope := range []string{"site", "ordo", "reminders"} {
		event, _ := ParseEvent(scope + " prayer-form:priest")
		if len(event.Dimensions) != 0 {
			t.Errorf("form counted without an office: %+v", event)
		}
	}
}
