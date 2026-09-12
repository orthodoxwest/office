package web

import (
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestParseDays(t *testing.T) {
	all, err := parseDays("")
	if err != nil {
		t.Fatal(err)
	}
	for d, on := range all {
		if !on {
			t.Errorf("empty spec: expected %v enabled", time.Weekday(d))
		}
	}

	wk, err := parseDays("mon-fri")
	if err != nil {
		t.Fatal(err)
	}
	if wk[time.Sunday] || wk[time.Saturday] {
		t.Errorf("mon-fri should not include weekend: %v", wk)
	}
	if !wk[time.Monday] || !wk[time.Friday] {
		t.Errorf("mon-fri should include mon and fri: %v", wk)
	}

	wrap, err := parseDays("sat-sun")
	if err != nil {
		t.Fatal(err)
	}
	if !wrap[time.Saturday] || !wrap[time.Sunday] || wrap[time.Monday] {
		t.Errorf("sat-sun should wrap the week end: %v", wrap)
	}

	mixed, err := parseDays("mon-wed,sat")
	if err != nil {
		t.Fatal(err)
	}
	if !mixed[time.Tuesday] || !mixed[time.Saturday] || mixed[time.Friday] {
		t.Errorf("mon-wed,sat parsed wrong: %v", mixed)
	}

	if _, err := parseDays("monday"); err == nil {
		t.Error("expected error for full day name")
	}
}

func TestParseICSConfigValidation(t *testing.T) {
	for _, bad := range []string{
		"",                    // no hours
		"lauds=6am",           // bad time format
		"lauds=06:45&days=xx", // bad day
		"lauds=06:45&alarm=-5",
		"lauds=06:45&alarm=abc",
		"lauds=06:45&tz=Nowhere/Nowhere",
		"lauds=06:45&horizon=0",
		"lauds=06:45&horizon=9999",
	} {
		q, _ := url.ParseQuery(bad)
		if _, err := parseICSConfig(q); err == nil {
			t.Errorf("expected error for query %q", bad)
		}
	}

	q, _ := url.ParseQuery("vespers=18:00&lauds=06:45&alarm=none&tz=America/New_York&days=mon-fri&horizon=30")
	cfg, err := parseICSConfig(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.hours) != 2 || cfg.hours[0].name != "lauds" || cfg.hours[1].name != "vespers" {
		t.Errorf("expected hours in canonical order, got %+v", cfg.hours)
	}
	if cfg.alarm != -1 {
		t.Errorf("expected alarm disabled, got %d", cfg.alarm)
	}
	if cfg.loc.String() != "America/New_York" {
		t.Errorf("expected tz to parse, got %v", cfg.loc)
	}
	if cfg.horizon != 30 {
		t.Errorf("expected horizon 30, got %d", cfg.horizon)
	}
}

func TestBuildICS(t *testing.T) {
	s := &Server{cache: newYearCache("../../data")}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.ParseQuery("lauds=06:45&tz=America/New_York&horizon=7&alarm=15")
	cfg, err := parseICSConfig(q)
	if err != nil {
		t.Fatal(err)
	}

	// Start so that Christmas 2026 (a Friday) falls inside the horizon.
	now := time.Date(2026, 12, 20, 12, 0, 0, 0, loc)
	body, err := s.buildICS(cfg, "https://office.example", now)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(body, "BEGIN:VCALENDAR\r\n") || !strings.HasSuffix(body, "END:VCALENDAR\r\n") {
		t.Error("expected VCALENDAR envelope with CRLF line endings")
	}
	if got := strings.Count(body, "BEGIN:VEVENT"); got != 7 {
		t.Errorf("expected 7 events (one hour x 7 days), got %d", got)
	}
	if !strings.Contains(body, "SUMMARY:Lauds — Nativity of Our Lord Jesus Christ") {
		t.Error("expected Christmas feast in summary")
	}
	// 06:45 EST is 11:45 UTC.
	if !strings.Contains(body, "DTSTART:20261225T114500Z") {
		t.Error("expected EST 06:45 converted to 11:45Z")
	}
	if !strings.Contains(body, "UID:lauds-2026-12-25@awrv-office") {
		t.Error("expected stable per-day UID")
	}
	if !strings.Contains(body, "TRIGGER:-PT15M") {
		t.Error("expected 15-minute alarm trigger")
	}
	if !strings.Contains(body, "URL:https://office.example/lauds/2026-12-25") {
		t.Error("expected event URL back to the hour page")
	}

	for i, line := range strings.Split(body, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line %d exceeds 75 octets (%d): %q", i+1, len(line), line)
		}
	}
}

func TestBuildICSDayFilterAndNoAlarm(t *testing.T) {
	s := &Server{cache: newYearCache("../../data")}

	q, _ := url.ParseQuery("vespers=18:00&days=sun&alarm=none&horizon=14")
	cfg, err := parseICSConfig(q)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC) // a Monday
	body, err := s.buildICS(cfg, "https://office.example", now)
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.Count(body, "BEGIN:VEVENT"); got != 2 {
		t.Errorf("expected 2 Sunday events in 14 days, got %d", got)
	}
	if strings.Contains(body, "BEGIN:VALARM") {
		t.Error("expected no VALARM when alarm=none")
	}
	if !strings.Contains(body, "DTSTART:20260614T180000Z") {
		t.Error("expected first Sunday vespers at 18:00 UTC")
	}
}

func TestBuildICSSpansYearBoundary(t *testing.T) {
	s := &Server{cache: newYearCache("../../data")}

	q, _ := url.ParseQuery("compline=21:00&horizon=10")
	cfg, err := parseICSConfig(q)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 12, 28, 12, 0, 0, 0, time.UTC)
	body, err := s.buildICS(cfg, "https://office.example", now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "UID:compline-2027-01-03@awrv-office") {
		t.Error("expected events to continue into 2027")
	}
	if got := strings.Count(body, "BEGIN:VEVENT"); got != 10 {
		t.Errorf("expected 10 events across the year boundary, got %d", got)
	}
}

func TestEscapeICS(t *testing.T) {
	got := escapeICS("a;b,c\\d\ne")
	if got != `a\;b\,c\\d\ne` {
		t.Errorf("unexpected escaping: %q", got)
	}
}

func TestFoldLine(t *testing.T) {
	for _, line := range []string{
		"",
		strings.Repeat("a", 75),
		strings.Repeat("a", 150),
		strings.Repeat("a", 225),
		strings.Repeat("a", 74) + strings.Repeat("é", 100),
		strings.Repeat("🕯", 80),
	} {
		var folded strings.Builder
		foldLine(&folded, line)
		for _, physical := range strings.Split(folded.String(), "\r\n") {
			if len(physical) > 75 {
				t.Errorf("physical line has %d bytes, want at most 75", len(physical))
			}
			if !utf8.ValidString(physical) {
				t.Errorf("fold split a UTF-8 character: %q", physical)
			}
		}
		unfolded := strings.ReplaceAll(strings.TrimSuffix(folded.String(), "\r\n"), "\r\n ", "")
		if unfolded != line {
			t.Errorf("unfolding changed the original line")
		}
	}
}

func TestBuildICSDatesAcrossMidnightDST(t *testing.T) {
	s := &Server{cache: newYearCache("../../data")}
	q, _ := url.ParseQuery("lauds=06:45&tz=America/Santiago&horizon=3")
	cfg, err := parseICSConfig(q)
	if err != nil {
		t.Fatal(err)
	}
	// September 6 begins at 01:00 locally. Carrying the request's 00:30
	// clock into AddDate normalizes it back to September 5, duplicating
	// that day's UID and omitting September 6 altogether.
	now := time.Date(2026, 9, 5, 0, 30, 0, 0, cfg.loc)
	body, err := s.buildICS(cfg, "https://office.example", now)
	if err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-09-05", "2026-09-06", "2026-09-07"} {
		uid := "UID:lauds-" + date + "@awrv-office\r\n"
		if got := strings.Count(body, uid); got != 1 {
			t.Errorf("%s occurs %d times, want once", strings.TrimSpace(uid), got)
		}
	}
	for _, start := range []string{"20260905T104500Z", "20260906T094500Z", "20260907T094500Z"} {
		if !strings.Contains(body, "DTSTART:"+start+"\r\n") {
			t.Errorf("missing local 06:45 start %s", start)
		}
	}
}
