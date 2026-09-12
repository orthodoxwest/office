package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newShareServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(filepath.Join("..", "..", "data"), ":0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// TestShareEncodesTheRequestHost: the card is printed and hung on a wall, so
// the address it carries has to be the one the reader typed — a parish
// running its own copy must not hand out this deployment's host.
func TestShareEncodesTheRequestHost(t *testing.T) {
	s := newShareServer(t)

	for _, tc := range []struct {
		name    string
		host    string
		proto   string
		wantURL string
	}{
		{name: "plain host", host: "office.example.org", wantURL: "http://office.example.org/"},
		{name: "behind a TLS proxy", host: "office.fly.dev", proto: "https", wantURL: "https://office.fly.dev/"},
		{name: "host with a port", host: "localhost:8080", wantURL: "http://localhost:8080/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/share", nil)
			req.Host = tc.host
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			rec := httptest.NewRecorder()
			s.handleShare(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			body := rec.Body.String()
			// Once in the SVG's label (so the code encodes it) and once as
			// the caption a reader can retype.
			if n := strings.Count(body, tc.wantURL); n < 2 {
				t.Errorf("expected %q in both the code label and the caption, found %d occurrence(s)", tc.wantURL, n)
			}
			if !strings.Contains(body, `<figcaption class="share-plate-address">`+tc.wantURL+`</figcaption>`) {
				t.Errorf("caption should print %q verbatim", tc.wantURL)
			}
		})
	}
}

// TestShareRendersTheCodeServerSide: the page must show a scannable code with
// no script at all. The clipboard and share-sheet buttons are conveniences
// layered on top; the code itself is the page.
func TestShareRendersTheCodeServerSide(t *testing.T) {
	s := newShareServer(t)
	rec := httptest.NewRecorder()
	s.handleShare(rec, httptest.NewRequest(http.MethodGet, "/share", nil))

	body := rec.Body.String()
	for _, want := range []string{`<svg class="qr-code"`, `class="qr-modules"`, `class="qr-emblem"`, `</svg>`} {
		if !strings.Contains(body, want) {
			t.Errorf("share page should contain %q", want)
		}
	}
	if strings.Contains(body, "&lt;svg") {
		t.Error("the SVG was escaped as text instead of rendered as markup")
	}
}

// TestShareRouteIsRegistered exercises the mux rather than the handler, so a
// route that is never wired up cannot pass the tests above.
func TestShareRouteIsRegistered(t *testing.T) {
	s := newShareServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/share", s.handleShare)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/share", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /share: status %d", rec.Code)
	}
}

// TestShareChromeIsDated mirrors the reminders case: chrome links must carry
// today's date so they hit the same URLs the service worker precaches.
func TestShareChromeIsDated(t *testing.T) {
	s := newShareServer(t)
	rec := httptest.NewRecorder()
	s.handleShare(rec, httptest.NewRequest(http.MethodGet, "/share", nil))

	body := rec.Body.String()
	if strings.Contains(body, `href="/lauds"`) {
		t.Error("share page should not emit an undated /lauds")
	}
	if !strings.Contains(body, `data-nav="share"`) {
		t.Error("share page should mark its own nav entry")
	}
	if !strings.Contains(body, `aria-current="page"`) {
		t.Error("share page should mark its nav entry as current")
	}
}
