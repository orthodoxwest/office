document.documentElement.classList.add("js");

// Initials have two independent decisions: their place in the prayer's
// hierarchy, and their optical fit. CSS holds the font-specific letter
// profiles; this enhancement identifies the letter and opening word without
// changing the text. Prose can adapt to its measure, while short responses
// stay modest and metrical/chant openings keep their deliberate two-line cap.
(function () {
  var openings = Array.from(document.querySelectorAll([
    ".psalm-verses .verse:first-child:not(.numbered)",
    ".chapter .liturgical-block .plain-line:first-child",
    ".chapter .liturgical-block .versicle-line:first-child .sigil-text",
    ".collect .liturgical-block .plain-line:first-child",
    ".corporate-lord-prayer-officiant",
    ".short-responsory-opening .sigil-text",
    ".hymn-stanza-opening .hymn-line:first-child",
    ".marian-antiphon .chant-line-opening",
  ].join(",")));
  if (!openings.length) return;

  openings.forEach(function (opening) {
    var node = opening.firstChild;
    if (!node || node.nodeType !== Node.TEXT_NODE) return;
    var match = /^(\s*)([\p{L}][\p{L}\p{M}'’-]*)/u.exec(node.textContent);
    if (!match) return;
    var letter = /^\p{L}\p{M}*/u.exec(match[2])[0];
    opening.dataset.initial = letter.normalize("NFC").toUpperCase();
    // A standalone O or I keeps its actual word space. Give the next word
    // the small-cap transition instead of treating it as a kerned suffix.
    var start = match[1].length + letter.length;
    var end = match[0].length;
    if (start === end) {
      opening.dataset.initialStandalone = "true";
      var next = /^(\s+)([\p{L}][\p{L}\p{M}'’-]*)/u.exec(node.textContent.slice(end));
      if (!next) return;
      start = end + next[1].length;
      end += next[0].length;
    }
    var word = node.splitText(start);
    word.splitText(end - start);
    var span = document.createElement("span");
    span.className = "initial-word";
    word.parentNode.insertBefore(span, word);
    span.appendChild(word);
  });

  openings.filter(function (opening) {
    return opening.matches(".short-responsory-opening .sigil-text");
  }).forEach(function (opening) { opening.classList.add("initial-raised"); });
  var adaptive = openings.filter(function (opening) {
    return !opening.matches(".hymn-line, .chant-line-opening, .short-responsory-opening .sigil-text");
  });

  var measures = new WeakMap();
  var frame = 0;
  function typeset() {
    frame = 0;
    var changed = adaptive.filter(function (opening) {
      var style = getComputedStyle(opening);
      var width = opening.getBoundingClientRect().width;
      var key = [width, style.fontSize, style.lineHeight].join("/");
      if (measures.get(opening) === key) return false;
      measures.set(opening, key);
      return width > 0;
    });
    // Batch writes and reads: one layout measures every candidate. Remember
    // width and font metrics, not height, so our own changes do not cause a
    // ResizeObserver loop or toggle between two competing line breaks.
    changed.forEach(function (opening) { opening.classList.add("initial-raised"); });
    var singleLines = changed.map(function (opening) {
      return opening.getBoundingClientRect().height <=
        parseFloat(getComputedStyle(opening).lineHeight) + 1;
    });
    changed.forEach(function (opening, index) {
      opening.classList.toggle("initial-raised", singleLines[index]);
    });
  }
  function schedule() {
    if (!frame) frame = window.requestAnimationFrame(typeset);
  }
  function refresh() {
    measures = new WeakMap();
    schedule();
  }
  typeset();
  if (document.fonts) {
    document.fonts.ready.then(refresh);
    document.fonts.addEventListener("loadingdone", refresh);
  }
  if ("ResizeObserver" in window) {
    var observer = new ResizeObserver(schedule);
    adaptive.forEach(function (opening) { observer.observe(opening); });
  }
  window.addEventListener("resize", schedule);
  window.addEventListener("officeleaderchange", refresh);
  window.addEventListener("beforeprint", function () {
    measures = new WeakMap();
    typeset();
  });
  window.addEventListener("afterprint", refresh);
})();

// Appearance and screen dimensions accompany every usage beacon; office pages
// also report the selected prayer form. These describe how the page
// is being rendered rather than which page it is: the appearance actually on
// screen (Nave or Apse, whether chosen or inherited from the device), and
// whether this is a phone-shaped reading — the 700px layout breakpoint, or a
// coarse pointer, which catches tablets and a phone held in landscape. Both
// are read at send time; the server counts each like any
// other scope, once per browser per day.
//
// Each is reported as "family:value", the name it is stored under, so a value
// belongs to exactly one family and a family retired later cannot be confused
// with a value name reused by a different one (see usage.Dimensions).
function usageBeaconBody(scope) {
  var leader = document.documentElement.getAttribute("data-leader") || "private";
  var leaderToken = document.body.classList.contains("page-hour") && ["private", "deacon", "priest"].indexOf(leader) >= 0 ? " prayer-form:" + leader : "";
  if (!window.matchMedia) {
    return scope + leaderToken;
  }
  var forced = document.documentElement.getAttribute("data-theme");
  var dark = forced === "dark" ||
    (forced !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  var handheld = window.matchMedia("(max-width: 700px), (pointer: coarse)").matches;
  return scope +
    (dark ? " appearance:apse" : " appearance:nave") +
    (handheld ? " screen:mobile" : " screen:desktop") + leaderToken;
}

(function () {
  document.cookie = "tz=" + Intl.DateTimeFormat().resolvedOptions().timeZone + ";path=/;SameSite=Lax";

  // Appearance: Default (device) / Nave (light) / Apse (dark). Persisted in
  // localStorage so links and the service-worker cache stay theme-free. A
  // matching pre-paint script in layout.html applies the choice before CSS.
  // Only explicit button clicks write storage — passive load never invents a choice.
  var THEME_KEY = "office-theme";
  var THEME_LIGHT_COLOR = "#fdf1e6";
  var THEME_DARK_COLOR = "#121c28";

  var readStoredTheme = function () {
    try {
      return localStorage.getItem(THEME_KEY);
    } catch {
      return null;
    }
  };

  var writeStoredTheme = function (value) {
    try {
      localStorage.setItem(THEME_KEY, value);
    } catch {
      // Private mode / blocked storage — theme still applies for this page.
    }
  };

  var normalizeThemeChoice = function (value) {
    if (value === "light" || value === "dark" || value === "default") {
      return value;
    }
    return null;
  };

  var effectiveThemeChoice = function () {
    return normalizeThemeChoice(readStoredTheme()) || "default";
  };

  var syncThemeColorMeta = function () {
    var forced = document.documentElement.getAttribute("data-theme");
    var metas = document.querySelectorAll('meta[name="theme-color"]');
    if (!metas.length) {
      return;
    }
    if (forced === "light") {
      metas.forEach(function (m) {
        m.setAttribute("content", THEME_LIGHT_COLOR);
      });
      return;
    }
    if (forced === "dark") {
      metas.forEach(function (m) {
        m.setAttribute("content", THEME_DARK_COLOR);
      });
      return;
    }
    metas.forEach(function (m) {
      var media = m.getAttribute("media") || "";
      if (media.indexOf("dark") !== -1) {
        m.setAttribute("content", THEME_DARK_COLOR);
      } else {
        m.setAttribute("content", THEME_LIGHT_COLOR);
      }
    });
  };

  // Paint the DOM for a choice without writing localStorage.
  var paintThemeChoice = function (choice) {
    choice = normalizeThemeChoice(choice) || "default";
    if (choice === "light" || choice === "dark") {
      document.documentElement.setAttribute("data-theme", choice);
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
    syncThemeColorMeta();
    document.querySelectorAll(".theme-option[data-theme-choice]").forEach(function (btn) {
      var on = btn.getAttribute("data-theme-choice") === choice;
      btn.setAttribute("aria-pressed", on ? "true" : "false");
    });
  };

  // Let the room dim rather than snap when someone picks an appearance. The
  // transitions live behind .theme-anim in style.css, added only around an
  // explicit click, so the pre-paint script and any system light/dark flip
  // still repaint instantly and no load ever animates.
  var themeAnimTimer = null;
  var flashThemeTransition = function () {
    var root = document.documentElement;
    root.classList.add("theme-anim");
    if (themeAnimTimer) {
      clearTimeout(themeAnimTimer);
    }
    themeAnimTimer = setTimeout(function () {
      themeAnimTimer = null;
      root.classList.remove("theme-anim");
    }, 320);
  };

  // The Apse starfield is a background-image, so it cannot ease alongside
  // the color crossfade above — it can only snap. Dip it to transparent,
  // swap the theme (and so the underlying image) while it's invisible, then
  // let style.css's opacity transition climb it back to full: the swap
  // itself never renders, so the vault fades rather than flashing on or off.
  var VAULT_FADE_MS = 100;
  var vaultFadeTimer = null;

  // User action: paint + persist.
  var applyThemeChoice = function (choice) {
    choice = normalizeThemeChoice(choice) || "default";
    flashThemeTransition();
    var root = document.documentElement;
    if (window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      paintThemeChoice(choice);
      writeStoredTheme(choice);
      return;
    }
    root.classList.add("vault-hidden");
    if (vaultFadeTimer) {
      clearTimeout(vaultFadeTimer);
    }
    vaultFadeTimer = setTimeout(function () {
      vaultFadeTimer = null;
      paintThemeChoice(choice);
      writeStoredTheme(choice);
      root.classList.remove("vault-hidden");
    }, VAULT_FADE_MS);
  };

  paintThemeChoice(effectiveThemeChoice());

  document.querySelectorAll(".theme-option[data-theme-choice]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var choice = btn.getAttribute("data-theme-choice");
      if (choice === "light" || choice === "dark" || choice === "default") {
        applyThemeChoice(choice);
      }
    });
  });

  // Text size: Smaller / Default / Larger. Same contract as the theme control —
  // localStorage only (no URL params, no server), a matching pre-paint script in
  // layout.html, and storage written only on an explicit click.
  var TEXT_SIZE_KEY = "office-text-size";

  var readStoredTextSize = function () {
    try {
      return localStorage.getItem(TEXT_SIZE_KEY);
    } catch {
      return null;
    }
  };

  var writeStoredTextSize = function (value) {
    try {
      localStorage.setItem(TEXT_SIZE_KEY, value);
    } catch {
      // Private mode / blocked storage — the size still applies for this page.
    }
  };

  var normalizeTextSizeChoice = function (value) {
    if (value === "small" || value === "large" || value === "default") {
      return value;
    }
    return null;
  };

  var effectiveTextSizeChoice = function () {
    return normalizeTextSizeChoice(readStoredTextSize()) || "default";
  };

  // Paint the DOM for a choice without writing localStorage.
  var paintTextSizeChoice = function (choice) {
    choice = normalizeTextSizeChoice(choice) || "default";
    if (choice === "small" || choice === "large") {
      document.documentElement.setAttribute("data-text-size", choice);
    } else {
      document.documentElement.removeAttribute("data-text-size");
    }
    document.querySelectorAll(".text-size-option[data-text-size-choice]").forEach(function (btn) {
      var on = btn.getAttribute("data-text-size-choice") === choice;
      btn.setAttribute("aria-pressed", on ? "true" : "false");
    });
  };

  // User action: paint + persist.
  var applyTextSizeChoice = function (choice) {
    choice = normalizeTextSizeChoice(choice) || "default";
    paintTextSizeChoice(choice);
    writeStoredTextSize(choice);
  };

  paintTextSizeChoice(effectiveTextSizeChoice());

  document.querySelectorAll(".text-size-option[data-text-size-choice]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var choice = normalizeTextSizeChoice(btn.getAttribute("data-text-size-choice"));
      if (choice) {
        applyTextSizeChoice(choice);
      }
    });
  });

  var banner = document.getElementById("site-banner");
  if (banner) {
    var dismissButton = banner.querySelector("[data-dismiss-banner]");
    if (dismissButton) {
      dismissButton.addEventListener("click", function () {
        banner.hidden = true;
      });
    }
  }

  var offlineIndicator = document.createElement("div");
  offlineIndicator.className = "offline-indicator";
  offlineIndicator.textContent = "Offline";
  offlineIndicator.setAttribute("role", "status");
  offlineIndicator.setAttribute("aria-live", "polite");
  offlineIndicator.hidden = true;
  document.body.appendChild(offlineIndicator);

  // While the indicator is showing "Offline" we poll to detect recovery,
  // since the browser's "online" event does not fire when navigator.onLine
  // was a false negative to begin with. Polling stops once we're back online.
  //
  // The browser "offline" event is *not* treated as definitive: mobile
  // browsers fire it during wifi↔cell handoffs and brief radio blips while
  // still online. We only show the pill after consecutive failed probes.
  var recoveryTimer = null;
  var failStreak = 0;
  var FAIL_STREAK_TO_SHOW = 2;
  var ONLINE_CHECK_TIMEOUT_MS = 4000;

  var setOfflineIndicator = function (offline) {
    offlineIndicator.hidden = !offline;
    if (offline && !recoveryTimer) {
      recoveryTimer = setInterval(checkOnline, 4000);
    } else if (!offline && recoveryTimer) {
      clearInterval(recoveryTimer);
      recoveryTimer = null;
    }
  };

  // checkOnline always probes the network rather than trusting
  // navigator.onLine, which reports false negatives on desktop browsers
  // (after sleep/wake or a network change) and would otherwise show the
  // offline banner — and serve stale pages — while fully online. /sw.js is
  // used because the service worker passes it straight through to the
  // network instead of answering from cache.
  function checkOnline() {
    var url = "/sw.js?online-check=" + Date.now();
    var options = { cache: "no-store" };
    var timer;

    if ("AbortController" in window) {
      var controller = new AbortController();
      options.signal = controller.signal;
      timer = setTimeout(function () {
        controller.abort();
      }, ONLINE_CHECK_TIMEOUT_MS);
    }

    var done = function () {
      if (timer) {
        clearTimeout(timer);
      }
    };

    fetch(url, options).then(function (resp) {
      done();
      if (resp.ok) {
        failStreak = 0;
        setOfflineIndicator(false);
      } else {
        failStreak += 1;
        if (failStreak >= FAIL_STREAK_TO_SHOW) {
          setOfflineIndicator(true);
        }
      }
    }).catch(function () {
      done();
      failStreak += 1;
      if (failStreak >= FAIL_STREAK_TO_SHOW) {
        setOfflineIndicator(true);
      } else if (!recoveryTimer) {
        // First failure while looking online: re-probe soon without flashing
        // the pill for a transient blip.
        setTimeout(checkOnline, 1500);
      }
    });
  }

  window.addEventListener("online", checkOnline);
  window.addEventListener("offline", function () {
    // Probe rather than trusting the event — wifi/cell handoffs fire this
    // spuriously. A real outage will fail the probe and show the pill.
    checkOnline();
  });
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) {
      checkOnline();
    }
  });
  checkOnline();

  // localDateSlug formats a date as YYYY-MM-DD in the device's local timezone.
  function localDateSlug(d) {
    var m = String(d.getMonth() + 1);
    var day = String(d.getDate());
    return d.getFullYear() + "-" + (m.length < 2 ? "0" + m : m) + "-" + (day.length < 2 ? "0" + day : day);
  }

  // documentDateSlug is the liturgical day this document is about (URL or
  // home card only). Returns null when the page has no day identity (error,
  // reminders, bare calendar year) so callers can fall back to local today
  // for chrome without treating unknown pages as "stale yesterday".
  function documentDateSlug() {
    var pathMatch = location.pathname.match(
      /^\/(?:lauds|prime|terce|sext|none|vespers|compline)\/(\d{4}-\d{2}-\d{2})\/?$/
    );
    if (pathMatch) {
      return pathMatch[1];
    }
    var params = new URLSearchParams(location.search);
    var q = params.get("date");
    if (q && /^\d{4}-\d{2}-\d{2}$/.test(q)) {
      return q;
    }
    var card = document.querySelector(".home-prayer-card[data-date-slug]");
    if (card) {
      var slug = card.getAttribute("data-date-slug");
      if (slug && /^\d{4}-\d{2}-\d{2}$/.test(slug)) {
        return slug;
      }
    }
    return null;
  }

  function pageDateSlug() {
    return documentDateSlug() || localDateSlug(new Date());
  }

  function todayHrefForPage(today) {
    var path = location.pathname.replace(/\/$/, "");
    var hourMatch = path.match(/^\/(lauds|prime|terce|sext|none|vespers|compline)/);
    if (hourMatch) {
      return "/" + hourMatch[1] + "/" + today;
    }
    return "/?date=" + today;
  }

  // ensureTodayControl keeps recovery chrome honest when the open document is
  // not local today. SWR can leave a page that was "today" at render time
  // frozen past midnight with ShowToday=false and no server-rendered control;
  // a long-lived PWA also needs a one-tap path that is not buried under
  // "Change date".
  function ensureTodayControl(today) {
    var docDate = documentDateSlug();
    var onToday = !docDate || docDate === today;
    var href = todayHrefForPage(today);

    // Prominent notice (outside the date disclosure).
    var notices = document.querySelectorAll(".not-today-notice");
    if (onToday) {
      notices.forEach(function (n) {
        n.hidden = true;
      });
    } else if (notices.length) {
      notices.forEach(function (n) {
        n.hidden = false;
        var a = n.querySelector("a.today-link");
        if (a) {
          a.setAttribute("href", href);
        }
      });
    } else {
      var anchor =
        document.querySelector(".hour-meta") ||
        document.querySelector(".home-day-head h1") ||
        document.querySelector(".home-day-head");
      if (anchor) {
        var p = document.createElement("p");
        p.className = "not-today-notice";
        // Live region only for inject: appears without a navigation after
        // midnight. Server-rendered historical notices stay quiet.
        p.setAttribute("role", "status");
        var a = document.createElement("a");
        a.className = "today-link not-today-link";
        a.setAttribute("href", href);
        a.textContent = "Go to today";
        p.appendChild(a);
        anchor.insertAdjacentElement("afterend", p);
      }
    }

    if (onToday) {
      return;
    }

    // Secondary Today inside the date-jump form — always, even when the
    // prominent notice already supplied a .today-link (overnight inject).
    var form = document.querySelector(".date-jump-form");
    if (form) {
      var formToday = form.querySelector("a.today-link");
      if (formToday) {
        formToday.setAttribute("href", href);
        formToday.hidden = false;
      } else {
        var formLink = document.createElement("a");
        formLink.className = "today-link";
        formLink.setAttribute("href", href);
        formLink.textContent = "Today";
        form.appendChild(formLink);
      }
    }

    // Any other today-links (prominent notice, etc.) keep the same target.
    document.querySelectorAll("a.today-link").forEach(function (link) {
      link.setAttribute("href", href);
      link.hidden = false;
    });
  }

  // syncDatedNavigation stamps the top banner (and home prayer card when
  // present) with dated URLs so navigation hits the service-worker precache
  // keys rather than undated /lauds shells. Safe to run on every load: server
  // already emits dated hrefs when NavDate is set; this corrects cached pages
  // and timezone-edge undated markup. Also re-run on visibilitychange after
  // midnight so long-lived tabs regain a Today control.
  //
  // Brand is special: it always means "home for local today", not the page's
  // selected day. Hour/ordo links keep the page day so intentional historical
  // browsing still hops within that day.
  function syncDatedNavigation() {
    var today = localDateSlug(new Date());
    var navDate = pageDateSlug();
    var year = navDate.slice(0, 4);

    var brand = document.querySelector('[data-nav="home"]');
    if (brand) {
      brand.setAttribute("href", "/?date=" + today);
      // Brand is "current" only on today's home. Historical home and SWR pages
      // frozen past midnight still have page-home but brand leaves the day.
      var docDate = documentDateSlug();
      var brandIsCurrent =
        document.body.classList.contains("page-home") &&
        (!docDate || docDate === today);
      if (brandIsCurrent) {
        brand.classList.add("active");
        brand.setAttribute("aria-current", "page");
      } else {
        brand.classList.remove("active");
        brand.removeAttribute("aria-current");
      }
    }

    document.querySelectorAll('[data-nav="hour"][data-hour]').forEach(function (link) {
      var hour = link.getAttribute("data-hour");
      if (hour) {
        link.setAttribute("href", "/" + hour + "/" + navDate);
      }
    });

    var ordo = document.querySelector('[data-nav="calendar"]');
    if (ordo) {
      // Always use navDate's year so #d-DATE exists on that year's table.
      ordo.setAttribute("href", "/calendar/" + year + "#d-" + navDate);
    }

    ensureTodayControl(today);

    // "Today" shortcuts always mean local today, not the page's selected day.
    document.querySelectorAll("a.today-link").forEach(function (link) {
      link.setAttribute("href", todayHrefForPage(today));
    });

    // Home prayer card: re-stamp hour links for the card's day (or today).
    var card = document.querySelector(".home-prayer-card[data-date-slug]");
    if (card) {
      var cardDate = card.getAttribute("data-date-slug") || today;
      card.querySelectorAll(".home-hour-link[data-hour]").forEach(function (link) {
        var hour = link.getAttribute("data-hour");
        if (hour) {
          link.setAttribute("href", "/" + hour + "/" + cardDate);
        }
      });
    }
  }

  var lastSyncedDay = localDateSlug(new Date());
  function syncChromeIfNeeded() {
    syncDatedNavigation();
    updatePrayNow();
    markCalendarToday();
    window.dispatchEvent(new Event("officenavigation"));
  }

  document.addEventListener("visibilitychange", function () {
    if (document.hidden) {
      return;
    }
    var day = localDateSlug(new Date());
    if (day !== lastSyncedDay) {
      lastSyncedDay = day;
      syncChromeIfNeeded();
    } else {
      // Still re-stamp Today hrefs / pray-now on return (clock may have moved
      // within the day; cheap and keeps long-lived tabs honest).
      syncDatedNavigation();
      updatePrayNow();
    }
  });

  // bfcache restore (mobile Safari / Android often freezes a PWA this way)
  // must re-evaluate the day the same way a foreground return does.
  window.addEventListener("pageshow", function (e) {
    if (!e.persisted) {
      return;
    }
    lastSyncedDay = localDateSlug(new Date());
    syncChromeIfNeeded();
  });

  // Ships closed; desktop CSS shows the nav regardless of [open]. Once
  // scripting is available, keep [open] set on wide screens too (harmless
  // cosmetically, but matches disclosure semantics) and re-collapse on any
  // live resize back to narrow after a tap had opened it.
  var siteMenu = document.querySelector(".site-menu");
  if (siteMenu && "matchMedia" in window) {
    var narrowMenu = window.matchMedia("(max-width: 700px)");
    var syncSiteMenu = function () {
      if (narrowMenu.matches) {
        siteMenu.removeAttribute("open");
      } else {
        siteMenu.setAttribute("open", "");
      }
    };
    syncSiteMenu();
    if (narrowMenu.addEventListener) {
      narrowMenu.addEventListener("change", syncSiteMenu);
    }
  }

  // Ordo day details are disclosures only where the layout has no room to
  // show them outright. They ship open — a closed <details> contributes no
  // height, so CSS alone cannot re-reveal the contents on a wide screen, and
  // the no-JS document should show the day in full. Collapse them where the
  // toggle is visible, same trade as the site menu above.
  if ("matchMedia" in window) {
    [
      { selector: "details.day-commemorations", collapseBelow: "(max-width: 700px)" },
      { selector: "details.day-office-details", collapseBelow: "(max-width: 919px)" },
    ].forEach(function (spec) {
      var items = document.querySelectorAll(spec.selector);
      if (!items.length) {
        return;
      }
      var narrow = window.matchMedia(spec.collapseBelow);
      var sync = function () {
        items.forEach(function (item) {
          if (narrow.matches) {
            item.removeAttribute("open");
          } else {
            item.setAttribute("open", "");
          }
        });
      };
      sync();
      if (narrow.addEventListener) {
        narrow.addEventListener("change", sync);
      }
    });
  }

  // Disclosure transitions are scoped to .motion-ready in CSS. Setting it a
  // frame after the load-time [open] syncing above means nothing unfolds or
  // collapses while the page is still settling; only later taps animate.
  var markMotionReady = function () {
    document.documentElement.classList.add("motion-ready");
  };
  if (typeof window.requestAnimationFrame === "function") {
    window.requestAnimationFrame(function () {
      window.requestAnimationFrame(markMotionReady);
    });
  } else {
    markMotionReady();
  }

  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/sw.js").then(function (reg) {
      // Pick up deploys while the PWA stays open across days.
      var askUpdate = function () {
        if (reg.update) {
          reg.update().catch(function () {});
        }
      };
      document.addEventListener("visibilitychange", function () {
        if (!document.hidden) {
          askUpdate();
        }
      });
      // Periodic check in case the tab stays foregrounded all day.
      setInterval(askUpdate, 60 * 60 * 1000);

      return navigator.serviceWorker.ready.then(function (ready) {
        if (ready.active) {
          ready.active.postMessage({ type: "precache" });
        }
        return ready;
      });
    }).catch(function (err) {
      // Offline reading is an enhancement; the site works without it.
      console.warn("service worker registration failed:", err);
    });
  }

  // Fires a one-off usage beacon for an explicit action rather than a page
  // view — used below when a reminder feed link is actually generated.
  // Bot filtering and per-day/per-browser dedup happen server-side, so a
  // repeat click is harmless. Best-effort: never surfaces an error to the
  // person praying.
  var reportUsageAction = function (scope) {
    try {
      fetch("/api/usage", {
        method: "POST",
        headers: { "X-Office-Usage": "1", "Content-Type": "text/plain" },
        body: usageBeaconBody(scope),
        credentials: "same-origin",
        cache: "no-store"
      }).catch(function () {});
    } catch {
      // Ignored — best effort.
    }
  };

  var remindersForm = document.getElementById("reminders-form");
  if (remindersForm) {
    var urlEl = document.getElementById("reminder-url");
    var webcalEl = document.getElementById("reminder-webcal");
    var copyBtn = document.getElementById("reminder-copy");
    var copiedEl = document.getElementById("reminder-copied");
    var addressEl = document.querySelector(".reminder-address");

    var buildFeedURL = function () {
      var params = [];
      remindersForm.querySelectorAll("input[name=hour]:checked").forEach(function (cb) {
        var t = remindersForm.querySelector("input[name=time-" + cb.value + "]");
        if (t && t.value) {
          params.push(cb.value + "=" + encodeURIComponent(t.value));
        }
      });
      var days = [];
      remindersForm.querySelectorAll("input[name=day]:checked").forEach(function (cb) {
        days.push(cb.value);
      });
      if (days.length > 0 && days.length < 7) {
        params.push("days=" + days.join(","));
      }
      var alarm = remindersForm.querySelector("select[name=alarm]").value;
      if (alarm !== "10") {
        params.push("alarm=" + alarm);
      }
      try {
        params.push("tz=" + encodeURIComponent(Intl.DateTimeFormat().resolvedOptions().timeZone));
      } catch {
        // Feed falls back to UTC times if the timezone is unavailable.
      }
      return location.host + "/office.ics?" + params.join("&");
    };

    var syncReminderHourRows = function () {
      remindersForm.querySelectorAll(".reminder-hour-row").forEach(function (row) {
        var checkbox = row.querySelector('input[name="hour"]');
        var time = row.querySelector('input[type="time"]');
        var selected = Boolean(checkbox && checkbox.checked);
        row.classList.toggle("is-selected", selected);
        if (time) {
          time.disabled = !selected;
          time.required = selected;
        }
      });
    };

    var update = function () {
      syncReminderHourRows();
      var selectedHours = remindersForm.querySelectorAll("input[name=hour]:checked");
      var selectedDays = remindersForm.querySelectorAll("input[name=day]:checked");
      var invalidTime = Array.from(selectedHours).some(function (checkbox) {
        var time = remindersForm.querySelector("input[name=time-" + checkbox.value + "]");
        return !time || !time.validity.valid;
      });
      var message = "";
      if (selectedHours.length === 0) {
        message = "Select at least one hour above.";
      } else if (invalidTime) {
        message = "Choose a time for each selected hour.";
      } else if (selectedDays.length === 0) {
        message = "Select at least one day above.";
      }
      var valid = message === "";
      var feed = valid ? buildFeedURL() : "";
      urlEl.textContent = valid ? "https://" + feed : message;
      webcalEl.classList.toggle("is-disabled", !valid);
      webcalEl.setAttribute("aria-disabled", valid ? "false" : "true");
      if (!valid) {
        webcalEl.removeAttribute("href");
        webcalEl.setAttribute("tabindex", "-1");
      } else {
        webcalEl.href = "webcal://" + feed;
        webcalEl.removeAttribute("tabindex");
      }
      copyBtn.disabled = !valid;
      copiedEl.textContent = valid ? "Copied." : message;
      copiedEl.hidden = valid;
    };

    remindersForm.addEventListener("change", update);
    update();

    webcalEl.addEventListener("click", function (event) {
      if (webcalEl.getAttribute("aria-disabled") === "true") {
        event.preventDefault();
        return;
      }
      reportUsageAction("reminders");
    });

    copyBtn.addEventListener("click", function () {
      if (copyBtn.disabled) {
        return;
      }
      var showCopyFallback = function () {
        copiedEl.textContent = "Copy unavailable. The calendar address is shown below.";
        copiedEl.hidden = false;
        if (addressEl) {
          addressEl.open = true;
        }
      };
      if (!navigator.clipboard || typeof navigator.clipboard.writeText !== "function") {
        showCopyFallback();
        return;
      }
      navigator.clipboard.writeText(urlEl.textContent).then(function () {
        copiedEl.textContent = "Copied.";
        copiedEl.hidden = false;
        reportUsageAction("reminders");
      }).catch(showCopyFallback);
    });
  }

  document.querySelectorAll(".date-jump-form").forEach(function (form) {
    var input = form.querySelector(".date-jump");
    if (!input) {
      return;
    }
    input.addEventListener("change", function () {
      if (input.value) {
        form.requestSubmit();
      }
    });
  });

  // currentHourInfo returns the office most likely being prayed at the given
  // local time, plus a day offset (0 or -1) locating which calendar day it
  // belongs to. Boundaries mirror currentHourEntry in handlers.go: midnight-2am
  // belongs to the previous day's Compline, not the day that has just begun.
  // One ordered table drives both the current-office choice and its next
  // refresh boundary. Keep it aligned with currentHourEntry in handlers.go;
  // midnight Compline belongs to the preceding calendar day.
  var OFFICE_SCHEDULE = [
    { start: 0, slug: "compline", label: "Compline", offset: -1 },
    { start: 2, slug: "lauds", label: "Lauds", offset: 0 },
    { start: 7, slug: "prime", label: "Prime", offset: 0 },
    { start: 9, slug: "terce", label: "Terce", offset: 0 },
    { start: 11, slug: "sext", label: "Sext", offset: 0 },
    { start: 13, slug: "none", label: "None", offset: 0 },
    { start: 17, slug: "vespers", label: "Vespers", offset: 0 },
    { start: 20, slug: "compline", label: "Compline", offset: 0 },
  ];
  var HOUR_NAMES = OFFICE_SCHEDULE.reduce(function (names, entry) {
    names[entry.slug] = entry.label;
    return names;
  }, {});

  function currentHourInfo(d) {
    var h = d.getHours();
    for (var i = OFFICE_SCHEDULE.length - 1; i >= 0; i -= 1) {
      if (h >= OFFICE_SCHEDULE[i].start) {
        return {
          slug: OFFICE_SCHEDULE[i].slug,
          offset: OFFICE_SCHEDULE[i].offset,
        };
      }
    }
    return { slug: "compline", offset: -1 };
  }

  function nextOfficeBoundary(d) {
    for (var i = 0; i < OFFICE_SCHEDULE.length; i += 1) {
      var candidate = new Date(d.getTime());
      candidate.setHours(OFFICE_SCHEDULE[i].start, 0, 0, 0);
      if (candidate.getTime() > d.getTime()) {
        return candidate;
      }
    }
    var midnight = new Date(d.getTime());
    midnight.setDate(midnight.getDate() + 1);
    midnight.setHours(0, 0, 0, 0);
    return midnight;
  }

  function setHourCurrent(link, isCurrent) {
    link.classList.toggle("is-current", isCurrent);
    var state = link.querySelector(".home-hour-link-state");
    if (isCurrent) {
      link.setAttribute("aria-current", "time");
      if (!state) {
        state = document.createElement("span");
        // sr-only to match the server-rendered marker in home.html: the tinted
        // cell and the "Pray {hour}" invitation carry "now" visually.
        state.className = "home-hour-link-state sr-only";
        state.textContent = "Now";
        link.appendChild(state);
      }
    } else {
      link.removeAttribute("aria-current");
      if (state) {
        state.parentNode.removeChild(state);
      }
    }
  }

  // updatePrayNow recomputes the "pray now" shortcut on the home page from the
  // device clock. The same value is rendered server-side, but the home page is
  // a cacheable document, so a cached copy would otherwise freeze whichever
  // hour was current when it was fetched. Computing it here keeps the shortcut
  // correct no matter how old the cached page is.
  function updatePrayNow() {
    var card = document.querySelector(".home-prayer-card[data-date-slug]");
    if (!card) {
      return;
    }
    var dateSlug = card.getAttribute("data-date-slug");
    var now = new Date();
    var currentInfo = currentHourInfo(now);
    var officeDate = new Date(now.getTime());
    officeDate.setDate(officeDate.getDate() + currentInfo.offset);
    var info =
      dateSlug === localDateSlug(now) || dateSlug === localDateSlug(officeDate)
        ? currentInfo
        : null;
    var prayNow = card.querySelector(".pray-now");
    var matched = false;

    card.querySelectorAll(".home-hour-link").forEach(function (link) {
      var isCurrent = info !== null && info.offset === 0 && link.getAttribute("data-hour") === info.slug;
      setHourCurrent(link, isCurrent);
      if (isCurrent && prayNow) {
        var name = link.querySelector(".home-hour-link-name");
        prayNow.setAttribute("href", link.getAttribute("href"));
        prayNow.textContent = "Pray " + (name ? name.textContent : "Now");
        matched = true;
      }
    });

    if (!matched && info !== null && info.offset !== 0 && prayNow) {
      var offsetDate = new Date(now.getTime());
      offsetDate.setDate(offsetDate.getDate() + info.offset);
      prayNow.setAttribute("href", "/" + info.slug + "/" + localDateSlug(offsetDate));
      prayNow.textContent = "Pray " + (HOUR_NAMES[info.slug] || info.slug);
      matched = true;
    }

    if (!matched && prayNow) {
      var lauds = card.querySelector('.home-hour-link[data-hour="lauds"]');
      if (lauds) {
        prayNow.setAttribute("href", lauds.getAttribute("href"));
      }
      prayNow.textContent = "Open Lauds";
    }
  }

  // Home, Ordo, and an open office may remain visible across a time boundary
  // without producing a visibility event. Schedule one refresh rather than
  // polling: home uses the next office boundary; Ordo and an office only need
  // midnight. Midnight is also a home boundary because Compline before 2am
  // belongs to the previous liturgical day.
  var timedChromeRefreshTimer = null;
  function scheduleTimedChromeRefresh() {
    var hasHomeCard = Boolean(document.querySelector(".home-prayer-card[data-date-slug]"));
    var hasCalendar = Boolean(document.querySelector(".calendar"));
    var hasOfficeHour = Boolean(document.querySelector(".office-hour"));
    if (!hasHomeCard && !hasCalendar && !hasOfficeHour) {
      return;
    }
    if (timedChromeRefreshTimer) {
      clearTimeout(timedChromeRefreshTimer);
    }
    var now = new Date();
    var next = hasHomeCard ? nextOfficeBoundary(now) : null;
    if (next === null) {
      next = new Date(now.getTime());
      next.setDate(next.getDate() + 1);
      next.setHours(0, 0, 0, 0);
    }
    timedChromeRefreshTimer = setTimeout(function () {
      timedChromeRefreshTimer = null;
      syncChromeIfNeeded();
      scheduleTimedChromeRefresh();
    }, Math.max(0, next.getTime() - now.getTime()) + 250);
  }
  syncChromeIfNeeded();
  scheduleTimedChromeRefresh();

  // markCalendarToday highlights today's row on the ordo page and reveals
  // the header "Today" jump link. Applied client-side because calendar pages
  // are served from the service-worker cache, so a server-rendered marker
  // would freeze on whichever day the page was fetched. The jump link stays
  // hidden when today's row isn't on the displayed year.
  function markCalendarToday() {
    document.querySelectorAll(".calendar tr.day.is-today").forEach(function (previous) {
      previous.classList.remove("is-today");
      previous.removeAttribute("aria-current");
    });
    var todayLink = document.getElementById("calendar-today-link");
    if (todayLink) {
      todayLink.hidden = true;
      todayLink.removeAttribute("href");
    }
    var row = document.getElementById("d-" + localDateSlug(new Date()));
    if (row && row.classList.contains("day")) {
      row.classList.add("is-today");
      row.setAttribute("aria-current", "date");
      if (todayLink) {
        todayLink.setAttribute("href", "#" + row.id);
        todayLink.hidden = false;
      }
    }
  }

  // calendarRowClicks makes the whole ordo row navigate to the day's home
  // page. The day-number link stays as the keyboard/no-JS path; clicks on
  // links, rank tooltips, and the commemorations disclosure keep their own
  // behavior, and selecting text never navigates.
  var calendarEl = document.querySelector(".calendar");
  if (calendarEl) {
    calendarEl.addEventListener("click", function (e) {
      if (e.target.closest("a, abbr, details")) {
        return;
      }
      var selection = window.getSelection();
      if (selection && !selection.isCollapsed) {
        return;
      }
      var row = e.target.closest("tr.day");
      if (!row) {
        return;
      }
      var link = row.querySelector(".day-num a, .day-mobile-date");
      if (link) {
        window.location.href = link.getAttribute("href");
      }
    });
  }

  var officeHour = document.querySelector(".office-hour");

  // Keep the screen awake on hour pages only (not home / ordo / reminders).
  // Opening an hour is the intent signal; the lock releases when the tab is
  // hidden or the user navigates away, and is re-acquired on return.
  if (officeHour && "wakeLock" in navigator) {
    var hourWakeLock = null;
    var requestHourWakeLock = function () {
      if (document.visibilityState !== "visible") {
        return;
      }
      navigator.wakeLock.request("screen").then(function (lock) {
        hourWakeLock = lock;
        lock.addEventListener("release", function () {
          if (hourWakeLock === lock) {
            hourWakeLock = null;
          }
        });
      }).catch(function () {
        // Unsupported, denied, or non-secure context — prayer still works.
      });
    };
    requestHourWakeLock();
    document.addEventListener("visibilitychange", function () {
      if (document.visibilityState === "visible") {
        requestHourWakeLock();
      }
    });
  }

  // Gold hairline under the color band: progress through the prayer itself.
  // The page continues into hour navigation, assurance, issue reporting, and
  // appearance controls after .elements. The line remains at zero through the
  // page header and banner, starts when the prayer reaches the top of the
  // viewport, and reaches 100% when its end reaches the bottom. It then remains
  // complete for the rest of the document.
  if (officeHour) {
    var progress = document.querySelector(".hour-scroll-progress");
    var progressBar = document.querySelector(".hour-scroll-progress-bar");
    var prayerContent = officeHour.querySelector(".elements");
    if (progress && progressBar && prayerContent) {
      var progressTicking = false;
      var updateHourScrollProgress = function () {
        var scrollTop = window.scrollY || document.documentElement.scrollTop || 0;
        var prayerRect = prayerContent.getBoundingClientRect();
        var prayerStart = scrollTop + prayerRect.top;
        var prayerEnd = scrollTop + prayerRect.bottom;
        var startScroll = Math.max(0, prayerStart);
        var completionScroll = Math.max(startScroll, prayerEnd - window.innerHeight);
        var progressRange = completionScroll - startScroll;
        var ratio =
          progressRange <= 0
            ? scrollTop >= completionScroll
              ? 1
              : 0
            : Math.min(1, Math.max(0, (scrollTop - startScroll) / progressRange));
        progressBar.style.transform = "scaleX(" + ratio + ")";
        progress.setAttribute("aria-valuenow", String(Math.round(ratio * 100)));
        progressTicking = false;
      };
      var onHourScroll = function () {
        if (!progressTicking) {
          progressTicking = true;
          window.requestAnimationFrame(updateHourScrollProgress);
        }
      };
      window.addEventListener("scroll", onHourScroll, { passive: true });
      window.addEventListener("resize", onHourScroll);
      // Opening session prayers changes the prayer boundary without necessarily
      // producing a window resize or scroll event. ResizeObserver keeps the
      // semantic endpoint honest; older browsers still update on the next
      // scroll, and the native disclosure remains fully usable.
      if ("ResizeObserver" in window) {
        var prayerResizeObserver = new ResizeObserver(onHourScroll);
        prayerResizeObserver.observe(prayerContent);
      }
      updateHourScrollProgress();
    }
  }
})();

// Record engaged use of a current page, never the service worker's background
// preloads, crawlers, or a scrape of the dated archive.
(function () {
  var scope = "site";
  var hours = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];
  var hour = hours.find(function (name) { return document.body.classList.contains("page-" + name); });
  if (hour) {
    scope = hour;
  } else if (document.body.classList.contains("page-calendar")) {
    scope = "ordo";
  } else if (!["home", "reminders"].some(function (name) {
    return document.body.classList.contains("page-" + name);
  })) {
    return;
  }

  // Only a current page counts. The dated archive is unbounded and stays
  // freely crawlable, so counting it would let one scraper mint a fresh
  // "unique browser" per URL; today's surface is a handful of pages however
  // hard anyone crawls. data-usage-when is an ISO day, a bare year for the
  // ordo, or absent on pages that are always current (see render.UsageWhen).
  var when = document.body.getAttribute("data-usage-when") || "";
  var parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/New_York", year: "numeric", month: "2-digit", day: "2-digit"
  });
  var today = function () {
    var got = {};
    parts.formatToParts(new Date()).forEach(function (part) { got[part.type] = part.value; });
    return got.year + "-" + got.month + "-" + got.day;
  };
  // Re-checked on every attempt, so a tab left open across midnight ages out
  // of the window instead of counting forever.
  var current = function (day) {
    if (!when) return true;
    if (/^\d{4}$/.test(when)) return Math.abs(Number(when) - Number(day.slice(0, 4))) <= 1;
    if (!/^\d{4}-\d{2}-\d{2}$/.test(when)) return false;
    return Math.abs(Date.parse(when + "T00:00:00Z") - Date.parse(day + "T00:00:00Z")) <= 864e5;
  };

  var recorded = new Set();
  var pending = false;
  var engaged = false;
  function record() {
    if (!engaged || document.visibilityState !== "visible" || !navigator.onLine || pending) return;
    var day = today();
    var body = usageBeaconBody(scope);
    var key = day + " " + body;
    if (recorded.has(key) || !current(day)) return;
    pending = true;
    var controller = new AbortController();
    var timeout = window.setTimeout(function () { controller.abort(); }, 4000);
    fetch("/api/usage", {
      method: "POST",
      headers: { "X-Office-Usage": "1", "Content-Type": "text/plain" },
      body: body,
      credentials: "same-origin",
      cache: "no-store",
      signal: controller.signal
    }).then(function (response) {
      if (response.ok) recorded.add(key);
    }).catch(function () {
      // Best effort: never delay prayer or queue offline browsing history.
    }).finally(function () {
      window.clearTimeout(timeout);
      pending = false;
      // A selection made while the previous beacon was in flight still
      // records the form actually selected, without duplicating page totals.
      if (usageBeaconBody(scope) !== body) record();
    });
  }

  // Engagement gate: a headless scraper renders, snapshots and moves on, so
  // nothing counts until someone touches the page or leaves it open a while.
  // Dwell accrues only while visible, so a background tab never qualifies.
  var DWELL_MS = 8000;
  var dwelt = 0;
  var since = 0;
  var timer = 0;
  function pause() {
    if (!since) return;
    dwelt += Date.now() - since;
    since = 0;
    window.clearTimeout(timer);
    timer = 0;
  }
  function engage() {
    if (engaged) return;
    engaged = true;
    pause();
    record();
  }
  function resume() {
    if (engaged || since || document.visibilityState !== "visible") return;
    since = Date.now();
    timer = window.setTimeout(engage, Math.max(0, DWELL_MS - dwelt));
  }
  ["pointerdown", "keydown", "touchstart", "wheel"].forEach(function (type) {
    window.addEventListener(type, engage, { once: true, passive: true });
  });
  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") {
      resume();
      record();
    } else {
      pause();
    }
  });
  window.addEventListener("pageshow", function () { resume(); record(); });
  window.addEventListener("online", record);
  window.addEventListener("officeleaderchange", function () { engage(); record(); });
  // A foreground page left open across midnight belongs to the new day too.
  window.setInterval(record, 60000);
  resume();
})();
