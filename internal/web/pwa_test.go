package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleServiceWorkerInjectsVersion(t *testing.T) {
	s := &Server{version: "abc123def456"}
	rec := httptest.NewRecorder()
	s.handleServiceWorker(rec, httptest.NewRequest("GET", "/sw.js", nil))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("expected text/javascript content type, got %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected no-cache, got %q", cc)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"office-" + VERSION`) {
		t.Errorf("expected service worker source in body")
	}
	if !strings.Contains(body, `var VERSION = "abc123def456"`) {
		t.Errorf("expected version to be injected, body still has: %s", body[:80])
	}
	if strings.Contains(body, "__VERSION__") {
		t.Errorf("expected __VERSION__ placeholder to be replaced")
	}
}

func TestServiceWorkerCachesOfflineNavigationTargets(t *testing.T) {
	src, err := files.ReadFile("static/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
		`"/"`,
		`"/reminders"`,
		`"/calendar/" + years[y]`,
		`"/?date=" + slug`,
		`"/" + HOURS[h] + "/" + slug`,
		`return precacheUpcoming();`,
		`plain.searchParams.delete("theme")`,
		`PAGE_NETWORK_TIMEOUT_MS = 2500`,
		`fetchWithTimeout(req, PAGE_NETWORK_TIMEOUT_MS)`,
		`class=\"offline-page\"`,
		`/static/style.css`,
		`Open saved home page`,
		// Static assets must revalidate, not pure cache-first, so deploys
		// land without waiting for a full worker reinstall.
		`staleWhileRevalidate`,
		`cache: "reload"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("service worker is missing offline navigation support %q", want)
		}
	}

	// Static /static/ requests must go through SWR, not the page network-first path.
	if !strings.Contains(body, `url.pathname.indexOf("/static/") === 0`) ||
		!strings.Contains(body, `event.respondWith(staleWhileRevalidate(req))`) {
		t.Errorf("service worker should route /static/ through staleWhileRevalidate")
	}
}

func TestOfflineFallbackDoesNotIncludeConstructionBanner(t *testing.T) {
	src, err := files.ReadFile("static/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, unwanted := range []string{
		`class=\"site-banner\"`,
		`data-dismiss-banner`,
		`under active development`,
		// Pages must be network-first whenever reachable; trusting
		// navigator.onLine here served stale pages while online.
		`isKnownOffline`,
		`navigator.onLine === false`,
	} {
		if strings.Contains(body, unwanted) {
			t.Errorf("offline fallback should not include construction banner markup %q", unwanted)
		}
	}
}

func TestLayoutIncludesConstructionBanner(t *testing.T) {
	src, err := files.ReadFile("templates/layout.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
		`class="site-banner"`,
		`id="site-banner"`,
		`data-dismiss-banner`,
		`under active development`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout is missing construction banner markup %q", want)
		}
	}
}

func TestAppScriptShowsOfflineIndicator(t *testing.T) {
	src, err := files.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, want := range []string{
		`offline-indicator`,
		`/sw.js?online-check=`,
		`cache: "no-store"`,
		`window.addEventListener("online", checkOnline)`,
		`window.addEventListener("offline", function ()`,
		// Recovery polling so a false-negative offline reading self-heals.
		`recoveryTimer`,
		// Require consecutive failed probes so wifi↔cell handoffs do not flash the pill.
		`failStreak`,
		`FAIL_STREAK_TO_SHOW`,
		// Pick up deploys while a long-lived PWA session stays open.
		`reg.update`,
		// "Pray now" is recomputed client-side so a cached home page is not frozen.
		`updatePrayNow`,
		`data-hour`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("app script is missing offline indicator support %q", want)
		}
	}

	// The offline check must never trust navigator.onLine without probing the
	// network — that false-negative read is what showed the banner while online.
	if strings.Contains(body, "navigator.onLine === false") {
		t.Errorf("app script should probe the network rather than trust navigator.onLine === false")
	}
	// Browser "offline" events fire spuriously on mobile; must not show immediately.
	if strings.Contains(body, "setOfflineIndicator(true);\n  });") &&
		strings.Contains(body, `window.addEventListener("offline"`) {
		// Only fail if offline handler still sets the pill directly without probing.
		offlineIdx := strings.Index(body, `window.addEventListener("offline"`)
		probeIdx := strings.Index(body[offlineIdx:], "checkOnline()")
		directShow := strings.Index(body[offlineIdx:offlineIdx+200], "setOfflineIndicator(true)")
		if directShow >= 0 && (probeIdx < 0 || directShow < probeIdx) {
			t.Errorf("offline event handler should probe via checkOnline rather than show the pill immediately")
		}
	}

	for _, unwanted := range []string{
		`siteBannerDismissed`,
		`localStorage.getItem`,
		`localStorage.setItem`,
	} {
		if strings.Contains(body, unwanted) {
			t.Errorf("construction banner dismissal should not persist via %q", unwanted)
		}
	}
}

func TestStaticAssetsSendNoCache(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/static/", staticFileServer(http.FS(files)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/static/style.css", nil))
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control: no-cache on static assets, got %q", cc)
	}
}

func TestComputeVersionDeterministicAndDataSensitive(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	v1 := computeVersion(dir)
	v2 := computeVersion(dir)
	if v1 != v2 {
		t.Errorf("expected deterministic version, got %q then %q", v1, v2)
	}
	if len(v1) != 12 {
		t.Errorf("expected 12-char version, got %q", v1)
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v3 := computeVersion(dir); v3 == v1 {
		t.Errorf("expected version to change when data changes")
	}
}
