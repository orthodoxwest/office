document.documentElement.classList.add("js");

(function () {
  document.cookie = "tz=" + Intl.DateTimeFormat().resolvedOptions().timeZone + ";path=/;SameSite=Lax";

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
  var recoveryTimer = null;

  var setOfflineIndicator = function (offline) {
    offlineIndicator.hidden = !offline;
    if (offline && !recoveryTimer) {
      recoveryTimer = setInterval(checkOnline, 5000);
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
      }, 2500);
    }

    var done = function () {
      if (timer) {
        clearTimeout(timer);
      }
    };

    fetch(url, options).then(function (resp) {
      done();
      setOfflineIndicator(!resp.ok);
    }).catch(function () {
      done();
      setOfflineIndicator(true);
    });
  }

  window.addEventListener("online", checkOnline);
  window.addEventListener("offline", function () {
    // A definite offline signal: show immediately, then poll for recovery.
    setOfflineIndicator(true);
  });
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) {
      checkOnline();
    }
  });
  checkOnline();

  updatePrayNow();
  markCalendarToday();

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
    navigator.serviceWorker.register("/sw.js").then(function () {
      return navigator.serviceWorker.ready;
    }).then(function (reg) {
      if (reg.active) {
        reg.active.postMessage({ type: "precache" });
      }
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

  // localDateSlug formats a date as YYYY-MM-DD in the device's local timezone.
  function localDateSlug(d) {
    var m = String(d.getMonth() + 1);
    var day = String(d.getDate());
    return d.getFullYear() + "-" + (m.length < 2 ? "0" + m : m) + "-" + (day.length < 2 ? "0" + day : day);
  }

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
})();
