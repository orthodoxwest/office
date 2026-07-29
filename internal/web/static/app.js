document.documentElement.classList.add("js");

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

  // User action: paint + persist.
  var applyThemeChoice = function (choice) {
    choice = normalizeThemeChoice(choice) || "default";
    flashThemeTransition();
    paintThemeChoice(choice);
    writeStoredTheme(choice);
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
  }
  syncChromeIfNeeded();

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

  var remindersForm = document.getElementById("reminders-form");
  if (remindersForm) {
    var urlEl = document.getElementById("reminder-url");
    var webcalEl = document.getElementById("reminder-webcal");
    var copyBtn = document.getElementById("reminder-copy");
    var copiedEl = document.getElementById("reminder-copied");

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

    var update = function () {
      var feed = buildFeedURL();
      var none = remindersForm.querySelectorAll("input[name=hour]:checked").length === 0;
      urlEl.textContent = none ? "Select at least one hour above." : "https://" + feed;
      webcalEl.href = "webcal://" + feed;
      copiedEl.hidden = true;
    };

    remindersForm.addEventListener("change", update);
    update();

    copyBtn.addEventListener("click", function () {
      navigator.clipboard.writeText(urlEl.textContent).then(function () {
        copiedEl.hidden = false;
      }).catch(function () {
        // Clipboard unavailable (e.g. non-secure context); the URL is visible to copy by hand.
      });
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
  var HOUR_NAMES = {
    lauds: "Lauds",
    prime: "Prime",
    terce: "Terce",
    sext: "Sext",
    none: "None",
    vespers: "Vespers",
    compline: "Compline",
  };
  function currentHourInfo(d) {
    var h = d.getHours();
    if (h < 2) return { slug: "compline", offset: -1 };
    if (h >= 2 && h < 7) return { slug: "lauds", offset: 0 };
    if (h >= 7 && h < 9) return { slug: "prime", offset: 0 };
    if (h >= 9 && h < 11) return { slug: "terce", offset: 0 };
    if (h >= 11 && h < 13) return { slug: "sext", offset: 0 };
    if (h >= 13 && h < 17) return { slug: "none", offset: 0 };
    if (h >= 17 && h < 20) return { slug: "vespers", offset: 0 };
    return { slug: "compline", offset: 0 };
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
    var info = dateSlug === localDateSlug(now) ? currentHourInfo(now) : null;
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

  // markCalendarToday highlights today's row on the ordo page and reveals
  // the header "Today" jump link. Applied client-side because calendar pages
  // are served from the service-worker cache, so a server-rendered marker
  // would freeze on whichever day the page was fetched. The jump link stays
  // hidden when today's row isn't on the displayed year.
  function markCalendarToday() {
    var row = document.getElementById("d-" + localDateSlug(new Date()));
    if (row && row.classList.contains("day")) {
      row.classList.add("is-today");
      row.setAttribute("aria-current", "date");
      var todayLink = document.getElementById("calendar-today-link");
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
