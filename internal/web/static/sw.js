/* Service worker for the Divine Office PWA.
 *
 * Served at /sw.js by the Go server, which stamps the version placeholder
 * below with a hash of the binary + data directory. A deploy that changes anything affecting
 * rendered pages therefore changes this file, reinstalls the worker, and
 * starts a fresh cache.
 *
 * Static assets are referenced with ?v=VERSION (same stamp HTML uses). That
 * makes each deploy a new URL for CSS/JS, so cache-first cannot pair a new
 * page with a previous stylesheet — the main risk during active development.
 *
 * Page strategy (per URL class):
 *   - Dated hours, /?date=YYYY-MM-DD, /calendar/YYYY, /reminders →
 *     stale-while-revalidate (serve precache immediately, refresh in background).
 *   - Undated /, /lauds, /calendar → redirect to today's dated equivalent so
 *     navigation shares the same cache keys the precache fills.
 *   - /lauds?date=D → redirect to /lauds/D (canonical dated path).
 *   - /static/* → cache-first on versioned URLs.
 *
 * Install only precaches the shell + today so skipWaiting is not blocked on
 * the full 14-day window; activate claims quickly, then app message + free
 * precacheUpcoming fill the rest.
 */

var VERSION = "__VERSION__";
var CACHE = "office-" + VERSION;
var META_CACHE = "office-meta";
var PRECACHE_DAYS = 14;
// Bounds cold SWR miss only (cache hit revalidates in background unbounded).
var PAGE_NETWORK_TIMEOUT_MS = 2500;
var HOURS = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];
var ASSET_Q = "?v=" + VERSION;
var DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
var HOUR_DATED_RE = /^\/(lauds|prime|terce|sext|none|vespers|compline)\/(\d{4}-\d{2}-\d{2})$/;
var CALENDAR_YEAR_RE = /^\/calendar\/\d{4}$/;

function assetURL(path) {
  return path + ASSET_Q;
}

var CORE_ASSETS = [
  assetURL("/static/style.css"),
  assetURL("/static/app.js"),
  assetURL("/static/favicon.svg"),
  assetURL("/static/manifest.webmanifest"),
  assetURL("/static/icons/icon-192.png"),
  assetURL("/static/icons/icon-512.png"),
  // Fonts are requested by style.css without ?v= (relative @font-face URLs),
  // so precache the bare paths. They still live in the versioned Cache bucket
  // (office-VERSION), which is dropped on activate after a deploy.
  "/static/fonts/eb-garamond-regular.woff2",
  "/static/fonts/eb-garamond-italic.woff2",
  "/static/fonts/eb-garamond-bold.woff2"
];

// networkFetch bypasses the browser HTTP cache so install/precache/SWR always
// stores the bytes the origin serves for this deploy (not a pre-deploy disk
// entry under an unversioned HTML URL).
function networkFetch(req) {
  return fetch(req, { cache: "reload" });
}

function putIfOk(cache, req, resp) {
  if (resp && resp.ok) {
    return cache.put(req, resp.clone()).then(function () {
      return resp;
    });
  }
  return Promise.resolve(resp);
}

function precacheURLs(cache, urls) {
  return Promise.all(urls.map(function (u) {
    return networkFetch(u).then(function (resp) {
      return putIfOk(cache, u, resp);
    }).catch(function () {
      // Offline or transient failure during install: runtime path / next sync fills in.
    });
  }));
}

function todayShellURLs() {
  var today = localDateSlug(new Date());
  var urls = ["/?date=" + today, "/reminders"];
  for (var h = 0; h < HOURS.length; h++) {
    urls.push("/" + HOURS[h] + "/" + today);
  }
  urls.push("/calendar/" + new Date().getFullYear());
  return urls;
}

function precacheCore(cache) {
  return precacheURLs(cache, CORE_ASSETS.concat(todayShellURLs()));
}

self.addEventListener("install", function (event) {
  // Only shell + today: do not block skipWaiting on the full 14-day window.
  event.waitUntil(
    caches.open(CACHE).then(function (cache) {
      return precacheCore(cache);
    }).then(function () {
      return self.skipWaiting();
    })
  );
});

self.addEventListener("activate", function (event) {
  // Claim quickly; do not extend activate with the full 14-day precache (browsers
  // may kill long activate). precacheUpcoming is kicked fire-and-forget; the
  // page also posts {type:"precache"} after registration.
  event.waitUntil(
    caches.keys().then(function (names) {
      return Promise.all(names.map(function (name) {
        if (name !== CACHE && name !== META_CACHE) {
          return caches.delete(name);
        }
        return Promise.resolve(false);
      }));
    }).then(function () {
      return self.clients.claim();
    }).then(function () {
      precacheUpcoming();
    })
  );
});

// localDateSlug formats a date as YYYY-MM-DD in the device's local timezone
// (toISOString would shift the day near midnight).
function localDateSlug(d) {
  var m = String(d.getMonth() + 1);
  var day = String(d.getDate());
  return d.getFullYear() + "-" + (m.length < 2 ? "0" + m : m) + "-" + (day.length < 2 ? "0" + day : day);
}

function addUniqueURL(urls, candidate) {
  if (candidate && urls.indexOf(candidate) < 0) {
    urls.push(candidate);
  }
}

// normalizePathname strips a trailing slash (except root) so classification
// and cache keys match precache entries.
function normalizePathname(pathname) {
  if (pathname.length > 1 && pathname.charAt(pathname.length - 1) === "/") {
    return pathname.slice(0, -1);
  }
  return pathname;
}

// datedEquivalent returns the stable dated cache key for undated or alternate
// forms, or null when the URL is already canonical / not mappable.
function datedEquivalent(url) {
  var today = localDateSlug(new Date());
  var path = normalizePathname(url.pathname);
  var qDate = url.searchParams.get("date");

  if (path === "/") {
    if (!qDate) {
      return "/?date=" + today;
    }
    return null;
  }

  // Bare hour shell: /lauds or /lauds?date=YYYY-MM-DD → /lauds/DATE
  if (HOURS.indexOf(path.replace(/^\//, "")) >= 0) {
    var hour = path.replace(/^\//, "");
    if (qDate && DATE_RE.test(qDate)) {
      return "/" + hour + "/" + qDate;
    }
    if (!qDate) {
      return "/" + hour + "/" + today;
    }
    // Invalid ?date= — let the server render an error.
    return null;
  }

  if (path === "/calendar") {
    // Mirror server handleCalendar: year page anchored at today.
    return "/calendar/" + new Date().getFullYear() + "#d-" + today;
  }
  return null;
}

// isSWRPage is true for URLs the precache fills and that are safe to serve
// from the versioned Cache bucket immediately (revalidated in background).
function isSWRPage(url) {
  var path = normalizePathname(url.pathname);
  if (path === "/reminders") {
    return true;
  }
  if (path === "/" && DATE_RE.test(url.searchParams.get("date") || "")) {
    return true;
  }
  if (HOUR_DATED_RE.test(path)) {
    return true;
  }
  if (CALENDAR_YEAR_RE.test(path)) {
    return true;
  }
  return false;
}

// canonicalCacheKey is the string key used when storing SWR responses so
// trailing-slash and theme variants land on the same precache entry.
function canonicalCacheKey(url) {
  var path = normalizePathname(url.pathname);
  if (path === "/reminders") {
    return "/reminders";
  }
  if (path === "/" && DATE_RE.test(url.searchParams.get("date") || "")) {
    return "/?date=" + url.searchParams.get("date");
  }
  var hourMatch = path.match(HOUR_DATED_RE);
  if (hourMatch) {
    return "/" + hourMatch[1] + "/" + hourMatch[2];
  }
  if (CALENDAR_YEAR_RE.test(path)) {
    return path;
  }
  return path + url.search;
}

// fallbackURLs returns cache candidates for offline / cold-miss recovery.
function fallbackURLs(url) {
  var urls = [];
  var path = normalizePathname(url.pathname);
  var dated = datedEquivalent(url);
  if (dated) {
    // Cache Storage keys do not include the hash.
    addUniqueURL(urls, dated.split("#")[0]);
  }
  var canonical = canonicalCacheKey(url);
  if (canonical) {
    addUniqueURL(urls, canonical);
  }
  // Trailing-slash twin of dated hour/calendar.
  if (path !== url.pathname) {
    addUniqueURL(urls, path + (url.search || ""));
  }
  if (url.searchParams.get("theme")) {
    var plain = new URL(url.href);
    plain.searchParams.delete("theme");
    plain.pathname = normalizePathname(plain.pathname);
    addUniqueURL(urls, plain.pathname + plain.search);
  }
  return urls;
}

function cacheMatchFirst(cache, candidates) {
  if (candidates.length === 0) {
    return Promise.resolve(undefined);
  }
  return cache.match(candidates[0]).then(function (cached) {
    if (cached) {
      return cached;
    }
    return cacheMatchFirst(cache, candidates.slice(1));
  });
}

function fetchWithTimeout(req, timeoutMS) {
  if ("AbortController" in self) {
    var controller = new AbortController();
    var timer = setTimeout(function () {
      controller.abort();
    }, timeoutMS);
    return fetch(req, { signal: controller.signal, cache: "reload" }).then(function (resp) {
      clearTimeout(timer);
      return resp;
    }).catch(function (err) {
      clearTimeout(timer);
      throw err;
    });
  }

  return Promise.race([
    networkFetch(req),
    new Promise(function (_, reject) {
      setTimeout(function () {
        reject(new Error("network timeout"));
      }, timeoutMS);
    })
  ]);
}

function offlineResponse() {
  return new Response(
    "<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"UTF-8\">" +
    "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">" +
    "<meta name=\"description\" content=\"Benedictine Divine Office offline page.\">" +
    "<title>Offline - Divine Office</title>" +
    "<link rel=\"stylesheet\" href=\"" + assetURL("/static/style.css") + "\">" +
    "<link rel=\"icon\" type=\"image/svg+xml\" href=\"" + assetURL("/static/favicon.svg") + "\">" +
    "<link rel=\"manifest\" href=\"" + assetURL("/static/manifest.webmanifest") + "\">" +
    "<meta name=\"theme-color\" content=\"#faf2ec\">" +
    "<script src=\"" + assetURL("/static/app.js") + "\" defer></script></head><body>" +
    "<a class=\"skip-link\" href=\"#main-content\">Skip to content</a>" +
    "<header class=\"site-header\"><div class=\"site-nav-shell\">" +
    "<a class=\"site-brand\" href=\"/\" data-nav=\"home\"><span aria-hidden=\"true\">✠</span> Daily Office</a>" +
    "<details class=\"site-menu\" open><summary>Menu</summary><nav aria-label=\"Primary\">" +
    "<a href=\"/lauds\" data-nav=\"hour\" data-hour=\"lauds\">Lauds</a>" +
    "<a href=\"/prime\" data-nav=\"hour\" data-hour=\"prime\">Prime</a>" +
    "<a href=\"/terce\" data-nav=\"hour\" data-hour=\"terce\">Terce</a>" +
    "<a href=\"/sext\" data-nav=\"hour\" data-hour=\"sext\">Sext</a>" +
    "<a href=\"/none\" data-nav=\"hour\" data-hour=\"none\">None</a>" +
    "<a href=\"/vespers\" data-nav=\"hour\" data-hour=\"vespers\">Vespers</a>" +
    "<a href=\"/compline\" data-nav=\"hour\" data-hour=\"compline\">Compline</a>" +
    "<span class=\"nav-divider\" aria-hidden=\"true\"></span>" +
    "<a href=\"/calendar\" data-nav=\"calendar\">Ordo</a>" +
    "<a class=\"nav-secondary\" href=\"/reminders\" data-nav=\"reminders\">Reminders</a>" +
    "</nav></details></div></header><main id=\"main-content\">" +
    "<section class=\"offline-page\" aria-labelledby=\"offline-heading\">" +
    "<p class=\"home-kicker\">Offline</p>" +
    "<h1 id=\"offline-heading\">This page is not saved</h1>" +
    "<p>The page you requested is not available in the offline cache.</p>" +
    "<p>Recently visited pages and the next two weeks of hours are available after the app has synced online.</p>" +
    "<p class=\"offline-actions\"><a class=\"pray-now\" href=\"/\" data-nav=\"home\">Open saved home page</a></p>" +
    "</section></main><footer><p>Benedictine Divine Office</p></footer></body></html>",
    { status: 503, headers: { "Content-Type": "text/html; charset=utf-8" } }
  );
}

// cacheFirst serves a cached static asset immediately when present, otherwise
// fetches from the network and stores the result. Safe only because asset URLs
// include ?v=VERSION: a deploy changes the URL, so a new page never reuses a
// previous deploy's CSS/JS cache entry under the same key.
function cacheFirst(req) {
  return caches.open(CACHE).then(function (cache) {
    return cache.match(req).then(function (cached) {
      if (cached) {
        return cached;
      }
      return networkFetch(req).then(function (resp) {
        return putIfOk(cache, req, resp);
      });
    });
  });
}

// staleWhileRevalidate serves a cached page immediately and refreshes it from
// the network in the background. Safe for dated liturgical pages within a
// versioned Cache bucket: activate deletes office-OLD on deploy, so HTML never
// pairs with the previous deploy's CSS. Revalidation always uses cache:"reload"
// so a post-deploy HTML body is not re-poisoned from the browser HTTP cache.
function staleWhileRevalidate(req, url) {
  return caches.open(CACHE).then(function (cache) {
    var storeKey = canonicalCacheKey(url);
    var candidates = fallbackURLs(url);
    addUniqueURL(candidates, storeKey);
    // Prefer exact request match first when present.
    candidates.unshift(req);

    return cacheMatchFirst(cache, candidates).then(function (cached) {
      var revalidate = networkFetch(req).then(function (resp) {
        return putIfOk(cache, storeKey, resp);
      }).catch(function () {
        return undefined;
      });

      if (cached) {
        revalidate.then(function () {});
        return cached;
      }

      // Cold miss: bound wait so a hung network falls back to near-miss cache keys.
      return fetchWithTimeout(req, PAGE_NETWORK_TIMEOUT_MS).then(function (resp) {
        return putIfOk(cache, storeKey, resp);
      }).catch(function () {
        return cacheMatchFirst(cache, candidates).then(function (fallback) {
          if (fallback) {
            return fallback;
          }
          return offlineResponse();
        });
      });
    });
  });
}

// redirectToDated sends undated/alternate navigations to the dated cache key the
// precache fills, so start_url "/" and bare /lauds share keys with SWR pages.
function redirectToDated(url, datedPath) {
  return Response.redirect(new URL(datedPath, url.origin).href, 302);
}

// networkFirstWithFallback is retained for non-dated pages that are not SWR
// targets (errors, odd paths). Prefer cache after a failed network attempt.
function networkFirstWithFallback(req, url) {
  return caches.open(CACHE).then(function (cache) {
    var candidates = fallbackURLs(url);
    candidates.push(req);
    return fetchWithTimeout(req, PAGE_NETWORK_TIMEOUT_MS).then(function (resp) {
      if (resp.ok) {
        cache.put(canonicalCacheKey(url) || req, resp.clone());
      }
      return resp;
    }).catch(function () {
      return cacheMatchFirst(cache, candidates).then(function (cached) {
        if (cached) {
          return cached;
        }
        return offlineResponse();
      });
    });
  });
}

self.addEventListener("fetch", function (event) {
  var req = event.request;
  if (req.method !== "GET") {
    return;
  }
  var url = new URL(req.url);
  if (url.origin !== self.location.origin) {
    return;
  }

  // Static assets: cache-first on versioned URLs (see cacheFirst).
  if (url.pathname.indexOf("/static/") === 0) {
    event.respondWith(cacheFirst(req));
    return;
  }

  // The worker itself is never cached.
  if (url.pathname === "/sw.js") {
    return;
  }

  // ICS feed is personalized and always network.
  if (url.pathname === "/office.ics") {
    return;
  }

  // Undated / alternate forms → dated URL (same keys as precache + SWR).
  var dated = datedEquivalent(url);
  if (dated) {
    event.respondWith(redirectToDated(url, dated));
    return;
  }

  // Dated hours, dated home, year calendar, reminders: serve cache first.
  if (isSWRPage(url)) {
    event.respondWith(staleWhileRevalidate(req, url));
    return;
  }

  // Anything else: network with offline fallback.
  event.respondWith(networkFirstWithFallback(req, url));
});

// fetchInBatches fetches URLs a few at a time, caching successes and
// ignoring individual failures.
function fetchInBatches(cache, urls, batchSize) {
  if (urls.length === 0) {
    return Promise.resolve();
  }
  var batch = urls.slice(0, batchSize);
  var rest = urls.slice(batchSize);
  return Promise.all(batch.map(function (u) {
    return networkFetch(u).then(function (resp) {
      if (resp.ok) {
        return cache.put(u, resp);
      }
    }).catch(function () {
      // Offline or transient failure: skip; next sync will retry.
    });
  })).then(function () {
    return fetchInBatches(cache, rest, batchSize);
  });
}

// pruneOldPages deletes cached dated pages older than today.
function pruneOldPages(cache, today) {
  return cache.keys().then(function (keys) {
    return Promise.all(keys.map(function (req) {
      var m = new URL(req.url).pathname.match(/(\d{4}-\d{2}-\d{2})/) ||
        new URL(req.url).search.match(/date=(\d{4}-\d{2}-\d{2})/);
      if (m && m[1] < today) {
        return cache.delete(req);
      }
      return Promise.resolve(false);
    }));
  });
}

// precacheUpcoming fetches the home page and all seven hours for the next
// PRECACHE_DAYS days, at most once per day per worker version.
function precacheUpcoming() {
  var today = localDateSlug(new Date());
  var stampKey = "/__precache-stamp";
  var stampValue = VERSION + ":" + today;

  return caches.open(META_CACHE).then(function (meta) {
    return meta.match(stampKey).then(function (resp) {
      return resp ? resp.text() : "";
    }).then(function (existing) {
      if (existing === stampValue) {
        return;
      }
      return caches.open(CACHE).then(function (cache) {
        var urls = [];
        var years = [];
        var d = new Date();
        for (var i = 0; i < PRECACHE_DAYS; i++) {
          var slug = localDateSlug(d);
          urls.push("/?date=" + slug);
          addUniqueURL(years, String(d.getFullYear()));
          for (var h = 0; h < HOURS.length; h++) {
            urls.push("/" + HOURS[h] + "/" + slug);
          }
          d.setDate(d.getDate() + 1);
        }
        for (var y = 0; y < years.length; y++) {
          urls.push("/calendar/" + years[y]);
        }
        urls.push("/reminders");
        return pruneOldPages(cache, today).then(function () {
          return fetchInBatches(cache, urls, 6);
        });
      }).then(function () {
        return meta.put(stampKey, new Response(stampValue));
      });
    });
  });
}

self.addEventListener("message", function (event) {
  if (event.data && event.data.type === "precache") {
    event.waitUntil(precacheUpcoming());
  }
});
