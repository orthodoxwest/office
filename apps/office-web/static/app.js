document.documentElement.classList.add("js");

// A feast's page leads back to the search; reached from one, back to that search,
// with the words the reader typed, as the apps' back does.
(function () {
  var back = document.querySelector(".find-back");
  if (!back || !document.referrer) return;
  var from = new URL(document.referrer);
  if (from.origin === location.origin && from.pathname === back.pathname) back.href = from.pathname + from.search;
})();

// A psalm's number and its Latin incipit share a line, a dot between them.
// When the line cannot hold both, the incipit stands on its own line beneath
// and the dot goes: a separator ending a line alone separates nothing. CSS
// cannot see where a line broke, so the labels are measured with the dot in
// place, and re-measured when the measure or the text size changes.
(function () {
  var labels = Array.from(document.querySelectorAll(".item-label > .label-sep")).map(function (sep) { return sep.parentNode; });
  if (!labels.length) return;
  var scheduled = false;
  function measure() {
    scheduled = false;
    labels.forEach(function (label) { label.classList.remove("stacked"); });
    var stacked = labels.filter(function (label) {
      var sep = label.querySelector(".label-sep");
      var incipit = label.querySelector(".psalm-incipit");
      return incipit.getBoundingClientRect().top - sep.getBoundingClientRect().top > 2;
    });
    stacked.forEach(function (label) { label.classList.add("stacked"); });
  }
  function schedule() {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(measure);
  }
  measure();
  window.addEventListener("resize", schedule);
  if ("ResizeObserver" in window) new ResizeObserver(schedule).observe(labels[0].closest(".elements") || document.body);
  if (document.fonts && document.fonts.ready) document.fonts.ready.then(schedule);
})();

// Source capitals at prose openings use real small caps, preserving the text.
// The first letter stays a full capital ("I CONFESS"), the setting a service
// book gives an opening without an ornamented initial; the rest of the run
// takes small caps. Ornamented openings are handled by the enhancement below.
(function () {
  document.querySelectorAll(".liturgical-block > .plain-line:first-child").forEach(function (opening) {
    if (opening.closest(".collect, .chapter")) return;
    var walker = document.createTreeWalker(opening, NodeFilter.SHOW_TEXT);
    var node = walker.nextNode();
    if (!node) return;
    var match = /^(\s*\p{Lu}\p{M}*)([\p{Lu}\p{M}’'-]*(?:\s+[\p{Lu}][\p{Lu}\p{M}’'-]*)*)(?=[\s,;:.!?]|$)/u.exec(node.textContent);
    if (!match || !/\p{Lu}{2}/u.test(match[0])) return;
    // A one-letter first word keeps its space outside the small caps.
    var space = /^\s*/u.exec(match[2])[0].length;
    var word = node.splitText(match[1].length + space);
    word.splitText(match[2].length - space);
    var span = document.createElement("span");
    span.className = "opening-small-caps";
    word.parentNode.insertBefore(span, word);
    span.appendChild(word);
  });
})();

// A rubric term a newcomer stumbles on ("secretly") opens a short note. The
// server marks the term and carries the note in its attributes
// (presentation::gloss); here the term becomes a control, and one note,
// shared by every term, opens above the word so it never covers the prayer
// it explains. The term stays an inline span with a button's role: a real
// <button> lays out as inline-block, which let the rubric's closing stop
// wrap alone onto the next line. A tap elsewhere or Escape closes it.
(function () {
  var terms = document.querySelectorAll(".gloss-term[data-gloss]");
  if (!terms.length) return;
  // Kept in the page and emptied rather than hidden, so a screen reader
  // hears each note as it is written into the live region.
  var note = document.createElement("div");
  note.className = "gloss-note closed";
  note.id = "gloss-note";
  note.setAttribute("role", "status");
  var title = document.createElement("span");
  title.className = "gloss-title";
  var body = document.createElement("p");
  note.append(title, body);
  document.body.append(note);
  var open = null;

  function place() {
    if (!open) return;
    // A term wrapped over two lines points from its first line above, or
    // from its last line below.
    var rects = open.getClientRects();
    if (!rects.length) return;
    var margin = 12;
    var w = note.offsetWidth;
    var h = note.offsetHeight;
    var below = rects[0].top - h - 10 < margin;
    var r = below ? rects[rects.length - 1] : rects[0];
    var mid = r.left + r.width / 2;
    var left = Math.max(margin, Math.min(mid - w / 2, document.documentElement.clientWidth - margin - w));
    note.classList.toggle("below", below);
    note.style.left = left + window.scrollX + "px";
    note.style.top = (below ? r.bottom + 10 : r.top - h - 10) + window.scrollY + "px";
    note.style.setProperty("--gloss-arrow", Math.max(14, Math.min(mid - left, w - 14)) + "px");
  }

  function close(refocus) {
    if (!open) return;
    var was = open;
    open.setAttribute("aria-expanded", "false");
    open = null;
    note.classList.add("closed");
    title.textContent = "";
    body.textContent = "";
    if (refocus) was.focus();
  }

  function show(term) {
    if (open) open.setAttribute("aria-expanded", "false");
    open = term;
    title.textContent = "";
    body.textContent = "";
    term.setAttribute("aria-expanded", "true");
    // Written a frame after it was emptied, so reopening the same term is
    // still a change the live region announces.
    requestAnimationFrame(function () {
      if (open !== term) return;
      title.textContent = term.dataset.glossTitle || "";
      body.textContent = term.dataset.gloss;
      note.classList.remove("closed");
      place();
    });
  }

  function toggle(term) {
    if (open === term) close(false);
    else show(term);
  }

  terms.forEach(function (term) {
    term.setAttribute("role", "button");
    term.setAttribute("tabindex", "0");
    term.setAttribute("aria-expanded", "false");
    term.setAttribute("aria-controls", "gloss-note");
    term.addEventListener("click", function () { toggle(term); });
    term.addEventListener("keydown", function (e) {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        toggle(term);
      }
    });
  });

  // iOS Safari sends no click for a tap on plain text, so a tap elsewhere is
  // heard on pointerup (as the header's overlays hear it); click stays for
  // the keyboard and finds the note already closed after a mouse.
  function outside(e) {
    if (open && !open.contains(e.target) && !note.contains(e.target)) close(false);
  }
  document.addEventListener("pointerup", function (e) {
    if (e.button === 0) outside(e);
  });
  document.addEventListener("click", outside);
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && open) close(true);
  });
  window.addEventListener("resize", place);
  // A text-size choice or a prayer-form switch moves the word under the note.
  if ("ResizeObserver" in window) new ResizeObserver(place).observe(document.body);
  if (document.fonts && document.fonts.ready) document.fonts.ready.then(place);
})();

// Initials have two independent decisions: their place in the prayer's
// hierarchy, and their optical fit. CSS holds the font-specific letter
// profiles; this enhancement identifies the letter and opening word without
// changing the text. Prose can adapt to its measure, while short responses
// stay modest and psalm/metrical/chant openings keep their two-line cap.
// Chapters, collects (including Marian collects), and the corporate Lord's
// Prayer share the prose profile; brief versicles receive no ornament.
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
    // A psalm whose antiphon has just said its opening words begins inside
    // the span that mutes them; the letter and its word are still the first
    // text, and the initial keeps its ornament colour over the muted run.
    while (node && node.nodeType === Node.ELEMENT_NODE && node.classList.contains("unrepeated")) node = node.firstChild;
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

  // Measure the words rather than the paragraph's box: the initial can extend
  // below a single text line, and the mediant sits above the text baseline.
  function textRows(opening) {
    var leading = parseFloat(getComputedStyle(opening).lineHeight);
    var walker = document.createTreeWalker(opening, NodeFilter.SHOW_TEXT);
    var range = document.createRange();
    var rows = [];
    var node;
    while ((node = walker.nextNode())) {
      var start = node === opening.firstChild ?
        (/^\s*\p{L}\p{M}*/u.exec(node.textContent) || [""])[0].length : 0;
      var words = node.textContent.slice(start).matchAll(/\S+/g);
      for (var word of words) {
        range.setStart(node, start + word.index);
        range.setEnd(node, start + word.index + word[0].length);
        for (var rect of range.getClientRects()) {
          if (!rect.width || !rect.height) continue;
          var row = rows.find(function (line) { return Math.abs(line.top - rect.top) < leading / 2; });
          if (!row) {
            row = { top: rect.top, left: rect.left, right: rect.right, words: 0 };
            rows.push(row);
          }
          row.left = Math.min(row.left, rect.left);
          row.right = Math.max(row.right, rect.right);
          row.words++;
        }
      }
    }
    return rows.sort(function (a, b) { return a.top - b.top; });
  }
  function balance(rows) {
    if (rows.length !== 2) return 0;
    var widths = rows.map(function (row) { return row.right - row.left; });
    return Math.min.apply(null, widths) / Math.max.apply(null, widths);
  }

  // Cache width and font metrics, not height: our own line-break changes
  // must not restart the ResizeObserver cycle. Layout writes stay batched.
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
    // Measure psalms in their finished initial size. Short prose still uses
    // its compact setting to decide whether a raised initial is appropriate.
    changed.forEach(function (opening) {
      opening.classList.remove("initial-divided", "initial-elevated");
      opening.classList.toggle("initial-raised", !opening.matches(".psalm-verses .verse"));
    });
    var candidates = [];
    var elevated = [];
    var singleLines = changed.map(function (opening) {
      var leading = parseFloat(getComputedStyle(opening).lineHeight);
      var height = opening.getBoundingClientRect().height;
      if (opening.matches(".psalm-verses .verse") && height <= leading + 1) elevated.push(opening);
      if (opening.matches(".psalm-verses .verse") && opening.querySelector(".mediant") &&
          height > leading * 1.5 && height < leading * 2.5) {
        var rows = textRows(opening);
        // Avoid a one- or two-word tail occupying less than 30% of line one.
        // Ordinary wrapping and genuinely single-line openings need no break.
        if (rows.length === 2 && rows[1].words <= 2 &&
            rows[1].right - rows[1].left < (rows[0].right - rows[0].left) * 0.3) {
          candidates.push({ opening: opening, balance: balance(rows) });
        }
      }
      return height <= leading + 1;
    });
    changed.forEach(function (opening, index) {
      opening.classList.toggle("initial-raised",
        !opening.matches(".psalm-verses .verse") && singleLines[index]);
    });
    // Compare a real alternate setting, keeping the semantic break only if
    // both halves still occupy two lines and their balance clearly improves.
    candidates.forEach(function (candidate) { candidate.opening.classList.add("initial-divided"); });
    var keep = candidates.map(function (candidate) {
      var rows = textRows(candidate.opening);
      return rows.length === 2 && balance(rows) > candidate.balance + 0.15;
    });
    candidates.forEach(function (candidate, index) {
      candidate.opening.classList.toggle("initial-divided", keep[index]);
    });
    // A drop cap needs text beneath its first line. When the whole opening
    // fits, raise the same full-size letter to that line's baseline instead.
    // Use the lower contour's clearance: the old upper-line word tuck would
    // collide with the foot of letters such as A or L in a raised setting.
    elevated.forEach(function (opening) { opening.classList.add("initial-elevated"); });
    var reflowed = elevated.filter(function (opening) {
      return textRows(opening).length > 1 && opening.querySelector(".mediant");
    });
    // Raising can make the first word wider. At that narrow boundary, prefer
    // two complete half-verses beside a dropped initial to a new short tail.
    reflowed.forEach(function (opening) {
      opening.classList.remove("initial-elevated");
      opening.classList.add("initial-divided");
    });
    var dividedFits = reflowed.map(function (opening) { return textRows(opening).length === 2; });
    reflowed.forEach(function (opening, index) {
      if (!dividedFits[index]) {
        opening.classList.remove("initial-divided");
        opening.classList.add("initial-elevated");
      }
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
  window.addEventListener("officemartyrologychange", refresh);
  window.addEventListener("beforeprint", function () {
    measures = new WeakMap();
    typeset();
  });
  window.addEventListener("afterprint", refresh);
})();

// Home's versicle takes only the height the arch's head has to spare. Where
// it would make the card taller, and so push the hours down the screen, the
// head goes without it; measured again whenever the type, the window or the
// head's other lines change.
(function () {
  var versicle = document.querySelector(".home-versicle");
  var card = versicle && versicle.closest(".home-hero");
  if (!card) return;
  var frame = 0;
  // Fitted with "Change date" shut, as Android's and iOS's are: open, the
  // head is held at its folded height (below) or giving it up, and a
  // measure then would show or hide the versicle by the picker's room, to
  // jump when it folds. Measured again once it has folded and let go.
  var picker = card.querySelector("details.home-date-nav");
  function fit() {
    frame = 0;
    if (picker && picker.open) return;
    versicle.hidden = true;
    var without = card.getBoundingClientRect().height;
    versicle.hidden = false;
    versicle.hidden = card.getBoundingClientRect().height > without + 0.5;
  }
  function schedule() {
    if (!frame) frame = window.requestAnimationFrame(fit);
  }
  fit();
  if (document.fonts) document.fonts.ready.then(schedule);
  window.addEventListener("resize", schedule);
  if ("ResizeObserver" in window) new ResizeObserver(schedule).observe(versicle.parentNode);
  if (picker) {
    picker.addEventListener("toggle", function () {
      // The fold's 200ms, and the head let go after it.
      if (!picker.open) window.setTimeout(schedule, 300);
    });
  }
})();

// Appearance and screen dimensions accompany every usage beacon; office pages
// also report the selected prayer form, Prime whether its Martyrology is
// shown, and an hour with a score whether its notation is. These describe how the page
// is being rendered rather than which page it is: the appearance actually on
// screen (Nave or Apse, whether chosen or inherited from the device), and
// whether this is a phone-shaped reading — the 700px layout breakpoint, or a
// coarse pointer, which catches tablets and a phone held in landscape — and
// whether it is a browser tab or the installed web app (the native apps
// report their own). All are read at send time; the server counts each like
// any other scope, once per browser per day.
//
// Each is reported as "family:value", the name it is stored under, so a value
// belongs to exactly one family and a family retired later cannot be confused
// with a value name reused by a different one (see presentation::usage, whose
// app_beacon writes the native apps' beacons in the same order).
// Whether this browser first opened the Office today. It keeps that reporting day locally (a
// browser that used the Office before this was recorded, shown by a stored setting or the
// offline copy, keeps "before"); only first or returning is sent, so nothing links one day's
// beacon to another's. Safari erases this storage after a week away, so the server also keeps
// two dateless flags (usage.rs) that say whether it has counted this browser before. The mark is made on the first page load, counted or not, so a later page
// served by the offline copy is not mistaken for an old reader. Without storage nothing is said.
var USAGE_FIRST_KEY = "office-first-counted";
function usageVisitToken() {
  try {
    var parts = {};
    new Intl.DateTimeFormat("en-CA", { timeZone: "America/New_York", year: "numeric", month: "2-digit", day: "2-digit" })
      .formatToParts(new Date()).forEach(function (part) { parts[part.type] = part.value; });
    var day = parts.year + "-" + parts.month + "-" + parts.day;
    var first = localStorage.getItem(USAGE_FIRST_KEY);
    if (!first) {
      var used = !!(navigator.serviceWorker && navigator.serviceWorker.controller);
      for (var i = 0; !used && i < localStorage.length; i++) used = /^office-/.test(localStorage.key(i) || "");
      first = used ? "before" : day;
      localStorage.setItem(USAGE_FIRST_KEY, first);
    }
    return first === day ? " visit:first" : " visit:returning";
  } catch {
    return "";
  }
}
usageVisitToken();

function usageBeaconBody(scope) {
  var leader = document.documentElement.getAttribute("data-leader") || "private";
  var leaderToken = document.body.classList.contains("page-hour") && ["private", "deacon", "priest"].indexOf(leader) >= 0 ? " prayer-form:" + leader : "";
  var installed = navigator.standalone === true ||
    (!!window.matchMedia && window.matchMedia("(display-mode: standalone), (display-mode: fullscreen)").matches);
  var clientToken = installed ? " client:pwa" : " client:browser";
  // Prime, on a day with a Martyrology reading: whether the reader's setting
  // shows it or leaves the rubric in its place.
  var martyrologyToken = scope === "prime" && document.querySelector("[data-martyrology-variant]")
    ? (document.documentElement.getAttribute("data-martyrology") === "on" ? " martyrology:shown" : " martyrology:hidden")
    : "";
  // An hour with a scored hymn or Marian antiphon: whether the reader's Chant
  // setting shows the notation or the words alone.
  var chantToken = document.body.classList.contains("page-hour") && document.querySelector('[data-chant-variant="on"]')
    ? (document.documentElement.getAttribute("data-chant") === "on" ? " chant:on" : " chant:off")
    : "";
  clientToken = martyrologyToken + chantToken + usageVisitToken() + clientToken;
  if (!window.matchMedia) {
    return scope + leaderToken + clientToken;
  }
  var forced = document.documentElement.getAttribute("data-theme");
  var dark = forced === "dark" ||
    (forced !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  var handheld = window.matchMedia("(max-width: 700px), (pointer: coarse)").matches;
  return scope +
    (dark ? " appearance:apse" : " appearance:nave") +
    (handheld ? " screen:mobile" : " screen:desktop") + leaderToken + clientToken;
}

// A painted rule: a stroke of one pen from (x1, y1) to (x2, y2), horizontal
// or vertical, w wide. Its weight breathes with the hand (seeded, so a line
// is the same on every visit, and on every front: the apps' ruledLine) and
// it lifts to a point at each end. Home rules its frieze, invitation and
// hours with it; the hours rule their way on.
function seeded(seed) {
  var s = seed >>> 0;
  return function () {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
    return s / 4294967296;
  };
}

function rule(x1, y1, x2, y2, w, seed) {
  var r = seeded(seed);
  var horizontal = Math.abs(x2 - x1) >= Math.abs(y2 - y1);
  var length = horizontal ? x2 - x1 : y2 - y1;
  var n = Math.max(6, Math.round(length / 14));
  var phase1 = r() * 6.28, phase2 = r() * 6.28, drift = (r() - 0.5) * 0.5;
  var above = [], below = [];
  for (var i = 0; i <= n; i += 1) {
    var t = i / n, d = t * length;
    // The pen lifts over the last 4.5px at each end.
    var end = Math.min(1, Math.min(d, length - d) / 4.5);
    var weight = w * (0.86 + 0.16 * Math.sin(phase1 + t * 5.1) + 0.08 * Math.sin(phase2 + t * 17)) * (0.35 + 0.65 * end);
    // The line itself barely wanders.
    var c = drift * (t - 0.5) + 0.12 * Math.sin(phase2 + t * 3.3);
    above.push([d, c - weight / 2]);
    below.push([d, c + weight / 2]);
  }
  return "M" + above.concat(below.reverse()).map(function (p) {
    return horizontal ? (x1 + p[0]).toFixed(2) + "," + (y1 + p[1]).toFixed(2) : (x1 + p[1]).toFixed(2) + "," + (y1 + p[0]).toFixed(2);
  }).join("L") + "Z";
}

// plainClickOpensHere: a primary click with no modifier key, on a link that
// opens in this tab (not a new one, not a download).
function plainClickOpensHere(e, link) {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey &&
    (!link.target || link.target === "_self") && !link.hasAttribute("download");
}
(function () {
  document.cookie = "tz=" + Intl.DateTimeFormat().resolvedOptions().timeZone + ";path=/;SameSite=Lax";

  // Appearance: Default (device) / Nave (light) / Apse (dark). Persisted in
  // localStorage so links and the service-worker cache stay theme-free. A
  // matching pre-paint script in layout.html applies the choice before CSS.
  // Only explicit button clicks write storage — passive load never invents a choice.
  var THEME_KEY = "office-theme";
  var THEME_LIGHT_COLOR = "#faf3e9";
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

  // The plaster wall and Apse starfield cannot ease alongside
  // the color crossfade above. Dip these layers to transparent,
  // swap their background images while invisible, then
  // let style.css's opacity transition climb it back to full: the swap
  // itself never renders, so the material fades rather than flashing on or off.
  // Wait on the dip itself rather than its nominal length: a busy frame can
  // start it late, and a swap on a clock would then land mid-fade. The
  // timer only bounds the wait for a dip that stalls; a loaded device can
  // take several frames to start one.
  var VAULT_FADE_MS = 100;
  var VAULT_FADE_LIMIT_MS = 1000;
  var vaultFadeTurn = 0;

  var vaultDipped = function (root) {
    if (!root.getAnimations) {
      return new Promise(function (resolve) {
        setTimeout(resolve, VAULT_FADE_MS);
      });
    }
    // getAnimations flushes style, so the dip's transitions exist by now.
    var dips = root.getAnimations({ subtree: true }).filter(function (anim) {
      return anim.transitionProperty === "opacity";
    });
    return Promise.race([
      Promise.all(dips.map(function (anim) {
        return anim.finished.catch(function () {});
      })),
      new Promise(function (resolve) {
        setTimeout(resolve, VAULT_FADE_LIMIT_MS);
      }),
    ]);
  };

  // User action: paint + persist.
  var applyThemeChoice = function (choice) {
    choice = normalizeThemeChoice(choice) || "default";
    flashThemeTransition();
    var root = document.documentElement;
    if (window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      // Supersede any dip still pending from before the setting changed.
      vaultFadeTurn++;
      root.classList.remove("vault-hidden");
      paintThemeChoice(choice);
      writeStoredTheme(choice);
      return;
    }
    // The wall holds transparent only under .theme-anim, so keep it on until
    // the swap: a slow dip outlasting the crossfade's timer would otherwise
    // bring the wall back before the swap and show it.
    if (themeAnimTimer) {
      clearTimeout(themeAnimTimer);
      themeAnimTimer = null;
    }
    root.classList.add("vault-hidden");
    var turn = ++vaultFadeTurn;
    vaultDipped(root).then(function () {
      if (turn !== vaultFadeTurn) {
        return;
      }
      // The colours crossfade from the swap, so keep .theme-anim for the
      // whole of it even when the dip ran long.
      flashThemeTransition();
      paintThemeChoice(choice);
      writeStoredTheme(choice);
      // Lift the wall a frame after the swap, so the new material is in place
      // while it is still transparent (WebKit reports a pseudo-element's
      // after-change style for a transition not yet started).
      requestAnimationFrame(function () {
        if (turn === vaultFadeTurn) {
          root.classList.remove("vault-hidden");
        }
      });
    });
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

  // User action: paint + persist. The line the reader is at stays where it
  // stands while the words grow or shrink around it (leader.js).
  var applyTextSizeChoice = function (choice) {
    choice = normalizeTextSizeChoice(choice) || "default";
    var paint = function () { paintTextSizeChoice(choice); };
    if (window.officeHoldPlace) window.officeHoldPlace(paint);
    else paint();
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

  // Martyrology at Prime: Off (the default) / On. The same contract as the
  // text size: localStorage only, so every reader's Prime page (and its
  // offline copy) is the same; the pre-paint script in layout.html sets
  // data-martyrology, and style.css shows the rubric or the reading.
  var MARTYROLOGY_KEY = "office-martyrology";

  var paintMartyrologyChoice = function (choice) {
    if (choice === "on") {
      document.documentElement.setAttribute("data-martyrology", "on");
    } else {
      document.documentElement.removeAttribute("data-martyrology");
    }
    document.querySelectorAll(".martyrology-option[data-martyrology-choice]").forEach(function (btn) {
      btn.setAttribute("aria-pressed", btn.getAttribute("data-martyrology-choice") === choice ? "true" : "false");
    });
  };

  var storedMartyrology = null;
  try {
    storedMartyrology = localStorage.getItem(MARTYROLOGY_KEY);
  } catch {
    // Blocked storage: the Martyrology stays off.
  }
  paintMartyrologyChoice(storedMartyrology === "on" ? "on" : "off");

  document.querySelectorAll(".martyrology-option[data-martyrology-choice]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var choice = btn.getAttribute("data-martyrology-choice") === "on" ? "on" : "off";
      paintMartyrologyChoice(choice);
      try {
        localStorage.setItem(MARTYROLOGY_KEY, choice);
      } catch {
        // Private mode / blocked storage — the choice still applies for this page.
      }
      // Prime's usage beacon reports what is now on the page.
      window.dispatchEvent(new Event("officemartyrologychange"));
    });
  });

  // Chant: Off (the default) / On, offered once the data holds a score. The
  // same contract as the Martyrology: localStorage only, so every page (and
  // its offline copy) carries the text and the score alike; the pre-paint
  // script in layout.html sets data-chant, and style.css shows one.
  var CHANT_KEY = "office-chant";

  var paintChantChoice = function (choice) {
    if (choice === "on") {
      document.documentElement.setAttribute("data-chant", "on");
    } else {
      document.documentElement.removeAttribute("data-chant");
    }
    document.querySelectorAll(".chant-option[data-chant-choice]").forEach(function (btn) {
      btn.setAttribute("aria-pressed", btn.getAttribute("data-chant-choice") === choice ? "true" : "false");
    });
  };

  var storedChant = null;
  try {
    storedChant = localStorage.getItem(CHANT_KEY);
  } catch {
    // Blocked storage: chant stays off.
  }
  paintChantChoice(storedChant === "on" ? "on" : "off");

  document.querySelectorAll(".chant-option[data-chant-choice]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var choice = btn.getAttribute("data-chant-choice") === "on" ? "on" : "off";
      paintChantChoice(choice);
      try {
        localStorage.setItem(CHANT_KEY, choice);
      } catch {
        // Private mode / blocked storage — the choice still applies for this page.
      }
      window.dispatchEvent(new Event("officechantchange"));
    });
  });

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
  // network instead of answering from cache. HEAD, so a page view does not
  // download the worker's ten kilobytes only to learn the network is there.
  function checkOnline() {
    var url = "/sw.js?online-check=" + Date.now();
    var options = { cache: "no-store", method: "HEAD" };
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

  // ordoDayURL is the ordo's page for a day's month, at the day's row.
  function ordoDayURL(slug) {
    return "/calendar/" + slug.slice(0, 4) + "/" + slug.slice(5, 7) + "#d-" + slug;
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

  // The page's own kind of page, dated: an hour stays that hour.
  function datedHrefForPage(slug) {
    var path = location.pathname.replace(/\/$/, "");
    var hourMatch = path.match(/^\/(lauds|prime|terce|sext|none|vespers|compline)/);
    if (hourMatch) {
      return "/" + hourMatch[1] + "/" + slug;
    }
    return "/?date=" + slug;
  }

  // ensureTodayControl keeps recovery chrome honest when the open document is
  // not local today. SWR can leave a page that was "today" at render time
  // frozen past midnight with ShowToday=false and no server-rendered control;
  // a long-lived PWA also needs a one-tap path that is not buried under
  // "Change date".
  function ensureTodayControl(today) {
    var docDate = documentDateSlug();
    var onToday = !docDate || docDate === today;
    var href = datedHrefForPage(today);

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
        document.querySelector(".home-day-head .commemorations") ||
        document.querySelector(".home-day-head .home-versicle") ||
        document.querySelector(".home-day-head .penitential") ||
        document.querySelector(".home-day-head .octave-note") ||
        document.querySelector(".home-day-head .feast") ||
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
      // navDate's own month, so #d-DATE exists on the page it opens.
      ordo.setAttribute("href", ordoDayURL(navDate));
    }

    ensureTodayControl(today);

    // "Today" shortcuts always mean local today, not the page's selected day.
    document.querySelectorAll("a.today-link").forEach(function (link) {
      link.setAttribute("href", datedHrefForPage(today));
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
    updateAboutNow();
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
      updateAboutNow();
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

  // A tap outside an overlay puts it away. iOS Safari sends a click only to
  // what it judges tappable (links, controls, elements with their own click
  // listener); a tap on the page's text or the header's bare bar produces no
  // click at all, so a document click listener never hears it there.
  // pointerup reaches every element, and a touch that becomes a scroll ends
  // in pointercancel instead, so it stands for the tap. click stays for the
  // keyboard; a mouse sends both, and the second finds it already closed. A
  // right or middle button sends no click, so its pointerup leaves it open.
  // Safari moves a touch's click onto a control near the finger, so a tap
  // just beside the overlay's own summary closes it on pointerup and then
  // clicks the summary, which would open it again: that click is dropped.
  var onTapOutside = function (overlay, handler) {
    var closedByTap = -1;
    document.addEventListener("pointerup", function (e) {
      closedByTap = e.button === 0 && handler(e) ? e.timeStamp : -1;
    });
    document.addEventListener("click", function (e) {
      var summary = e.target.closest && e.target.closest("summary");
      if (closedByTap >= 0 && e.timeStamp - closedByTap < 1000 && summary && summary.parentElement === overlay) {
        e.preventDefault();
      } else {
        handler(e);
      }
      closedByTap = -1;
    });
  };

  // The header's overlays, at most one open at a time: the phone menu below
  // the breakpoint, the wide header's Settings above it. Each is a <details>,
  // so its [open] holds the state; overlayOpen reads it as one of
  // "menu", "settings" or null, and closeOverlay is the one way out, taken by
  // a tap outside, Escape and crossing the breakpoint (a Settings left open
  // on a narrow screen would otherwise come back when the screen widens).
  // Above the breakpoint the menu's [open] is the desktop nav, not an
  // overlay: it ships closed, desktop CSS shows the nav regardless, and
  // script keeps [open] set there to match disclosure semantics.
  // iOS and Android hold the same three states (Overlay in Model.swift and
  // Chrome.kt).
  var siteMenu = document.querySelector(".site-menu");
  var siteSettings = document.querySelector(".site-settings");
  var narrowScreen = "matchMedia" in window ? window.matchMedia("(max-width: 700px)") : null;
  var overlayOpen = function () {
    if (narrowScreen && narrowScreen.matches) {
      return siteMenu && siteMenu.open ? "menu" : null;
    }
    return siteSettings && siteSettings.open ? "settings" : null;
  };
  var overlayElement = function (overlay) {
    return overlay === "menu" ? siteMenu : overlay === "settings" ? siteSettings : null;
  };
  var closeOverlay = function (focusSummary) {
    var open = overlayElement(overlayOpen());
    if (!open) {
      return false;
    }
    open.removeAttribute("open");
    var summary = focusSummary && open.querySelector("summary");
    if (summary) {
      summary.focus();
    }
    return true;
  };

  if (narrowScreen) {
    var crossBreakpoint = function () {
      if (siteSettings) {
        siteSettings.removeAttribute("open");
      }
      if (siteMenu) {
        if (narrowScreen.matches) {
          siteMenu.removeAttribute("open");
        } else {
          siteMenu.setAttribute("open", "");
        }
      }
    };
    crossBreakpoint();
    if (narrowScreen.addEventListener) {
      narrowScreen.addEventListener("change", crossBreakpoint);
    }
  }

  // A tap outside the open overlay puts it away; taps inside (including the
  // preferences) leave it open.
  [siteMenu, siteSettings].forEach(function (overlay) {
    if (!overlay) {
      return;
    }
    onTapOutside(overlay, function (e) {
      return overlayElement(overlayOpen()) === overlay && !overlay.contains(e.target) && closeOverlay(false);
    });
  });
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      closeOverlay(true);
    }
  });

  // A link in the open phone menu puts the menu away before the page goes,
  // at once rather than with its 160ms fade, so it does not ride the
  // cross-fade over the next page as a ghost. A page restored by Back comes
  // with the menu put away, as a tap outside leaves it.
  if (siteMenu) {
    siteMenu.addEventListener("click", function (e) {
      var link = e.target.closest && e.target.closest("a[href]");
      if (!link || e.defaultPrevented || overlayOpen() !== "menu" || plainClickOpensHere(e, link) === false) {
        return;
      }
      siteMenu.classList.add("is-leaving");
      siteMenu.removeAttribute("open");
    });
    window.addEventListener("pageshow", function () {
      siteMenu.classList.remove("is-leaving");
    });
  }

  // The brand on today's home leads where the reader already is: it puts an
  // open overlay away and does nothing else, as in the apps (AppModel.goHome),
  // rather than reload an identical page. syncDatedNavigation keeps its
  // aria-current to today's home.
  document.addEventListener("click", function (e) {
    var brand = e.target.closest && e.target.closest(".site-brand[aria-current='page']");
    if (brand && !e.defaultPrevented && plainClickOpensHere(e, brand)) {
      e.preventDefault();
      closeOverlay(false);
    }
  });

  // Native disclosures start closed at every width, including without JS.
  // A deliberate bulk action is useful for comparing offices across the year.
  var calendarExpand = document.querySelector(".calendar-expand");
  if (calendarExpand) {
    calendarExpand.hidden = false;
    // Thirty days unfolding together is a page in turmoil: the month fades
    // out, every day opens or closes at once, and the month fades back in,
    // as the apps' ordo does (Chrome.kt's MONTH_OUT_MS and MONTH_IN_MS).
    // Without motion, or before the page has settled, the days simply change.
    var turnAll = null;
    calendarExpand.addEventListener("click", function () {
      // Pressed again mid-turn: the first turn lands at once, and this one
      // starts from it.
      if (turnAll) {
        turnAll();
      }
      var expand = calendarExpand.getAttribute("aria-pressed") !== "true";
      var months = Array.prototype.slice.call(document.querySelectorAll(".month-table"));
      var animated = months.length > 0 && typeof months[0].animate === "function" &&
        document.documentElement.classList.contains("motion-ready") &&
        window.matchMedia && window.matchMedia("(prefers-reduced-motion: no-preference)").matches;
      // The days change with their transitions held off (style.css,
      // .is-turning-all) until the month has faded back in.
      function change() {
        document.querySelectorAll(".day-disclosures details").forEach(function (item) {
          // A day folding on its own lands first, or its fold would undo this.
          if (item.stopFold) {
            item.stopFold();
          }
          item.open = expand;
        });
        calendarExpand.setAttribute("aria-pressed", String(expand));
        calendarExpand.textContent = expand ? "Hide office details" : "Show office details";
      }
      if (!animated) {
        change();
        return;
      }
      var changed = false;
      var fades = [];
      var timer = 0;
      function fade(from, to, options) {
        fades.forEach(function (f) {
          f.cancel();
        });
        fades = months.map(function (month) {
          return month.animate([{ opacity: from }, { opacity: to }], options);
        });
        return fades[0].finished;
      }
      function end() {
        if (!changed) {
          changed = true;
          change();
        }
        document.documentElement.classList.remove("is-turning-all");
        turnAll = null;
        window.clearTimeout(timer);
        fades.forEach(function (f) {
          f.cancel();
        });
      }
      turnAll = end;
      document.documentElement.classList.add("is-turning-all");
      // The fades' 210ms and a little over: a hidden tab may never finish them.
      timer = window.setTimeout(end, 400);
      fade(1, 0, { duration: 70, easing: "cubic-bezier(0.4, 0, 1, 1)", fill: "forwards" }).then(function () {
        if (turnAll !== end) {
          return;
        }
        changed = true;
        change();
        return fade(0, 1, { duration: 140, easing: "cubic-bezier(0, 0, 0.2, 1)" }).then(function () {
          if (turnAll === end) {
            end();
          }
        });
      }).catch(function () {});
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

  // Where the browser cannot ease a height to auto (no interpolate-size:
  // Safari, Firefox), the disclosures' unfold of style.css is turned here
  // instead, so they unfold on every browser as in the apps: the height over
  // 260ms on the standard curve and the contents fading in over 200ms after
  // 60; folding, 200ms and the contents out over 100ms, the disclosure held
  // open, marked .is-folding, until it has folded. Heard on the disclosure,
  // after the summary's own listeners (the prayers' kept choice reads the
  // state before the toggle), and never for a click another listener has
  // taken (home's "Change date" turns its stage itself). An hour's date in
  // the header turns it through details.turnFold, and the ordo's "Show office
  // details" lands a fold mid-turn through details.stopFold before it changes
  // the day. Without motion it is the native toggle.
  var easesToAuto = window.CSS && typeof CSS.supports === "function" && CSS.supports("interpolate-size", "allow-keywords");
  if (!easesToAuto && window.matchMedia && typeof document.documentElement.animate === "function") {
    var foldStillness = window.matchMedia("(prefers-reduced-motion: no-preference)");
    var STANDARD = "cubic-bezier(0.4, 0, 0.2, 1)";
    var folds = new WeakMap();
    // A box's height and edges as they stand, and folded away.
    var EDGES = ["paddingTop", "paddingBottom", "borderTopWidth", "borderBottomWidth", "marginTop", "marginBottom"];
    var tall = function (box) {
      return { height: box.getBoundingClientRect().height + "px" };
    };
    var edges = function (box) {
      var style = window.getComputedStyle(box);
      var at = tall(box);
      EDGES.forEach(function (edge) {
        at[edge] = style[edge];
      });
      return at;
    };
    var folded = function () {
      var at = { height: "0px" };
      EDGES.forEach(function (edge) {
        at[edge] = "0px";
      });
      return at;
    };
    document.querySelectorAll("details.hour-date-nav, details.session-prayers, details.day-office-details, details.reminder-address, details.reminder-help").forEach(function (details) {
      var summary = details.querySelector(":scope > summary");
      if (!summary) {
        return;
      }
      details.stopFold = function () {
        var running = folds.get(details);
        if (running) {
          running.stop();
        }
      };
      // Opens or closes the disclosure on the fold; false where it is left
      // to the native toggle.
      details.turnFold = function () {
        if (!document.documentElement.classList.contains("motion-ready") || !foldStillness.matches) {
          return false;
        }
        var running = folds.get(details);
        var opening = running ? !running.opening : !details.open;
        var contents = Array.prototype.filter.call(details.children, function (child) {
          return child !== summary;
        });
        // Where the disclosure gives its parts to the hour's controls grid
        // (display: contents, style.css) it has no box to fold, and WebKit
        // does not animate its ::details-content: its parts fold instead.
        var unboxed = window.getComputedStyle(details).display === "contents";
        var boxes = unboxed ? contents : [details];
        if (boxes.length === 0) {
          return false;
        }
        var measure = unboxed ? edges : tall;
        var from = boxes.map(measure);
        if (running) {
          running.stop();
        }
        var to;
        if (opening) {
          details.open = true;
          to = boxes.map(measure);
          if (unboxed && !running) {
            from = boxes.map(folded);
          }
        } else if (unboxed) {
          to = boxes.map(folded);
        } else {
          // Folded, the disclosure is its summary and its own edges: measured
          // with the contents set aside.
          var kept = contents.map(function (child) {
            var display = [child.style.getPropertyValue("display"), child.style.getPropertyPriority("display")];
            child.style.setProperty("display", "none", "important");
            return display;
          });
          to = [tall(details)];
          contents.forEach(function (child, i) {
            if (kept[i][0]) {
              child.style.setProperty("display", kept[i][0], kept[i][1]);
            } else {
              child.style.removeProperty("display");
            }
          });
        }
        boxes.forEach(function (box) {
          box.style.boxSizing = "border-box";
          box.style.overflow = "hidden";
          box.style.overflow = "clip";
        });
        details.classList.toggle("is-folding", !opening);
        var duration = opening ? 260 : 200;
        var turns = boxes.map(function (box, i) {
          return box.animate([from[i], to[i]], { duration: duration, easing: STANDARD });
        });
        contents.forEach(function (child) {
          turns.push(opening ?
            child.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 200, delay: 60, easing: "cubic-bezier(0, 0, 0.2, 1)", fill: "backwards" }) :
            child.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 100, easing: "cubic-bezier(0.4, 0, 1, 1)", fill: "forwards" }));
        });
        var timer = 0;
        var fold = {
          opening: opening,
          stop: function () {
            folds.delete(details);
            window.clearTimeout(timer);
            turns.forEach(function (turn) {
              turn.cancel();
            });
            details.classList.remove("is-folding");
            boxes.forEach(function (box) {
              box.style.removeProperty("box-sizing");
              box.style.removeProperty("overflow");
            });
          },
        };
        function land() {
          if (folds.get(details) !== fold) {
            return;
          }
          fold.stop();
          if (!opening) {
            details.open = false;
          }
        }
        folds.set(details, fold);
        // A hidden tab may never finish the turn.
        timer = window.setTimeout(land, duration + 150);
        turns[0].finished.then(land, function () {});
        return true;
      };
      details.addEventListener("click", function (event) {
        if (event.defaultPrevented || !(event.target instanceof Element) || event.target.closest("summary") !== summary) {
          return;
        }
        if (details.turnFold()) {
          event.preventDefault();
        }
      });
    });
  }

  // Registered after load: on a first visit the worker's install fetches the
  // shell, today's hours, and the ordo, and must not compete with this page
  // for its own fonts and images.
  var registerServiceWorker = function () {
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
  };
  if ("serviceWorker" in navigator) {
    if (document.readyState === "complete") {
      registerServiceWorker();
    } else {
      window.addEventListener("load", registerServiceWorker, { once: true });
    }
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
    var copyNoteEl = document.getElementById("reminder-copy-note");
    var addressEl = document.querySelector(".reminder-address");
    var ruleEl = document.getElementById("reminder-rule");
    var ruleWords = {};
    try {
      ruleWords = JSON.parse(document.getElementById("reminder-rule-words").textContent);
    } catch {
      ruleWords = null;
    }
    var stillRule = function () {
      return !window.matchMedia || window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    };
    var feedAddress = "";

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

    // The rule, written out (presentation::reminders, whose tests give the sentences): the
    // hours in the day's order with an Oxford comma from three, the days, how long before.
    var ruleJoint = function (i, n) {
      if (i === n - 1) {
        return "";
      }
      if (n === 2) {
        return " " + ruleWords.and + " ";
      }
      return i === n - 2 ? ", " + ruleWords.and + " " : ", ";
    };
    var ruleList = function (items) {
      return items.map(function (item, i) {
        return item + ruleJoint(i, items.length);
      }).join("");
    };
    var ruleDays = function (days) {
      var chosen = [];
      var left = [];
      days.forEach(function (on, i) {
        (on ? chosen : left).push(ruleWords.days[i]);
      });
      if (chosen.length === 0) {
        return null;
      }
      if (chosen.length === 7) {
        return ruleWords.everyDay;
      }
      if (days.join() === "true,true,true,true,true,false,false") {
        return ruleWords.weekdays;
      }
      return chosen.length <= 3 ? ruleWords.on + " " + ruleList(chosen) : ruleWords.except + " " + ruleList(left);
    };
    // A time in the reader's own clock ("6:45 AM"), held to its figures by a no-break space.
    var ruleClock = (function () {
      var format = null;
      try {
        format = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" });
      } catch {
        format = null;
      }
      return function (value) {
        var parts = value.split(":");
        if (!format || parts.length < 2) {
          return value;
        }
        return format.format(new Date(2026, 0, 1, Number(parts[0]), Number(parts[1]))).replace(/[\s\u202f]+(?=\D+$)/, "\u00a0");
      };
    })();
    var rulePhrases = function (invalidTime) {
      var hours = [];
      remindersForm.querySelectorAll("input[name=hour]:checked").forEach(function (cb) {
        var t = remindersForm.querySelector("input[name=time-" + cb.value + "]");
        hours.push({ slug: cb.value, time: t && t.value ? ruleClock(t.value) : "" });
      });
      var days = Array.from(remindersForm.querySelectorAll("input[name=day]")).map(function (cb) {
        return cb.checked;
      });
      var daysPhrase = ruleDays(days);
      if (hours.length === 0 || daysPhrase === null) {
        var empty = hours.length > 0 ? ruleWords.noDay : days.indexOf(true) >= 0 ? ruleWords.noHour : ruleWords.noHourOrDay;
        return [{ key: "empty", text: empty }];
      }
      if (invalidTime) {
        return [{ key: "empty", text: ruleWords.noTime }];
      }
      var out = [];
      hours.forEach(function (h, i) {
        out.push({ key: h.slug, text: ruleWords.hours[h.slug] });
        out.push({ key: h.slug + "-time", text: " " + ruleWords.at + " " + h.time });
        var joint = ruleJoint(i, hours.length);
        if (joint) {
          out.push({ key: h.slug + "-joint", text: joint });
        }
      });
      var alarm = remindersForm.querySelector("select[name=alarm]").value;
      out.push({ key: "days", text: ", " + daysPhrase });
      out.push({ key: "lead", text: ", " + (ruleWords.leads[alarm] || alarm + " minutes before") + "." });
      return out;
    };
    // Only the phrases whose words changed cross-fade: out in 90ms, in over 180ms after a beat.
    // The rule is a polite, atomic live region, so every change to its words lands in one
    // moment (after the beat) and is read once, whole, rather than phrase by phrase.
    var ruleTimer = 0;
    var paintRule = function (phrases, quiet) {
      if (!ruleEl) {
        return;
      }
      var spans = function () {
        var kept = {};
        ruleEl.querySelectorAll("span[data-key]").forEach(function (span) {
          kept[span.dataset.key] = span;
        });
        return kept;
      };
      var still = quiet || stillRule();
      var apply = function () {
        var kept = spans();
        var prev = null;
        ruleEl.classList.toggle("is-empty", phrases[0].key === "empty");
        phrases.forEach(function (p) {
          var span = kept[p.key];
          delete kept[p.key];
          if (!span) {
            span = document.createElement("span");
            span.dataset.key = p.key;
            span.textContent = p.text;
            if (!still) {
              span.classList.add("is-out");
              window.requestAnimationFrame(function () {
                window.requestAnimationFrame(function () {
                  span.classList.remove("is-out");
                });
              });
            }
          } else {
            if (span.textContent !== p.text) {
              span.textContent = p.text;
            }
            span.classList.remove("is-out");
          }
          // Moved only where it is out of place, so an unchanged phrase is left alone.
          var at = prev ? prev.nextSibling : ruleEl.firstChild;
          if (at !== span) {
            ruleEl.insertBefore(span, at);
          }
          prev = span;
        });
        Object.keys(kept).forEach(function (key) {
          kept[key].remove();
        });
      };
      window.clearTimeout(ruleTimer);
      if (still) {
        apply();
        return;
      }
      // What will change or go fades out now; the new words come in together after the beat.
      var wanted = {};
      phrases.forEach(function (p) {
        wanted[p.key] = p.text;
      });
      var kept = spans();
      var same = Object.keys(kept).length === phrases.length;
      Object.keys(kept).forEach(function (key) {
        if (wanted[key] !== kept[key].textContent) {
          kept[key].classList.add("is-out");
          same = false;
        }
      });
      if (same) {
        apply();
        return;
      }
      ruleTimer = window.setTimeout(apply, 150);
    };

    // The calendar address, breakable after its slashes and joints rather than anywhere.
    var setAddress = function (text) {
      urlEl.textContent = "";
      text.replace(/([/?&])/g, "$1\n").split("\n").forEach(function (piece, i) {
        if (i > 0) {
          urlEl.appendChild(document.createElement("wbr"));
        }
        urlEl.appendChild(document.createTextNode(piece));
      });
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
      remindersForm.querySelectorAll(".reminder-day").forEach(function (day) {
        var checkbox = day.querySelector("input");
        day.classList.toggle("is-selected", Boolean(checkbox && checkbox.checked));
      });
    };

    var copiedTimer = 0;
    var settleCopied = function () {
      window.clearTimeout(copiedTimer);
      copyBtn.classList.remove("is-copied");
      copiedEl.textContent = "";
    };

    var update = function (quiet) {
      syncReminderHourRows();
      var selectedHours = remindersForm.querySelectorAll("input[name=hour]:checked");
      var selectedDays = remindersForm.querySelectorAll("input[name=day]:checked");
      var invalidTime = Array.from(selectedHours).some(function (checkbox) {
        var time = remindersForm.querySelector("input[name=time-" + checkbox.value + "]");
        return !time || !time.validity.valid;
      });
      var valid = selectedHours.length > 0 && selectedDays.length > 0 && !invalidTime;
      var phrases = ruleWords ? rulePhrases(invalidTime) : [{ key: "empty", text: "" }];
      paintRule(phrases, quiet === true);
      feedAddress = valid ? "https://" + buildFeedURL() : "";
      setAddress(valid ? feedAddress : phrases[0].text);
      webcalEl.classList.toggle("is-disabled", !valid);
      webcalEl.setAttribute("aria-disabled", valid ? "false" : "true");
      if (!valid) {
        webcalEl.removeAttribute("href");
        webcalEl.setAttribute("tabindex", "-1");
      } else {
        webcalEl.href = "webcal://" + feedAddress.slice("https://".length);
        webcalEl.removeAttribute("tabindex");
      }
      copyBtn.disabled = !valid;
      settleCopied();
      copyNoteEl.hidden = true;
    };

    remindersForm.addEventListener("change", update);
    // A tap on an unchosen hour's greyed time chooses the hour (the field lets the tap through).
    remindersForm.addEventListener("click", function (event) {
      var cell = event.target.closest && event.target.closest(".reminder-time");
      var checkbox = cell && cell.closest(".reminder-hour-row").querySelector('input[name="hour"]');
      if (!checkbox || checkbox.checked) {
        return;
      }
      checkbox.checked = true;
      checkbox.dispatchEvent(new Event("change", { bubbles: true }));
    });
    update(true);

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
        copyNoteEl.textContent = "Copy unavailable. The calendar address is shown below.";
        copyNoteEl.hidden = false;
        if (addressEl) {
          addressEl.open = true;
        }
      };
      if (!navigator.clipboard || typeof navigator.clipboard.writeText !== "function") {
        showCopyFallback();
        return;
      }
      navigator.clipboard.writeText(feedAddress).then(function () {
        // The link's own words say so for a moment, and nothing beside it moves.
        settleCopied();
        copyBtn.classList.add("is-copied");
        copiedEl.textContent = "Copied.";
        copiedTimer = window.setTimeout(settleCopied, 2400);
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

  // The date picker. Pages without script keep the native date field, but
  // its popup is the platform's own (a spinning wheel, a system sheet) and
  // belongs to no room here. With script, "Change date" unfolds a month set
  // like a printed kalendar instead: Sundays in red, the page's day
  // underlined in gold as chosen things are elsewhere, today in the ordo's
  // wash, and every day a link to that day's page.
  //
  // Keyboard follows the grid pattern: one day is in the tab order, arrows
  // move by day and week, Page Up/Down by month (with Shift, by year), Home
  // and End to the week's ends. The title turns the grid into the year's
  // months. Escape steps back out: months to days, days to the summary.
  var MONTH_NAMES = [
    "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December",
  ];
  var WEEKDAY_NAMES = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
  // The picker offers a working span of years; the native field's min and
  // max in the templates match it.
  var PICKER_FIRST_YEAR = 1950;
  var PICKER_LAST_YEAR = 2150;

  // Dates are UTC midnights built with setUTCFullYear, which neither maps
  // years 0-99 onto the 1900s nor shifts across daylight saving, and which
  // carries overflowing days and months into the next month or year.
  function civilDate(year, month, day) {
    var date = new Date(0);
    date.setUTCFullYear(year, month, day);
    return date;
  }

  function parseSlug(slug) {
    var m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(slug || "");
    return m ? civilDate(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : null;
  }

  function civilSlug(date) {
    var pad = function (n, width) {
      var text = String(n);
      while (text.length < width) {
        text = "0" + text;
      }
      return text;
    };
    return pad(date.getUTCFullYear(), 4) + "-" + pad(date.getUTCMonth() + 1, 2) + "-" + pad(date.getUTCDate(), 2);
  }

  // The same day of another month, or that month's last day when it is
  // shorter.
  function shiftMonths(date, months) {
    var first = civilDate(date.getUTCFullYear(), date.getUTCMonth() + months, 1);
    var last = civilDate(first.getUTCFullYear(), first.getUTCMonth() + 1, 0).getUTCDate();
    return civilDate(first.getUTCFullYear(), first.getUTCMonth(), Math.min(date.getUTCDate(), last));
  }

  function inPickerRange(date) {
    var year = date.getUTCFullYear();
    return year >= PICKER_FIRST_YEAR && year <= PICKER_LAST_YEAR;
  }

  // A page dated outside the span (typed into the address) opens the picker
  // at the nearer end, so its controls can still move.
  function clampToPickerRange(date) {
    if (date.getUTCFullYear() < PICKER_FIRST_YEAR) {
      return civilDate(PICKER_FIRST_YEAR, 0, 1);
    }
    if (date.getUTCFullYear() > PICKER_LAST_YEAR) {
      return civilDate(PICKER_LAST_YEAR, 11, 31);
    }
    return date;
  }

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) {
      node.className = className;
    }
    if (text) {
      node.textContent = text;
    }
    return node;
  }

  function buildDatePicker(details) {
    var picker = el("div", "date-picker");
    picker.setAttribute("role", "group");
    picker.setAttribute("aria-label", "Choose a date");

    var head = el("div", "date-picker-head");
    var back = el("button", "date-picker-step", "\u2039");
    var title = el("button", "date-picker-title");
    var forward = el("button", "date-picker-step", "\u203a");
    [back, title, forward].forEach(function (button) {
      button.type = "button";
      head.appendChild(button);
    });
    var announce = el("span", "sr-only");
    announce.setAttribute("aria-live", "polite");
    head.appendChild(announce);

    var body = el("div", "date-picker-body");
    var grid = el("table", "date-picker-days");
    grid.setAttribute("role", "grid");
    var headRow = el("tr");
    WEEKDAY_NAMES.forEach(function (name) {
      var th = el("th");
      th.scope = "col";
      var abbr = el("abbr", "", name.charAt(0));
      abbr.title = name;
      th.appendChild(abbr);
      headRow.appendChild(th);
    });
    var thead = el("thead");
    thead.appendChild(headRow);
    var weeks = el("tbody");
    grid.appendChild(thead);
    grid.appendChild(weeks);

    var months = el("div", "date-picker-months");
    months.setAttribute("role", "grid");
    months.setAttribute("aria-label", "Months");
    for (var row = 0; row < 4; row++) {
      var monthRow = el("div");
      monthRow.setAttribute("role", "row");
      for (var col = 0; col < 3; col++) {
        var index = row * 3 + col;
        var cell = el("div");
        cell.setAttribute("role", "gridcell");
        var month = el("button", "date-picker-month", MONTH_NAMES[index].slice(0, 3));
        month.type = "button";
        month.setAttribute("data-month", String(index));
        cell.appendChild(month);
        monthRow.appendChild(cell);
      }
      months.appendChild(monthRow);
    }
    body.appendChild(grid);
    body.appendChild(months);
    picker.appendChild(head);
    picker.appendChild(body);

    // `focus` is the one day (or month) in the tab order; the view shows its
    // month (or year).
    var state = { view: "days", focus: null, chosen: null, today: null };

    function reset() {
      state.today = localDateSlug(new Date());
      state.chosen = pageDateSlug();
      state.focus = clampToPickerRange(parseSlug(state.chosen) || parseSlug(state.today));
      state.view = "days";
    }

    function titleText() {
      var year = state.focus.getUTCFullYear();
      return state.view === "days" ? MONTH_NAMES[state.focus.getUTCMonth()] + " " + year : String(year);
    }

    // What the grid shows, so an opening that would show the same again
    // leaves it alone rather than rebuilding it under the unfolding.
    var drawn = null;

    function drawing() {
      return [state.view, civilSlug(state.focus), state.chosen, state.today].join(" ");
    }

    function render() {
      drawn = drawing();
      picker.setAttribute("data-view", state.view);
      var days = state.view === "days";
      title.textContent = titleText();
      title.setAttribute("aria-label", titleText() + (days ? ", choose another month" : ", back to days"));
      back.setAttribute("aria-label", days ? "Previous month" : "Previous year");
      forward.setAttribute("aria-label", days ? "Next month" : "Next year");
      var step = days ? 1 : 12;
      back.disabled = !inPickerRange(shiftMonths(civilDate(state.focus.getUTCFullYear(), state.focus.getUTCMonth(), 1), -step));
      forward.disabled = !inPickerRange(shiftMonths(civilDate(state.focus.getUTCFullYear(), state.focus.getUTCMonth(), 1), step));
      grid.setAttribute("aria-label", MONTH_NAMES[state.focus.getUTCMonth()] + " " + state.focus.getUTCFullYear());
      renderDays();
      renderMonths();
    }

    // Six weeks always, blank where the month is not, so paging through
    // months never moves what lies below.
    function renderDays() {
      var year = state.focus.getUTCFullYear();
      var month = state.focus.getUTCMonth();
      var lead = civilDate(year, month, 1).getUTCDay();
      var length = civilDate(year, month + 1, 0).getUTCDate();
      var focusSlug = civilSlug(state.focus);
      weeks.textContent = "";
      for (var week = 0; week < 6; week++) {
        var tr = el("tr");
        for (var weekday = 0; weekday < 7; weekday++) {
          var td = el("td");
          var day = week * 7 + weekday - lead + 1;
          if (day >= 1 && day <= length) {
            var date = civilDate(year, month, day);
            var slug = civilSlug(date);
            var link = el("a", "date-picker-day");
            link.href = datedHrefForPage(slug);
            link.setAttribute("data-date", slug);
            link.tabIndex = slug === focusSlug ? 0 : -1;
            link.setAttribute(
              "aria-label",
              WEEKDAY_NAMES[weekday] + ", " + MONTH_NAMES[month] + " " + day + ", " + year,
            );
            link.appendChild(el("span", "", String(day)));
            if (slug === state.today) {
              link.setAttribute("aria-current", "date");
            }
            if (slug === state.chosen) {
              td.setAttribute("aria-selected", "true");
            }
            td.appendChild(link);
          }
          tr.appendChild(td);
        }
        weeks.appendChild(tr);
      }
    }

    function renderMonths() {
      var year = state.focus.getUTCFullYear();
      var chosen = parseSlug(state.chosen);
      var today = parseSlug(state.today);
      months.querySelectorAll(".date-picker-month").forEach(function (button) {
        var month = Number(button.getAttribute("data-month"));
        button.tabIndex = month === state.focus.getUTCMonth() ? 0 : -1;
        button.setAttribute("aria-label", MONTH_NAMES[month] + " " + year);
        var holds = function (date) {
          return date && date.getUTCFullYear() === year && date.getUTCMonth() === month;
        };
        if (holds(chosen)) {
          button.parentNode.setAttribute("aria-selected", "true");
        } else {
          button.parentNode.removeAttribute("aria-selected");
        }
        if (holds(today)) {
          button.setAttribute("aria-current", "date");
        } else {
          button.removeAttribute("aria-current");
        }
      });
    }

    function focusCurrent() {
      var target =
        state.view === "days"
          ? weeks.querySelector('[data-date="' + civilSlug(state.focus) + '"]')
          : months.querySelector('[data-month="' + state.focus.getUTCMonth() + '"]');
      if (target) {
        target.focus();
      }
    }

    // Moves the roving focus, re-rendering when it leaves the shown month
    // or year. Stops at the calendar's ends rather than wrapping.
    function moveTo(date, keepFocus) {
      if (!inPickerRange(date)) {
        return;
      }
      var shown = titleText();
      state.focus = date;
      render();
      if (titleText() !== shown) {
        announce.textContent = titleText();
      }
      if (!keepFocus) {
        focusCurrent();
      }
    }

    function stepBy(direction) {
      moveTo(shiftMonths(state.focus, direction * (state.view === "days" ? 1 : 12)), true);
    }

    back.addEventListener("click", function () {
      stepBy(-1);
    });
    forward.addEventListener("click", function () {
      stepBy(1);
    });
    title.addEventListener("click", function () {
      state.view = state.view === "days" ? "months" : "days";
      render();
      announce.textContent = titleText();
    });

    months.addEventListener("click", function (event) {
      var button = event.target.closest(".date-picker-month");
      if (!button) {
        return;
      }
      state.view = "days";
      var target = shiftMonths(state.focus, Number(button.getAttribute("data-month")) - state.focus.getUTCMonth());
      moveTo(target);
      announce.textContent = titleText();
    });

    grid.addEventListener("keydown", function (event) {
      var date = state.focus;
      var d = date.getUTCDate();
      var y = date.getUTCFullYear();
      var m = date.getUTCMonth();
      var next;
      switch (event.key) {
        case "ArrowLeft":
          next = civilDate(y, m, d - 1);
          break;
        case "ArrowRight":
          next = civilDate(y, m, d + 1);
          break;
        case "ArrowUp":
          next = civilDate(y, m, d - 7);
          break;
        case "ArrowDown":
          next = civilDate(y, m, d + 7);
          break;
        case "Home":
          next = civilDate(y, m, d - date.getUTCDay());
          break;
        case "End":
          next = civilDate(y, m, d + 6 - date.getUTCDay());
          break;
        case "PageUp":
          next = shiftMonths(date, event.shiftKey ? -12 : -1);
          break;
        case "PageDown":
          next = shiftMonths(date, event.shiftKey ? 12 : 1);
          break;
        case " ":
          // Links answer Enter natively; a grid cell answers Space too.
          event.preventDefault();
          event.target.closest(".date-picker-day").click();
          return;
        default:
          return;
      }
      event.preventDefault();
      moveTo(next);
    });

    months.addEventListener("keydown", function (event) {
      var shift = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -3, ArrowDown: 3, PageUp: -12, PageDown: 12 }[event.key];
      if (!shift) {
        return;
      }
      event.preventDefault();
      moveTo(shiftMonths(state.focus, shift));
    });

    // A day or month keeps its place in the tab order when clicked or
    // tapped, so Tab back into the grid returns to it.
    grid.addEventListener("focusin", function (event) {
      var link = event.target.closest(".date-picker-day");
      if (link && link.getAttribute("data-date") !== civilSlug(state.focus)) {
        state.focus = parseSlug(link.getAttribute("data-date"));
        weeks.querySelectorAll(".date-picker-day").forEach(function (other) {
          other.tabIndex = other === link ? 0 : -1;
        });
      }
    });

    picker.addEventListener("keydown", function (event) {
      if (event.key !== "Escape") {
        return;
      }
      event.preventDefault();
      if (state.view === "months") {
        state.view = "days";
        render();
        title.focus();
        return;
      }
      // Through the control, so the fold is the one a tap makes: the
      // summary, or on an hour the date that stands for it.
      var control = details.dateControl || details.querySelector("summary");
      control.click();
      control.focus();
    });

    // Each opening starts from the page's own day, and from the current
    // local date: a tab left open overnight must not mark yesterday.
    details.addEventListener("toggle", function () {
      if (details.open) {
        reset();
        if (drawing() !== drawn) {
          render();
        }
      }
    });

    reset();
    render();
    return picker;
  }

  document.querySelectorAll("details.hour-date-nav").forEach(function (details) {
    var nav = details.querySelector(".day-nav");
    if (!nav) {
      return;
    }
    nav.insertAdjacentElement("afterend", buildDatePicker(details));
    details.classList.add("has-date-picker");
  });

  // On an hour the date in the header opens the picker, and the disclosure's
  // "Change date" summary stays hidden (style.css): the header is quieter
  // while praying, and home keeps the labelled control. The date is set as
  // text, not drawn as a button, but it is one to a screen reader, which
  // hears that it changes the date and whether the picker is open.
  document.querySelectorAll(".hour-header .hour-meta-date").forEach(function (date) {
    var details = document.getElementById("hour-date-nav");
    if (!details || !details.classList.contains("has-date-picker")) {
      return;
    }
    var button = el("button", date.className);
    button.type = "button";
    button.setAttribute("aria-controls", details.id);
    button.textContent = date.textContent;
    button.setAttribute("aria-label", date.textContent + ", change date");
    var sync = function () {
      button.setAttribute("aria-expanded", details.open ? "true" : "false");
    };
    // On the disclosures' fold where script turns it (app.js, turnFold).
    button.addEventListener("click", function () {
      if (!(details.turnFold && details.turnFold())) {
        details.open = !details.open;
      }
    });
    details.addEventListener("toggle", sync);
    sync();
    details.dateControl = button;
    date.replaceWith(button);
  });

  // Home's "Change date" turns the stage under the band, the invitation and
  // the hours, into the calendar in their place, and back: the day's turn
  // and the picker move out of the disclosure, which keeps its summary as
  // the control, into the stage beside the hours (style.css, "Change date
  // turns the stage"). The turn is one motion on the disclosures' curve:
  //
  // - the stage eases from the hours' height to the calendar's, the hours
  //   fading out and the calendar in;
  // - the head above the day gives up the difference where its spare
  //   height holds it, so nothing scrolls;
  // - where it does not, the head keeps its height, the card lengthens by
  //   the difference, and the page rises with the card's foot frame by
  //   frame, to rest with it and its gutter in view. Pressing the arch down
  //   first and then scrolling for the rest read as two jerks.
  //
  // Folding is the reverse. Without motion (reduced motion, or before the
  // page has settled) the faces change at once and the page is brought to
  // the card's foot at once. Android's and iOS's frontispieces turn the same
  // way (HomeScreen).
  document.querySelectorAll(".page-home .home-hero").forEach(function (hero) {
    var details = hero.querySelector("details.home-date-nav");
    var stage = hero.querySelector(".home-stage");
    var hours = stage && stage.querySelector(".home-hours-face");
    var head = hero.querySelector(".home-summary");
    var summary = details && details.querySelector(":scope > summary");
    var nav = details && details.querySelector(".day-nav");
    var picker = details && details.querySelector(".date-picker");
    if (!stage || !hours || !head || !summary || !nav || !picker || !window.matchMedia) {
      return;
    }
    var chooser = document.createElement("div");
    // has-date-picker: the native field stays behind for no-script visitors only.
    chooser.className = "home-chooser has-date-picker";
    chooser.id = "home-chooser";
    chooser.appendChild(nav);
    chooser.appendChild(picker);
    stage.appendChild(chooser);
    details.classList.add("has-chooser");
    summary.setAttribute("aria-controls", chooser.id);

    var stillness = window.matchMedia("(prefers-reduced-motion: no-preference)");
    // The open the page is following, so a later turn stops it.
    var following = 0;
    // The head held at its folded height while the calendar is shown.
    var held = false;
    // The turn under way: its end, run early when turned again.
    var turning = null;

    function show(open) {
      hero.classList.toggle("is-choosing", open);
      chooser.inert = !open;
      hours.inert = open;
    }

    function sizes() {
      return {
        stage: stage.getBoundingClientRect().height,
        head: head.getBoundingClientRect().height,
        hero: hero.getBoundingClientRect().height,
      };
    }

    function hold(height) {
      held = true;
      head.style.flexGrow = "0";
      head.style.minHeight = height + "px";
    }

    function release() {
      held = false;
      head.style.removeProperty("flex-grow");
      head.style.removeProperty("min-height");
    }

    // Brings the card's foot, and the gutter it rests on, into view: each
    // frame of the turn by as much as it has passed the screen's foot, or
    // at once without motion.
    function follow(gap, animated) {
      var run = ++following;
      function overflow() {
        return hero.getBoundingClientRect().bottom + gap - window.innerHeight;
      }
      if (!animated) {
        var by = overflow();
        if (by > 0.5) {
          window.scrollBy(0, by);
        }
        return;
      }
      // The turn's 260ms, and a frame or two to see it settle.
      var until = performance.now() + 300;
      function step(now) {
        if (run !== following || !details.open) {
          return;
        }
        var over = overflow();
        if (over > 0.5) {
          window.scrollBy(0, over);
        }
        if (now < until) {
          window.requestAnimationFrame(step);
        }
      }
      window.requestAnimationFrame(step);
    }

    function turn(open) {
      if (turning) {
        turning();
      }
      following++;
      var animated = document.documentElement.classList.contains("motion-ready") &&
        stillness.matches && typeof stage.animate === "function";
      // The gutter under the card at rest, which the open keeps; a phone's
      // where the card stood past the screen's foot already (Android's Gutter).
      var rest = window.innerHeight - hero.getBoundingClientRect().bottom;
      var gap = rest > 0 && rest <= 32 ? rest : 16;
      var from = sizes();
      var wasHeld = held;
      release();
      details.open = open;
      show(open);
      var to = sizes();
      if (open && to.hero > from.hero + 0.5) {
        // The calendar lengthens the card even with the head given up:
        // the head keeps its height, and the page rises instead.
        hold(from.head);
      } else if (!open && wasHeld) {
        // Held, the head is already its folded height; let it go once the
        // stage has turned back.
        hold(from.head);
      }
      if (!animated) {
        if (!open) {
          release();
        } else {
          follow(gap, false);
        }
        return;
      }
      var outgoing = open ? hours : chooser;
      var incoming = open ? chooser : hours;
      stage.classList.add("is-turning");
      stage.style.height = from.stage + "px";
      void stage.offsetHeight;
      // The disclosures' duration and Material's standard curve, as the
      // apps' unfolding.
      stage.style.transition = "height 260ms cubic-bezier(0.4, 0, 0.2, 1)";
      stage.style.height = to.stage + "px";
      var fades = [
        outgoing.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 120, easing: "ease-in", fill: "both" }),
        incoming.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 200, delay: 60, easing: "ease-out", fill: "both" }),
      ];
      var timer = 0;
      function end() {
        turning = null;
        window.clearTimeout(timer);
        stage.removeEventListener("transitionend", ended);
        fades.forEach(function (fade) {
          fade.cancel();
        });
        stage.classList.remove("is-turning");
        stage.style.removeProperty("transition");
        stage.style.removeProperty("height");
        if (!details.open && held) {
          release();
        }
      }
      function ended(event) {
        if (event.target === stage && event.propertyName === "height") {
          end();
        }
      }
      turning = end;
      stage.addEventListener("transitionend", ended);
      // transitionend never comes where nothing moved, or in a hidden tab.
      timer = window.setTimeout(end, 400);
      if (open) {
        follow(gap, true);
      }
    }

    // The summary toggles the disclosure itself, so the stage turns in the
    // same frame and nothing of the other state is ever painted.
    summary.addEventListener("click", function (event) {
      event.preventDefault();
      turn(!details.open);
    });

    // Turned while settling (a resize) the stage and head start again from
    // the page's own layout.
    window.addEventListener("resize", function () {
      if (turning) {
        turning();
      }
      if (held && !details.open) {
        release();
      }
    });

    show(details.open);
  });

  // The home panel's courses are clipped to its pointed head by --arch-w,
  // the card's width, which style.css gives in container units. An element
  // styled in container units is restyled and laid out again whenever
  // anything in its container moves, so every frame of "Change date"
  // unfolding re-laid every course of the head and its long clip-paths
  // (some 6ms a frame on a desktop, several times that on a phone). The same
  // width in pixels, kept here as the card's width changes, costs nothing
  // between those changes; without script the container units stand.
  document.querySelectorAll(".page-home .home-hero").forEach(function (hero) {
    if (!("ResizeObserver" in window)) {
      return;
    }
    new ResizeObserver(function (entries) {
      var entry = entries[entries.length - 1];
      var box = entry.contentBoxSize && entry.contentBoxSize[0];
      var width = box ? box.inlineSize : entry.contentRect.width;
      hero.style.setProperty("--arch-cqi", width + "px");
    }).observe(hero);
  });

  // Home: the painted page (style.css). Two things CSS cannot draw.
  //
  // The rules: the frieze's oxblood edges and leaf fillets, the invitation's
  // double frame (the lining, and inside it the day's colour stopping short
  // of the corners), and the hours' table, ruled above and below and between
  // the periods, none upright. Each is a stroke of one pen: its weight
  // breathes with the hand (seeded, so a line is the same on every visit)
  // and it lifts to a point at each end. A host so ruled carries .is-ruled
  // and gives up the CSS borders that stand without script.
  //
  // The stars: by night the head of the arch is the parish apse's painted
  // sky, on a phone and in the wide niche. Its gold-leaf stars are one fixed field for each head
  // (tools/genstars.py, ornaments/stars.json, fetched only when the sky is
  // shown), and the painter keeps them off the words: every star that would
  // touch a laid-out line of the day block (14px either side, 6px above and
  // below), the cross, or the 9px above the frieze is left out. Measured
  // again whenever the head's lines can move.
  document.querySelectorAll(".page-home .home-hero").forEach(function (hero) {
    var NS = "http://www.w3.org/2000/svg";
    var frieze = hero.querySelector(".home-prayer-card h2");
    var prayNow = hero.querySelector(".pray-now");
    var table = hero.querySelector(".home-hour-links");
    var sky = hero.querySelector(".home-sky");
    var head = hero.querySelector(".home-day-head");
    var crown = hero.querySelector(".home-crown");

    // The wide niche's zoom (--niche-zoom), 1 on a phone.
    function heroZoom() {
      return parseFloat(getComputedStyle(hero).zoom) || 1;
    }

    function layer(host) {
      var svg = host.querySelector(":scope > svg.home-rules");
      if (!svg) {
        svg = document.createElementNS(NS, "svg");
        svg.setAttribute("class", "home-rules");
        svg.setAttribute("aria-hidden", "true");
        svg.setAttribute("focusable", "false");
        host.appendChild(svg);
        host.classList.add("is-ruled");
      }
      svg.textContent = "";
      return svg;
    }

    function stroke(svg, cls, d) {
      var path = document.createElementNS(NS, "path");
      path.setAttribute("class", cls);
      path.setAttribute("d", d);
      svg.appendChild(path);
    }

    function ruleAll() {
      var w, h, svg;
      if (frieze) {
        w = frieze.offsetWidth;
        h = frieze.offsetHeight;
        svg = layer(frieze);
        stroke(svg, "r-edge", rule(0, 0.6, w, 0.6, 1.3, 11));
        stroke(svg, "r-edge", rule(0, h - 0.6, w, h - 0.6, 1.3, 12));
        stroke(svg, "r-fillet", rule(0, 2.2, w, 2.2, 0.9, 13));
        stroke(svg, "r-fillet", rule(0, h - 2.2, w, h - 2.2, 0.9, 14));
      }
      if (prayNow) {
        var g = 3.5;
        w = prayNow.offsetWidth;
        h = prayNow.offsetHeight;
        svg = layer(prayNow);
        stroke(svg, "r-lining", rule(0, 0.5, w, 0.5, 1.25, 21));
        stroke(svg, "r-lining", rule(0, h - 0.5, w, h - 0.5, 1.25, 22));
        stroke(svg, "r-lining", rule(0.5, 0, 0.5, h, 1.25, 23));
        stroke(svg, "r-lining", rule(w - 0.5, 0, w - 0.5, h, 1.25, 24));
        // The frontal, finer, stopping short of the corners.
        stroke(svg, "r-day", rule(g + 1.5, g, w - g - 1.5, g, 0.7, 25));
        stroke(svg, "r-day", rule(g + 1.5, h - g, w - g - 1.5, h - g, 0.7, 26));
        stroke(svg, "r-day", rule(g, g + 1.5, g, h - g - 1.5, 0.7, 27));
        stroke(svg, "r-day", rule(w - g, g + 1.5, w - g, h - g - 1.5, 0.7, 28));
      }
      if (table) {
        w = table.offsetWidth;
        h = table.offsetHeight;
        var top = table.getBoundingClientRect().top;
        var zoomed = heroZoom() !== 1;
        svg = layer(table);
        stroke(svg, "r-frame", rule(0, 0.5, w, 0.5, 1.3, 31));
        stroke(svg, "r-frame", rule(0, h - 0.5, w, h - 0.5, 1.3, 32));
        // Each period's top in the table's own pixels, which are the SVG's.
        // The wide niche is zoomed and a client rect is measured on the
        // glass, so a rule placed from one drifted down the table by the
        // zoom: there the group's offsetTop (its offset parent is the
        // table) places it. The phone's panel is not zoomed, and keeps the
        // client rect's fraction of a pixel.
        Array.prototype.slice.call(table.querySelectorAll(".home-hour-group"), 1).forEach(function (group, i) {
          var y = zoomed ? group.offsetTop + 0.5 : group.getBoundingClientRect().top - top + 0.5;
          stroke(svg, "r-inner", rule(0, y, w, y, 0.9, 41 + i));
        });
      }
    }

    var fields = null;
    var loading = false;
    function load() {
      if (loading || !window.fetch) {
        return;
      }
      loading = true;
      fetch(sky.getAttribute("data-stars")).then(function (resp) {
        return resp.ok ? resp.json() : null;
      }).then(function (data) {
        fields = data;
        schedule();
      }).catch(function () {});
    }

    function placeStars() {
      var old = sky.querySelector(":scope > svg.home-stars");
      if (old) {
        old.remove();
      }
      if (getComputedStyle(sky).display === "none") {
        delete hero.dataset.stars;
        return;
      }
      if (!fields) {
        load();
        return;
      }
      var cs = getComputedStyle(hero);
      var list = fields[{ "0.44": "phone", "0.56": "tall", "0.64": "taller", "0.5": "niche", "0.62": "upright" }[cs.getPropertyValue("--arch-rise").trim()] || "phone"];
      if (!list || !frieze) {
        return;
      }
      // Client rects are measured on the glass; the wide niche is zoomed
      // (--niche-zoom), and the stars are drawn in its own pixels, so every
      // measure is taken back by the zoom (1 on a phone).
      var zoom = heroZoom();
      function own(q) {
        return { left: q.left / zoom, top: q.top / zoom, right: q.right / zoom, bottom: q.bottom / zoom, width: q.width / zoom, height: q.height / zoom };
      }
      var hr = own(hero.getBoundingClientRect());
      var sr = own(sky.getBoundingClientRect());
      var band = own(frieze.getBoundingClientRect()).top - hr.top;
      // The reserve: every laid-out line of the day block, and the cross.
      var reserve = [];
      var walker = document.createTreeWalker(head, NodeFilter.SHOW_TEXT);
      for (var node = walker.nextNode(); node; node = walker.nextNode()) {
        var parent = node.parentElement;
        if (!node.nodeValue.trim() || parent.closest(".sr-only, .home-date-comma, [hidden]")) {
          continue;
        }
        // A second commemoration's "and" is generated, left of its text.
        var item = parent.closest(".commemorations li");
        var lead = item && item.previousElementSibling ? 2.6 * parseFloat(getComputedStyle(parent).fontSize) : 0;
        var range = document.createRange();
        range.selectNodeContents(node);
        Array.prototype.forEach.call(range.getClientRects(), function (rect) {
          var q = own(rect);
          if (q.width > 1) {
            reserve.push([q.left - hr.left - 14 - lead, q.top - hr.top - 6, q.right - hr.left + 14, q.bottom - hr.top + 6]);
          }
        });
      }
      if (crown) {
        var c = own(crown.getBoundingClientRect());
        reserve.push([c.left - hr.left - 11, c.top - hr.top - 11, c.right - hr.left + 11, c.bottom - hr.top + 11]);
      }
      // A first-class feast's stars stand a size larger and fully lit.
      var first = document.body.classList.contains("rank-first-class");
      var k = first ? 1.14 : 1;
      var w = hr.width;
      var svg = document.createElementNS(NS, "svg");
      svg.setAttribute("class", "home-stars");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("focusable", "false");
      svg.setAttribute("width", sr.width.toFixed(1));
      svg.setAttribute("height", sr.height.toFixed(1));
      // Leaf under a lamp: the stars nearest the day's words take the light.
      var cardSpace = document.createElementNS(NS, "g");
      cardSpace.setAttribute("transform", "translate(" + (hr.left - sr.left).toFixed(2) + " " + (hr.top - sr.top).toFixed(2) + ")");
      cardSpace.innerHTML = '<defs><radialGradient id="home-leaf" gradientUnits="userSpaceOnUse" cx="' + (w / 2).toFixed(1) + '" cy="' + (band * 0.62).toFixed(1) + '" r="' + (w * 0.72).toFixed(1) + '"><stop class="s-hi" offset="0"/><stop class="s-mid" offset="0.6"/><stop class="s-lo" offset="1"/></radialGradient></defs>';
      var count = 0;
      list.forEach(function (star) {
        var x = star[0] * w, y = star[1] * w, r = star[2] * k, outline = star[4];
        if (y + r > band - 9) {
          return;
        }
        if (reserve.some(function (q) { return x + r > q[0] && x - r < q[2] && y + r > q[1] && y - r < q[3]; })) {
          return;
        }
        var points = [];
        for (var i = 0; i < outline.length; i += 2) {
          points.push((x + outline[i] * k).toFixed(2) + "," + (y + outline[i + 1] * k).toFixed(2));
        }
        var path = document.createElementNS(NS, "path");
        path.setAttribute("d", "M" + points.join("L") + "Z");
        path.setAttribute("fill", "url(#home-leaf)");
        path.setAttribute("fill-opacity", first ? Math.min(1, star[3] + 0.25).toFixed(2) : String(star[3]));
        cardSpace.appendChild(path);
        count += 1;
      });
      svg.appendChild(cardSpace);
      sky.appendChild(svg);
      hero.dataset.stars = String(count);
    }

    var frame = 0;
    function paint() {
      frame = 0;
      ruleAll();
      if (sky && head) {
        placeStars();
      }
    }
    function schedule() {
      if (!frame) {
        frame = window.requestAnimationFrame(paint);
      }
    }
    // WebKit can run this deferred script before style.css has applied, and
    // rules measured then were drawn across an unstyled page (the table's
    // joints some 900px below it) until the next resize. The hero is a
    // container only once the stylesheet stands: paint now if it is, else
    // when the page has loaded (its stylesheet with it).
    if (getComputedStyle(hero).containerType === "inline-size") {
      paint();
    } else {
      window.addEventListener("load", paint);
    }
    if (document.fonts) {
      document.fonts.ready.then(schedule);
    }
    window.addEventListener("resize", schedule);
    if ("ResizeObserver" in window) {
      var watch = new ResizeObserver(schedule);
      watch.observe(hero);
      if (head) {
        watch.observe(head);
      }
      // The sky shortens with the head when "Change date" turns the stage
      // to the calendar, inside a card that keeps its height.
      if (sky) {
        watch.observe(sky);
      }
    }
    // The sky comes and goes with the theme.
    new MutationObserver(schedule).observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    if (window.matchMedia) {
      var scheme = window.matchMedia("(prefers-color-scheme: dark)");
      if (scheme.addEventListener) {
        scheme.addEventListener("change", schedule);
      }
    }
  });

  // currentHourInfo returns the office most likely being prayed at the given
  // local time, plus a day offset (0 or -1) locating which calendar day it
  // belongs to. Boundaries mirror presentation::current_hour_entry: midnight-2am
  // belongs to the previous day's Compline, not the day that has just begun.
  // One ordered table drives both the current-office choice and its next
  // refresh boundary.
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
        // sr-only to match the server-rendered marker in home.html: the pool
        // of light under the hour and the "Pray {hour}" invitation carry
        // "now" visually.
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

  // The invitation's words, leaving its hand-ruled frame (an aria-hidden SVG
  // child, see "Home: the painted page") in place.
  function setPrayNowLabel(prayNow, label) {
    var words = Array.prototype.find.call(prayNow.childNodes, function (n) { return n.nodeType === 3 && n.nodeValue.trim(); });
    if (words) {
      words.nodeValue = label;
    } else {
      prayNow.insertBefore(document.createTextNode(label), prayNow.firstChild);
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
        prayNow.setAttribute("href", link.getAttribute("href"));
        setPrayNowLabel(prayNow, "Pray " + (HOUR_NAMES[info.slug] || info.slug));
        matched = true;
      }
    });

    if (!matched && info !== null && info.offset !== 0 && prayNow) {
      var offsetDate = new Date(now.getTime());
      offsetDate.setDate(offsetDate.getDate() + info.offset);
      prayNow.setAttribute("href", "/" + info.slug + "/" + localDateSlug(offsetDate));
      setPrayNowLabel(prayNow, "Pray " + (HOUR_NAMES[info.slug] || info.slug));
      matched = true;
    }

    if (!matched && prayNow) {
      var lauds = card.querySelector('.home-hour-link[data-hour="lauds"]');
      if (lauds) {
        prayNow.setAttribute("href", lauds.getAttribute("href"));
      }
      setPrayNowLabel(prayNow, "Open Lauds");
    }
  }

  // About's table marks the hour it is now as home's directory does (none
  // from midnight to 2am, which is yesterday's Compline), and the page's foot
  // carries home's invitation, words and link alike, by the device's clock:
  // the server's copy was marked by its own, and may be cached.
  function updateAboutNow() {
    var table = document.querySelector(".about-hours");
    if (!table) {
      return;
    }
    var now = new Date();
    var info = currentHourInfo(now);
    table.querySelectorAll(".about-hour[data-hour]").forEach(function (row) {
      var isCurrent = info.offset === 0 && row.getAttribute("data-hour") === info.slug;
      row.classList.toggle("is-current", isCurrent);
      var link = row.querySelector("dt a");
      if (isCurrent) {
        link.setAttribute("aria-current", "time");
      } else {
        link.removeAttribute("aria-current");
      }
    });
    var door = document.querySelector(".about-door .pray-now");
    if (door) {
      var officeDate = new Date(now.getTime());
      officeDate.setDate(officeDate.getDate() + info.offset);
      door.setAttribute("href", "/" + info.slug + "/" + localDateSlug(officeDate));
      setPrayNowLabel(door, "Pray " + (HOUR_NAMES[info.slug] || info.slug));
    }
  }

  // Home, About, Ordo, and an open office may remain visible across a time boundary
  // without producing a visibility event. Home and About refresh at the next
  // office boundary; Ordo and an office only need midnight. Midnight is also a home
  // boundary because Compline before 2am belongs to the previous liturgical
  // day.
  //
  // The boundary is held as a wall-clock instant and checked in short steps,
  // not awaited with one long timeout: timer delays stop counting while a
  // phone sleeps or freezes a backgrounded PWA, and Android does not always
  // deliver a visibility event on return, so a single timeout set at Sext
  // could still be pending at Vespers.
  var TIMED_REFRESH_STEP_MS = 60 * 1000;
  var timedChromeRefreshTimer = null;
  var timedChromeRefreshAt = null;
  function timedChromeRefreshBoundary(now) {
    if (document.querySelector(".home-prayer-card[data-date-slug], .about-hours")) {
      return nextOfficeBoundary(now);
    }
    if (!document.querySelector(".calendar") && !document.querySelector(".office-hour")) {
      return null;
    }
    var midnight = new Date(now.getTime());
    midnight.setDate(midnight.getDate() + 1);
    midnight.setHours(0, 0, 0, 0);
    return midnight;
  }
  function scheduleTimedChromeRefresh() {
    if (timedChromeRefreshTimer) {
      clearTimeout(timedChromeRefreshTimer);
      timedChromeRefreshTimer = null;
    }
    var now = new Date();
    if (timedChromeRefreshAt === null) {
      var next = timedChromeRefreshBoundary(now);
      if (next === null) {
        return;
      }
      timedChromeRefreshAt = next.getTime() + 250;
    }
    timedChromeRefreshTimer = setTimeout(
      catchUpWithClock,
      Math.min(TIMED_REFRESH_STEP_MS, Math.max(0, timedChromeRefreshAt - now.getTime())),
    );
  }
  // catchUpWithClock refreshes the chrome once the wall clock has passed the
  // pending boundary, then schedules the next one.
  function catchUpWithClock() {
    timedChromeRefreshTimer = null;
    if (timedChromeRefreshAt !== null && Date.now() >= timedChromeRefreshAt) {
      timedChromeRefreshAt = null;
      lastSyncedDay = localDateSlug(new Date());
      syncChromeIfNeeded();
    }
    scheduleTimedChromeRefresh();
  }
  // Page Lifecycle resume (a frozen page thawing) and window focus are the
  // other signals a returning PWA can give; check the clock on each.
  document.addEventListener("resume", catchUpWithClock);
  window.addEventListener("focus", catchUpWithClock);
  syncChromeIfNeeded();
  scheduleTimedChromeRefresh();

  // markCalendarToday highlights today's row on the ordo page and updates
  // the header "Today" jump link. Applied client-side because calendar pages
  // are served from the service-worker cache, so a server-rendered marker
  // would freeze on whichever day the page was fetched. The link always has
  // a reserved place, and leads to the current year when browsing another.
  function markCalendarToday() {
    document.querySelectorAll(".calendar tr.day.is-today").forEach(function (previous) {
      previous.classList.remove("is-today");
      previous.removeAttribute("aria-current");
    });
    var todayLink = document.getElementById("calendar-today-link");
    if (todayLink) {
      todayLink.setAttribute("href", ordoDayURL(localDateSlug(new Date())));
    }
    // Today's month in the strip: its lozenge and spoken name follow the
    // local date too.
    var thisMonth = localDateSlug(new Date()).slice(0, 7);
    document.querySelectorAll(".month-jump a[data-month]").forEach(function (link) {
      var name = (link.getAttribute("aria-label") || "").replace(/, this month$/, "");
      var today = link.getAttribute("data-month") === thisMonth;
      link.classList.toggle("is-today-month", today);
      link.setAttribute("aria-label", today ? name + ", this month" : name);
    });
    var row = document.getElementById("d-" + localDateSlug(new Date()));
    if (row && row.classList.contains("day")) {
      row.classList.add("is-today");
      row.setAttribute("aria-current", "date");
      if (todayLink) {
        todayLink.setAttribute("href", "#" + row.id);
      }
    }
  }

  // calendarRowClicks makes the whole ordo row navigate to the day's home
  // page. The day-number link stays as the keyboard/no-JS path; clicks on
  // links, rank tooltips, and the office details disclosure keep their own
  // behavior, and selecting text never navigates.
  var calendarEl = document.querySelector(".calendar");
  if (calendarEl) {
    var monthLinks = calendarEl.querySelectorAll(".month-jump a");
    var monthHeadings = calendarEl.querySelectorAll(".month h2");
    var monthFrame = null;
    var markVisibleMonth = function () {
      monthFrame = null;
      var current = monthHeadings[0];
      monthHeadings.forEach(function (heading) {
        if (heading.getBoundingClientRect().top <= 100) current = heading;
      });
      monthLinks.forEach(function (link) {
        if (current && link.hash === "#" + current.parentElement.id) {
          link.setAttribute("aria-current", "location");
        } else {
          link.removeAttribute("aria-current");
        }
      });
    };
    // Only the whole year's strip jumps within the page; a month page names
    // its own month from the server, and the frontispiece none.
    if (calendarEl.getAttribute("data-view") === "all") {
      window.addEventListener("scroll", function () {
        if (monthFrame === null) monthFrame = requestAnimationFrame(markVisibleMonth);
      }, { passive: true });
      markVisibleMonth();
    }

    // A day asked for (#d-…, Today) comes to rest with the whole day before
    // it in view above, under the pinned month heading: style.css allows one
    // plain row (--day-row); here the row before gives its own height, before
    // an in-page link scrolls, and on arrival, where the browser has already
    // brought the day to that plain row's place. The month's first day has
    // its heading above it instead.
    var restUnderDayBefore = function (row) {
      var before = row && row.matches("tr.day") && row.previousElementSibling;
      if (before && before.matches("tr.day")) {
        row.style.setProperty("--day-row", before.offsetHeight + "px");
      }
    };
    document.addEventListener("click", function (e) {
      var link = e.target.closest && e.target.closest('a[href^="#d-"]');
      if (link) {
        restUnderDayBefore(document.getElementById(link.getAttribute("href").slice(1)));
      }
    }, true);
    var readerMoved = false;
    var askedDay = location.hash.indexOf("#d-") === 0 ? document.getElementById(location.hash.slice(1)) : null;
    var arrival = window.performance && performance.getEntriesByType ? performance.getEntriesByType("navigation")[0] : null;
    if (askedDay && askedDay.matches("tr.day")) {
      restUnderDayBefore(askedDay);
      // Not over a place the browser restored (a reload, Back).
      if (!arrival || arrival.type === "navigate") {
        askedDay.scrollIntoView({ block: "start", behavior: "instant" });
      }
    }

    // The day arrived at (#d-…) keeps its gold ground until the reader moves
    // on: the first scroll of their own lets it go (.target-released), so it
    // never stands as a second "you are here" beside Today. The browser's own
    // scroll to the day is not the reader's; a new day asked for marks again.
    var noteReaderMoved = function () { readerMoved = true; };
    ["wheel", "touchmove", "keydown"].forEach(function (type) {
      window.addEventListener(type, noteReaderMoved, { passive: true });
    });
    window.addEventListener("scroll", function () {
      if (readerMoved && location.hash) {
        calendarEl.classList.add("target-released");
      }
    }, { passive: true });
    window.addEventListener("hashchange", function () {
      readerMoved = false;
      calendarEl.classList.remove("target-released");
      // A day asked for in place (a typed address, Back across days), which
      // no link's click prepared.
      var day = location.hash.indexOf("#d-") === 0 ? document.getElementById(location.hash.slice(1)) : null;
      if (day && day.matches("tr.day") && !day.style.getPropertyValue("--day-row")) {
        restUnderDayBefore(day);
        day.scrollIntoView({ block: "start", behavior: "instant" });
      }
    });

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
  // The page continues into hour navigation, issue reporting, and
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

// A finger on a control. A touch browser may set :active late (Chromium once
// the finger has lifted) or, in iOS Safari, only where a touch listener is
// heard; and a tap is over in a few frames, before a press that waited could
// be seen. So the control under a finger that rests on it for 90ms without
// moving takes .is-pressed (a thumb starting a scroll lights nothing, as in a
// native list), as does the control a tap lands on, at once. A link to
// another of the site's pages keeps it until the page goes, so the last thing
// seen is the press; anything else lets go at once, and style.css eases the
// release over 200ms (and draws .is-pressed as :active). An ordo row is one
// control with the day's own links (calendarRowClicks). A mouse has :active,
// and its click is marked the same way.
(function () {
  if (!window.PointerEvent) return;
  var PRESSABLE = "a[href], button:not(:disabled), summary, label, [role='button']";
  var ROW_LINKS = ".day-feast-name, .day-mobile-date, .day-num a";
  var REST_MS = 90;
  var SLOP_PX = 10;
  var KEEP_MS = 8000;
  var held = null;
  var timer = 0;
  var startX = 0;
  var startY = 0;

  function pressable(target) {
    if (!target || !target.closest) return null;
    var row = target.closest(".js .month-table tr.day");
    if (row && !target.closest("abbr, details")) {
      var link = target.closest("a");
      if (!link || link.matches(ROW_LINKS)) return row;
    }
    return target.closest(PRESSABLE);
  }
  function lift() {
    window.clearTimeout(timer);
    timer = 0;
    if (held) held.classList.remove("is-pressed");
    held = null;
  }
  function clearAll() {
    lift();
    document.querySelectorAll(".is-pressed").forEach(function (el) {
      el.classList.remove("is-pressed");
    });
  }
  // Whether a click on el leaves for another of the site's pages.
  function leaves(e, el) {
    if (el.matches("tr.day")) {
      // The day's own link, opened in a new tab or window, leaves the row here.
      var own = e.target.closest("a[href]");
      if (own && !plainClickOpensHere(e, own)) return false;
      var selection = window.getSelection && window.getSelection();
      return !(selection && !selection.isCollapsed);
    }
    if (!el.matches("a[href]") || e.defaultPrevented || !plainClickOpensHere(e, el)) return false;
    if (el.origin !== location.origin) return false;
    return !(el.hash && el.pathname === location.pathname && el.search === location.search);
  }

  document.addEventListener("pointerdown", function (e) {
    lift();
    if (e.pointerType === "mouse" || !e.isPrimary) return;
    var el = pressable(e.target);
    if (!el) return;
    held = el;
    startX = e.clientX;
    startY = e.clientY;
    timer = window.setTimeout(function () {
      timer = 0;
      el.classList.add("is-pressed");
    }, REST_MS);
  }, { passive: true });
  document.addEventListener("pointermove", function (e) {
    if (held && e.isPrimary && Math.abs(e.clientX - startX) + Math.abs(e.clientY - startY) > SLOP_PX) lift();
  }, { passive: true });
  document.addEventListener("pointercancel", lift, { passive: true });
  document.addEventListener("pointerup", function () {
    // The click follows, and decides; a finger lifted off the control has none.
    var was = held;
    window.setTimeout(function () {
      if (held === was) lift();
    }, 100);
  }, { passive: true });
  // Last, after the page's own handlers have had their say (defaultPrevented).
  document.addEventListener("click", function (e) {
    var el = e.detail ? pressable(e.target) : null;
    if (held && held !== el) lift();
    window.clearTimeout(timer);
    timer = 0;
    held = null;
    if (!el) return;
    el.classList.add("is-pressed");
    if (leaves(e, el)) {
      window.setTimeout(function () { el.classList.remove("is-pressed"); }, KEEP_MS);
      return;
    }
    // Seen for a frame, then eased off.
    window.requestAnimationFrame(function () {
      window.requestAnimationFrame(function () { el.classList.remove("is-pressed"); });
    });
  });
  // iOS Safari applies :active only where a touch listener is heard; this
  // one, never removed, lets it apply everywhere.
  document.addEventListener("touchstart", function () {}, { passive: true });
  // A page restored by Back is shown at rest.
  window.addEventListener("pageshow", function (e) {
    if (e.persisted) clearAll();
  });
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
  window.addEventListener("officemartyrologychange", function () { engage(); record(); });
  window.addEventListener("officechantchange", function () { engage(); record(); });
  // A foreground page left open across midnight belongs to the new day too.
  window.setInterval(record, 60000);
  resume();
})();

// The prayers before and after the Office open by the hour's default (Lauds,
// Prime and Vespers open) until the reader opens or closes them; that choice
// is then kept for the hour's section on this device, in localStorage only, as
// "sext/Prayers Before the Office": true. hour.html applies it before paint.
// Only a tap on the summary is a choice, not the page settling its defaults.
(function () {
  var PRAYERS_KEY = "office-prayers-open";
  document.querySelectorAll(".elements > details.session-prayers[data-prayers]").forEach(function (details) {
    var summary = details.querySelector(":scope > summary");
    if (!summary) return;
    summary.addEventListener("click", function () {
      // The summary's own toggle follows this listener, so the choice is the
      // state it will leave; one still folding shut (app.js's fold, where
      // the browser cannot ease to auto) is open, and this opens it again.
      var open = details.classList.contains("is-folding") || !details.open;
      try {
        var chosen = JSON.parse(localStorage.getItem(PRAYERS_KEY) || "{}");
        if (!chosen || typeof chosen !== "object" || Array.isArray(chosen)) chosen = {};
        chosen[details.getAttribute("data-prayers")] = open;
        localStorage.setItem(PRAYERS_KEY, JSON.stringify(chosen));
      } catch {
        /* Storage refused: the hour keeps its default next time. */
      }
    });
  });
})();

// Home marks the hours prayed: an hour counts once its end is reached after
// its words were read on screen at a praying pace, so scrolling through in a
// few seconds, or waiting at the end, does not mark it. presentation::Reading
// is the rule; these three mirror its constants (office-web tests they agree).
// The marks stay on this device a week, in localStorage only.
(function () {
  var PRAYED_WORDS_PER_SECOND = 8;
  var PRAYED_SHARE = 0.7;
  var PRAYED_LONGEST_STEP = 5;
  var PRAYED_KEY = "office-prayed";
  var KEEP_DAYS = 7;
  var hours = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];
  var day = document.body.getAttribute("data-usage-when") || "";
  if (!/^\d{4}-\d{2}-\d{2}$/.test(day)) return;

  function load() {
    try {
      var stored = JSON.parse(localStorage.getItem(PRAYED_KEY) || "{}");
      return stored && typeof stored === "object" && !Array.isArray(stored) ? stored : {};
    } catch {
      return {};
    }
  }

  function slugDaysAgo(days) {
    var d = new Date();
    d.setDate(d.getDate() - days);
    var m = String(d.getMonth() + 1);
    var dd = String(d.getDate());
    return d.getFullYear() + "-" + (m.length < 2 ? "0" + m : m) + "-" + (dd.length < 2 ? "0" + dd : dd);
  }

  function markPrayed(hour) {
    var prayed = load();
    var oldest = slugDaysAgo(KEEP_DAYS);
    Object.keys(prayed).forEach(function (d) {
      if (d < oldest || !Array.isArray(prayed[d])) delete prayed[d];
    });
    var list = prayed[day] || [];
    if (list.indexOf(hour) < 0) list.push(hour);
    prayed[day] = list;
    try {
      localStorage.setItem(PRAYED_KEY, JSON.stringify(prayed));
    } catch {
      /* Storage refused: the hour simply goes unmarked. */
    }
  }

  // Home: each hour prayed on the shown day carries a small gilt cross at its
  // name's shoulder, and says "Prayed" to a screen reader.
  if (document.body.classList.contains("page-home")) {
    var paint = function () {
      var list = load()[day];
      document.querySelectorAll(".home-hour-link[data-hour]").forEach(function (link) {
        var isPrayed = Array.isArray(list) && list.indexOf(link.getAttribute("data-hour")) >= 0;
        var mark = link.querySelector(".home-hour-prayed");
        link.classList.toggle("is-prayed", isPrayed);
        if (isPrayed && !mark) {
          mark = document.createElement("span");
          mark.className = "home-hour-prayed";
          mark.innerHTML = '<span aria-hidden="true">✠</span><span class="sr-only">Prayed</span>';
          (link.querySelector(".home-hour-link-name") || link).appendChild(mark);
        } else if (!isPrayed && mark) {
          mark.remove();
        }
      });
    };
    paint();
    window.addEventListener("pageshow", paint);
    window.addEventListener("storage", function (e) {
      if (e.key === PRAYED_KEY || e.key === null) paint();
    });
    return;
  }

  var hour = hours.find(function (name) { return document.body.classList.contains("page-" + name); });
  var content = document.querySelector(".office-hour .elements");
  var end = document.querySelector(".hour-end-mark");
  if (!hour || !content || !end) return;

  var cursor = 0;
  var read = 0;
  var done = false;
  var last = Date.now();

  // The words of the shown text, in order: each text node's count and box.
  // A form or reading hidden by the reader's settings has no box and no words.
  // The words are counted once and the boxes measured once, relative to the
  // hour's top, then again only when the text may have moved within the hour:
  // a node or attribute changed under it or on the root (a setting, a fold, a
  // gloss, a score drawn anew) or the hour's box changed size (the window, the
  // text size, a face arriving). Between, each second compares numbers.
  var nodes = null;
  var boxes = null;
  var measured = null;
  new MutationObserver(function (records) {
    if (records.some(function (r) { return r.type !== "attributes"; })) nodes = null;
    boxes = null;
  }).observe(content, { subtree: true, childList: true, characterData: true, attributes: true });
  new MutationObserver(function () { boxes = null; })
    .observe(document.documentElement, { attributes: true });
  new MutationObserver(function () { boxes = null; })
    .observe(document.body, { attributes: true });
  if (document.fonts) document.fonts.addEventListener("loadingdone", function () { boxes = null; });

  function measure(origin) {
    if (!nodes) {
      nodes = [];
      var walker = document.createTreeWalker(content, NodeFilter.SHOW_TEXT);
      for (var node = walker.nextNode(); node; node = walker.nextNode()) {
        var count = (node.nodeValue.match(/\S*[A-Za-z0-9]\S*/g) || []).length;
        if (count) nodes.push({ node: node, words: count });
      }
    }
    var range = document.createRange();
    boxes = [];
    nodes.forEach(function (entry) {
      range.selectNodeContents(entry.node);
      var rects = range.getClientRects();
      if (!rects.length) return;
      boxes.push({ top: rects[0].top - origin.top, bottom: rects[rects.length - 1].bottom - origin.top, words: entry.words });
    });
    measured = { width: origin.width, height: origin.height };
  }

  function wordsOnScreen() {
    var origin = content.getBoundingClientRect();
    if (!boxes || origin.width !== measured.width || origin.height !== measured.height) measure(origin);
    var total = 0;
    var first = -1;
    var through = 0;
    // The screen's top and foot, measured from the hour's top.
    var top = -origin.top;
    var bottom = window.innerHeight - origin.top;
    for (var i = 0; i < boxes.length; i++) {
      var box = boxes[i];
      if (box.bottom > top && box.top < bottom) {
        if (first < 0) first = total;
        through = total + box.words;
      }
      total += box.words;
    }
    // Nothing on screen: above the text, none of it is passed; below, all of it.
    if (first < 0) first = through = origin.bottom <= 0 ? total : 0;
    return { total: total, first: first, through: through };
  }

  function tick() {
    var now = Date.now();
    var seconds = (now - last) / 1000;
    last = now;
    if (done || document.visibilityState !== "visible") return;
    var seen = wordsOnScreen();
    // presentation::Reading::advance
    cursor = Math.max(cursor, seen.first);
    var pace = Math.min(Math.max(seconds, 0), PRAYED_LONGEST_STEP) * PRAYED_WORDS_PER_SECOND;
    var step = Math.max(0, Math.min(pace, seen.through - cursor));
    cursor += step;
    read += step;
    var atEnd = end.getBoundingClientRect().top < window.innerHeight;
    if (atEnd && seen.total > 0 && read >= seen.total * PRAYED_SHARE) {
      done = true;
      markPrayed(hour);
    }
  }

  document.addEventListener("visibilitychange", function () {
    last = Date.now();
  });
  window.setInterval(tick, 1000);
})();

// Closing the prayers after the Office near the end of an hour would shorten
// the page under the screen, and the browser would pull the page down to its
// new end: the summary under the reader's finger would jump. The page keeps its
// height through the fold instead, and gives the room back as the reader
// scrolls up into it, never moving what is on the screen.
(function () {
  var root = document.documentElement;
  var held = 0;
  var release = function () {
    var end = document.body.getBoundingClientRect().bottom + window.scrollY;
    var needed = Math.ceil(window.scrollY + window.innerHeight);
    if (end >= needed) {
      held = 0;
      root.style.minHeight = "";
      window.removeEventListener("scroll", release);
    } else if (needed < held) {
      held = needed;
      root.style.minHeight = held + "px";
    }
  };
  document.querySelectorAll(".page-hour .elements > details.session-prayers > summary").forEach(function (summary) {
    summary.addEventListener("click", function () {
      if (!summary.parentNode.open) return;
      held = Math.ceil(window.scrollY + window.innerHeight);
      root.style.minHeight = held + "px";
      window.addEventListener("scroll", release, { passive: true });
    });
  });
})();

// The end of an hour: its way on is ruled under with home's pen where the
// wall's field begins, so the powdering and the vault are cut under a line
// as they are under the header's beam (style.css "The way on"). Without
// script the rule is a border.
(function () {
  var nav = document.querySelector(".page-hour .hour-continuation");
  if (!nav) return;
  var NS = "http://www.w3.org/2000/svg";
  var width = 0, height = 0;
  function draw() {
    var w = nav.offsetWidth, h = nav.offsetHeight;
    if (!w || (w === width && h === height)) return;
    width = w;
    height = h;
    var svg = nav.querySelector(":scope > svg.hour-rules");
    if (!svg) {
      svg = document.createElementNS(NS, "svg");
      svg.setAttribute("class", "hour-rules");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("focusable", "false");
      nav.appendChild(svg);
      nav.classList.add("is-ruled");
    }
    svg.textContent = "";
    var path = document.createElementNS(NS, "path");
    path.setAttribute("d", rule(0, h - 0.5, w, h - 0.5, 1.2, 51));
    svg.appendChild(path);
  }
  draw();
  if ("ResizeObserver" in window) new ResizeObserver(draw).observe(nav);
  else window.addEventListener("resize", draw);
  if (document.fonts) document.fonts.ready.then(draw);
})();

// About's foot: home's invitation, ruled as home rules it (the lining, and
// inside it the frontal stopping short of the corners), with home's seeds.
// Without script its frame is the CSS border.
(function () {
  var tablet = document.querySelector(".about-door .pray-now");
  if (!tablet) return;
  var NS = "http://www.w3.org/2000/svg";
  function draw() {
    var w = tablet.offsetWidth, h = tablet.offsetHeight, g = 3.5;
    if (!w || !h) return;
    var svg = tablet.querySelector(":scope > svg.home-rules");
    if (!svg) {
      svg = document.createElementNS(NS, "svg");
      svg.setAttribute("class", "home-rules");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("focusable", "false");
      tablet.appendChild(svg);
      tablet.classList.add("is-ruled");
    }
    svg.textContent = "";
    [
      ["r-lining", 0, 0.5, w, 0.5, 1.25, 21],
      ["r-lining", 0, h - 0.5, w, h - 0.5, 1.25, 22],
      ["r-lining", 0.5, 0, 0.5, h, 1.25, 23],
      ["r-lining", w - 0.5, 0, w - 0.5, h, 1.25, 24],
      ["r-day", g + 1.5, g, w - g - 1.5, g, 0.7, 25],
      ["r-day", g + 1.5, h - g, w - g - 1.5, h - g, 0.7, 26],
      ["r-day", g, g + 1.5, g, h - g - 1.5, 0.7, 27],
      ["r-day", w - g, g + 1.5, w - g, h - g - 1.5, 0.7, 28],
    ].forEach(function (s) {
      var path = document.createElementNS(NS, "path");
      path.setAttribute("class", s[0]);
      path.setAttribute("d", rule(s[1], s[2], s[3], s[4], s[5], s[6]));
      svg.appendChild(path);
    });
  }
  // As the feast page's: WebKit can run this before style.css applies.
  if (getComputedStyle(document.documentElement).getPropertyValue("--bg")) {
    draw();
  } else {
    window.addEventListener("load", draw);
  }
  if ("ResizeObserver" in window) new ResizeObserver(draw).observe(tablet);
  else window.addEventListener("resize", draw);
  if (document.fonts) document.fonts.ready.then(draw);
})();

// The feast page (style.css "The feast page"): ruled with home's pen. The
// head's course of the lining, broken by the cross (and on a first-class
// feast doubled by a finer rule in its colour); the standing's double frame,
// as home's invitation; one stroke under each section's heading; the years
// ruled above, below and between, as home's hours; and on a first-class feast
// from 760px an upright course between the texts and the rail. Without
// script each is a CSS border.
(function () {
  var page = document.querySelector(".page-feast .feast-page");
  if (!page) return;
  var NS = "http://www.w3.org/2000/svg";
  var head = page.querySelector(".feast-head");
  var standing = page.querySelector(".feast-standing");
  var body = page.querySelector(".feast-body");
  var first = head && head.classList.contains("is-first");
  var plain = head && head.classList.contains("is-plain");

  function layer(host) {
    var svg = host.querySelector(":scope > svg.feast-rules");
    if (!svg) {
      svg = document.createElementNS(NS, "svg");
      svg.setAttribute("class", "feast-rules");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("focusable", "false");
      host.appendChild(svg);
      host.classList.add("is-ruled");
    }
    svg.textContent = "";
    return svg;
  }

  function stroke(svg, cls, d) {
    var path = document.createElementNS(NS, "path");
    path.setAttribute("class", cls);
    path.setAttribute("d", d);
    svg.appendChild(path);
  }

  function ruleAll() {
    var w, h, svg;
    if (head && !plain) {
      w = head.offsetWidth;
      svg = layer(head);
      var mid = w / 2, gap = first ? 27 : 24;
      stroke(svg, "r-lining", rule(0, 0.5, mid - gap, 0.5, 1.25, 51));
      stroke(svg, "r-lining", rule(mid + gap, 0.5, w, 0.5, 1.25, 52));
      if (first) {
        stroke(svg, "r-day", rule(1.5, 4, mid - gap - 3, 4, 0.8, 53));
        stroke(svg, "r-day", rule(mid + gap + 3, 4, w - 1.5, 4, 0.8, 54));
      }
    }
    if (standing) {
      var g = 3.5;
      w = standing.offsetWidth;
      h = standing.offsetHeight;
      svg = layer(standing);
      stroke(svg, "r-lining", rule(0, 0.5, w, 0.5, 1.25, 61));
      stroke(svg, "r-lining", rule(0, h - 0.5, w, h - 0.5, 1.25, 62));
      stroke(svg, "r-lining", rule(0.5, 0, 0.5, h, 1.25, 63));
      stroke(svg, "r-lining", rule(w - 0.5, 0, w - 0.5, h, 1.25, 64));
      // The frontal, finer, stopping short of the corners.
      stroke(svg, "r-day", rule(g + 1.5, g, w - g - 1.5, g, 0.7, 65));
      stroke(svg, "r-day", rule(g + 1.5, h - g, w - g - 1.5, h - g, 0.7, 66));
      stroke(svg, "r-day", rule(g, g + 1.5, g, h - g - 1.5, 0.7, 67));
      stroke(svg, "r-day", rule(w - g, g + 1.5, w - g, h - g - 1.5, 0.7, 68));
      var next = standing.classList.contains("is-monastic") && standing.querySelector(".feast-next");
      if (next) {
        svg = layer(next);
        stroke(svg, "r-inner", rule(0, 0.5, next.offsetWidth, 0.5, 0.9, 69));
      }
    }
    page.querySelectorAll(".feast-section > h2").forEach(function (h2, i) {
      svg = layer(h2);
      stroke(svg, "r-frame", rule(0, h2.offsetHeight - 0.5, h2.offsetWidth, h2.offsetHeight - 0.5, 1.3, 71 + i));
    });
    var list = page.querySelector(".feast-years .find-years");
    if (list && list.querySelector(":scope > li")) {
      svg = layer(list);
      var top = list.getBoundingClientRect().top;
      w = list.offsetWidth;
      var rows = Array.prototype.map.call(list.querySelectorAll(":scope > li"), function (li) { return li.getBoundingClientRect(); });
      rows.forEach(function (r, i) {
        stroke(svg, i ? "r-inner" : "r-frame", rule(0, r.top - top + 0.5, w, r.top - top + 0.5, i ? 0.9 : 1.3, 81 + i));
      });
      var last = rows[rows.length - 1];
      stroke(svg, "r-frame", rule(0, last.bottom - top - 0.5, w, last.bottom - top - 0.5, 1.3, 81 + rows.length));
    }
    var texts = body && body.querySelector(".feast-texts");
    if (first && texts && standing && body) {
      svg = layer(body);
      if (window.matchMedia("(min-width: 760px)").matches) {
        var x = (texts.offsetLeft + texts.offsetWidth + standing.offsetLeft) / 2;
        h = body.offsetHeight;
        stroke(svg, "r-lining", rule(x, 4, x, h - 4, 1.25, 57));
        stroke(svg, "r-day", rule(x + 3.5, 6, x + 3.5, h - 6, 0.8, 58));
      }
    }
  }

  // WebKit can run this deferred script before style.css has applied, and
  // rules measured then are drawn across an unstyled page until the next
  // resize. The wall's tokens stand only once the stylesheet does: rule now
  // if they do, else when the page has loaded (its stylesheet with it).
  if (getComputedStyle(document.documentElement).getPropertyValue("--bg")) {
    ruleAll();
  } else {
    window.addEventListener("load", ruleAll);
  }
  // Redrawn whenever the page's box changes: a new width, and the reader's
  // text size, which moves every line without touching the window.
  var size = page.offsetWidth + "x" + page.offsetHeight;
  function remeasure() {
    var now = page.offsetWidth + "x" + page.offsetHeight;
    if (now !== size) {
      size = now;
      ruleAll();
    }
  }
  if ("ResizeObserver" in window) new ResizeObserver(remeasure).observe(page);
  else window.addEventListener("resize", remeasure);
  if (document.fonts) document.fonts.ready.then(ruleAll);
})();
