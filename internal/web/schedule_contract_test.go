package web

import (
	"regexp"
	"strconv"
	"testing"
	"time"
)

func TestClientOfficeScheduleMatchesServer(t *testing.T) {
	src, err := files.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`\{ start: (\d+), slug: "([^"]+)", label: "([^"]+)", offset: (-?\d+) \}`)
	matches := pattern.FindAllSubmatch(src, -1)
	if len(matches) != len(currentHourSchedule) {
		t.Fatalf("client has %d office boundaries, server has %d", len(matches), len(currentHourSchedule))
	}

	for i, match := range matches {
		start, err := strconv.Atoi(string(match[1]))
		if err != nil {
			t.Fatalf("parsing client start %q: %v", match[1], err)
		}
		offset, err := strconv.Atoi(string(match[4]))
		if err != nil {
			t.Fatalf("parsing client offset %q: %v", match[4], err)
		}
		got := currentHourBoundary{
			Start:  start,
			Slug:   string(match[2]),
			Label:  string(match[3]),
			Offset: offset,
		}
		if got != currentHourSchedule[i] {
			t.Errorf("boundary %d: client %+v, server %+v", i, got, currentHourSchedule[i])
		}
	}
}

func TestCurrentHourEntryAtEveryBoundary(t *testing.T) {
	for _, boundary := range currentHourSchedule {
		now := time.Date(2026, time.March, 15, boundary.Start, 0, 0, 0, time.Local)
		slug, label, offset := currentHourEntry(now)
		if slug != boundary.Slug || label != boundary.Label || offset != boundary.Offset {
			t.Errorf("%02d:00: got (%q, %q, %d), want (%q, %q, %d)",
				boundary.Start, slug, label, offset, boundary.Slug, boundary.Label, boundary.Offset)
		}
	}
}
