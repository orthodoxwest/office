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

  // Home's crown (below: "Home: the crown's play") turns the look over with
  // a play of its own; any other change of theme ends it first.
  var crownPlay = null;

  // User action: paint + persist.
  var applyThemeChoice = function (choice) {
    choice = normalizeThemeChoice(choice) || "default";
    // The crown's play, if one is running, ends here: this choice is the
    // later word.
    if (crownPlay) {
      crownPlay.end();
    }
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

  // Native disclosures start closed at every width, including without JS.
  // A deliberate bulk action is useful for comparing offices across the year.
  var calendarExpand = document.querySelector(".calendar-expand");
  if (calendarExpand) {
    calendarExpand.hidden = false;
    calendarExpand.addEventListener("click", function () {
      var expand = calendarExpand.getAttribute("aria-pressed") !== "true";
      document.querySelectorAll(".day-disclosures details").forEach(function (item) {
        item.open = expand;
      });
      calendarExpand.setAttribute("aria-pressed", String(expand));
      calendarExpand.textContent = expand ? "Hide office details" : "Show office details";
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
    button.addEventListener("click", function () {
      details.open = !details.open;
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
    // The stars stand in the vault's host in both looks (hidden by day), so
    // a change of look never waits on a fetch or a repaint to have them.
    var starHost = hero.querySelector(".home-vault-stars") || sky;
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
      var old = starHost.querySelector(":scope > svg.home-stars");
      if (starHost === sky && getComputedStyle(sky).display === "none") {
        if (old) {
          old.remove();
        }
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
      var sr = own(starHost.getBoundingClientRect());
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
      // The same head, the same field: leave the stars that stand. (The
      // theme's observer asks for a repaint a frame after every change.)
      var key = [list.length, k, w.toFixed(1), band.toFixed(1), sr.width.toFixed(1), sr.height.toFixed(1), (hr.left - sr.left).toFixed(1), (hr.top - sr.top).toFixed(1)]
        .concat(reserve.map(function (q) { return q.map(function (n) { return n.toFixed(1); }).join(" "); })).join("|");
      if (old && old.getAttribute("data-key") === key) {
        return;
      }
      if (old) {
        old.remove();
      }
      svg.setAttribute("data-key", key);
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
        // Where it stands and how large, for the crown's play.
        path.setAttribute("data-y", (y / band).toFixed(4));
        path.setAttribute("data-r", r.toFixed(2));
        path.setAttribute("fill-opacity", first ? Math.min(1, star[3] + 0.25).toFixed(2) : String(star[3]));
        cardSpace.appendChild(path);
        count += 1;
      });
      svg.appendChild(cardSpace);
      starHost.appendChild(svg);
      hero.dataset.stars = String(count);
    }

    var frame = 0;
    // The rules do not depend on the look, only on the sizes of what they
    // rule, so they are keyed on those sizes, as the stars are keyed: a
    // change of look reads a few boxes and rebuilds nothing.
    var ruled = "";
    function rulesKey() {
      var parts = [heroZoom()];
      [frieze, prayNow, table].forEach(function (el) {
        parts.push(el ? el.offsetWidth + "x" + el.offsetHeight : "-");
      });
      if (table) {
        var top = table.getBoundingClientRect().top;
        Array.prototype.forEach.call(table.querySelectorAll(".home-hour-group"), function (group) {
          parts.push(group.offsetTop + "/" + (group.getBoundingClientRect().top - top).toFixed(2));
        });
      }
      return parts.join(",");
    }
    function paint() {
      frame = 0;
      var key = rulesKey();
      if (key !== ruled) {
        ruled = key;
        ruleAll();
      }
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
    // The words have a span of their own (home.html), which the crown's play
    // names; a cached page from before it has none.
    var span = prayNow.querySelector(".pray-now-label");
    if (span) {
      span.textContent = label;
      return;
    }
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

  // Home, Ordo, and an open office may remain visible across a time boundary
  // without producing a visibility event. Home refreshes at the next office
  // boundary; Ordo and an office only need midnight. Midnight is also a home
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
    if (document.querySelector(".home-prayer-card[data-date-slug]")) {
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

  // Home: the crown's play. A tap on the consecration cross at the head of
  // the arch turns the look over: Nave to Apse plays as dusk, Apse to Nave as
  // dawn (presentation::crown has the rule and the timetable; style.css's
  // block of this name says how the picture is put together). Home only: the
  // same mark ends every hour, and there it stays paint.
  (function () {
    "use strict";

    var root = document.documentElement;
    var hero = document.querySelector(".page-home .home-hero");
    var tap = hero && hero.querySelector(".home-crown-tap");
    if (!hero || !tap) {
      return;
    }
    var crown = hero.querySelector(".home-crown");
    var summary = hero.querySelector(".home-summary");
    var head = hero.querySelector(".home-day-head");
    var sky = hero.querySelector(".home-sky");
    var fill = hero.querySelector(".home-arch-fill");
    var vault = hero.querySelector(".home-vault");
    var ground = hero.querySelector(".home-vault-ground");
    var starHost = hero.querySelector(".home-vault-stars");
    var glow = hero.querySelector(".home-glow");
    var back = hero.querySelector(".home-back > span");
    var frieze = hero.querySelector(".home-prayer-card > h2");
    if (!crown || !summary || !head || !sky || !fill || !vault || !ground || !starHost || !glow || !back || !frieze) {
      return;
    }
    var mq = function (q) {
      return window.matchMedia ? window.matchMedia(q) : null;
    };
    var device = mq("(prefers-color-scheme: dark)");
    var still = mq("(prefers-reduced-motion: reduce)");
    var contrast = mq("(prefers-contrast: more)");
    var forced = mq("(forced-colors: active)");

    // The timetable (presentation::crown mirrors every figure here to the
    // native apps; office-web tests the two agree). Times are ms from the
    // tap: when the look itself turns, the front crossing the head and the
    // spread over the page (each a start and a length), the veil's first and
    // last, and the end. Light arrives faster than it leaves.
    var CROWN_PLAY = {
      dusk: { turn: 650, front: [800, 1700], spread: [2200, 1750], veil: [2000, 3500], end: 4000, inkAt: 0.6, glowEase: [0.55, 0.1, 0.7, 1] },
      dawn: { turn: 650, front: [800, 1600], spread: [1850, 1800], veil: [1650, 3600], end: 3700, inkAt: 0.33, glowEase: [0.5, 0.2, 0.7, 1] },
    };
    var FRONT_EASE = [0.3, 0, 0.7, 1];
    // Near enough even: it starts from nothing under the frieze, so it must
    // not dawdle there, and it ends at the corner, so it need not brake.
    var SPREAD_EASE = [0.4, 0, 0.5, 1];
    // On a wide screen the spread has further to go.
    var SPREAD_WIDE = 100;
    // The arch's feather, in card widths at the shape's first size.
    var SPREAD_FEATHER = 0.42;
    // The spread's shape is done by this share of its span, growing by an
    // even ratio bent a little toward its start; the page's own opacity
    // rises over nearly all of it (a glow filling a room, not a front
    // crossing it).
    var SPREAD_SHAPE_PART = 0.45;
    var SPREAD_BEND = 0.8;
    var SPREAD_GLOW_PART = 0.96;
    var VEIL_LEAD = 1.4;
    var VEIL_PART = 0.8;
    // A second tap is kept only if it comes this long after the last, and a
    // touch elsewhere runs the rest out in RUSH.
    var TAP_APART = 450;
    var RUSH = 300;
    // Where in a mask's radius its feather is half through (style.css's stops).
    var MID = 0.89, CORE = 0.78;
    var WORDS = 12, ROWS = 10;

    // ---- The arch, from tools/genarch.py's figures, in card widths: the
    // head style.css clips is the head the spread opens out of. ----
    // genarch:begin: tools/genarch.py writes this block; edit the script, not the figures.
    var ARCHES = {
      "0.44": { haunch: 0.3, centre: 0.9, depth: 0.781098, radius: 1.284944 }, // phone
      "0.56": { haunch: 0.4, centre: 1, depth: 0.535519, radius: 1.204227 }, // tall
      "0.64": { haunch: 0.4, centre: 1.05, depth: 0.364522, radius: 1.145236 }, // taller
      "0.5": { haunch: 0.34, centre: 0.95, depth: 0.632955, radius: 1.219052 }, // niche
      "0.62": { haunch: 0.38, centre: 1.02, depth: 0.372603, radius: 1.120563 }, // upright
    };
    // genarch:end
    function archNow() {
      var key = getComputedStyle(hero).getPropertyValue("--arch-rise").trim();
      var f = ARCHES[key] || ARCHES["0.44"];
      var rise = parseFloat(ARCHES[key] ? key : "0.44"), h = f.haunch, c = f.centre, cy = -f.depth, R = f.radius;
      var len = Math.hypot(h - c, -cy);
      var turn = Math.atan2(cy + (-cy) * R / len, c + (h - c) * R / len - h);
      var left = function (x, y) {
        if (y <= 0) {
          return x;
        }
        if (Math.atan2(y, x - h) >= turn) {
          return h - Math.hypot(x - h, y);
        }
        return R - Math.hypot(x - c, y - cy);
      };
      // How far inside the head a point is, in card widths (negative: outside).
      // x across from the left springing, y up from the springing line.
      return {
        rise: rise,
        inside: function (x, y) {
          return Math.min(left(x, y), left(1 - x, y));
        },
      };
    }
    function smooth(a, b, x) {
      var t = Math.max(0, Math.min(1, (x - a) / (b - a)));
      return t * t * (3 - 2 * t);
    }
    function deviceLook() {
      return device && device.matches ? "dark" : "light";
    }
    function lookOf(choice) {
      return choice === "light" || choice === "dark" ? choice : deviceLook();
    }
    function look() {
      return lookOf(root.getAttribute("data-theme"));
    }

    // The button says what it will do, in the Menu row's own words, and
    // nothing else gives it away.
    function label() {
      tap.setAttribute("aria-label", look() === "dark" ? "Switch to Nave" : "Switch to Apse");
    }
    label();
    new MutationObserver(label).observe(root, { attributes: true, attributeFilter: ["data-theme"] });
    // The themes do nothing under forced colours, so neither does the button.
    // It is shown once the set-up is done (idle time, well inside a second
    // of load), so no tap ever pays for the set-up.
    var setUp = false;
    function reveal() {
      setUp = true;
      tap.hidden = !!(forced && forced.matches);
    }

    // The rule (presentation::crown::after_crown_tap, mirrored here as the
    // hour schedule is). The look always turns over. A look that matches the
    // device's is saved as Default, so the crown never strands a reader off
    // Default; and the tap back undoes the tap exactly: the choice the last
    // tap turned away from comes back when the new tap lands on its look (so
    // Apse chosen on a dark phone comes back as Apse, not as Default).
    function afterCrownTap(choice, deviceDark, restore) {
      var dark = !(choice === "default" ? deviceDark : choice === "dark");
      if (restore && (restore === "default" ? deviceDark : restore === "dark") === dark) {
        return { choice: restore, restore: null };
      }
      return { choice: dark === deviceDark ? "default" : dark ? "dark" : "light", restore: choice };
    }
    // What the last tap turned away from, and what it wrote: kept in memory
    // only, and only while the saved choice is still the one the tap wrote.
    var turned = null;

    // Cubic bezier: the time (0..1) at which the eased progress is p.
    function timeAt(ease, p) {
      if (p <= 0) { return 0; }
      if (p >= 1) { return 1; }
      function axis(a, b, t) {
        return 3 * a * t * (1 - t) * (1 - t) + 3 * b * t * t * (1 - t) + t * t * t;
      }
      var lo = 0, hi = 1;
      for (var i = 0; i < 24; i++) {
        var mid = (lo + hi) / 2;
        if (axis(ease[1], ease[3], mid) < p) { lo = mid; } else { hi = mid; }
      }
      return axis(ease[0], ease[2], (lo + hi) / 2);
    }

    // Cubic bezier: the eased progress at time x (0..1).
    function easeAt(ease, x) {
      if (x <= 0) { return 0; }
      if (x >= 1) { return 1; }
      function axis(a, b, t) {
        return 3 * a * t * (1 - t) * (1 - t) + 3 * b * t * t * (1 - t) + t * t * t;
      }
      var lo = 0, hi = 1;
      for (var i = 0; i < 24; i++) {
        var mid = (lo + hi) / 2;
        if (axis(ease[0], ease[2], mid) < x) { lo = mid; } else { hi = mid; }
      }
      return axis(ease[1], ease[3], (lo + hi) / 2);
    }

    function K() {
      return parseFloat(getComputedStyle(root).getPropertyValue("--crown-k")) || 1;
    }
    function px(n) {
      return n.toFixed(1) + "px";
    }
    function copy(to, from, props) {
      props.forEach(function (k) {
        if (from[k] !== undefined) {
          to.style[k] = from[k];
        }
      });
    }
    var GROUND = ["backgroundImage", "backgroundSize", "backgroundPosition", "backgroundRepeat", "backgroundBlendMode", "backgroundColor"];
    var PAINT = GROUND.concat(["maskImage", "maskSize", "maskPosition", "maskRepeat", "maskComposite", "webkitMaskImage", "webkitMaskSize", "webkitMaskPosition", "webkitMaskRepeat", "webkitMaskComposite"]);

    // ---- One playing at a time, and one way out of it. ----
    var playing = null, plain = false, lastTap = 0, applying = false;
    function applyOwn(choice) {
      applying = true;
      try { applyThemeChoice(choice); } finally { applying = false; }
    }

    function teardown(how) {
      var p = playing;
      if (!p) {
        return;
      }
      playing = null;
      if (how === "crossfade") {
        // Not begun yet: the Menu's own change paints it, crossfade and all.
        p.painted = true;
      }
      p.timers.forEach(clearTimeout);
      // "fade": the transition is under way but is not to be played (it was
      // interrupted while the stills were being made, or this browser cannot
      // drive its parts). It is left to run as the browser's own plain
      // crossfade: with our classes gone below, none of our rules hold.
      var fade = how === "fade" && p.vt && !p.settled;
      if (fade && !p.called) {
        // The stills are not taken yet: the change waits for the callback.
        p.fadePaint = true;
      }
      if (p.vt && !fade) {
        try { p.vt.skipTransition(); } catch { /* already gone */ }
      }
      // Stored at the tap; if the paint had not come yet, it comes now.
      if (!p.painted && !p.fadePaint) {
        paintThemeChoice(p.choice);
      } else if (p.bars) {
        p.bars();
      }
      var unclass = function (el, name) {
        el.classList.remove(name);
        if (el.getAttribute("class") === "") {
          el.removeAttribute("class");
        }
      };
      p.g.units.forEach(function (el, i) { unclass(el, "crown-u" + (i + 1)); });
      p.g.rows.forEach(function (el, i) { unclass(el, "crown-r" + (i + 1)); });
      [root, hero].forEach(function (host) {
        Array.prototype.slice.call(host.classList).forEach(function (c) {
          if (c.indexOf("crown-") === 0) {
            host.classList.remove(c);
          }
        });
      });
      (p.extra || []).forEach(function (a) { try { a.cancel(); } catch { /* already gone */ } });
      Array.prototype.slice.call(root.style).forEach(function (k) {
        if (k.indexOf("--crown-") === 0 && k !== "--crown-k") {
          root.style.removeProperty(k);
        }
      });
      if (p.cross) { p.cross.remove(); }
      if (p.edge) { p.edge.remove(); }
      ground.removeAttribute("style");
      back.removeAttribute("style");
      Array.prototype.forEach.call(starHost.querySelectorAll("path"), function (path) {
        path.style.removeProperty("--crown-d");
        unstyle(path);
      });
      Array.prototype.forEach.call(starHost.querySelectorAll("stop"), function (stop) {
        stop.style.removeProperty("stop-color");
        unstyle(stop);
      });
      if (p.watch) { p.watch.disconnect(); }
      p.undo.forEach(function (fn) { fn(); });
      p.g.wash.forEach(function (kv) { glow.style.removeProperty(kv[0]); });
      p.g.frieze.forEach(function (kv) { frieze.style.removeProperty(kv[0]); });
      [root, ground, back, glow, frieze].forEach(unstyle);
      if (!root.getAttribute("style")) {
        root.removeAttribute("style");
      }
      if (how === "crossfade") {
        applyOwn(p.choice);
      }
      // The way back was summed at settle; otherwise in idle time.
      prep = p.back && p.back.m === measured ? p.back : null;
      soon();
    }

    // An interruption does not cut: what is left plays out in `ms`, then it
    // is over. Before the change of theme has begun there is nothing to
    // play out, and the Menu's crossfade takes it.
    function rush(ms) {
      var p = playing;
      if (!p || p.rushing) {
        return;
      }
      if (!p.vt) {
        teardown("crossfade");
        return;
      }
      if (!p.ready) {
        teardown("fade");
        return;
      }
      p.rushing = true;
      p.timers.forEach(clearTimeout);
      p.timers = [];
      var left = (p.end - p.plan.turn - (performance.now() - p.ready) / p.k) * p.k;
      var rate = Math.max(1, left / ms);
      document.getAnimations().forEach(function (a) {
        var name = a.animationName || "";
        var pseudo = a.effect && a.effect.pseudoElement || "";
        if (name.indexOf("crown-") === 0 || pseudo.indexOf("::view-transition") === 0) {
          try { a.playbackRate = rate; } catch { /* already gone */ }
        }
      });
      setTimeout(function () {
        if (playing === p) {
          teardown();
        }
      }, ms + 30);
    }

    function later(p, ms, fn) {
      p.timers.push(setTimeout(function () {
        if (playing === p) {
          fn();
        }
      }, ms * p.k));
    }

    // ---- Set-up: everything that reads the page is done in
    // idle time after load and kept, reads before writes. The tap only
    // writes. ----

    function headUnits() {
      var out = [];
      Array.prototype.forEach.call(head.children, function (el) {
        if (!el.getClientRects().length || !el.textContent.trim()) {
          return;
        }
        var kids = Array.prototype.filter.call(el.children, function (kid) {
          return kid.textContent.trim();
        });
        var blocks = kids.length > 1 && kids.every(function (kid) {
          return getComputedStyle(kid).display === "block";
        });
        (blocks && el.tagName !== "H1" ? kids : [el]).forEach(function (unit) {
          out.push(unit);
        });
      });
      return out.length > WORDS ? [head] : out;
    }
    function readLook(cs) {
      var out = { opacity: cs.opacity };
      PAINT.forEach(function (k) { out[k] = cs[k]; });
      return out;
    }
    function readStars() {
      var svg = starHost.querySelector("svg.home-stars");
      if (!svg) {
        return [];
      }
      return Array.prototype.map.call(svg.querySelectorAll("path"), function (path) {
        var q = path.getBoundingClientRect();
        return { path: path, x: q.left + q.width / 2, y: q.top + q.height / 2, size: parseFloat(path.getAttribute("data-r")) || 1 };
      });
    }

    // The two looks (the cross's paint, the head's two grounds, the leaf's
    // highlight, the stars' leaf). They do not depend on scroll or size, so
    // they are read once: the look showing as it stands, the other by
    // turning the theme for an instant in idle time, once per page.
    var looks = {}, boxes = {};
    function readLooks(name, v, zoom) {
      var ccs = getComputedStyle(crown);
      var L = looks[name] = { crown: readLook(ccs), gilt: /gild\.png|gradient/.test(ccs.backgroundImage) };
      [["sky", sky], ["fill", fill]].forEach(function (pair) {
        var cs = getComputedStyle(pair[1]), q = pair[1].getBoundingClientRect(), out = {};
        GROUND.forEach(function (k) { out[k] = cs[k]; });
        L[pair[0]] = out;
        // (Each layer's box is taken in the theme whose ground it is.)
        if (q.width && pair[0] === (name === "dark" ? "sky" : "fill")) {
          // Where the layer lies about the vault, in its own px.
          boxes[pair[0]] = [(q.left - v.left) / zoom, (q.top - v.top) / zoom, (q.width - v.width) / zoom, (q.height - v.height) / zoom];
        }
      });
      L.hi = getComputedStyle(hero).getPropertyValue("--sky-leaf-hi").trim();
      // The frieze's ground and edge (custom properties: not mid-transition).
      var fcs = getComputedStyle(frieze);
      L.fz = [fcs.getPropertyValue("--inscription-ground").trim(), fcs.getPropertyValue("--inscription-edge").trim()];
      if (name === "dark") {
        L.inks = Array.prototype.map.call(starHost.querySelectorAll("stop"), function (stop) { return getComputedStyle(stop).stopColor; });
      }
    }

    // The page's measures, read together (idle time, and again only when a
    // ResizeObserver or the stars' arrival says they have changed).
    var measured = null, prep = null, prepTimer = null;
    function measure() {
      var hcs = getComputedStyle(hero);
      var zoom = parseFloat(hcs.zoom) || 1;
      var m = {
        sx: window.scrollX, sy: window.scrollY, zoom: zoom,
        lift: (parseFloat(hcs.getPropertyValue("--arch-lift")) || 2) * parseFloat(getComputedStyle(root).fontSize) * zoom,
        vr: vault.getBoundingClientRect(), hr: hero.getBoundingClientRect(), cr: crown.getBoundingClientRect(),
        fz: frieze.getBoundingClientRect(), W2: window.innerWidth, H2: window.innerHeight,
      };
      m.units = headUnits();
      m.unitRects = m.units.map(function (el) { return el.getBoundingClientRect(); });
      m.rows = rowUnits();
      m.rowRects = m.rows.map(function (el) { return el.getBoundingClientRect(); });
      m.last = head.getBoundingClientRect().bottom;
      Array.prototype.forEach.call(head.querySelectorAll("*"), function (el) {
        var q = el.getBoundingClientRect();
        if (q.height && el.textContent.trim() && q.bottom > m.last) {
          m.last = q.bottom;
        }
      });
      m.stars = readStars();
      m.stops = Array.prototype.slice.call(starHost.querySelectorAll("stop"));
      var cur = look(), other = cur === "dark" ? "light" : "dark";
      if (!looks[cur] || (cur === "dark" && looks.dark.inks.length !== m.stops.length)) {
        readLooks(cur, m.vr, zoom);
      }
      if (!looks[other]) {
        var attr = root.getAttribute("data-theme");
        root.setAttribute("data-theme", other);
        readLooks(other, m.vr, zoom);
        if (attr === null) {
          root.removeAttribute("data-theme");
        } else {
          root.setAttribute("data-theme", attr);
        }
      }
      return m;
    }
    function rowUnits() {
      var out = [];
      // (With the header's words: they too change ink in one move, not through
      // a half-mix in which neither ink reads.)
      [".site-header .site-brand", ".site-header details > summary", ".site-header nav", ".page-home .pray-now-label", ".page-home .home-hour-group", ".page-home .home-day-meta"].forEach(function (sel) {
        Array.prototype.forEach.call(document.querySelectorAll(sel), function (el) {
          if (el.getClientRects().length) {
            out.push(el);
          }
        });
      });
      return out.slice(0, ROWS);
    }

    // The sums for one direction: no reads. Scroll is taken off here, so a
    // scrolled page needs no new measures.
    function compute(m, dir) {
      var plan = CROWN_PLAY[dir];
      var g = { dir: dir, m: m, sx: window.scrollX, sy: window.scrollY, vars: [] };
      var ox = m.sx - g.sx, oy = m.sy - g.sy;
      var at = function (q) {
        return { left: q.left + ox, top: q.top + oy, width: q.width, height: q.height, right: q.right + ox, bottom: q.bottom + oy };
      };
      var zoom = m.zoom, lift = m.lift, vr = at(m.vr), hr = at(m.hr), cr = at(m.cr);
      var W2 = m.W2, H2 = m.H2, units = m.units, unitRects = m.unitRects.map(at), last = m.last + oy;
      var stars = m.stars.map(function (s) { return { path: s.path, x: s.x + ox, y: s.y + oy, size: s.size }; });
      var cur = dir === "dusk" ? "light" : "dark", other = dir === "dusk" ? "dark" : "light";
      g.oldLook = looks[cur].crown;
      g.newLook = looks[other].crown;
      g.gilt = looks[cur].gilt;
      g.hi = looks.dark.hi;
      // The frieze is live throughout (a still of it goes soft): it keeps its
      // old ground under the new theme until the page's front leaves it.
      g.frieze = [["--crown-fz-g0", looks[cur].fz[0]], ["--crown-fz-e0", looks[cur].fz[1]], ["--crown-fz-g1", looks[other].fz[0]], ["--crown-fz-e1", looks[other].fz[1]]];
      g.kept = looks[cur][dir === "dusk" ? "fill" : "sky"];
      g.newGround = looks[other][dir === "dusk" ? "sky" : "fill"];

      // ---- Sums. ----
      var set = function (k, val) { g.vars.push([k, val]); };
      // The vault's box. At dawn it runs to the lining's band (style.css), 8px
      // wider each side than it stands at rest.
      var grow = dir === "dawn" ? 8 * zoom : 0;
      var v = { left: vr.left - grow, top: vr.top, width: vr.width + 2 * grow, height: vr.height, bottom: vr.bottom };
      var inset = (v.left - hr.left) / zoom;
      var apex = lift + 1.0523 * inset * zoom;
      var hc = v.height - apex;
      var w = v.width;
      var a, cy, r0, ra, r;
      if (dir === "dusk") {
        // About a point 0.6 of the head above its own, twice as wide as tall.
        a = 2;
        cy = apex - 0.6 * hc;
        r0 = 0.6 * hc;
        ra = (v.height - cy) / MID;
        r = Math.sqrt(w * w / (4 * a * a) + 1.6 * hc * 1.6 * hc) / CORE + 4;
      } else {
        // A low dome about a point 0.3 of the head under the horizon.
        a = 2.5;
        cy = v.height + 0.3 * hc;
        r0 = 0.3 * hc;
        ra = (cy - apex) / MID;
        r = 1.3 * hc / CORE + 4;
      }
      r = Math.max(r, ra + 1);
      set("--crown-a", String(a));
      set("--crown-cx", px(w / 2));
      set("--crown-cy", px(cy));
      set("--crown-r0", px(r0));
      set("--crown-ra", px(ra));
      set("--crown-r", px(r));
      set("--crown-front-at", String(plan.front[0] - plan.turn));
      set("--crown-front-dur", String(plan.front[1]));
      // When the front's middle is at a height y of the viewport (ms from the
      // tap, by the plan). It reaches the far edge 0.95 of the way through.
      var frontAt = function (y) {
        var need = Math.abs(y - (v.top + cy)) / MID;
        return plan.front[0] + 0.95 * plan.front[1] * timeAt(FRONT_EASE, (need - r0) / (ra - r0));
      };

      // The page's glow: the arch closed with a round foot, one image, grown
      // by ratio about the middle of the springing line.
      var arch = archNow();
      var W = v.width + 2 * (v.left - hr.left);
      var ax = hr.left + W / 2;
      var ys = v.top + lift + arch.rise * W;
      var yc = v.bottom;
      var fw = SPREAD_FEATHER, F = 0.5;
      var s0 = 1 / (1 + fw);
      var D = Math.max(0.05, (yc - ys) / (W * s0) - F * (1 + fw));
      var shape = function (x, y) {
        if (y >= -D) {
          return arch.inside(x, y);
        }
        return 0.5 * (1 - Math.hypot((x - 0.5) / 0.5, (y + D) / F));
      };
      var corner = function (sc) {
        return Math.min.apply(null, [[0, 0], [W2, 0], [0, H2], [W2, H2]].map(function (c) {
          return shape((c[0] - ax) / (W * sc) + 0.5, (ys - c[1]) / (W * sc));
        }));
      };
      var s1 = 1;
      while (corner(s1) < fw / 2 && s1 < 60) {
        s1 += 0.05;
      }
      s1 *= 1.02;
      var wide = W2 > 700 ? SPREAD_WIDE : 0;
      var dur = plan.spread[1] + wide;
      var m0 = fw / 2 + 0.012, top = arch.rise + 2.4 * m0, deep = D + F * (1 + fw) + 0.02;
      g.mask = { arch: arch, shape: shape, fw: fw, m: m0, top: top, deep: deep, key: [W.toFixed(0), arch.rise, D.toFixed(3)].join("/") };
      // The shape's sizes. Half by ratio, half evenly, along SPREAD_EASE: that
      // is its pace for the first 400ms, which is not to change. After that
      // it runs on ahead and has covered the page by SPREAD_SHAPE_PART of the span,
      // so the end of the change everywhere is the glow's opacity coming to
      // rest (the slowest part of its curve), not a late edge arriving.
      var NODES = 8, nodes = [], vnodes = [];
      var sizeAt = function (tau) {
        var e = easeAt(SPREAD_EASE, tau / dur);
        return 0.5 * s0 * Math.pow(s1 / s0, e) + 0.5 * (s0 + e * (s1 - s0));
      };
      // The veil's shape grows half by ratio and half evenly along
      // SPREAD_EASE for its first 400 ms (the halo round the window), then
      // runs on ahead, done by VEIL_PART.
      var vDur = Math.round(dur * VEIL_PART);
      for (var vi = 0; vi <= NODES; vi += 1) {
        var tn = vDur * vi / NODES;
        vnodes.push(sizeAt(tn <= 400 ? tn : 400 + (tn - 400) * (dur - 400) / (vDur - 400)));
      }
      // The page's own shape grows by an even ratio, a little faster at
      // first (SPREAD_BEND), and is done by SPREAD_SHAPE_PART of the span:
      // it starts at once under the halo, its feather takes as long to pass
      // the foot as to pass the wall beside the arch, and it has reached
      // every place while the glow's opacity is still low. The opacity, over
      // nearly the whole span and near enough even, does the rest: every
      // place has begun early and none is finished before the end. (Before,
      // the shape kept its slow pace for 400 ms and then took the far field,
      // most of the glass, in a few frames at half opacity.)
      var shapeDur = Math.round(dur * SPREAD_SHAPE_PART), bend = SPREAD_BEND;
      for (var ni = 0; ni <= NODES; ni += 1) {
        nodes.push(s0 * Math.pow(s1 / s0, Math.pow(ni / NODES, bend)));
      }
      nodes.forEach(function (sc, i) {
        set("--crown-hw" + i, px((1 + 2 * m0) * W * sc));
        set("--crown-hh" + i, px((top + deep) * W * sc));
        set("--crown-hx" + i, px(ax - (1 + 2 * m0) * W * sc / 2));
        set("--crown-hy" + i, px(ys - top * W * sc));
        // The veil's shape leads the page's old one, so it is whole where
        // the page is half way.
        var vs = vnodes[i] * VEIL_LEAD;
        set("--crown-vw" + i, px((1 + 2 * m0) * W * vs));
        set("--crown-vh" + i, px((top + deep) * W * vs));
        set("--crown-vx" + i, px(ax - (1 + 2 * m0) * W * vs / 2));
        set("--crown-vy" + i, px(ys - top * W * vs));
      });
      g.last = [px((1 + 2 * m0) * W * s1 * 1.3) + " " + px((top + deep) * W * s1 * 1.3), px(ax - (1 + 2 * m0) * W * s1 * 1.3 / 2) + " " + px(ys - top * W * s1 * 1.3)];
      var part = SPREAD_GLOW_PART, glowEase = plan.glowEase;
      var alphaAt = function (X, Y, time) {
        var u = Math.max(0, Math.min(1, (time - plan.spread[0]) / dur));
        var f = Math.min(1, (time - plan.spread[0]) / shapeDur) * NODES, i = Math.max(0, Math.min(NODES - 1, Math.floor(f)));
        var sc = nodes[i] + (Math.max(0, f) - i) * (nodes[i + 1] - nodes[i]);
        return smooth(-fw / 2, fw / 2, shape((X - ax) / (W * sc) + 0.5, (ys - Y) / (W * sc))) * easeAt(glowEase, u / part);
      };
      var when = function (X, Y, level) {
        for (var i = 0; i <= 120; i += 1) {
          var time = plan.spread[0] + dur * i / 120;
          if (alphaAt(X, Y, time) >= level) {
            return time;
          }
        }
        return plan.spread[0] + dur;
      };
      set("--crown-spread-at", String(plan.spread[0] - plan.turn));
      set("--crown-spread-dur", String(dur));
      set("--crown-shape-dur", String(shapeDur));
      set("--crown-vshape-dur", String(vDur));
      set("--crown-glow-dur", String(Math.round(dur * part)));
      set("--crown-glow-ease", "cubic-bezier(" + glowEase.join(",") + ")");
      g.covered = Math.max.apply(null, [[0, 0], [W2, 0], [0, H2], [W2, H2], [W2 / 2, H2], [W2 / 2, 0]].map(function (c) {
        return when(c[0], c[1], 0.96);
      })) + 40;

      g.geom = { s0: s0, s1: s1, W: W };

      // The light on the horizon stops under the last line of the day block.
      var room = Math.max(0.13 * hc, Math.min(0.5 * hc, v.bottom - last - 4 * zoom));
      // (These two are all the first act needs; they go on the glow itself, so
      // the tap does not restyle the document. The rest go on the root with
      // the change of theme.)
      g.wash = [["--crown-wash-h", px(room / zoom)], ["--crown-wash-rx", px(Math.min(0.48 * w, 2.6 * room) / zoom)]];

      // The day's lines dip as the head's front reaches them.
      g.units = units;
      units.forEach(function (el, i) {
        var q = unitRects[i];
        set("--crown-w" + (i + 1), String(Math.round(frontAt(q.top + q.height * 0.6) - plan.turn)));
      });

      // The rows change ink quickly, one after another, each as the page under
      // it is half turned and never closer than 60ms to the row before: no
      // moment with every label faint at once.
      g.rows = m.rows;
      var inkAt = plan.inkAt;
      var order = m.rowRects.map(at).map(function (q, i) { return [when(q.left + q.width / 2, q.top + q.height / 2, inkAt), i]; });
      order.sort(function (p1, p2) { return p1[0] - p2[0] || p1[1] - p2[1]; });
      var mid = (order.length - 1) / 2, base = order.length ? order[Math.floor(mid)][0] : 0;
      order.forEach(function (pair, n) {
        // About the middle row's own moment, 60ms apart, but no earlier than a
        // row's own half-mix less 150ms nor later than it plus 150.
        // (Each at its own moment, where the ground under it can carry the
        // ink it is given; never two within 25ms.)
        // The rows have much the same moment now (the shape has long passed
        // them all and they share the glow's opacity), so they are dealt out
        // about the middle one, 32ms apart, none further than 130ms from its
        // own: no frame with every label mid-change.
        var t = Math.max(pair[0] - 130, Math.min(pair[0] + 130, base + (n - Math.floor(mid)) * 32));
        if (n && t < order[n - 1][2] + 25) { t = order[n - 1][2] + 25; }
        pair[2] = t;
        set("--crown-r" + (pair[1] + 1), String(Math.round(t - plan.turn)));
        var rowEl = m.rows[pair[1]];
        if (rowEl && (rowEl.matches(".is-current") || rowEl.querySelector(".is-current"))) {
          set("--crown-dn" + (pair[1] + 1), "crown-ink-dim-soft");
          set("--crown-un" + (pair[1] + 1), "crown-ink-undim-soft");
        }
      });
      g.lastDip = order.length ? order[order.length - 1][2] : 0;
      // The stills go when the page has visibly turned and the last row has
      // its new ink; the veil is timed to be gone by then.
      g.settle = Math.max(g.covered, g.lastDip + 220, plan.spread[0] + 1100, plan.veil[1] - 80, plan.spread[0] + dur * part - 50) + 80;
      // (The settle now comes near the plan's end: the play is over a little
      // after it, never before.)
      g.end = Math.max(plan.end, g.covered + 20, g.settle + 90);
      set("--crown-veil-at", String(plan.veil[0] - plan.turn));
      set("--crown-veil-dur", String(plan.veil[1] - plan.veil[0]));
      // Where the veil is densest (the springing line's middle) and how far
      // it thins, on the glass.
      set("--crown-ax", px(ax)); set("--crown-ys", px(ys)); set("--crown-aw", px(W));
      set("--crown-lining-at", String(Math.round(when(vr.left, vr.top + vr.height * 0.6, 0.5) - plan.turn)));

      // Each star's turn. Dusk: ms from the change of theme, in the wake of
      // the dark, the largest first. Dawn: ms from the tap, ahead of the light.
      g.starTurns = function (list) {
        var sizes = list.map(function (s) { return s.size; });
        var big = Math.max.apply(null, sizes), small = Math.min.apply(null, sizes);
        var nearest = -1, nearestD = Infinity;
        list.forEach(function (s, i) {
          var d = Math.hypot(s.x - (cr.left + cr.width / 2), s.y - (cr.top + cr.height / 2));
          if (d < nearestD) { nearestD = d; nearest = i; }
        });
        return list.map(function (s, i) {
          var rank = big > small ? (big - s.size) / (big - small) : 0;
          var seed = ((i * 7919) % 97) / 97;
          var t;
          if (dir === "dusk") {
            t = frontAt(s.y) + 160 + 620 * rank + 260 * seed;
            // A star out in the moulding's margin (outside the inner lining,
            // by the arch's own sums) waits for the wall's night.
            if (arch.inside((s.x - hr.left) / W, (ys - s.y) / W) < (v.left - hr.left + 9 * zoom) / W) {
              t = Math.max(t, when(s.x, s.y, 0.5) + 200);
            }
            return Math.round(Math.min(t, g.end - 1050) - plan.turn);
          }
          t = frontAt(s.y) - 850 - 450 * rank - 160 * seed;
          t = Math.max(150, Math.min(t, 1650));
          return Math.round(i === nearest ? 1750 : t);
        });
      };
      g.stars = stars;
      g.turns = g.starTurns(stars);
      g.stops = dir === "dawn" ? m.stops : [];
      g.inks = looks.dark.inks || [];

      // Where the stand-ins lie (each layer's box about the vault's).
      var box = function (d) {
        return {
          left: (d[0] + (vr.left - v.left) / zoom).toFixed(2) + "px", top: d[1].toFixed(2) + "px",
          width: (vr.width / zoom + d[2]).toFixed(2) + "px", height: (vr.height / zoom + d[3]).toFixed(2) + "px",
        };
      };
      g.backBox = box(boxes[dir === "dusk" ? "fill" : "sky"] || [0, 0, 0, 0]);
      g.groundBox = box(boxes[dir === "dusk" ? "sky" : "fill"] || [0, 0, 0, 0]);
      return g;
    }

    // The glow's mask: one small image (the feather is wide; 120px across is
    // plenty), made once per geometry and kept as a blob. The head is
    // genarch's (ARCHES); the foot is as deep as this page stands below the
    // springing, so the whole is drawn here.
    var maskCache = {};
    function maskFor(g, sync) {
      var k = g.mask.key, hit = maskCache[k];
      if (hit && (hit.url || !sync)) {
        return hit.url;
      }
      // One mask at a time: the last one's blob is given back.
      Object.keys(maskCache).forEach(function (old) {
        if (maskCache[old].blob) {
          URL.revokeObjectURL(maskCache[old].blob);
        }
        delete maskCache[old];
      });
      var mk = g.mask, NX = 120, q = NX / (1 + 2 * mk.m), NY = Math.ceil((mk.top + mk.deep) * q);
      var cv = document.createElement("canvas");
      cv.width = NX;
      cv.height = NY;
      var ctx = cv.getContext("2d");
      var img = ctx.createImageData(NX, NY);
      for (var row = 0; row < NY; row += 1) {
        var y = mk.top - (row + 0.5) / q;
        for (var i = 0; i < NX; i += 1) {
          img.data[(row * NX + i) * 4 + 3] = Math.round(255 * smooth(-mk.fw / 2, mk.fw / 2, mk.shape((i + 0.5) / q - mk.m, y)));
        }
      }
      ctx.putImageData(img, 0, 0);
      hit = maskCache[k] = { url: null };
      if (sync || !cv.toBlob) {
        hit.url = 'url("' + cv.toDataURL("image/png") + '")';
      } else {
        cv.toBlob(function (blob) {
          if (blob && !hit.url && maskCache[k] === hit) {
            hit.blob = URL.createObjectURL(blob);
            hit.url = 'url("' + hit.blob + '")';
          }
        });
      }
      return hit.url;
    }

    function dirNow() {
      return look() === "dark" ? "dawn" : "dusk";
    }
    function ready() {
      if (playing) {
        return prep;
      }
      if (!measured) {
        measured = measure();
        prep = null;
      }
      reveal();
      var dir = dirNow();
      if (!prep || prep.dir !== dir || prep.m !== measured || prep.sx !== window.scrollX || prep.sy !== window.scrollY) {
        prep = compute(measured, dir);
        maskFor(prep, false);
      }
      return prep;
    }
    function soon() {
      clearTimeout(prepTimer);
      prepTimer = setTimeout(function () {
        if (window.requestIdleCallback) {
          window.requestIdleCallback(ready, { timeout: 2000 });
        } else {
          ready();
        }
      }, 300);
    }
    // New measures are wanted (a resize, late fonts, the stars arriving).
    function stale() {
      measured = null;
      soon();
    }
    function unstyle(el) {
      if (el.getAttribute("style") === "") {
        el.removeAttribute("style");
      }
    }

    function crossLooks(p, g) {
      var cross = document.createElement("span");
      cross.className = "crown-cross";
      cross.setAttribute("aria-hidden", "true");
      var mk = function (cls, lookNow) {
        var s = document.createElement("span");
        s.className = cls;
        if (lookNow) {
          copy(s, lookNow, PAINT);
          s.style.setProperty("--crown-op", lookNow.opacity);
        }
        cross.appendChild(s);
        return s;
      };
      mk("crown-cross-old", g.oldLook).style.opacity = g.oldLook.opacity;
      mk("crown-cross-new", g.newLook);
      var catchLight = mk("crown-cross-catch", null);
      if (g.hi) {
        catchLight.style.setProperty("--crown-catch-ink", g.hi);
      }
      if (p.dir === "dusk" && g.gilt) {
        hero.classList.add("crown-gilt");
      }
      crown.insertAdjacentElement("afterend", cross);
      p.cross = cross;
    }
    function setTurns(list, turnsNow, since) {
      list.forEach(function (s, i) {
        s.path.style.setProperty("--crown-d", String(Math.round(turnsNow[i] - (since || 0))));
      });
    }
    // The stars' leaf is the night's: at dawn it must not take the day's
    // tokens when the theme turns under them.
    function pinStars() {
      var stops = Array.prototype.slice.call(starHost.querySelectorAll("stop"));
      var inks = stops.map(function (stop) { return getComputedStyle(stop).stopColor; });
      stops.forEach(function (stop, i) { stop.style.stopColor = inks[i]; });
    }

    function play(to, choice) {
      var dir = to === "dark" ? "dusk" : "dawn";
      var g = ready();
      if (!g || g.dir !== dir) {
        applyOwn(choice);
        return;
      }
      writeStoredTheme(choice);
      var plan = CROWN_PLAY[dir];
      var p = playing = {
        dir: dir, choice: choice, plan: plan, k: K(), t0: performance.now(), g: g,
        timers: [], undo: [], painted: false, vt: null, ready: 0,
        covered: g.covered, end: g.end, geom: g.geom,
      };
      // Writes only from here.
      g.wash.forEach(function (kv) { glow.style.setProperty(kv[0], kv[1]); });
      g.frieze.forEach(function (kv) { if (kv[1]) { frieze.style.setProperty(kv[0], kv[1]); } });
      g.units.forEach(function (el, i) { el.classList.add("crown-u" + (i + 1)); });
      g.rows.forEach(function (el, i) { el.classList.add("crown-r" + (i + 1)); });
      setTurns(g.stars, g.turns, 0);
      crossLooks(p, g);
      // The stars' leaf is the night's: at dawn it must not take the day's
      // tokens when the theme turns under them.
      g.stops.forEach(function (stop, i) { stop.style.stopColor = g.inks[i]; });
      // (On the hero, not the root: the tap restyles the arch, not the page.)
      hero.classList.add("crown-on", "crown-" + dir);
      // A star field that lands late (a tap before stars.json has come) takes
      // its turns from where the clock stands.
      p.watch = new MutationObserver(function () {
        if (playing !== p) {
          return;
        }
        g.stars = readStars();
        var clock = dir === "dusk" ? (p.ready ? (performance.now() - p.ready) / p.k : 0) : (performance.now() - p.t0) / p.k;
        setTurns(g.stars, g.starTurns(g.stars), clock);
        if (dir === "dawn") {
          pinStars();
        }
        measured = null;
      });
      p.watch.observe(starHost, { childList: true });

      later(p, plan.turn, function () {
        g.vars.forEach(function (kv) { root.style.setProperty(kv[0], kv[1]); });
        root.style.setProperty("--crown-m-arch", maskFor(g, true));
        // Under the stills, the ground of the page that is leaving: a browser
        // that loses a still shows that, never its own white.
        root.style.setProperty("--crown-floor", getComputedStyle(document.body).backgroundColor);
        root.classList.add("crown-vt", "crown-" + dir);
        // The first act holds its breath while the browser makes its stills,
        // and goes on from where it was: nothing jumps when they are up.
        var held = document.getAnimations().filter(function (a) {
          return (a.animationName || "").indexOf("crown-") === 0 && a.playState === "running";
        });
        held = held.map(function (a) {
          a.pause();
          return { a: a, at: a.currentTime };
        });
        var resume = function () {
          // (Only what is still paused: one the play's end has cancelled
          // would start again from nothing.) By start time, anchored to
          // where each was held, on the clock of the frame that shows it:
          // play() would leave them pending through the transition's first,
          // heaviest frames; the timeline's own clock is still the frame's
          // before the stills (the act would leap the hold), and
          // performance.now() is past the frame being drawn (it would step
          // back). The frame's own time is neither.
          var mine = held;
          held = [];
          if (!mine.length) return;
          requestAnimationFrame(function (ts) {
            mine.forEach(function (h) {
              try {
                if (playing === p && h.a.playState === "paused") {
                  if (typeof ts === "number" && typeof h.at === "number") {
                    h.a.startTime = ts - h.at / (h.a.playbackRate || 1);
                  } else {
                    h.a.play();
                  }
                }
              } catch { /* already gone */ }
            });
          });
        };
        var veil = document.createElement("div");
        veil.className = "crown-veil";
        veil.setAttribute("aria-hidden", "true");
        p.edge = veil;
        var vt = p.vt = document.startViewTransition(function () {
          p.called = true;
          if (playing !== p) {
            if (p.fadePaint) {
              paintThemeChoice(choice);
            }
            return;
          }
          // The browser's own bars turn with the page, not at this frame.
          var metas = Array.prototype.slice.call(document.querySelectorAll('meta[name="theme-color"]'));
          var was = metas.map(function (mt) { return mt.getAttribute("content"); });
          paintThemeChoice(choice);
          p.painted = true;
          var will = metas.map(function (mt) { return mt.getAttribute("content"); });
          metas.forEach(function (mt, i) { mt.setAttribute("content", was[i]); });
          // (Their turn comes mid-spread, or at the end if it never does; the
          // theme itself is painted once.)
          p.bars = function () {
            metas.forEach(function (mt, i) { mt.setAttribute("content", will[i]); });
            p.bars = null;
          };
          root.classList.add("crown-new");
          hero.classList.add("crown-new");
          copy(ground, g.groundBox, ["left", "top", "width", "height"]);
          copy(ground, g.newGround, GROUND);
          copy(back, g.backBox, ["left", "top", "width", "height"]);
          copy(back, g.kept, GROUND);
          document.body.appendChild(veil);
        });
        // One clock: everything from here is timed from the transition's
        // first frame, however long the browser took to make its stills.
        vt.ready.then(function () {
          if (playing !== p) {
            return;
          }
          p.ready = performance.now();
          // Can this browser drive the transition's parts from script? The
          // rush and the last top-up need both of these; a browser that takes
          // the calls and does nothing (no throw) gets its own plain
          // crossfade, now and for the rest of the page's life.
          var able;
          try {
            var trial = root.animate({ opacity: [1, 1] }, { duration: 1, pseudoElement: "::view-transition-new(root)" });
            able = !!(trial && trial.effect && trial.effect.pseudoElement === "::view-transition-new(root)") &&
              document.getAnimations().some(function (a) {
                return a !== trial && a.effect && String(a.effect.pseudoElement || "").indexOf("::view-transition") === 0;
              });
            trial.cancel();
            // And the page that is leaving must be standing there as a still
            // for the whole of the first act: no box for it, no play.
            able = able && ["::view-transition-group(root)", "::view-transition-old(root)"].every(function (pseudo) {
              var box = getComputedStyle(root, pseudo);
              return parseFloat(box.width) > 0 && parseFloat(box.height) > 0 && box.display !== "none" && box.opacity === "1";
            });
          } catch {
            able = false;
          }
          if (!able) {
            // The stills are taken under our names, and to drop the names now
            // would end the transition in a cut. So the names stay for the
            // quarter second of a plain crossfade (style.css, .crown-plain: rules
            // only, nothing driven from script), the live page beneath is
            // run to its settled look, and then everything goes.
            plain = true;
            p.timers.forEach(clearTimeout);
            p.timers = [];
            root.classList.add("crown-plain");
            held.forEach(function (h) { try { h.a.finish(); } catch { /* already gone */ } });
            held = [];
            try {
              document.getAnimations().forEach(function (a) {
                var pseudo = a.effect && a.effect.pseudoElement || "";
                if ((a.animationName || "").indexOf("crown-") === 0 && String(pseudo).indexOf("::view-transition") !== 0) {
                  try { a.finish(); } catch { /* already gone */ }
                }
              });
            } catch { /* already gone */ }
            p.settled = true;
            var done = function () {
              if (playing === p) {
                teardown();
              }
            };
            vt.finished.then(done, done);
            p.timers.push(setTimeout(done, 1200));
            return;
          }
          resume();
          later(p, plan.spread[0] + 0.45 * plan.spread[1] - plan.turn, function () {
            if (p.bars) {
              p.bars();
            }
          });
          // The stills go when the page has visibly turned, not when the
          // shape has finished growing off-screen: from then the page is live.
          // The last few hundredths are brought up over 250ms first, so
          // dropping the stills changes nothing on the glass.
          var settle = g.settle - plan.turn;
          later(p, settle - 250, function () {
            var opts = { duration: 250 * p.k, fill: "forwards", easing: "ease-out" };
            p.extra = [];
            try {
              opts.pseudoElement = "::view-transition-new(root)";
              p.extra.push(root.animate({ maskSize: [g.last[0]], webkitMaskSize: [g.last[0]], maskPosition: [g.last[1]], webkitMaskPosition: [g.last[1]] }, opts));
              opts = { duration: 250 * p.k, fill: "forwards", easing: "ease-out", pseudoElement: "::view-transition-new(crown-veil)" };
              p.extra.push(root.animate({ opacity: [0] }, opts));
            } catch { /* already gone */ }
          });
          later(p, settle, function () {
            try { vt.skipTransition(); } catch { /* already gone */ }
          });
        }, function () {
          // The browser would not play it (a resize while it made its
          // stills, a hidden tab): the change is simply made.
          resume();
          if (playing === p) {
            teardown();
          }
        });
        var over = function () {
          if (playing !== p) {
            return;
          }
          root.classList.remove("crown-vt");
          veil.remove();
          p.settled = true;
          if (p.rushing || !p.ready) {
            teardown();
            return;
          }
          // A tap on the cross while the page was turning was kept: the way
          // back plays now.
          if (p.again) {
            // (Not a tap of its own: one that follows closely still counts.)
            var tapped = lastTap;
            teardown();
            toggle();
            lastTap = tapped;
            return;
          }
          // From here the page is its live self: the vault's clip and its
          // copied ground are done with (the clip cut the star by the cross
          // and showed as a hairline inside the lining until the end).
          hero.classList.add("crown-set");
          // The way back is summed now, so a second tap is as cheap as the first.
          setTimeout(function () {
            if (playing === p && measured === g.m) {
              if (dir === "dusk" && (!looks.dark.inks || looks.dark.inks.length !== g.m.stops.length)) {
                looks.dark.inks = g.m.stops.map(function (stop) { return getComputedStyle(stop).stopColor; });
              }
              p.back = compute(g.m, dir === "dusk" ? "dawn" : "dusk");
            }
          }, 0);
          var remains = p.end - plan.turn - (performance.now() - p.ready) / p.k;
          if (remains > 0) {
            later(p, remains, teardown);
          } else {
            teardown();
          }
        };
        vt.finished.then(over, over);
      });
    }

    // ---- Everything that ends it early. ----
    var cut = function () { teardown(); };
    var soft = function () { rush(RUSH); };
    // A scroll: the old page's still cannot move with the page, and every
    // named part does, so to run the rest out under it tears the picture.
    // The stills go at once. (Before the change of theme, as any other.)
    var scrolled = function () {
      var p = playing;
      if (p && p.vt && p.ready && !p.settled) {
        teardown();
      } else {
        soft();
      }
    };
    // A new viewport may be a new layout: the looks are read again too (one
    // probe), as they are not for anything less.
    window.addEventListener("resize", function () {
      looks.light = looks.dark = null;
      stale();
    });
    if (window.ResizeObserver) {
      var sizes = new ResizeObserver(function () {
        measured = null;
        if (!playing) {
          soon();
        }
      });
      sizes.observe(hero);
      sizes.observe(head);
    }
    ["pagehide", "pageswap", "resize", "orientationchange"].forEach(function (type) {
      window.addEventListener(type, cut);
    });
    window.addEventListener("pageshow", function (e) {
      if (e.persisted) {
        cut();
      }
    });
    document.addEventListener("visibilitychange", cut);
    // Any scroller, not only the window: scroll does not bubble.
    document.addEventListener("scroll", scrolled, { passive: true, capture: true });
    window.addEventListener("touchmove", soft, { passive: true });
    if (device && device.addEventListener) {
      device.addEventListener("change", function () {
        soft();
        label();
      });
    }
    // A reader who asks for stillness or contrast while it plays gets it.
    [still, contrast, forced].forEach(function (q) {
      if (q && q.addEventListener) {
        q.addEventListener("change", function () {
          if (q.matches) {
            cut();
          }
          // Forced colours coming or going take the button with them, as a
          // load under them would.
          if (q === forced && setUp) {
            tap.hidden = q.matches;
          }
        });
      }
    });
    new MutationObserver(function () {
      if (!playing) {
        soon();
      }
    }).observe(root, { attributes: true, attributeFilter: ["data-theme"] });
    new MutationObserver(function () {
      if (!playing) {
        stale();
      }
    }).observe(starHost, { childList: true });
    var panels = Array.prototype.slice.call(document.querySelectorAll("details.site-menu, details.site-settings, details.home-date-nav"));
    panels.forEach(function (d) {
      d.addEventListener("toggle", function () {
        if (d.open) {
          soft();
        }
      });
    });
    // An open panel a reader can see (the wide header keeps the nav open
    // without showing a panel: its summary is not displayed).
    function panelOpen() {
      return panels.some(function (d) {
        var summaryEl = d.querySelector(":scope > summary");
        return d.open && summaryEl && summaryEl.getClientRects().length > 0;
      });
    }

    // While the stills are up the browser sends every pointer to the root,
    // whatever pointer-events says. So a press anywhere but the cross runs
    // the rest of the play out, and a click that lands on the root is given
    // at once to the control under the point, found by its box, with its
    // modifiers: no waiting for the transition to end.
    function overTap(x, y) {
      var q = tap.getBoundingClientRect();
      return x >= q.left && x <= q.right && y >= q.top && y <= q.bottom;
    }
    function controlAt(x, y) {
      var found = null;
      Array.prototype.forEach.call(document.querySelectorAll("a[href], button, summary, input, select, textarea, label"), function (el) {
        var q = el.getBoundingClientRect();
        if (q.width && q.height && x >= q.left && x <= q.right && y >= q.top && y <= q.bottom && !el.hidden && !el.disabled) {
          found = el;
        }
      });
      return found;
    }
    // The hand-over is made from pointerup, not from the click: a browser
    // need not send a click to a root that is not interactive (iOS Safari
    // has not always), and pointerup comes whatever the target. The click
    // that may follow is swallowed, so nothing is given twice.
    // Modified and middle clicks on a link open a tab, in the foreground
    // (shift too: a tab, not a window; alt is not treated as a download), and
    // a right-click while the stills are up gets the page's menu, not the
    // link's. Both are accepted for the few seconds this lasts.
    var press = null, handed = null, given = null;
    function give(x, y, e, aux) {
      var el = controlAt(x, y);
      if (!el) {
        return false;
      }
      if (el === tap) {
        // Pressed while the page was turning: kept, or (let go after the
        // settle) the way back at once.
        if (!aux) {
          toggle();
        }
        return true;
      }
      var link = el.closest("a[href]");
      if (link && (aux || e.ctrlKey || e.metaKey || e.shiftKey)) {
        window.open(link.href, "_blank", "noopener");
        return true;
      }
      if (!aux) {
        // What opens a panel ends the play first: the panel's links are
        // rows of the transition and its ground is not, so for a few frames
        // they would hang on the vault without it.
        if (el.tagName === "SUMMARY" && playing) {
          teardown();
        }
        // Focus goes where a real click would have put it.
        try { el.focus({ preventScroll: true }); } catch { /* already gone */ }
        given = el;
        el.click();
      }
      return true;
    }
    document.addEventListener("pointerdown", function (e) {
      press = null;
      if (playing && playing.vt && !playing.settled) {
        press = { id: e.pointerId, x: e.clientX, y: e.clientY, button: e.button };
        if (!overTap(e.clientX, e.clientY)) {
          rush(250);
        }
      }
    }, true);
    document.addEventListener("pointercancel", function () {
      press = null;
    }, true);
    document.addEventListener("pointerup", function (e) {
      var pr = press;
      press = null;
      if (!pr || pr.id !== e.pointerId || pr.button !== e.button || (e.button !== 0 && e.button !== 1) ||
          Math.hypot(e.clientX - pr.x, e.clientY - pr.y) > 10) {
        return;
      }
      given = null;
      if (give(e.clientX, e.clientY, e, e.button === 1)) {
        handed = { t: performance.now(), x: e.clientX, y: e.clientY, el: given };
        // This pointerup names the root, not what was pressed: the page's
        // own listeners would read it as a tap outside (and shut the Menu
        // this press has just opened). The click handed over speaks for it.
        e.stopPropagation();
      }
    }, true);
    var near = function (e) {
      return handed && e.isTrusted && performance.now() - handed.t < 700 && Math.hypot(e.clientX - handed.x, e.clientY - handed.y) < 12;
    };
    // After a touch the browser sends its mouse events too, to the root: the
    // mousedown would take the focus the hand-over has just placed.
    document.addEventListener("mousedown", function (e) {
      if (near(e) && e.target === root) {
        e.preventDefault();
      }
    }, true);
    var handOn = function (e) {
      // Only the middle button is an auxiliary click to hand over: a
      // right-click is the browser's.
      if (e.type === "auxclick" && e.button !== 1) {
        return;
      }
      if (handed && e.isTrusted && performance.now() - handed.t < 700 && Math.hypot(e.clientX - handed.x, e.clientY - handed.y) < 12) {
        // The browser's own click for a press already handed over.
        var was = handed.el;
        handed = null;
        if (was && was.isConnected && document.activeElement !== was && (document.activeElement === document.body || !document.activeElement)) {
          try { was.focus({ preventScroll: true }); } catch { /* already gone */ }
        }
        e.preventDefault();
        e.stopPropagation();
        return;
      }
      // (A click on the root with no press of ours behind it: handed over
      // all the same.)
      if (e.target !== root || !e.isTrusted) {
        return;
      }
      if (give(e.clientX, e.clientY, e, e.type === "auxclick")) {
        e.preventDefault();
        e.stopPropagation();
      }
    };
    document.addEventListener("click", handOn, true);
    document.addEventListener("auxclick", handOn, true);

    function toggle() {
      if (playing) {
        // Locked only while the page is turning; once it has visibly
        // settled the same tap is the way back, whatever is still twinkling.
        if (!playing.settled) {
          // Kept, once: the way back plays at the settle. A further tap
          // changes the reader's mind again. (Not the taps of a double tap
          // or a flurry: only one that comes a moment after the last.)
          // And only once the stills are up: before that the page has not
          // begun to turn, and a slow second tap of a double tap is not a
          // change of mind. (A way back's own opening is another matter: it
          // began by itself, its tap is long past, and a tap there is a new
          // one.)
          var now = performance.now();
          if ((playing.ready || now - lastTap > 2 * TAP_APART) && now - lastTap > TAP_APART) {
            playing.again = !playing.again;
          }
          lastTap = now;
          return;
        }
        teardown();
      }
      lastTap = performance.now();
      var stored = effectiveThemeChoice();
      var after = afterCrownTap(stored, deviceLook() === "dark", turned && turned.wrote === stored ? turned.from : null);
      var choice = after.choice;
      var to = lookOf(choice);
      turned = after.restore ? { from: after.restore, wrote: choice } : null;
      // The Menu's own change, crossfade and all: for a browser without view
      // transitions, a reader who asked for stillness or contrast, a black
      // day (no sky is played over Good Friday or All Souls), and a page
      // with a panel open over it.
      if (!document.startViewTransition || (still && still.matches) || (contrast && contrast.matches) ||
          hero.classList.contains("day-color-black") || panelOpen() || plain) {
        applyOwn(choice);
        return;
      }
      // (The choice is kept from the tap: a reader who goes straight on to
      // "Pray" has still chosen.)
      play(to, choice);
    }

    // A click, never a press; and not the end of a drag (the cross sits where
    // a thumb begins a pull).
    var down = null;
    tap.addEventListener("pointerdown", function (e) {
      down = { x: e.clientX, y: e.clientY, moved: false };
    });
    tap.addEventListener("pointermove", function (e) {
      if (down && Math.hypot(e.clientX - down.x, e.clientY - down.y) > 8) {
        down.moved = true;
      }
    });
    tap.addEventListener("pointercancel", function () {
      down = null;
    });
    tap.addEventListener("click", function (e) {
      var dragged = down && (down.moved || Math.hypot(e.clientX - down.x, e.clientY - down.y) > 8);
      down = null;
      if (!dragged) {
        toggle();
      }
    });

    // Any other route to a change of theme ends the play first
    // (applyThemeChoice); the play's own calls to it are fenced.
    crownPlay = {
      end: function () {
        if (playing && !applying) {
          teardown();
        }
      },
    };
    if (document.readyState === "complete") {
      stale();
    } else {
      window.addEventListener("load", stale);
    }
  })();
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
      // state it will leave.
      var open = !details.open;
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
  function wordsOnScreen() {
    var walker = document.createTreeWalker(content, NodeFilter.SHOW_TEXT);
    var range = document.createRange();
    var total = 0;
    var first = -1;
    var through = 0;
    var bottom = window.innerHeight;
    for (var node = walker.nextNode(); node; node = walker.nextNode()) {
      var words = (node.nodeValue.match(/\S*[A-Za-z0-9]\S*/g) || []).length;
      if (!words) continue;
      range.selectNodeContents(node);
      var rects = range.getClientRects();
      if (!rects.length) continue;
      var top = rects[0].top;
      var low = rects[rects.length - 1].bottom;
      if (low > 0 && top < bottom) {
        if (first < 0) first = total;
        through = total + words;
      }
      total += words;
    }
    // Nothing on screen: above the text, none of it is passed; below, all of it.
    if (first < 0) first = through = content.getBoundingClientRect().bottom <= 0 ? total : 0;
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
