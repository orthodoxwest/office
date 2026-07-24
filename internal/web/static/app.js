document.documentElement.classList.add("js");

(function () {
  document.cookie = "tz=" + Intl.DateTimeFormat().resolvedOptions().timeZone + ";path=/;SameSite=Lax";

  // Appearance: Default (device) / Nave (light) / Apse (dark). Persisted in
  // localStorage so links and the service-worker cache stay theme-free. A
  // matching pre-paint script in layout.html applies the choice before CSS.
  // Only explicit button clicks write storage — passive load never invents a choice.
  var THEME_KEY = "office-theme";
  var THEME_LIGHT_COLOR = "#faf2ec";
  var THEME_DARK_COLOR = "#121c28";

  var readStoredTheme = function () {
    try {
      return localStorage.getItem(THEME_KEY);
    } catch (err) {
      return null;
    }
  };

  var writeStoredTheme = function (value) {
    try {
      localStorage.setItem(THEME_KEY, value);
    } catch (err) {
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

  // User action: paint + persist.
  var applyThemeChoice = function (choice) {
    choice = normalizeThemeChoice(choice) || "default";
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

  // ensureTodayControl injects or rewrites a Today link when the open document
  // is not local today (SWR can leave a "today" page frozen past midnight with
  // ShowToday=false and no server-rendered Today control).
  function ensureTodayControl(today) {
    var docDate = documentDateSlug();
    if (!docDate || docDate === today) {
      return;
    }
    var href = todayHrefForPage(today);
    var existing = document.querySelectorAll("a.today-link");
    if (existing.length) {
      existing.forEach(function (link) {
        link.setAttribute("href", href);
        link.hidden = false;
      });
      return;
    }
    var form = document.querySelector(".date-jump-form");
    if (!form) {
      return;
    }
    var link = document.createElement("a");
    link.className = "today-link";
    link.setAttribute("href", href);
    link.textContent = "Today";
    form.appendChild(link);
  }

  // syncDatedNavigation stamps the top banner (and home prayer card when
  // present) with dated URLs so navigation hits the service-worker precache
  // keys rather than undated /lauds shells. Safe to run on every load: server
  // already emits dated hrefs when NavDate is set; this corrects cached pages
  // and timezone-edge undated markup. Also re-run on visibilitychange after
  // midnight so long-lived tabs regain a Today control.
  function syncDatedNavigation() {
    var today = localDateSlug(new Date());
    var navDate = pageDateSlug();
    var year = navDate.slice(0, 4);

    var brand = document.querySelector('[data-nav="home"]');
    if (brand) {
      brand.setAttribute("href", "/?date=" + navDate);
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

  // The full navigation remains open in the no-JS document. On small screens,
  // collapse it once scripting is available so prayer text gets the viewport.
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
      } catch (err) {
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

  // currentHourSlug returns the office most likely being prayed at the given
  // local time. Boundaries mirror currentHourEntry in handlers.go.
  function currentHourSlug(d) {
    var h = d.getHours();
    if (h >= 5 && h < 7) return "lauds";
    if (h >= 7 && h < 9) return "prime";
    if (h >= 9 && h < 11) return "terce";
    if (h >= 11 && h < 13) return "sext";
    if (h >= 13 && h < 17) return "none";
    if (h >= 17 && h < 20) return "vespers";
    return "compline";
  }

  function setHourCurrent(link, isCurrent) {
    link.classList.toggle("is-current", isCurrent);
    var state = link.querySelector(".home-hour-link-state");
    if (isCurrent) {
      link.setAttribute("aria-current", "page");
      if (!state) {
        state = document.createElement("span");
        state.className = "home-hour-link-state";
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
    var current = dateSlug === localDateSlug(new Date()) ? currentHourSlug(new Date()) : null;
    var prayNow = card.querySelector(".pray-now");
    var matched = false;

    card.querySelectorAll(".home-hour-link").forEach(function (link) {
      var isCurrent = current !== null && link.getAttribute("data-hour") === current;
      setHourCurrent(link, isCurrent);
      if (isCurrent && prayNow) {
        var name = link.querySelector(".home-hour-link-name");
        prayNow.setAttribute("href", link.getAttribute("href"));
        prayNow.textContent = "Pray " + (name ? name.textContent : "Now");
        matched = true;
      }
    });

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

  // Gold hairline under the color band: progress through the hour page.
  if (officeHour) {
    var progress = document.querySelector(".hour-scroll-progress");
    var progressBar = document.querySelector(".hour-scroll-progress-bar");
    if (progress && progressBar) {
      var progressTicking = false;
      var updateHourScrollProgress = function () {
        var scrollTop = window.scrollY || document.documentElement.scrollTop || 0;
        var maxScroll = document.documentElement.scrollHeight - window.innerHeight;
        var ratio = maxScroll <= 0 ? 1 : Math.min(1, Math.max(0, scrollTop / maxScroll));
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
      updateHourScrollProgress();
    }
  }
})();
