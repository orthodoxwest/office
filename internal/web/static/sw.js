/* Service worker for the Divine Office PWA.
 *
 * Served at /sw.js by the Go server, which stamps the version placeholder
 * below with a hash of the binary + data directory. A deploy that changes anything affecting
 * rendered pages therefore changes this file, reinstalls the worker, and
 * starts a fresh cache.
 */

var VERSION = "__VERSION__";
var CACHE = "office-" + VERSION;
var META_CACHE = "office-meta";
var PRECACHE_DAYS = 14;
var PAGE_NETWORK_TIMEOUT_MS = 1500;
var HOURS = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];

var CORE_ASSETS = [
  "/static/style.css",
  "/static/app.js",
  "/static/favicon.svg",
  "/static/manifest.webmanifest",
  "/static/icons/icon-192.png",
  "/static/icons/icon-512.png"
];

var APP_SHELL_URLS = [
  "/",
  "/reminders"
];

self.addEventListener("install", function (event) {
  event.waitUntil(
    caches.open(CACHE).then(function (cache) {
      return cache.addAll(CORE_ASSETS.concat(APP_SHELL_URLS));
    }).then(function () {
      return precacheUpcoming();
    }).then(function () {
      return self.skipWaiting();
    })
  );
});

self.addEventListener("activate", function (event) {
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
      return precacheUpcoming();
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

// fallbackURLs returns cache fallback candidates for URLs that are commonly
// launched or navigated without the exact query string used during precache.
function fallbackURLs(url) {
  var urls = [];

  var today = localDateSlug(new Date());
  if (url.pathname === "/" && !url.searchParams.get("date")) {
    addUniqueURL(urls, "/?date=" + today);
  }
  var hour = url.pathname.replace(/^\/|\/$/g, "");
  if (HOURS.indexOf(hour) >= 0) {
    addUniqueURL(urls, "/" + hour + "/" + today);
  }
  if (url.pathname === "/calendar" || url.pathname === "/calendar/") {
    addUniqueURL(urls, "/calendar/" + new Date().getFullYear());
  }
  if (url.searchParams.get("theme")) {
    var plain = new URL(url.href);
    plain.searchParams.delete("theme");
    addUniqueURL(urls, plain.pathname + plain.search);
  }
  return urls;
}

function prefersFallbackFirst(url) {
  var hour = url.pathname.replace(/^\/|\/$/g, "");
  return (url.pathname === "/" && !url.searchParams.get("date")) ||
    HOURS.indexOf(hour) >= 0 ||
    url.pathname === "/calendar" ||
    url.pathname === "/calendar/";
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
    return fetch(req, { signal: controller.signal }).then(function (resp) {
      clearTimeout(timer);
      return resp;
    }).catch(function (err) {
      clearTimeout(timer);
      throw err;
    });
  }

  return Promise.race([
    fetch(req),
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
    "<link rel=\"stylesheet\" href=\"/static/style.css\">" +
    "<link rel=\"icon\" type=\"image/svg+xml\" href=\"/static/favicon.svg\">" +
    "<link rel=\"manifest\" href=\"/static/manifest.webmanifest\">" +
    "<meta name=\"theme-color\" content=\"#fdf9f2\">" +
    "<script src=\"/static/app.js\" defer></script></head><body>" +
    "<a class=\"skip-link\" href=\"#main-content\">Skip to content</a>" +
    "<header><nav aria-label=\"Primary\">" +
    "<a href=\"/\">Home</a><a href=\"/lauds\">Lauds</a><a href=\"/prime\">Prime</a>" +
    "<a href=\"/terce\">Terce</a><a href=\"/sext\">Sext</a><a href=\"/none\">None</a>" +
    "<a href=\"/vespers\">Vespers</a><a href=\"/compline\">Compline</a>" +
    "<a href=\"/calendar\">Calendar</a><a href=\"/reminders\">Reminders</a>" +
    "</nav></header><main id=\"main-content\">" +
    "<section class=\"offline-page\" aria-labelledby=\"offline-heading\">" +
    "<p class=\"home-kicker\">Offline</p>" +
    "<h1 id=\"offline-heading\">This page is not saved</h1>" +
    "<p>The page you requested is not available in the offline cache.</p>" +
    "<p>Recently visited pages and the next two weeks of hours are available after the app has synced online.</p>" +
    "<p class=\"offline-actions\"><a class=\"pray-now\" href=\"/\">Open saved home page</a></p>" +
    "</section></main><footer><p>Benedictine Divine Office</p></footer></body></html>",
    { status: 503, headers: { "Content-Type": "text/html; charset=utf-8" } }
  );
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

  // Static assets: cache-first.
  if (url.pathname.indexOf("/static/") === 0) {
    event.respondWith(
      caches.open(CACHE).then(function (cache) {
        return cache.match(req).then(function (cached) {
          if (cached) {
            return cached;
          }
          return fetch(req).then(function (resp) {
            if (resp.ok) {
              cache.put(req, resp.clone());
            }
            return resp;
          });
        });
      })
    );
    return;
  }

  // The worker itself is never cached.
  if (url.pathname === "/sw.js") {
    return;
  }

  // Pages: network-first, falling back to cache, then to today's dated
  // equivalent for undated URLs, then to a plain offline notice.
  //
  // We always attempt the network first rather than trusting
  // navigator.onLine, which reports false positives on desktop browsers
  // (after sleep/wake or a network change). When genuinely offline the
  // fetch rejects almost immediately, so the cache fallback is still fast;
  // PAGE_NETWORK_TIMEOUT_MS only bounds the wait on a slow-but-live network.
  event.respondWith(
    caches.open(CACHE).then(function (cache) {
      var candidates = fallbackURLs(url);
      if (prefersFallbackFirst(url)) {
        candidates.push(req);
      } else {
        candidates.unshift(req);
      }

      return fetchWithTimeout(req, PAGE_NETWORK_TIMEOUT_MS).then(function (resp) {
        if (resp.ok) {
          cache.put(req, resp.clone());
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
    })
  );
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
    return fetch(u).then(function (resp) {
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
