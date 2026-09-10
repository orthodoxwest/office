// The engine prepares every ordinary form in the cached document. This file
// selects a branch; it does not contain liturgical wording or rubric rules.
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
      var anchor = Array.from(document.querySelectorAll(".elements .leader-slot, .elements .psalm, .elements .canticle, .elements > .section-heading")).find(function (element) {
        var rect = element.getBoundingClientRect();
        return rect.bottom > 0 && rect.top < window.innerHeight;
      });
      var top = anchor ? anchor.getBoundingClientRect().top : 0;
      choice = event.target.value;
      try { localStorage.setItem("office-prayer-form", choice); } catch { propagate = true; }
      if (propagate) {
        var url = new URL(location.href);
        url.searchParams.set("form", choice);
        history.replaceState(null, "", url.pathname + url.search + url.hash);
      }
      syncControls();
      selector.open = false;
      selector.querySelector("summary").focus({ preventScroll: true });
      if (anchor) window.scrollBy(0, anchor.getBoundingClientRect().top - top);
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
