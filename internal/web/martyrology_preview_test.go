package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMartyrologyRequiresExplicitRequestAndDoesNotPersist(t *testing.T) {
	s, err := New("../../data", ":0")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		url     string
		preview bool
	}{
		{"/prime/2026-09-07", false},
		{"/prime/2026-09-07?preview=martyrology", true},
		{"/prime/2026-09-07", false},
		{"/prime/2026-09-07?preview=true", false},
		{"/prime?date=2026-09-07&preview=martyrology", true},
		{"/prime/2026-09-09?preview=martyrology", false},
	} {
		rec := httptest.NewRecorder()
		s.handleRoot(rec, httptest.NewRequest(http.MethodGet, tc.url, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", tc.url, rec.Code)
		}
		body := rec.Body.String()
		if got := strings.Contains(body, "Martyrology — September 8"); got != tc.preview {
			t.Errorf("%s: preview = %v, want %v", tc.url, got, tc.preview)
		}
		if got := strings.Contains(body, "this may laudably be done"); got == tc.preview {
			t.Errorf("%s: unexpected rubric presence %v", tc.url, got)
		}
		if strings.Contains(tc.url, "preview=") {
			if !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") || rec.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
				t.Errorf("%s: missing preview cache/index headers: %v", tc.url, rec.Header())
			}
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: preview must not persist in a cookie", tc.url)
		}
	}
}
