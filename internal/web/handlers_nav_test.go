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
