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
		`"/reminders"`,
		`"/calendar/" + years[y]`,
		`"/?date=" + slug`,
		`"/" + HOURS[h] + "/" + slug`,
		`todayShellURLs`,
		`datedEquivalent`,
		`isSWRPage`,
		`staleWhileRevalidate`,
		`redirectToDated`,
		`normalizePathname`,
		`canonicalCacheKey`,
		`PAGE_NETWORK_TIMEOUT_MS = 2500`,
		`fetchWithTimeout`,
		`plain.searchParams.delete("theme")`,
		`class=\"offline-page\"`,
		`/static/style.css`,
		`Open saved home page`,
		// Versioned static URLs + cache-first keep HTML/CSS matched across deploys.
		`cacheFirst`,
		`ASSET_Q`,
		`assetURL`,
		`cache: "reload"`,
		`eb-garamond-regular.woff2`,
		// Install must not block skipWaiting on the full 14-day precache.
		`return self.skipWaiting();`,
		`precacheCore(cache)`,
		// Calendar undated → year + today hash (mirrors server handleCalendar).
		`"#d-" + today`,
		// Bare hour with ?date=D redirects to /hour/D, not always today.
		`return "/" + hour + "/" + qDate`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("service worker is missing offline navigation support %q", want)
		}
	}

	// Static /static/ requests must go through cache-first, not the page SWR path.
	if !strings.Contains(body, `url.pathname.indexOf("/static/") === 0`) ||
		!strings.Contains(body, `event.respondWith(cacheFirst(req))`) {
		t.Errorf("service worker should route /static/ through cacheFirst")
	}

	// Dated liturgical pages use SWR so the 14-day precache serves immediately.
	if !strings.Contains(body, `event.respondWith(staleWhileRevalidate(req, url))`) {
		t.Errorf("service worker should route dated pages through staleWhileRevalidate")
	}
	// Undated today-targets redirect onto dated cache keys.
	if !strings.Contains(body, `event.respondWith(redirectToDated(url, dated))`) {
		t.Errorf("service worker should redirect undated navigations to dated URLs")
	}

	// Install should call skipWaiting after core/today only — not after precacheUpcoming.
	installIdx := strings.Index(body, `self.addEventListener("install"`)
	activateIdx := strings.Index(body, `self.addEventListener("activate"`)
	if installIdx < 0 || activateIdx < 0 || activateIdx < installIdx {
		t.Fatal("expected install and activate listeners in order")
	}
	installBlock := body[installIdx:activateIdx]
	if strings.Contains(installBlock, `precacheUpcoming`) {
		t.Errorf("install must not wait on full precacheUpcoming (blocks skipWaiting)")
	}
	activateBlock := body[activateIdx:]
	// Activate must kick precache without returning it from waitUntil (no long activate).
	if !strings.Contains(activateBlock, `precacheUpcoming()`) {
		t.Errorf("activate should kick full precacheUpcoming in the background")
	}
	if strings.Contains(activateBlock, `return precacheUpcoming()`) {
		t.Errorf("activate must not await precacheUpcoming inside waitUntil")
	}
}

// TestServiceWorkerRoutingContract documents the URL→strategy matrix the SW
// must implement. Pure string contracts (no JS runtime); keep in sync with sw.js.
func TestServiceWorkerRoutingContract(t *testing.T) {
	src, err := files.ReadFile("static/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	// Each case is a required behavior token cluster for a URL class.
	type routeCase struct {
		name string
		want []string
	}
	cases := []routeCase{
		{
			name: "undated home redirects to /?date=today",
			want: []string{
				`if (path === "/")`,
				`return "/?date=" + today`,
			},
		},
		{
			name: "bare hour with ?date= goes to /hour/date",
			want: []string{
				`if (qDate && DATE_RE.test(qDate))`,
				`return "/" + hour + "/" + qDate`,
			},
		},
		{
			name: "bare hour without date goes to today",
			want: []string{
				`return "/" + hour + "/" + today`,
			},
		},
		{
			name: "undated calendar includes today hash",
			want: []string{
				`return "/calendar/" + new Date().getFullYear() + "#d-" + today`,
			},
		},
		{
			name: "SWR revalidate bypasses HTTP cache",
			want: []string{
				`function networkFetch(req)`,
				`cache: "reload"`,
				`var revalidate = networkFetch(req)`,
			},
		},
		{
			name: "SWR cold miss is time-bounded",
			want: []string{
				`fetchWithTimeout(req, PAGE_NETWORK_TIMEOUT_MS)`,
			},
		},
		{
			name: "static stays cacheFirst not SWR",
			want: []string{
				`url.pathname.indexOf("/static/") === 0`,
				`event.respondWith(cacheFirst(req))`,
			},
		},
		{
			name: "ics and sw.js bypass page strategies",
			want: []string{
				`if (url.pathname === "/sw.js")`,
				`if (url.pathname === "/office.ics")`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, w := range tc.want {
				if !strings.Contains(body, w) {
					t.Errorf("missing %q", w)
				}
			}
		})
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
		// data-nav markers for client-side dated href stamping.
		`data-nav="home"`,
		`data-nav="hour"`,
		`data-hour="lauds"`,
		`data-nav="calendar"`,
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
		// Dated chrome links so the typical prayer path hits SW precache keys.
		`syncDatedNavigation`,
		`pageDateSlug`,
		`documentDateSlug`,
		`ensureTodayControl`,
		`data-nav`,
		// Re-stamp after midnight when a long-lived tab returns to the foreground.
		`lastSyncedDay`,
		`visibilitychange`,
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
