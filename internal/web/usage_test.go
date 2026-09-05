package web

import (
	"context"
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
	send := func(method, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://office.test/api/usage", strings.NewReader(body))
		r.Header.Set("X-Office-Usage", "1")
		r.Header.Set("Origin", origin)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		s.handleUsageEvent(w, r)
		return w
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
	rows, err := store.Daily(context.Background(), time.Now(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Users != 1 || rows[0].Hours[0] != 1 {
		t.Fatalf("rejected events affected counts: %+v", rows[0])
	}
	w := httptest.NewRecorder()
	s.handleUsageDashboard(w, httptest.NewRequest("GET", "/admin/usage?days=7", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Daily usage") || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Robots-Tag") == "" {
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
