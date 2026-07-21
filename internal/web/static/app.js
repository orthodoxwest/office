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

    var pushSection = document.getElementById("push-section");
    if (pushSection && "serviceWorker" in navigator && "PushManager" in window && "Notification" in window) {
      initPush(pushSection, remindersForm);
    }
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

  // urlB64ToUint8Array converts a base64url VAPID public key to the Uint8Array
  // that PushManager.subscribe expects as its applicationServerKey.
  function urlB64ToUint8Array(base64String) {
    var padding = "=".repeat((4 - (base64String.length % 4)) % 4);
    var base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
    var raw = atob(base64);
    var output = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i++) {
      output[i] = raw.charCodeAt(i);
    }
    return output;
  }

  // buildSchedule reads the reminder form into the push Schedule shape the
  // server stores (mirrors buildFeedURL, but structured rather than a URL).
  // An all-days selection is sent as an empty list, matching the ics "every
  // day" default.
  function buildSchedule(form) {
    var hours = {};
    form.querySelectorAll("input[name=hour]:checked").forEach(function (cb) {
      var t = form.querySelector("input[name=time-" + cb.value + "]");
      if (t && t.value) {
        hours[cb.value] = t.value;
      }
    });
    var days = [];
    form.querySelectorAll("input[name=day]:checked").forEach(function (cb) {
      days.push(cb.value);
    });
    if (days.length === 7) {
      days = [];
    }
    var alarm = form.querySelector("select[name=alarm]").value;
    var lead = alarm === "none" ? 0 : parseInt(alarm, 10) || 0;
    var tz = "";
    try {
      tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    } catch (err) {
      // Leave tz empty; the server falls back to UTC.
    }
    return { hours: hours, days: days, leadMinutes: lead, tz: tz };
  }

  function postJSON(url, body) {
    return fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
  }

  // initPush wires the "Push notifications" controls on the reminders page.
  // The section is revealed only when the server reports push is configured
  // (the vapid-public-key endpoint returns 200); otherwise it stays hidden and
  // the calendar-feed flow above remains the only reminder path.
  function initPush(section, form) {
    var statusEl = document.getElementById("push-status");
    var enableBtn = document.getElementById("push-enable");
    var testBtn = document.getElementById("push-test");
    var disableBtn = document.getElementById("push-disable");
    var publicKey = null;

    var setStatus = function (msg) {
      statusEl.textContent = msg;
    };
    var showSubscribed = function (subscribed) {
      testBtn.hidden = !subscribed;
      disableBtn.hidden = !subscribed;
      enableBtn.textContent = subscribed ? "Update schedule" : "Enable notifications";
    };

    fetch("/push/vapid-public-key", { cache: "no-store" }).then(function (resp) {
      if (!resp.ok) {
        throw new Error("push not configured");
      }
      return resp.json();
    }).then(function (data) {
      publicKey = data.publicKey;
      section.hidden = false;
      return navigator.serviceWorker.ready;
    }).then(function (reg) {
      return reg.pushManager.getSubscription();
    }).then(function (sub) {
      if (Notification.permission === "denied") {
        setStatus("Notifications are blocked in your browser settings.");
        enableBtn.disabled = true;
        return;
      }
      showSubscribed(!!sub);
      setStatus(sub ? "Notifications are on for this device." : "Notifications are off.");
    }).catch(function () {
      // Server push disabled or unsupported: leave the section hidden.
    });

    enableBtn.addEventListener("click", function () {
      enableBtn.disabled = true;
      setStatus("Enabling…");
      Notification.requestPermission().then(function (perm) {
        if (perm !== "granted") {
          setStatus("Permission denied. Allow notifications in your browser settings to continue.");
          return;
        }
        return navigator.serviceWorker.ready.then(function (reg) {
          return reg.pushManager.getSubscription().then(function (existing) {
            return existing || reg.pushManager.subscribe({
              userVisibleOnly: true,
              applicationServerKey: urlB64ToUint8Array(publicKey)
            });
          });
        }).then(function (sub) {
          return postJSON("/push/subscribe", {
            subscription: sub.toJSON(),
            schedule: buildSchedule(form)
          }).then(function (resp) {
            if (!resp.ok) {
              throw new Error("subscribe failed");
            }
            showSubscribed(true);
            setStatus("Notifications are on for this device.");
          });
        });
      }).catch(function () {
        setStatus("Could not enable notifications. Please try again.");
      }).then(function () {
        enableBtn.disabled = false;
      });
    });

    testBtn.addEventListener("click", function () {
      testBtn.disabled = true;
      setStatus("Sending a test notification…");
      navigator.serviceWorker.ready.then(function (reg) {
        return reg.pushManager.getSubscription();
      }).then(function (sub) {
        if (!sub) {
          throw new Error("no subscription");
        }
        return postJSON("/push/test", { endpoint: sub.endpoint });
      }).then(function (resp) {
        setStatus(resp.ok
          ? "Test sent — you should see a notification shortly."
          : "Test failed. Try turning notifications off and on again.");
      }).catch(function () {
        setStatus("Test failed. Try turning notifications off and on again.");
      }).then(function () {
        testBtn.disabled = false;
      });
    });

    disableBtn.addEventListener("click", function () {
      disableBtn.disabled = true;
      setStatus("Turning off…");
      navigator.serviceWorker.ready.then(function (reg) {
        return reg.pushManager.getSubscription();
      }).then(function (sub) {
        if (!sub) {
          return;
        }
        var endpoint = sub.endpoint;
        return sub.unsubscribe().then(function () {
          return postJSON("/push/unsubscribe", { endpoint: endpoint });
        });
      }).then(function () {
        showSubscribed(false);
        setStatus("Notifications are off.");
      }).catch(function () {
        setStatus("Could not turn off notifications.");
      }).then(function () {
        disableBtn.disabled = false;
      });
    });
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
