// Progressive enhancement of the aggregate report; no events or identifiers
// are requested, stored, or inferred here. The existing report works without JS.
(function () {
  "use strict";
  const root = document.querySelector(".usage-explorer");
  if (!root) return;
  const groups = JSON.parse(document.getElementById("usage-trend-data").textContent);
  if (!groups.length || !groups[0].Points.length) return;
  const get = id => document.getElementById(`usage-${id}`);
  const compare = get("compare"), interval = get("interval"), measure = get("measure"), observed = get("observed");
  const slider = get("explore-date"), chart = get("explore-chart");
  const params = new URLSearchParams(location.search);
  for (const [control, key] of [[compare, "compare"], [interval, "interval"], [measure, "measure"]]) {
    if (Array.from(control.options).some(option => option.value === params.get(key))) control.value = params.get(key);
  }
  observed.checked = params.get("start") !== "window";
  const date = day => new Date(`${day}T12:00:00Z`);
  const format = day => date(day).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric", timeZone: "UTC" });
  const total = counts => counts.reduce((sum, count) => sum + count, 0);
  const share = (count, sum) => sum ? `${(100 * count / sum).toFixed(1)}%` : "—";
  const range = bucket => bucket.first === bucket.last ? format(bucket.first) : `${format(bucket.first)} – ${format(bucket.last)}`;
  const node = (tag, text, className) => {
    const element = document.createElement(tag);
    if (text !== undefined) element.textContent = text;
    if (className) element.className = className;
    return element;
  };
  const svg = (tag, attrs) => {
    const element = document.createElementNS("http://www.w3.org/2000/svg", tag);
    Object.entries(attrs).forEach(([key, value]) => element.setAttribute(key, value));
    return element;
  };
  let group, buckets = [], current = 0;
  const x = index => buckets.length === 1 ? 360 : 8 + 704 * index / (buckets.length - 1);

  // Calendar weeks, with explicit partial boundaries at either end of the
  // selected window. A week with no observations has no invented percentage.
  function aggregate(points) {
    const result = [];
    for (const point of points) {
      const monday = date(point.Day);
      monday.setUTCDate(monday.getUTCDate() - (monday.getUTCDay() + 6) % 7);
      const key = interval.value === "week" ? monday.toISOString().slice(0, 10) : point.Day;
      let bucket = result[result.length - 1];
      if (!bucket || bucket.key !== key) {
        bucket = { key, first: point.Day, last: point.Day, counts: point.Counts.map(() => 0), days: 0, reported: 0 };
        result.push(bucket);
      }
      bucket.last = point.Day;
      bucket.days++;
      if (total(point.Counts)) bucket.reported++;
      point.Counts.forEach((count, i) => { bucket.counts[i] += count; });
    }
    return result;
  }

  function inspect(index) {
    current = Math.max(0, Math.min(index, buckets.length - 1));
    const bucket = buckets[current], sum = total(bucket.counts);
    slider.value = current;
    slider.setAttribute("aria-valuetext", range(bucket));
    get("explore-prev").disabled = current === 0;
    get("explore-next").disabled = current === buckets.length - 1;
    const cursor = get("explore-cursor");
    cursor.setAttribute("x1", x(current));
    cursor.setAttribute("x2", x(current));
    const today = group.Points[group.Points.length - 1].Day;
    get("inspect-date").textContent = `${range(bucket)}${bucket.last === today ? " · In progress" : ""}`;
    const values = get("inspect-values");
    values.replaceChildren();
    group.Series.forEach((label, i) => {
      const item = node("div");
      item.append(node("dt", label), node("dd", `${bucket.counts[i]} ${bucket.counts[i] === 1 ? "browser-day" : "browser-days"} · ${share(bucket.counts[i], sum)}`));
      values.append(item);
    });
    const partial = interval.value === "week" && bucket.days < 7 ? " Partial week in this view." : "";
    get("inspect-note").textContent = (sum ? `${bucket.reported} of ${bucket.days} ${bucket.days === 1 ? "day has" : "days have"} observations.` : "No observations. Shares are unavailable.") + partial;
  }

  function table() {
    const table = get("explore-table");
    table.replaceChildren();
    table.append(node("caption", `${group.Label}: browser-days and share, newest first`));
    const head = node("thead"), header = node("tr");
    for (const label of ["Period", ...group.Series, "Days observed"]) {
      const th = node("th", label); th.scope = "col"; header.append(th);
    }
    head.append(header); table.append(head);
    const body = node("tbody");
    buckets.slice().reverse().forEach(bucket => {
      const row = node("tr"), th = node("th", range(bucket));
      th.scope = "row"; row.append(th);
      const sum = total(bucket.counts);
      bucket.counts.forEach(count => row.append(node("td", `${count} · ${share(count, sum)}`)));
      row.append(node("td", `${bucket.reported} / ${bucket.days}`));
      body.append(row);
    });
    table.append(body);
  }

  function remember() {
    const url = new URL(location.href);
    url.searchParams.set("compare", compare.value);
    url.searchParams.set("interval", interval.value);
    url.searchParams.set("measure", measure.value);
    url.searchParams.set("start", observed.checked ? "observed" : "window");
    history.replaceState(null, "", url);
    links();
  }

  function links() {
    // Preserve the exploration when the server supplies a different window.
    const values = { compare: compare.value, interval: interval.value, measure: measure.value, start: observed.checked ? "observed" : "window" };
    document.querySelectorAll(".usage-window a").forEach(link => {
      const target = new URL(link.href);
      Object.entries(values).forEach(([key, value]) => target.searchParams.set(key, value));
      link.href = `${target.pathname}${target.search}`;
    });
  }

  function draw() {
    const previous = buckets[current];
    group = groups.find(item => item.Key === compare.value);
    let points = group.Points;
    const first = points.findIndex(point => total(point.Counts) > 0);
    if (observed.checked && first >= 0) points = points.slice(first);
    buckets = aggregate(points);
    const shares = measure.value === "share";
    const peak = Math.max(...buckets.flatMap(bucket => bucket.counts));
    const ceiling = shares ? 100 : Math.max(1, peak);
    const y = value => 210 - 200 * value / ceiling;
    const lines = get("explore-lines");
    lines.replaceChildren();
    group.Series.forEach((label, series) => {
      let path = "", connected = false;
      const dots = [];
      buckets.forEach((bucket, index) => {
        const sum = total(bucket.counts);
        if (!sum) { connected = false; return; }
        const value = shares ? 100 * bucket.counts[series] / sum : bucket.counts[series];
        const px = x(index), py = y(value);
        path += `${connected ? "L" : "M"}${px.toFixed(2)},${py.toFixed(2)} `;
        connected = true;
        // Isolated observations remain visible even on a long window.
        if (buckets.length <= 60 || !index || !total(buckets[index - 1].counts) || index === buckets.length - 1 || !total(buckets[index + 1].counts)) {
          dots.push(svg("circle", { cx: px, cy: py, r: 3, class: `usage-series-${series}` }));
        }
      });
      lines.append(svg("path", { d: path, class: `usage-series-${series}` }), ...dots);
    });
    get("explore-max").textContent = shares ? "100%" : peak;
    const legend = get("explore-legend");
    legend.replaceChildren();
    group.Series.forEach((label, i) => {
      const item = node("li"), key = svg("svg", { viewBox: "0 0 32 8", "aria-hidden": "true" });
      key.append(svg("path", { d: "M0 4H32", class: `usage-series-${i}` }));
      item.append(key, node("span", label)); legend.append(item);
    });
    const reported = group.Points.filter(point => total(point.Counts)).length;
    get("explore-coverage").textContent = `${reported} of ${group.Points.length} days in the selected window have ${group.Label.toLowerCase()} observations. ${observed.checked && first > 0 ? "Chart starts at the first observation. " : ""}${shares ? "Share of reported category browser-days." : "Counts are browser-days, summed within each period."}`;
    get("explore-first").textContent = format(points[0].Day);
    get("explore-last").textContent = format(points[points.length - 1].Day);
    get("explore-chart-title").textContent = `${group.Label}: ${shares ? "share of category" : "browser-days"} by ${interval.value}`;
    get("explore-empty").hidden = peak > 0;
    slider.max = buckets.length - 1;
    slider.disabled = buckets.length <= 1;
    const matched = previous ? buckets.findIndex(bucket => previous.last >= bucket.first && previous.last <= bucket.last) : -1;
    table();
    inspect(matched >= 0 ? matched : buckets.length - 1);
  }

  [compare, interval, measure, observed].forEach(control => control.addEventListener("change", () => { draw(); remember(); }));
  slider.addEventListener("input", () => inspect(Number(slider.value)));
  get("explore-prev").addEventListener("click", () => inspect(current - 1));
  get("explore-next").addEventListener("click", () => inspect(current + 1));
  function point(event) {
    const bounds = chart.getBoundingClientRect();
    const position = (event.clientX - bounds.left) / bounds.width * 720;
    inspect(Math.round((position - 8) / 704 * (buckets.length - 1)));
  }
  chart.addEventListener("pointerdown", point);
  chart.addEventListener("pointermove", event => { if (event.pointerType === "mouse") point(event); });
  draw();
  // Initial navigation leaves the URL alone, but restores bookmarked controls.
  links();
  root.hidden = false;
  document.querySelector(".usage-explore-link").hidden = false;
})();
