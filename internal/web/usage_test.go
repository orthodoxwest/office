package web

import (
	"context"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/render"
	"github.com/orthodoxwest/office/internal/usage"
)

func TestUsageEndpointAndDashboard(t *testing.T) {
	store, err := usage.Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	pages, err := render.New("test")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{usage: store, pages: pages}
	const humanUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
	sendAs := func(agent, method, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://office.test/api/usage", strings.NewReader(body))
		r.Header.Set("X-Office-Usage", "1")
		r.Header.Set("Origin", origin)
		r.Header.Set("User-Agent", agent)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		s.handleUsageEvent(w, r)
		return w
	}
	send := func(method, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		return sendAs(humanUA, method, body, origin, cookie)
	}
	first := send("POST", "lauds", "https://office.test", nil)
	if first.Code != 204 {
		t.Fatalf("event: %d %s", first.Code, first.Body)
	}
	cookies := first.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].Path != "/api/usage" {
		t.Fatalf("cookie: %+v", cookies)
	}
	if w := send("POST", "lauds", "https://office.test", cookies[0]); w.Code != 204 {
		t.Fatal(w.Code)
	}
	for _, tc := range []struct {
		method, body, origin string
		status               int
	}{
		{"GET", "lauds", "https://office.test", 405},
		{"POST", "matins", "https://office.test", 400},
		{"POST", strings.Repeat("a", 100), "https://office.test", 400},
		{"POST", "lauds", "https://evil.test", 403},
	} {
		if w := send(tc.method, tc.body, tc.origin, cookies[0]); w.Code != tc.status {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
	}
	// Crawlers are answered politely and dropped: no cookie, no count. Scraping
	// stays welcome; it just must not mint a "unique browser" per scraped URL.
	for _, agent := range []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36",
		"GPTBot/1.1", "ClaudeBot/1.0", "python-requests/2.32.3", "curl/8.7.1", "",
	} {
		w := sendAs(agent, "POST", "lauds", "https://office.test", nil)
		if w.Code != 204 || len(w.Result().Cookies()) != 0 {
			t.Fatalf("bot %q: %d cookies=%d", agent, w.Code, len(w.Result().Cookies()))
		}
	}
	// A phone whose model name merely embeds "bot" is a person, not a crawler.
	if w := sendAs("Mozilla/5.0 (Linux; Android 13; Cubot Note 20) AppleWebKit/537.36 Chrome/141.0.0.0 Mobile",
		"POST", "prime", "https://office.test", nil); w.Code != 204 || len(w.Result().Cookies()) != 1 {
		t.Fatalf("Cubot phone treated as bot: %d cookies=%d", w.Code, len(w.Result().Cookies()))
	}
	// A current client reports how it rendered the page alongside the scope;
	// a browser still serving app.js from the service-worker cache sends the
	// bare scope and must keep counting exactly as before.
	modern := send("POST", "terce appearance:apse screen:mobile", "https://office.test", nil)
	if modern.Code != 204 {
		t.Fatalf("dimensioned event: %d %s", modern.Code, modern.Body)
	}
	stale := send("POST", "sext", "https://office.test", nil)
	if stale.Code != 204 {
		t.Fatalf("stale client event: %d %s", stale.Code, stale.Body)
	}
	// A token from a newer build than this one loses the dimension, not the hour.
	if w := send("POST", "none appearance:nave transept:north", "https://office.test", nil); w.Code != 204 {
		t.Fatalf("unknown dimension: %d %s", w.Code, w.Body)
	}
	// The ordo page view and a generated reminder feed link are tracked as
	// their own scopes, distinct from the hours.
	if w := send("POST", "ordo", "https://office.test", nil); w.Code != 204 {
		t.Fatalf("ordo event: %d %s", w.Code, w.Body)
	}
	if w := send("POST", "reminders", "https://office.test", nil); w.Code != 204 {
		t.Fatalf("reminders event: %d %s", w.Code, w.Body)
	}
	rows, err := store.Daily(context.Background(), time.Now(), 7)
	if err != nil {
		t.Fatal(err)
	}
	// One human browser for lauds, one for prime; every bot agent above dropped.
	if rows[0].Users != 7 || rows[0].Hours[0] != 1 || rows[0].Hours[1] != 1 || rows[0].Ordo != 1 || rows[0].Reminders != 1 {
		t.Fatalf("rejected events affected counts: %+v", rows[0])
	}
	// Three cookieless browsers reached the hours above; only the two that
	// described themselves land in a dimension.
	if rows[0].Dimensions["appearance:apse"] != 1 || rows[0].Dimensions["screen:mobile"] != 1 ||
		rows[0].Dimensions["appearance:nave"] != 1 || rows[0].Dimensions["screen:desktop"] != 0 {
		t.Fatalf("dimension counts: %+v", rows[0].Dimensions)
	}
	w := httptest.NewRecorder()
	s.handleUsageDashboard(w, httptest.NewRequest("GET", "/admin/usage?days=7", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Daily usage") || !strings.Contains(w.Body.String(), "Ordo") || !strings.Contains(w.Body.String(), "Reminders") || !strings.Contains(w.Body.String(), "Nave vs Apse") || !strings.Contains(w.Body.String(), "Desktop vs Mobile") || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Robots-Tag") == "" {
		t.Fatalf("dashboard: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), cookies[0].Value) {
		t.Fatal("dashboard exposes identifier")
	}
	w = httptest.NewRecorder()
	s.handleUsageDashboard(w, httptest.NewRequest("GET", "/admin/usage?days=999999", nil))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	for _, days := range []string{"7", "30", "90", "366"} {
		w = httptest.NewRecorder()
		s.handleUsageDashboard(w, httptest.NewRequest("GET", "/admin/usage?days="+days+"&format=csv", nil))
		if w.Code != 200 || w.Header().Get("Content-Type") != "text/csv; charset=utf-8" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Robots-Tag") == "" || !strings.Contains(w.Header().Get("Content-Disposition"), "office-usage-"+days+"-days.csv") {
			t.Fatalf("CSV response: %d %+v", w.Code, w.Header())
		}
		records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
		wantRows := map[string]int{"7": 8, "30": 31, "90": 91, "366": 367}[days]
		if err != nil || len(records) != wantRows {
			t.Fatalf("CSV rows: %d, %v", len(records), err)
		}
		if records[0][0] != "date_america_new_york" || records[1][0] != rows[0].Day || records[1][1] != "7" || records[1][2] != "1" || records[1][9] != "1" || records[1][10] != "1" {
			t.Fatalf("CSV does not match dashboard counts: %v", records[:2])
		}
		for i, column := range records[0] {
			if column == "appearance:apse" && records[1][i] != "1" {
				t.Fatalf("CSV dimension: %v", records[1])
			}
		}
		if strings.Contains(w.Body.String(), cookies[0].Value) {
			t.Fatal("CSV exposes browser identifier")
		}
	}
	w = httptest.NewRecorder()
	s.handleUsageDashboard(w, httptest.NewRequest("HEAD", "/admin/usage?days=7&format=csv", nil))
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Disposition") == "" {
		t.Fatalf("CSV HEAD: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	s.handleUsageDashboard(w, httptest.NewRequest("GET", "/admin/usage?days=0&format=csv", nil))
	if w.Code != 400 {
		t.Fatalf("CSV accepts invalid period: %d", w.Code)
	}

	s.usage = nil
	if w = send("POST", "site", "https://office.test", nil); w.Code != 204 || len(w.Result().Cookies()) != 0 {
		t.Fatal("disabled tracking sets cookie")
	}
	w = httptest.NewRecorder()
	s.handleUsageDashboard(w, httptest.NewRequest("GET", "/admin/usage", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
