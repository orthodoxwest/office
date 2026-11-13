// Chant scores: neuma lays each out at its column's width, as the apps do, and
// again when that or the text size changes. The page carries the score at one
// width for a browser that can't run the engine (render-html chant.rs), and
// that score stands wherever this can't run: the engine is fetched only once a
// score is shown at a width it was not engraved at, and only by a browser that
// can run it.
//
// A score holds its ground. The column's width is kept in a cookie, and the
// server engraves the next page's scores at it, so they arrive laid out and
// nothing is drawn again. Where a score must be laid out again on arriving (a
// first visit, an offline copy made at another width), one on screen keeps its
// first drawing until it leaves the screen, and one off it changes with the
// reader's place held (leader.js), so the words being read never move.
//
// A module script (layout.html), so a browser without modules never parses the
// dynamic import below, and app.js stays readable to every browser.
(function () {
  // neuma.mjs is modern JavaScript (class static blocks, private methods).
  // Every engine that answers HTMLScriptElement.supports("importmap") runs it
  // (Chrome 96, Firefox 108, Safari 16.4; static blocks came in 94, 93 and
  // 16.4), and one that doesn't is spared the download.
  if (!("ResizeObserver" in window) || typeof WebAssembly !== "object") return;
  if (!(window.HTMLScriptElement && HTMLScriptElement.supports && HTMLScriptElement.supports("importmap"))) return;
  var engineURL = document.querySelector('meta[name="neuma"]');
  var metricsURL = document.querySelector('meta[name="neuma-metrics"]');
  var boxes = Array.from(document.querySelectorAll(".chant-width[data-gabc]"));
  if (!boxes.length || !engineURL) return;
  // Staff spaces per lyric em and SVG units per staff space, as chant.rs sets them.
  var LYRIC_SPACES = 2.45;
  var SCALE = 6;
  var engine = null;
  var attempts = 0;
  var serial = 0;
  var states = new Map();
  var font = "google";
  // The width in SVG units each score is drawn at: the server's, until laid out here.
  var shown = new Map(boxes.map(function (box) { return [box, parseFloat(box.getAttribute("data-chant-units")) || 0]; }));
  // Scores laid out once since the page arrived; and new drawings waiting for
  // their score to leave the screen.
  var settled = new Set();
  var waiting = new Map();
  var COOKIE = "chant-width";
  var told = null;

  // The score's column, in lyric ems and in SVG units, or null while unshown.
  var measure = function (box) {
    var score = box.parentNode;
    var width = score.clientWidth;
    if (!width) return null;
    var em = parseFloat(getComputedStyle(score).fontSize);
    return { em: width / em, units: (width / em) * LYRIC_SPACES * SCALE, px: em, column: width };
  };

  // The server engraves the next page's scores at this column (chant.rs at_width),
  // to the nearest sixteenth of an em (chant.rs WIDTH_STEPS): within 1/32 em,
  // 0.46 units, of the column, so the score it sends fits as one laid out here.
  var tell = function (em) {
    var value = (Math.round(em * 16) / 16).toFixed(4);
    if (value === told) return;
    told = value;
    document.cookie = COOKIE + "=" + value + ";path=/;max-age=31536000;SameSite=Lax";
  };

  // A sliver of a unit changes no line; a score within it is left alone.
  var fits = function (box, units) {
    return Math.abs((shown.get(box) || 0) - units) < 0.5;
  };

  var load = function () {
    if (!engine) {
      // A failed fetch (offline, say) is tried again the next time a score is
      // shown or resized. A document keeps a failed module for its URL, so each
      // try names it afresh; the fragment never reaches the cache or server.
      // The lyrics are measured in the face the page draws them in (the
      // app's own Garamond, as chant.rs and the apps measure them), so the
      // engine waits for its metrics; without them it measures Google's.
      var metrics = metricsURL
        ? fetch(metricsURL.content).then(function (r) { return r.ok ? r.arrayBuffer() : null; }).catch(function () { return null; })
        : Promise.resolve(null);
      engine = Promise.all([import(engineURL.content + "#" + attempts++), metrics]).then(function (got) {
        var neuma = got[0];
        return neuma.init().then(function () {
          if (got[1]) {
            try {
              neuma.setLyricMetrics(new Uint8Array(got[1]));
              font = "custom";
            } catch {
              // An unreadable table: Google's metrics, as before.
            }
          }
          return neuma;
        });
      });
      engine.catch(function () { engine = null; });
    }
    return engine;
  };

  var onScreen = function (element) {
    var rect = element.getBoundingClientRect();
    return rect.bottom > 0 && rect.top < window.innerHeight;
  };

  var show = function (box, drawn) {
    waiting.delete(box);
    shown.set(box, drawn.units);
    var draw = function () {
      box.style.width = (drawn.width / (LYRIC_SPACES * SCALE)).toFixed(3) + "em";
      box.style.aspectRatio = drawn.width.toFixed(2) + "/" + drawn.height.toFixed(2);
      box.innerHTML = drawn.svg;
      // The score's group carries the words it sets, not its SVG.
      var svg = box.firstElementChild;
      svg.removeAttribute("role");
      svg.removeAttribute("aria-label");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("focusable", "false");
    };
    if (window.officeHoldPlace) window.officeHoldPlace(draw);
    else draw();
  };

  // A score waiting to be drawn anew is drawn once it is off the screen.
  var away = "IntersectionObserver" in window ? new IntersectionObserver(function (entries) {
    entries.forEach(function (entry) {
      var box = byScore.get(entry.target);
      if (entry.isIntersecting || !waiting.has(box)) return;
      away.unobserve(entry.target);
      show(box, waiting.get(box));
    });
  }) : null;

  var layOut = function (neuma, box, first) {
    var column = measure(box);
    if (!column || fits(box, column.units)) return;
    var state = states.get(box);
    if (!state) {
      // As chant.rs's page_options, and the apps' Score.kt and Score.swift: the mode's
      // figure (the opening score's annotation header) over the initial, as the engine's default.
      var chant = new neuma.Chant(box.getAttribute("data-gabc"), {
        initial: box.getAttribute("data-chant-initial") === "1" ? 1 : 0,
        font: font,
        everyHyphen: true,
        initialScale: 0.5,
        initialMargin: 0,
        lyricDrop: 2.6,
        shortLastStaff: true,
        phraseLines: box.getAttribute("data-chant-phrases") === "1",
      });
      state = { chant: chant, prefix: box.getAttribute("data-chant-prefix") + "c" + serial++ };
      states.set(box, state);
    }
    var page = state.chant.layout(column.units, { scale: SCALE, prefix: state.prefix, ids: false, style: false });
    var drawn = { width: page.width, height: page.height, svg: page.svg, units: column.units };
    page.free();
    // Arriving, a score the reader can see keeps its first drawing until it
    // leaves the screen, unless the new one stands exactly in its place. A
    // change the reader makes (the window, the text size) draws it at once.
    var height = Math.min((drawn.width / (LYRIC_SPACES * SCALE)) * column.px, column.column) * drawn.height / drawn.width;
    if (first && away && onScreen(box.parentNode) && Math.abs(height - box.getBoundingClientRect().height) >= 1) {
      waiting.set(box, drawn);
      away.observe(box.parentNode);
      return;
    }
    show(box, drawn);
  };

  var pending = new Set();
  var scheduled = false;
  var run = function () {
    scheduled = false;
    var due = [];
    pending.forEach(function (box) {
      var column = measure(box);
      if (!column) return;
      tell(column.em);
      var first = !settled.has(box);
      settled.add(box);
      var held = waiting.get(box);
      if (held && Math.abs(held.units - column.units) >= 0.5) {
        // The column changed while the new drawing waited: it is stale.
        waiting.delete(box);
        if (away) away.unobserve(box.parentNode);
      }
      // Engraved at this column already, by the server or here.
      if (fits(box, column.units)) {
        if (held) { waiting.delete(box); if (away) away.unobserve(box.parentNode); }
        return;
      }
      if (!waiting.has(box)) due.push({ box: box, first: first });
    });
    pending.clear();
    if (!due.length) return;
    load().then(function (neuma) {
      due.forEach(function (one) {
        try {
          layOut(neuma, one.box, one.first);
        } catch {
          // This score keeps what it shows; the others are laid out still.
        }
      });
    }, function () {
      // No engine: the page's own scores stand.
    });
  };
  var schedule = function (box) {
    pending.add(box);
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(run);
  };

  var byScore = new Map(boxes.map(function (box) { return [box.parentNode, box]; }));
  var observer = new ResizeObserver(function (entries) {
    entries.forEach(function (entry) { schedule(byScore.get(entry.target)); });
  });
  byScore.forEach(function (box, score) { observer.observe(score); });
  // The text size can change the em without changing the column's width.
  new MutationObserver(function () { boxes.forEach(schedule); })
    .observe(document.documentElement, { attributes: true, attributeFilter: ["data-text-size"] });
})();
