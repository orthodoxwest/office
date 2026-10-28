// Chant scores: neuma lays each out at its column's width, as the apps do, and
// again when that or the text size changes. The page carries the score at one
// width for a browser that can't run the engine (render-html chant.rs), and
// that score stands wherever this can't run: the engine is fetched only once a
// score is shown, and only by a browser that can run it.
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
          // The verse strip (style.css) works only with the engine at hand.
          document.documentElement.classList.add("chant-live");
          return neuma;
        });
      });
      engine.catch(function () { engine = null; });
    }
    return engine;
  };

  var layOut = function (neuma, box) {
    var score = box.parentNode;
    var width = score.clientWidth;
    if (!width) return;
    var em = parseFloat(getComputedStyle(score).fontSize);
    var units = (width / em) * LYRIC_SPACES * SCALE;
    var state = states.get(box);
    if (!state) {
      state = { chant: null, prefix: box.getAttribute("data-chant-prefix") + "c" + serial++, units: 0 };
      states.set(box, state);
    }
    if (!state.chant) {
      // As chant.rs's page_options, and the apps' Score.kt and Score.swift.
      state.chant = new neuma.Chant(box.getAttribute("data-gabc"), {
        initial: box.getAttribute("data-chant-initial") === "1" ? 1 : 0,
        annotation: false,
        font: font,
        everyHyphen: true,
        initialScale: 0.5,
        initialMargin: 0,
        shortLastStaff: true,
        phraseLines: box.getAttribute("data-chant-phrases") === "1",
      });
      state.units = 0;
    }
    // A sliver of a unit changes no line; resizes within it are left alone.
    if (Math.abs(state.units - units) < 0.5) return;
    var page = state.chant.layout(units, { scale: SCALE, prefix: state.prefix, ids: false, style: false });
    state.units = units;
    box.style.width = (page.width / (LYRIC_SPACES * SCALE)).toFixed(3) + "em";
    box.style.aspectRatio = page.width.toFixed(2) + "/" + page.height.toFixed(2);
    box.innerHTML = page.svg;
    page.free();
    // The score's group carries the words it sets, not its SVG.
    var svg = box.firstElementChild;
    svg.removeAttribute("role");
    svg.removeAttribute("aria-label");
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("focusable", "false");
  };

  var pending = new Set();
  var scheduled = false;
  var run = function () {
    scheduled = false;
    var due = Array.from(pending).filter(function (box) { return box.parentNode.clientWidth > 0; });
    pending.clear();
    if (!due.length) return;
    load().then(function (neuma) {
      due.forEach(function (box) {
        try {
          layOut(neuma, box);
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
  // A tune shown once carries the strip of its hymn's verses: a tap sets that
  // verse's words under the notes, the first alone with the initial, as the
  // books letter only a hymn's opening.
  document.querySelectorAll(".chant-strip").forEach(function (strip) {
    var score = strip.previousElementSibling;
    var box = score && byScore.has(score) ? byScore.get(score) : null;
    if (!box) return;
    strip.addEventListener("click", function (event) {
      var button = event.target.closest(".chant-strip-verse");
      if (!button || button.getAttribute("aria-pressed") === "true") return;
      strip.querySelectorAll(".chant-strip-verse").forEach(function (other) {
        other.setAttribute("aria-pressed", other === button ? "true" : "false");
      });
      var first = button === strip.querySelector(".chant-strip-verse");
      box.setAttribute("data-gabc", button.getAttribute("data-gabc"));
      box.setAttribute("data-chant-initial", first ? "1" : "0");
      var state = states.get(box);
      if (state && state.chant) {
        state.chant.free();
        state.chant = null;
      }
      schedule(box);
    });
  });
  // The text size can change the em without changing the column's width.
  new MutationObserver(function () { boxes.forEach(schedule); })
    .observe(document.documentElement, { attributes: true, attributeFilter: ["data-text-size"] });
})();
