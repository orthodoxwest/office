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
	// Injected version must also stamp static asset URLs so SW and HTML share keys.
	if !strings.Contains(body, `"?v=" + VERSION`) && !strings.Contains(body, `var ASSET_Q = "?v=" + VERSION`) {
		t.Errorf("expected asset version query construction in service worker")
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
		// Versioned static URLs + cache-first keep HTML/CSS matched across deploys.
		`cacheFirst`,
		`ASSET_Q`,
		`assetURL`,
		`cache: "reload"`,
		`eb-garamond-regular.woff2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("service worker is missing offline navigation support %q", want)
		}
	}

	// Static /static/ requests must go through cache-first, not the page network-first path.
	if !strings.Contains(body, `url.pathname.indexOf("/static/") === 0`) ||
		!strings.Contains(body, `event.respondWith(cacheFirst(req))`) {
		t.Errorf("service worker should route /static/ through cacheFirst")
	}

	// SWR on unversioned URLs was the HTML/CSS desync path during deploys.
	if strings.Contains(body, `staleWhileRevalidate`) {
		t.Errorf("service worker should not use staleWhileRevalidate for static assets")
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
		// Build-stamped static URLs.
		`{{static "style.css"}}`,
		`{{static "app.js"}}`,
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

	// Construction banner must stay unpersisted (feedback window). Appearance
	// may use localStorage (office-theme); only ban banner-specific keys here.
	for _, unwanted := range []string{
		`siteBannerDismissed`,
		`banner-dismiss`,
		`bannerDismissed`,
	} {
		if strings.Contains(body, unwanted) {
			t.Errorf("construction banner dismissal should not persist via %q", unwanted)
		}
	}
	// Theme control is client-side only (keeps SW cache keys theme-free).
	for _, want := range []string{
		`office-theme`,
		`data-theme-choice`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("app script is missing theme control support %q", want)
		}
	}
}

func TestStaticAssetsCacheHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/static/", staticFileServer(http.FS(files)))

	// Versioned requests: long-lived immutable (HTML always stamps ?v=).
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/static/style.css?v=testhash12", nil))
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("expected immutable cache on versioned static asset, got %q", cc)
	}

	// Unversioned: revalidate (dev / ad-hoc fetches).
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest("GET", "/static/style.css", nil))
	if rec2.Code != 200 {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	if cc := rec2.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control: no-cache on unversioned static assets, got %q", cc)
	}
}

func TestStaticURL(t *testing.T) {
	if got := staticURL("style.css", "abc"); got != "/static/style.css?v=abc" {
		t.Errorf("staticURL = %q", got)
	}
	if got := staticURL("/fonts/x.woff2", "v1"); got != "/static/fonts/x.woff2?v=v1" {
		t.Errorf("staticURL = %q", got)
	}
	if got := staticURL("app.js", ""); got != "/static/app.js" {
		t.Errorf("empty version should omit query, got %q", got)
	}
}

func TestSelfHostedFontsPresent(t *testing.T) {
	for _, name := range []string{
		"static/fonts/eb-garamond-regular.woff2",
		"static/fonts/eb-garamond-italic.woff2",
		"static/fonts/eb-garamond-bold.woff2",
		"static/fonts/OFL-1.1.txt",
	} {
		if _, err := files.ReadFile(name); err != nil {
			t.Errorf("missing embedded font asset %s: %v", name, err)
		}
	}
	css, err := files.ReadFile("static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	body := string(css)
	if strings.Contains(body, "fonts.gstatic.com") {
		t.Errorf("style.css should not reference fonts.gstatic.com")
	}
	if !strings.Contains(body, `url("fonts/eb-garamond-regular.woff2")`) {
		t.Errorf("style.css should self-host EB Garamond")
	}
}

func TestCalendarFishUsesSprite(t *testing.T) {
	src, err := files.ReadFile("templates/calendar.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if !strings.Contains(body, `id="icon-fish"`) {
		t.Errorf("calendar should define a single fish symbol")
	}
	if !strings.Contains(body, `href="#icon-fish"`) {
		t.Errorf("calendar fish instances should use <use href=\"#icon-fish\">")
	}
	// Path data should appear once (in the symbol), not in the fish template body.
	if strings.Count(body, `M1 6 C5 1.2`) != 1 {
		t.Errorf("fish path data should appear once in the sprite, not per instance")
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
