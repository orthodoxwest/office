package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// brandTag returns the opening <a ...> for the site brand, or "".
func brandTag(body string) string {
	brandAt := strings.Index(body, `data-nav="home"`)
	if brandAt < 0 {
		return ""
	}
	tagStart := strings.LastIndex(body[:brandAt], "<a ")
	tagEnd := strings.Index(body[brandAt:], ">")
	if tagStart < 0 || tagEnd < 0 {
		return ""
	}
	return body[tagStart : brandAt+tagEnd]
}

func TestHomeAndHourEmitDatedNavLinks(t *testing.T) {
	dataDir := filepath.Join("..", "..", "data")
	s, err := New(dataDir, ":0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	today := time.Now().In(time.Local).Format("2006-01-02")
	laudsToday := regexp.MustCompile(`href="/lauds/` + regexp.QuoteMeta(today) + `"`)

	t.Run("home undated request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		s.handleHome(rec, req)
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, `href="/lauds"`) {
			t.Errorf("home should not emit undated href=/lauds")
		}
		if !strings.Contains(body, `data-nav="hour"`) {
			t.Errorf("home missing data-nav markers for client date stamping")
		}
		if !laudsToday.MatchString(body) {
			t.Errorf("home should emit href=/lauds/%s in chrome or prayer card", today)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("HTML Cache-Control: want no-cache, got %q", cc)
		}
	})

	t.Run("hour undated request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handleHour(rec, httptest.NewRequest(http.MethodGet, "/lauds", nil), "lauds", "")
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, `href="/vespers"`) {
			t.Errorf("hour page should not emit undated href=/vespers")
		}
		if !strings.Contains(body, `data-nav="hour"`) {
			t.Errorf("hour page missing data-nav markers")
		}
		if !strings.Contains(body, `href="/vespers/`+today+`"`) && !strings.Contains(body, `/vespers/`+today) {
			t.Errorf("hour chrome should link vespers for today %s", today)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("HTML Cache-Control: want no-cache, got %q", cc)
		}
	})

	t.Run("home explicit other day", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handleHome(rec, httptest.NewRequest(http.MethodGet, "/?date=2026-01-15", nil))
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `href="/lauds/2026-01-15"`) {
			t.Errorf("expected lauds link for selected day 2026-01-15")
		}
		if strings.Contains(body, `class="today-link" href="/"`) {
			t.Errorf("today link should be dated, not bare /")
		}
		wantToday := `class="today-link" href="/?date=` + today + `"`
		if !strings.Contains(body, wantToday) {
			t.Errorf("today link should be %s", wantToday)
		}
		// Brand escapes to undated home (→ today), not the selected historical day.
		if strings.Contains(body, `data-nav="home" href="/?date=2026-01-15"`) ||
			strings.Contains(body, `href="/?date=2026-01-15" data-nav="home"`) {
			t.Errorf("brand must not pin home to the selected historical day")
		}
		if !strings.Contains(body, `data-nav-home="today"`) {
			t.Errorf(`brand should mark data-nav-home="today"`)
		}
		if tag := brandTag(body); tag == "" {
			t.Errorf("brand data-nav=home missing")
		} else {
			if !strings.Contains(tag, `href="/"`) {
				t.Errorf("brand href should be undated /, got tag %q", tag)
			}
			if strings.Contains(tag, `aria-current`) {
				t.Errorf("historical home brand must not claim aria-current, tag %q", tag)
			}
			if strings.Contains(tag, "active") {
				t.Errorf("historical home brand must not be active, tag %q", tag)
			}
		}
		// Prominent recovery (not only inside collapsed Change date).
		if !strings.Contains(body, `class="not-today-notice"`) {
			t.Errorf("historical home should emit a not-today notice")
		}
		if !strings.Contains(body, `Go to today`) {
			t.Errorf("historical home should offer Go to today")
		}
		// Server historical notice is quiet (no live region).
		if strings.Contains(body, `not-today-notice" role="status"`) ||
			strings.Contains(body, `class="not-today-notice" role="status"`) {
			t.Errorf("server-rendered not-today notice should not use role=status")
		}
		// Hour chrome still keeps the selected day for intentional hopping.
		if !strings.Contains(body, `data-nav="hour" data-hour="lauds"`) {
			t.Errorf("hour nav markers missing")
		}
		if !strings.Contains(body, `href="/lauds/2026-01-15"`) {
			t.Errorf("hour nav should still target the selected day")
		}
		for _, slug := range []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"} {
			hourLink := `href="/` + slug + `/2026-01-15" data-hour="` + slug + `"`
			if got := strings.Count(body, hourLink); got != 1 {
				t.Errorf("home prayer directory has %d %s links, want exactly 1", got, slug)
			}
		}
		for _, daypart := range []string{"Morning", "Day", "Evening"} {
			if !strings.Contains(body, `>`+daypart+`</h3>`) {
				t.Errorf("home prayer directory is missing %s heading", daypart)
			}
		}
	})

	t.Run("hour explicit other day", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handleHour(rec, httptest.NewRequest(http.MethodGet, "/lauds/2026-01-15", nil), "lauds", "2026-01-15")
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `class="not-today-notice"`) {
			t.Errorf("historical hour should emit a not-today notice")
		}
		wantToday := `class="today-link not-today-link" href="/lauds/` + today + `"`
		if !strings.Contains(body, wantToday) {
			// Attribute order may vary; accept either class order.
			if !strings.Contains(body, `href="/lauds/`+today+`"`) || !strings.Contains(body, `Go to today`) {
				t.Errorf("historical hour Go to today should target /lauds/%s", today)
			}
		}
		if strings.Contains(body, `class="not-today-notice" role="status"`) {
			t.Errorf("server-rendered hour notice should not use role=status")
		}
		// Brand undated; hour nav stays on selected day.
		if tag := brandTag(body); tag == "" {
			t.Fatalf("brand missing")
		} else if !strings.Contains(tag, `href="/"`) {
			t.Errorf("brand href should be undated /, got %q", tag)
		}
		if !strings.Contains(body, `href="/vespers/2026-01-15"`) && !strings.Contains(body, `/vespers/2026-01-15`) {
			t.Errorf("hour chrome should keep vespers on the selected day")
		}
	})

	t.Run("today home brand is current", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handleHome(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, `class="not-today-notice"`) {
			t.Errorf("today home should not emit not-today notice")
		}
		tag := brandTag(body)
		if tag == "" {
			t.Fatalf("brand missing")
		}
		if !strings.Contains(tag, `aria-current="page"`) {
			t.Errorf("today home brand should have aria-current, tag %q", tag)
		}
		if !strings.Contains(tag, `active`) {
			t.Errorf("today home brand should be active, tag %q", tag)
		}
	})

	t.Run("calendar chrome is dated", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handleCalendar(rec, httptest.NewRequest(http.MethodGet, "/calendar/2026", nil))
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, `href="/lauds"`) {
			t.Errorf("calendar should not emit undated /lauds")
		}
		if !laudsToday.MatchString(body) {
			t.Errorf("calendar nav should include /lauds/%s", today)
		}
	})

	t.Run("reminders chrome is dated", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handleReminders(rec, httptest.NewRequest(http.MethodGet, "/reminders", nil))
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if !laudsToday.MatchString(body) {
			t.Errorf("reminders nav should include /lauds/%s", today)
		}
	})

	t.Run("error page chrome is dated", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handleError(rec, httptest.NewRequest(http.MethodGet, "/x", nil), http.StatusBadRequest, "bad")
		body := rec.Body.String()
		if !laudsToday.MatchString(body) {
			t.Errorf("error page nav should include /lauds/%s", today)
		}
	})
}
