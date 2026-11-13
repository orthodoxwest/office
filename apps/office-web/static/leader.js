// The engine prepares every ordinary form in the cached document. This file
// selects a branch; it does not contain liturgical wording or rubric rules.

// The reader's place, held while something changes the page around it: the
// prayer form here, the text size (app.js) and a chant score drawn anew
// (chant.js). The first block whose foot is below the top of the screen stays
// where it stands; one read from its middle, across the top of the screen,
// keeps the same share of itself above it, so a paragraph that grows with the
// text size keeps the line at the top in view. A reader at the very top of the
// page stays there.
window.officeHoldPlace = function (change, blocks) {
  var list = document.querySelectorAll(blocks || ".elements p, .elements :is(h2, h3, summary, .chant-score)");
  var anchor = null;
  var before = null;
  if (window.scrollY > 0) {
    for (var i = 0; i < list.length && !anchor; i++) {
      var rect = list[i].getBoundingClientRect();
      if (rect.bottom > 0 && rect.top < window.innerHeight && rect.height) {
        anchor = list[i];
        before = rect;
      }
    }
  }
  change();
  if (!anchor) return;
  var after = anchor.getBoundingClientRect();
  if (!after.height) return;
  var top = before.top < 0 ? (before.top / before.height) * after.height : before.top;
  if (Math.abs(after.top - top) >= 0.5) window.scrollBy(0, after.top - top);
};

(function () {
  var root = document.documentElement;
  var leaders = ["private", "deacon", "priest"];
  var requested = new URLSearchParams(location.search).get("form");
  var propagate = leaders.indexOf(requested) >= 0;
  var choice = root.getAttribute("data-leader") || "private";
  if (leaders.indexOf(choice) < 0) choice = "private";
  var selector = document.querySelector(".leader-selector");

  function syncControls() {
    root.setAttribute("data-leader", choice);
    document.querySelectorAll('input[name="office-prayer-form"]').forEach(function (input) {
      input.checked = input.value === choice;
    });
  }

  // An explicit review link overrides the saved preference for its navigation
  // chain, without overwriting that preference. Ordinary visits use storage.
  function syncLinks() {
    if (!propagate) return;
    document.querySelectorAll("a[href]").forEach(function (link) {
      var url = new URL(link.href, location.href);
      if (url.origin !== location.origin || !/^\/(?:$|(?:lauds|prime|terce|sext|none|vespers|compline|calendar|reminders)(?:\/|$))/.test(url.pathname)) return;
      url.searchParams.set("form", choice);
      link.href = url.pathname + url.search + url.hash;
    });
    document.querySelectorAll(".date-jump-form").forEach(function (form) {
      var input = form.querySelector('input[name="form"]');
      if (!input) {
        input = document.createElement("input");
        input.type = "hidden";
        input.name = "form";
        form.appendChild(input);
      }
      input.value = choice;
    });
  }

  syncControls();
  if (selector) {
    selector.querySelector("fieldset").disabled = false;
    selector.addEventListener("change", function (event) {
      if (event.target.name !== "office-prayer-form" || leaders.indexOf(event.target.value) < 0) return;
      choice = event.target.value;
      try { localStorage.setItem("office-prayer-form", choice); } catch { propagate = true; }
      if (propagate) {
        var url = new URL(location.href);
        url.searchParams.set("form", choice);
        history.replaceState(null, "", url.pathname + url.search + url.hash);
      }
      // A form changes whole slots of the office, so a slot is the place held.
      window.officeHoldPlace(function () {
        syncControls();
        selector.open = false;
        selector.querySelector("summary").focus({ preventScroll: true });
      }, ".elements .leader-slot, .elements .psalm, .elements .canticle, .elements > .section-heading");
      syncLinks();
      document.getElementById("leader-status").textContent = "Prayers updated. " + event.target.nextElementSibling.textContent + ".";
      window.dispatchEvent(new Event("officeleaderchange"));
    });
  }
  // Do not listen for storage changes: another tab must never change prayers
  // underneath someone who is reading this office.
  document.addEventListener("DOMContentLoaded", syncLinks);
  window.addEventListener("officenavigation", syncLinks);
})();
