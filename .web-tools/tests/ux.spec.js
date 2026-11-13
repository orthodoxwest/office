import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const testDate = "2026-03-15";

function violationFingerprints(results) {
  return results.violations
    .flatMap((violation) =>
      violation.nodes.map((node) => ({
        rule: violation.id,
        target: node.target.join(" "),
      })),
    )
    .sort((a, b) => `${a.rule}:${a.target}`.localeCompare(`${b.rule}:${b.target}`));
}

async function openDatedPage(page, path, theme = "light") {
  await page.addInitScript((storedTheme) => {
    localStorage.setItem("office-theme", storedTheme);
  }, theme);
  await page.goto(path);
  await page.evaluate(() => document.fonts.ready);
}

// Lays out the openings as a browser without initial-letter does (WebKit,
// Firefox): the stylesheet's own fallback block, applied unconditionally,
// with initial-letter turned off. Remove the returned tag to restore.
async function fallbackInitials(page) {
  const css = await page.evaluate(async () => (await fetch(document.querySelector('link[rel="stylesheet"][href*="style.css"]').href)).text());
  const start = css.indexOf("@supports (not (initial-letter: 2))");
  expect(start, "the stylesheet keeps a fallback for initial-letter").toBeGreaterThan(-1);
  const open = css.indexOf("{", start);
  let depth = 0;
  let end = open;
  for (; end < css.length; end++) {
    if (css[end] === "{") depth++;
    if (css[end] === "}" && --depth === 0) break;
  }
  return page.addStyleTag({ content: `*::first-letter { initial-letter: normal !important; }\n${css.slice(open + 1, end)}` });
}

// Phones reach Appearance and Text size from the foot of the site menu; wide
// screens from the header's Settings. Open whichever is showing, choose, and
// close it again so later measurements see the page. A `group` (the buttons' class)
// tells apart the settings that share a name, such as the Martyrology's and Chant's On.
async function choosePreference(page, name, group) {
  const wide = await page.locator(".site-settings > summary").isVisible();
  const disclosure = page.locator(wide ? ".site-settings" : ".site-menu");
  const summary = disclosure.locator(":scope > summary");
  await summary.click();
  const prefs = page.locator(wide ? ".settings-prefs" : ".menu-prefs");
  const button = prefs.getByRole("button", { name, exact: true });
  await (group ? button.and(page.locator(group)) : button).click();
  await summary.click();
  await expect(disclosure).not.toHaveAttribute("open", "");
}

// The raised initial increases the line box without adding a line of text.
function textLinesIn(el) {
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  const tops = [];
  let node;
  while ((node = walker.nextNode())) {
    // A posture cue's smaller type shares the line but not its glyph tops.
    if (node === el.firstChild || node.parentElement.closest(".mediant, .posture") || !node.textContent.trim()) continue;
    const range = document.createRange();
    range.setStart(node, node.textContent.search(/\S/));
    range.setEnd(node, node.textContent.trimEnd().length);
    for (const rect of range.getClientRects()) if (rect.width) tops.push(rect.top);
  }
  return 1 + (Math.max(...tops) - Math.min(...tops)) / parseFloat(getComputedStyle(el).lineHeight);
}

// The left edge of each row of text in `el`, top to bottom, leaving out the
// opening letter: a dropped initial's rows, then its turnovers.
function rowStartsIn(el) {
  const leading = parseFloat(getComputedStyle(el).lineHeight);
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  const rows = [];
  let node;
  let first = el.textContent.search(/\S/) >= 0;
  while ((node = walker.nextNode())) {
    if (node.parentElement.closest(".mediant, .posture")) continue;
    for (let i = 0; i < node.length; i++) {
      if (/\s/.test(node.textContent[i])) continue;
      if (first) { first = false; continue; } // The initial has its own box.
      const range = document.createRange();
      range.setStart(node, i);
      range.setEnd(node, i + 1);
      const rect = range.getBoundingClientRect();
      if (!rect.width) continue;
      const row = rows.find((r) => Math.abs(r.top - rect.top) < leading / 2);
      if (row) row.left = Math.min(row.left, rect.left);
      else rows.push({ top: rect.top, left: rect.left });
    }
  }
  return rows.sort((a, b) => a.top - b.top).map((r) => r.left);
}

async function openingTextLines(opening) {
  return opening.evaluate(textLinesIn);
}

// Every match in one round trip: an element-by-element loop over a whole
// alphabet of specimens costs hundreds of protocol calls per layout.
async function openingsTextLines(page, selector) {
  return page.evaluate(`[...document.querySelectorAll(${JSON.stringify(selector)})].map(${textLinesIn})`);
}

// Contour-fitting deliberately puts text inside the initial's rectangular
// advance box. Compare the two painted layers instead of treating that box
// as ink. Decode screenshots in the browser using public canvas APIs.
async function expectInitialInkClear(page, specimen, capSelector, label, baselineWord = null) {
  await specimen.evaluate(el => el.setAttribute("data-ink-specimen", ""));
  const root = "[data-ink-specimen]";
  // Only the specimen, padded for an initial that overhangs its box, is read:
  // decoding a whole long page costs seconds per layer.
  const clip = await specimen.evaluate(el => {
    const pad = 64;
    const box = el.getBoundingClientRect();
    const doc = document.documentElement;
    const x = Math.max(0, Math.floor(box.left + scrollX - pad));
    const y = Math.max(0, Math.floor(box.top + scrollY - pad));
    return {
      x, y,
      width: Math.min(doc.scrollWidth, Math.ceil(box.right + scrollX + pad)) - x,
      height: Math.min(doc.scrollHeight, Math.ceil(box.bottom + scrollY + pad)) - y,
    };
  });
  const layers = [];
  try {
    for (const cap of [true, false]) {
      const style = await page.addStyleTag({ content: `
        body, body * { color: transparent !important; }
        body ::first-letter { background: none !important; color: transparent !important; }
        ${root}, ${root} * { color: ${cap || baselineWord ? "transparent" : "#0000ff"} !important; }
        ${!cap && baselineWord ? `${root} ${baselineWord} { color: #0000ff !important; }` : ""}
        ${root} ${capSelector}::first-letter { background: none !important; color: ${cap ? "#ff0000" : "transparent"} !important; }
        header, nav, .hour-progress { visibility: hidden !important; }
      ` });
      try {
        // Whole-page coordinates remain stable when a specimen exceeds the
        // mobile visual viewport; an element screenshot can scroll it.
        layers.push((await page.screenshot({ fullPage: true, clip, animations: "disabled" })).toString("base64"));
      } finally {
        await style.evaluate(el => el.remove());
      }
    }
    const result = await page.evaluate(async encoded => {
      const decode = async data => {
        const img = new Image();
        img.src = "data:image/png;base64," + data;
        await img.decode();
        const canvas = document.createElement("canvas");
        canvas.width = img.width;
        canvas.height = img.height;
        const context = canvas.getContext("2d");
        context.drawImage(img, 0, 0);
        return context.getImageData(0, 0, img.width, img.height);
      };
      const [cap, text] = await Promise.all(encoded.map(decode));
      if (cap.width !== text.width || cap.height !== text.height) return { changed: true };
      const red = (x, y) => {
        if (x < 0 || y < 0 || x >= cap.width || y >= cap.height) return false;
        const i = (y * cap.width + x) * 4;
        return cap.data[i] > 170 && cap.data[i + 1] < 100 && cap.data[i + 2] < 100;
      };
      let contacts = 0, capPixels = 0, textPixels = 0, capBottom = 0, textBottom = 0;
      for (let y = 0; y < text.height; y++) {
        for (let x = 0; x < text.width; x++) {
          const i = (y * text.width + x) * 4;
          if (red(x, y)) { capPixels++; capBottom = y; }
          if (text.data[i] < 100 && text.data[i + 1] < 100 && text.data[i + 2] > 170) {
            textPixels++;
            textBottom = y;
            if (red(x, y) || red(x - 1, y) || red(x + 1, y) || red(x, y - 1) || red(x, y + 1)) contacts++;
          }
        }
      }
      return { contacts, capPixels, textPixels, capBottom, textBottom };
    }, layers);
    expect(result.changed, label).toBeUndefined();
    expect(result.capPixels, label).toBeGreaterThan(50);
    expect(result.textPixels, label).toBeGreaterThan(50);
    expect(result.contacts, label).toBe(0);
    if (baselineWord) expect(Math.abs(result.capBottom - result.textBottom), `${label} ink baseline`).toBeLessThanOrEqual(2);
  } finally {
    await specimen.evaluate(el => el.removeAttribute("data-ink-specimen"));
  }
}

// The server renders an undated home request for its own current local day.
// Read that rendered value before installing a browser clock, so tests of a
// foreground page crossing midnight do not assume a particular CI date.
//
// The first load of any fresh context has no tz cookie yet — app.js sets
// one from the browser's own timezone, but only after that first response
// already rendered using the server's time.Local fallback (the CI
// container's clock, not the browser's configured America/New_York).
// Reload once that cookie is in place so the slug we read — and that every
// later request in the test will also see — is computed in the one
// timezone the whole test actually runs in, rather than racing the
// container's clock across the day boundary for several hours a day.
async function serverTodaySlug(page) {
  await page.goto("/");
  await page.reload();
  const slug = await page.locator(".home-prayer-card").getAttribute("data-date-slug");
  expect(slug).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  return slug;
}

test("mobile navigation stays quiet until opened", async ({ page }) => {
  await openDatedPage(page, `/?date=${testDate}`);

  const menu = page.locator(".site-menu");
  await expect(menu).not.toHaveAttribute("open", "");

  // Home lists every hour on its prayer card; the menu does not repeat them.
  const primary = page.getByRole("navigation", { name: "Primary" });
  await page.getByText("Menu", { exact: true }).click();
  await expect(menu).toHaveAttribute("open", "");
  await expect(primary.getByRole("link", { name: "Ordo", exact: true })).toBeVisible();
  await expect(primary.locator('[data-nav="hour"]')).toHaveCount(0);
  // Preferences sit at the foot of the menu, not at the foot of the page.
  await expect(page.locator(".menu-prefs").getByRole("button", { name: "Apse", exact: true })).toBeVisible();
  await expect(page.locator(".site-settings")).toBeHidden();
  const [links, prefs] = await Promise.all([
    primary.boundingBox(),
    page.locator(".menu-prefs").boundingBox(),
  ]);
  expect(prefs.y).toBeGreaterThanOrEqual(links.y + links.height);

  // The menu is an overlay: a tap on the page or Escape puts it away, but a
  // preference tap inside it does not.
  await page.locator(".menu-prefs").getByRole("button", { name: "Nave", exact: true }).click();
  await expect(menu).toHaveAttribute("open", "");
  // The wall in the gutter at the screen's foot, beside the arch.
  await page.mouse.click(4, (await page.evaluate(() => innerHeight)) - 4);
  await expect(menu).not.toHaveAttribute("open", "");
  await page.getByText("Menu", { exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(menu).not.toHaveAttribute("open", "");
  await expect(page.locator(".site-menu > summary")).toBeFocused();

  // The phone's home is the arch alone: the colophon is the wide home's.
  await expect(page.locator("footer")).toBeHidden();

  // Hour pages keep the hours in the menu, in home's 2 / 3 / 2 bands.
  await openDatedPage(page, `/lauds/${testDate}`);
  await page.getByText("Menu", { exact: true }).click();
  await expect(primary.getByRole("link", { name: "Vespers", exact: true })).toBeVisible();
  const rows = await primary.locator('[data-nav="hour"]').evaluateAll((links) => {
    const byTop = new Map();
    for (const a of links) {
      const top = Math.round(a.getBoundingClientRect().top);
      byTop.set(top, [...(byTop.get(top) || []), a.dataset.hour]);
    }
    return [...byTop.values()];
  });
  expect(rows).toEqual([["lauds", "prime"], ["terce", "sext", "none"], ["vespers", "compline"]]);
});

test("psalm spacing groups each antiphon with its own psalm", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/lauds/${testDate}`);

  const spacing = await page.evaluate(() => {
    const kids = [...document.querySelector(".elements").children];
    const gap = (a, b) => b.getBoundingClientRect().top - a.getBoundingClientRect().bottom;
    const isPsalm = (el) => el.classList.contains("psalm") || el.classList.contains("canticle");
    const isAnt = (el) => el.classList.contains("antiphon");
    const withinGroup = [];
    const betweenGroups = [];
    for (let i = 0; i < kids.length - 1; i++) {
      const [a, b] = [kids[i], kids[i + 1]];
      if ((isAnt(a) && isPsalm(b)) || (isPsalm(a) && isAnt(b))) withinGroup.push(gap(a, b));
      if (isAnt(a) && isAnt(b)) betweenGroups.push(gap(a, b));
    }
    // The doxology is not the last verse and must not sit at verse spacing.
    const psalm = document.querySelector(".psalm");
    const verses = [...psalm.querySelectorAll(".verse.numbered")];
    const gloria = psalm.querySelector(".gloria-patri");
    return {
      within: Math.max(...withinGroup),
      between: Math.min(...betweenGroups),
      verseGap: gap(verses[0], verses[1]),
      gloriaGap: gap(verses[verses.length - 1], gloria),
    };
  });

  // An antiphon belongs to its psalm: the join inside a group must be clearly
  // tighter than the space between one psalm's closing antiphon and the next
  // psalm's opening one, or two ANT. lines in a row read as a stutter.
  expect(spacing.between).toBeGreaterThan(spacing.within * 2);
  expect(spacing.gloriaGap).toBeGreaterThan(spacing.verseGap * 1.5);
});

// The hour's vertical rhythm (render_blocks::Space, which the apps share): each
// space a step of the prayer's body line. Sext's run from the Alleluia through
// the hymn into the first psalm is the page that showed the steps uneven.
test("the hour's spaces are the rhythm's steps of the body line", async ({ page }) => {
  for (const width of [390, 800, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/sext/2026-10-29");
    const steps = await page.evaluate(() => {
      const line = parseFloat(getComputedStyle(document.querySelector(".elements")).getPropertyValue("--line"));
      const gap = (a, b) => (b.getBoundingClientRect().top - a.getBoundingClientRect().bottom) / line;
      const q = (s) => document.querySelector(s);
      const stanzas = [...document.querySelectorAll(".hymn .hymn-stanza")];
      const hymn = q(".hymn");
      const antiphon = hymn.nextElementSibling;
      const psalm = antiphon.nextElementSibling;
      return {
        line,
        prayer: parseFloat(getComputedStyle(q(".elements")).fontSize),
        section: gap(hymn.previousElementSibling, hymn.querySelector(".section-heading")),
        title: gap(hymn.querySelector(".section-heading"), q(".hymn-title")),
        hymn: gap(q(".hymn-title"), stanzas[0]),
        stanza: gap(stanzas[0], stanzas[1]),
        group: gap(stanzas[stanzas.length - 1], antiphon),
        antiphon: gap(antiphon, psalm.querySelector(".item-label")),
        label: gap(psalm.querySelector(".item-label"), psalm.querySelector(".verse")),
        verse: gap(...psalm.querySelectorAll(".verse")),
        gloria: gap([...psalm.querySelectorAll(".verse")].pop(), psalm.querySelector(".gloria-patri")),
      };
    });
    // The line is the prayer's body line, so the steps follow the text's size.
    expect(steps.line).toBeCloseTo(steps.prayer * 1.6, 1);
    const expected = { section: 1, title: 0.15, hymn: 0.3, stanza: 0.45, group: 0.75, antiphon: 0.3, label: 0.3, verse: 0.15, gloria: 0.45 };
    for (const [name, step] of Object.entries(expected)) {
      expect(steps[name], `${name} at ${width}px`).toBeCloseTo(step, 1);
    }
  }
});

test("the rhythm reaches into a prayer form's part and the Martyrology", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("office-prayer-form", "priest");
    localStorage.setItem("office-martyrology", "on");
  });
  await page.setViewportSize({ width: 390, height: 900 });
  await page.goto("/prime/2026-10-29");
  const steps = await page.evaluate(() => {
    const line = parseFloat(getComputedStyle(document.querySelector(".elements")).getPropertyValue("--line"));
    const shown = (e) => e.getClientRects().length > 0;
    const leaves = [...document.querySelectorAll(".elements p, .elements h2")].filter(shown);
    const at = (text) => leaves.findIndex((e) => e.textContent.includes(text));
    const above = (i) => (leaves[i].getBoundingClientRect().top - leaves[i - 1].getBoundingClientRect().bottom) / line;
    const heading = at("Martyrology for tomorrow");
    return {
      confession: above(at("Then the Confession is repeated")),
      absolution: above(at("After Confession has been made")),
      martyrology: above(heading),
      entry: above(heading + 1),
      thanks: above(leaves.findIndex((e, i) => i > heading && e.textContent.includes("Thanks be to God"))),
      // A section's label stands as a part's heading, as the apps' toggle row does.
      label: (() => {
        const after = [...document.querySelectorAll(".session-prayers")].pop();
        const box = after.querySelector("summary").getBoundingClientRect();
        const before = after.previousElementSibling.getBoundingClientRect();
        return (box.top - before.bottom) / line;
      })(),
      under: (() => {
        const after = [...document.querySelectorAll(".session-prayers")].pop();
        const box = after.querySelector("summary").getBoundingClientRect();
        return (after.querySelector(".session-prayers-content p").getBoundingClientRect().top - box.bottom) / line;
      })(),
    };
  });
  // Each matches render_blocks::space_before for the same blocks.
  const expected = { confession: 0.45, absolution: 0.45, martyrology: 1, entry: 0.3, thanks: 0.45, label: 1, under: 0.3 };
  for (const [name, step] of Object.entries(expected)) {
    expect(steps[name], name).toBeCloseTo(step, 1);
  }
});

test("hour progress completes with the prayer, before the administrative epilogue", async ({
  page,
}) => {
  await openDatedPage(page, `/lauds/${testDate}`);

  const progress = page.getByRole("progressbar", { name: "Progress through the prayer text" });
  await expect(progress).toHaveAttribute("aria-valuenow", "0");

  const boundary = await page.evaluate(() => {
    const prayer = document.querySelector(".elements");
    const prayerStart = prayer.getBoundingClientRect().top + window.scrollY;
    const prayerEnd = prayer.getBoundingClientRect().bottom + window.scrollY;
    return {
      prayerStart,
      completionScroll: Math.max(prayerStart, prayerEnd - window.innerHeight),
      documentEnd: document.documentElement.scrollHeight - window.innerHeight,
    };
  });
  expect(boundary.prayerStart).toBeGreaterThan(0);
  expect(boundary.completionScroll).toBeLessThan(boundary.documentEnd);

  // Reading progress does not accrue while moving through the page header.
  await page.evaluate((scrollTop) => window.scrollTo(0, scrollTop), boundary.prayerStart - 1);
  await expect.poll(async () => Number(await progress.getAttribute("aria-valuenow"))).toBe(0);

  const withinPrayer = boundary.prayerStart + (boundary.completionScroll - boundary.prayerStart) / 4;
  await page.evaluate((scrollTop) => window.scrollTo(0, scrollTop), withinPrayer);
  await expect.poll(async () => Number(await progress.getAttribute("aria-valuenow"))).toBeGreaterThan(
    0,
  );

  await page.evaluate((scrollTop) => window.scrollTo(0, scrollTop), boundary.completionScroll);
  await expect.poll(async () => Number(await progress.getAttribute("aria-valuenow"))).toBeGreaterThanOrEqual(
    99,
  );
});

test("home frontispiece keeps source, focus, and visual order aligned", async ({ page }) => {
  await openDatedPage(page, `/?date=${testDate}`);

  await expect(page.getByRole("heading", { name: "Morning", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Day", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Evening", exact: true })).toBeVisible();

  const order = await page.evaluate(() => {
    const day = document.querySelector(".home-summary");
    const prayer = document.querySelector(".home-prayer-card");
    const dateControl = document.querySelector(".home-day-meta");
    const top = (selector) => document.querySelector(selector).getBoundingClientRect().top;
    return {
      source:
        Boolean(day.compareDocumentPosition(prayer) & Node.DOCUMENT_POSITION_FOLLOWING) &&
        Boolean(prayer.compareDocumentPosition(dateControl) & Node.DOCUMENT_POSITION_FOLLOWING),
      positions: {
        day: top(".home-day-head"),
        prayer: top(".home-prayer-card"),
        dateControl: top(".home-day-meta"),
      },
      focusables: Array.from(
        document.querySelectorAll(
          ".home-hero a[href], .home-hero summary, .home-hero input, .home-hero button",
        ),
      )
        // The calendar stands inert in the hours' place until Change date turns to it.
        .filter((element) => !element.closest("[inert]"))
        // date + the feast's page + go-to-today (historical days) + pray + 7 hours + the first
        // week's introduction + change-date.
        .slice(0, 13)
        .map((element) => {
          if (element.matches(".home-date-link")) return "date";
          if (element.matches(".feast-link")) return "feast";
          // Recovery chrome when the landing day is not local today — intentional
          // after the stale-day fix; sits in the day identity, before Pray now.
          if (element.matches(".not-today-link")) return "go-to-today";
          if (element.matches(".pray-now")) return "pray";
          if (element.matches(".home-hour-link")) return element.getAttribute("data-hour");
          if (element.matches(".home-introduction")) return "introduction";
          if (element.matches("summary")) return "change-date";
          return "unexpected";
        }),
    };
  });
  expect(order.source).toBe(true);
  expect(order.positions.day).toBeLessThan(order.positions.prayer);
  expect(order.positions.prayer).toBeLessThan(order.positions.dateControl);
  // testDate is fixed in the past relative to "today", so Go to today is present.
  expect(order.focusables).toEqual([
    "date",
    "feast",
    "go-to-today",
    "pray",
    "lauds",
    "prime",
    "terce",
    "sext",
    "none",
    "vespers",
    "compline",
    "introduction",
    "change-date",
  ]);
});

test("home hour directory fits thumb targets across phone widths and text sizes", async ({ page }) => {
  await page.addInitScript(() => localStorage.removeItem("office-text-size"));

  for (const width of [320, 390, 540]) {
    await page.setViewportSize({ width, height: 844 });
    await openDatedPage(page, `/?date=${testDate}`);

    for (const size of ["default", "large", "small"]) {
      if (size !== "default") {
        await choosePreference(page, size === "large" ? "Larger text" : "Smaller text");
      }

      const geometry = await page.evaluate(() => ({
        overflows: document.documentElement.scrollWidth > window.innerWidth + 1,
        targetHeights: Array.from(document.querySelectorAll(".home-hour-link")).map(
          (link) => link.getBoundingClientRect().height,
        ),
        directoryFrame: (() => {
          const style = getComputedStyle(document.querySelector(".home-hour-links"));
          return {
            left: [style.borderLeftWidth, style.borderLeftStyle],
            right: [style.borderRightWidth, style.borderRightStyle],
          };
        })(),
        labelAlignment: getComputedStyle(
          document.querySelector(".home-hour-group-label"),
        ).justifyContent,
      }));
      expect(geometry.overflows, `${width}px/${size} should not overflow`).toBe(false);
      expect(Math.min(...geometry.targetHeights), `${width}px/${size} hour targets`).toBeGreaterThanOrEqual(
        44,
      );
      expect(geometry.directoryFrame, `${width}px/${size} directory frame`).toEqual({
        left: ["1px", "solid"],
        right: ["1px", "solid"],
      });
      expect(geometry.labelAlignment, `${width}px/${size} label alignment`).toBe("center");

      if (size === "large") {
        await choosePreference(page, "Default text size");
      }
    }
  }
});

test("current hour and frontispiece invitation update in Nave and Apse", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-03-15T10:00:00-04:00"));
  await openDatedPage(page, `/?date=${testDate}`);

  const invitation = page.locator(".pray-now");
  const current = page.locator('.home-hour-link[aria-current="time"]');
  await expect(invitation).toHaveText("Pray Terce");
  await expect(invitation).toHaveAttribute("href", `/terce/${testDate}`);
  await expect(current).toHaveAttribute("data-hour", "terce");
  // "Now" is announced, not drawn: the tinted cell and the invitation above
  // carry it visually, so the word is sr-only rather than a chip beside the
  // hour name. Assert it is in the accessibility tree and out of the picture.
  await expect(current.getByText("Now", { exact: true })).toBeAttached();
  // Clipped to nothing, not scrolled away: WebKit's IntersectionObserver
  // ignores the target's own clip, so measure the box rather than the viewport.
  const nowBox = await current.getByText("Now", { exact: true }).boundingBox();
  expect(nowBox.width * nowBox.height).toBeLessThanOrEqual(1);
  expect(await current.getByText("Now", { exact: true }).evaluate(el => getComputedStyle(el).clip)).toMatch(/rect\(0(px)?,? 0(px)?,? 0(px)?,? 0(px)?\)/);
  await expect(current.locator("xpath=ancestor::section[1]")).toContainText("Day");

  const naveState = await invitation.evaluate((element) => ({
    background: getComputedStyle(element).backgroundColor,
    borderStyle: getComputedStyle(element).borderTopStyle,
  }));
  expect(naveState.background).toBe("rgba(0, 0, 0, 0)");
  expect(naveState.borderStyle).toBe("solid");

  await choosePreference(page, "Apse");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const apseState = await invitation.evaluate((element) => ({
    background: getComputedStyle(element).backgroundColor,
    borderStyle: getComputedStyle(element).borderTopStyle,
  }));
  expect(apseState.background).toBe("rgba(0, 0, 0, 0)");
  expect(apseState.borderStyle).toBe("solid");
});

test("the early-morning invitation opens the previous day's Compline", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-03-15T01:00:00-04:00"));
  await openDatedPage(page, `/?date=${testDate}`);

  await expect(page.locator(".pray-now")).toHaveText("Pray Compline");
  await expect(page.locator(".pray-now")).toHaveAttribute("href", "/compline/2026-03-14");
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveCount(0);
});

test("the foreground home invitation advances at the next office boundary", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-03-15T10:59:00-04:00") });
  await openDatedPage(page, `/?date=${testDate}`);

  await expect(page.locator(".pray-now")).toHaveText("Pray Terce");
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveAttribute(
    "data-hour",
    "terce",
  );

  // The current hour says what it is under its name, and the line moves with it.
  const note = page.locator(".home-hour-link-note:visible");
  await expect(note).toHaveCount(1);
  await expect(note).toHaveText("Mid-morning prayer");

  await page.clock.fastForward("02:00");

  await expect(page.locator(".pray-now")).toHaveText("Pray Sext");
  await expect(note).toHaveText("Midday prayer");
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveAttribute(
    "data-hour",
    "sext",
  );
});

// The light moves with the hours (app.js HOUR_LIGHT, render_blocks::hour_light).
// Its animations are held almost still as they are made, so a test sets the
// pass's clock itself rather than racing it.
async function holdTheLight(page) {
  await page.addInitScript(() => {
    const animate = Element.prototype.animate;
    window.lightAnimations = [];
    Element.prototype.animate = function (...args) {
      const animation = animate.apply(this, args);
      animation.playbackRate = 1e-6;
      window.lightAnimations.push(animation);
      return animation;
    };
  });
}
async function lightAt(page, ms) {
  await page.evaluate((t) => {
    window.lightAnimations.forEach((a) => {
      if (a.playState !== "idle") a.currentTime = t;
    });
  }, ms);
  // The seat and the end are promises the animations settle.
  await page.evaluate(() => new Promise((done) => setTimeout(done, 0)));
}
const homeAtRest = (page) =>
  page.evaluate(() => ({
    passing: document.querySelectorAll(".is-passing").length,
    // The pass's own, not a control's CSS transitions.
    animations: window.lightAnimations.filter((a) => a.playState !== "idle").length,
    words: Array.from(document.querySelector(".pray-now").childNodes)
      .filter((n) => n.nodeType === 1 && n.matches(".pray-now-words")).length,
    current: Array.from(document.querySelectorAll(".home-hour-link.is-current")).map((l) => l.dataset.hour),
  }));

test("the light moves from Sext to None on an open home, and settles as a fresh page", async ({ page }) => {
  await holdTheLight(page);
  await page.clock.install({ time: new Date("2026-03-15T12:59:58-04:00") });
  await openDatedPage(page, `/?date=${testDate}`);
  const invitation = page.locator(".pray-now");
  await expect(invitation).toHaveText("Pray Sext");
  // Arrival never moves the light.
  expect(await page.evaluate(() => window.lightAnimations.length)).toBe(0);

  await page.clock.runFor(3000);
  // The hour has turned for a screen reader at once …
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveAttribute("data-hour", "none");
  await expect(invitation).toHaveAttribute("href", /\/none\//);
  // … while the eye sees the light leave Sext: both hours hold a pool.
  await lightAt(page, 300);
  await expect(page.locator(".home-hour-link.is-passing")).toHaveCount(2);
  await expect(invitation).toHaveText("Pray Sext");
  expect((await homeAtRest(page)).current).toEqual(["sext"]);
  // Past the seat, the new hour's look and words.
  await lightAt(page, 700);
  await expect(invitation).toHaveText("Pray None");
  expect((await homeAtRest(page)).current).toEqual(["none"]);
  // At the end nothing of the pass is left.
  await lightAt(page, 1400);
  expect(await homeAtRest(page)).toEqual({ passing: 0, animations: 0, words: 0, current: ["none"] });
  await expect(page.locator(".home-hour-link-note:visible")).toHaveText("Afternoon prayer");
});

test("a second turn of the hour during a pass settles the first", async ({ page }) => {
  await holdTheLight(page);
  await page.clock.install({ time: new Date("2026-03-15T12:59:58-04:00") });
  await openDatedPage(page, `/?date=${testDate}`);
  await page.clock.runFor(3000);
  await lightAt(page, 300);
  // Another turn arrives (a clock set forward) while the first is under way.
  await page.clock.setSystemTime(new Date("2026-03-15T17:00:01-04:00"));
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  // The first pass ends where it was going (None), and the next leaves from there.
  const passing = await page.evaluate(() => Array.from(document.querySelectorAll(".is-passing")).map((l) => l.dataset.hour));
  expect(passing).toEqual(["none", "vespers"]);
  await expect(page.locator(".pray-now")).toHaveText("Pray None");
  await lightAt(page, 1400);
  expect(await homeAtRest(page)).toEqual({ passing: 0, animations: 0, words: 0, current: ["vespers"] });
  await expect(page.locator(".pray-now")).toHaveText("Pray Vespers");
});

test.describe("under reduced motion", () => {
  test.use({ reducedMotion: "reduce" });
  test("the light does not move: the new hour comes at once", async ({ page }) => {
    await holdTheLight(page);
    await page.clock.install({ time: new Date("2026-03-15T12:59:58-04:00") });
    await openDatedPage(page, `/?date=${testDate}`);
    await page.clock.runFor(3000);
    await expect(page.locator(".pray-now")).toHaveText("Pray None");
    expect(await page.evaluate(() => window.lightAnimations.length)).toBe(0);
    expect(await homeAtRest(page)).toEqual({ passing: 0, animations: 0, words: 0, current: ["none"] });
  });
});

test("a page hidden at the turn of the hour is corrected on its return, without moving the light", async ({ page }) => {
  await holdTheLight(page);
  await page.addInitScript(() => {
    window.pageHidden = false;
    Object.defineProperty(document, "hidden", { get: () => window.pageHidden });
    Object.defineProperty(document, "visibilityState", { get: () => (window.pageHidden ? "hidden" : "visible") });
  });
  await page.clock.install({ time: new Date("2026-03-15T12:59:58-04:00") });
  await openDatedPage(page, `/?date=${testDate}`);
  await page.evaluate(() => { window.pageHidden = true; });
  await page.clock.runFor(3000);
  await page.evaluate(() => {
    window.pageHidden = false;
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await expect(page.locator(".pray-now")).toHaveText("Pray None");
  expect(await page.evaluate(() => window.lightAnimations.length)).toBe(0);
  expect(await homeAtRest(page)).toEqual({ passing: 0, animations: 0, words: 0, current: ["none"] });
});

test("a turn found late, after a sleeping timer, corrects home without moving the light", async ({ page }) => {
  await holdTheLight(page);
  await page.clock.install({ time: new Date("2026-03-15T12:00:00-04:00") });
  await openDatedPage(page, `/?date=${testDate}`);
  await page.clock.setSystemTime(new Date("2026-03-15T18:00:00-04:00"));
  await page.clock.runFor("01:01");
  await expect(page.locator(".pray-now")).toHaveText("Pray Vespers");
  expect(await page.evaluate(() => window.lightAnimations.length)).toBe(0);
});

test("the light leaves Compline at midnight and gathers on Lauds at two", async ({ page }) => {
  await holdTheLight(page);
  await page.clock.install({ time: new Date("2026-07-29T23:59:58-04:00") });
  await openDatedPage(page, "/?date=2026-07-29");
  await page.clock.runFor(3000);
  // Only the one pool dies; the invitation's words stay.
  await lightAt(page, 700);
  await expect(page.locator(".home-hour-link.is-passing")).toHaveCount(1);
  expect(await page.evaluate(() => document.querySelectorAll(".pray-now-words").length)).toBe(0);
  await lightAt(page, 1400);
  expect(await homeAtRest(page)).toEqual({ passing: 0, animations: 0, words: 0, current: [] });

  await page.clock.setSystemTime(new Date("2026-07-30T01:59:58-04:00"));
  await openDatedPage(page, "/?date=2026-07-30");
  await page.clock.runFor(3000);
  await lightAt(page, 300);
  await expect(page.locator(".home-hour-link.is-passing")).toHaveCount(1);
  await expect(page.locator(".pray-now")).toHaveText("Pray Compline");
  await lightAt(page, 1400);
  await expect(page.locator(".pray-now")).toHaveText("Pray Lauds");
  expect(await homeAtRest(page)).toEqual({ passing: 0, animations: 0, words: 0, current: ["lauds"] });
});

test("Compline hands on to tomorrow's Lauds", async ({ page }) => {
  await page.goto("/compline/2026-10-07");
  const next = page.locator(".next-hour");
  await expect(next).toHaveAttribute("href", "/lauds/2026-10-08");
  await expect(next.locator(".continuation-label")).toHaveText("Tomorrow");
});

// The reader moves down the page. The prayed mark reads only what is on
// screen, so a script scroll stands for a wheel or a swipe in every engine
// (mobile WebKit in Playwright has no mouse.wheel).
async function scrollHourBy(page, dy) {
  await page.evaluate(top => scrollBy({ top, behavior: "instant" }), dy);
}

// The end of the hour's text reaches the screen.
async function atHourEnd(page) {
  return page.evaluate(() => document.querySelector(".hour-end-mark").getBoundingClientRect().top < innerHeight);
}

test("scrolling through an hour in seconds does not mark it prayed", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-10-07T21:00:00") });
  await page.goto("/compline/2026-10-07");
  for (let i = 0; i < 3 && !(await atHourEnd(page)); i++) {
    await scrollHourBy(page, 3000);
    await page.clock.runFor(1000);
  }
  await page.evaluate(() => scrollTo(0, document.body.scrollHeight));
  // Waiting at the end reads only the end.
  await page.clock.runFor(120_000);
  await page.goto("/?date=2026-10-07");
  await expect(page.locator(".home-hour-link.is-prayed")).toHaveCount(0);
});

test("an hour read through at a praying pace is marked prayed on home", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-10-07T21:00:00") });
  await page.goto("/compline/2026-10-07");
  for (let i = 0; i < 60 && !(await atHourEnd(page)); i++) {
    await page.clock.runFor(20_000);
    await scrollHourBy(page, 400);
  }
  await page.clock.runFor(2000);
  await page.goto("/?date=2026-10-07");
  const compline = page.locator('.home-hour-link[data-hour="compline"]');
  await expect(compline).toHaveClass(/is-prayed/);
  await expect(compline.locator(".home-hour-prayed .sr-only")).toHaveText("Prayed");
  await expect(page.locator(".home-hour-link.is-prayed")).toHaveCount(1);
  // Another day's home is not marked.
  await page.goto("/?date=2026-10-06");
  await expect(page.locator(".home-hour-link.is-prayed")).toHaveCount(0);
});

test("the home invitation catches up when the clock jumps past a sleeping timer", async ({
  page,
}) => {
  // A sleeping phone or frozen PWA stops timer delays without stopping the
  // clock; the refresh must notice the wall clock rather than wait out a
  // timeout set at the previous hour.
  await page.clock.install({ time: new Date("2026-03-15T12:00:00-04:00") });
  await openDatedPage(page, `/?date=${testDate}`);
  await expect(page.locator(".pray-now")).toHaveText("Pray Sext");

  await page.clock.setSystemTime(new Date("2026-03-15T18:00:00-04:00"));
  await page.clock.runFor("01:01");

  await expect(page.locator(".pray-now")).toHaveText("Pray Vespers");
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveAttribute(
    "data-hour",
    "vespers",
  );
});

test("the foreground home keeps previous-day Compline current across midnight", async ({
  page,
}) => {
  await page.clock.install({ time: new Date("2026-07-29T23:59:00-04:00") });
  await openDatedPage(page, "/?date=2026-07-29");

  const invitation = page.locator(".pray-now");
  await expect(invitation).toHaveText("Pray Compline");
  await expect(invitation).toHaveAttribute("href", "/compline/2026-07-29");
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveAttribute(
    "data-hour",
    "compline",
  );

  await page.clock.fastForward("02:00");

  await expect(invitation).toHaveText("Pray Compline");
  await expect(invitation).toHaveAttribute("href", "/compline/2026-07-29");
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Go to today" })).toBeVisible();
});

test("the niche's cross stands clear under the lining at every desktop width", async ({ page }) => {
  // The cross was once placed from the head's height: where the head
  // flattens (tablets, narrow windows) the lining ran through it.
  for (const width of [701, 820, 1024, 1280, 1440, 1920]) {
    await page.setViewportSize({ width, height: 1100 });
    await openDatedPage(page, `/?date=${testDate}`);
    const air = await page.evaluate(() => {
      const zoom = Number(getComputedStyle(document.querySelector(".home-hero")).zoom) || 1;
      const lining = document.querySelector(".home-lining").getBoundingClientRect();
      const crown = document.querySelector(".home-crown").getBoundingClientRect();
      const heading = document.querySelector("#home-date-heading").getBoundingClientRect();
      // The lining's band, its gap and its inner hairline take 11px.
      return { above: (crown.top - lining.top) / zoom - 11, below: (heading.top - crown.bottom) / zoom };
    });
    expect(air.above, `${width}px: under the lining`).toBeGreaterThanOrEqual(6);
    expect(air.below, `${width}px: over the date`).toBeGreaterThanOrEqual(12);
  }
});

test("an upright tablet's niche fills the page and keeps the hours in view", async ({ page }) => {
  // At a laptop's size it took two-thirds of an iPad's width and left a third
  // of the screen bare below it. Upright, the niche is a lancet: narrower
  // than the screen, but it takes the screen's height down to "Change date".
  for (const [width, height] of [[820, 1180], [834, 1194], [1024, 1366]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, `/?date=${testDate}`);
    const niche = await page.evaluate(() => {
      const hero = document.querySelector(".home-hero").getBoundingClientRect();
      const meta = document.querySelector(".home-date-nav > summary").getBoundingClientRect();
      return {
        share: hero.width / innerWidth,
        height: hero.height / innerHeight,
        metaBottom: meta.bottom,
        overflow: document.documentElement.scrollWidth - innerWidth,
      };
    });
    expect(niche.share, `${width}x${height}`).toBeGreaterThan(0.6);
    expect(niche.height, `${width}x${height}: the niche takes the height`).toBeGreaterThan(0.75);
    expect(niche.metaBottom, `${width}x${height}: "Change date" in view`).toBeLessThan(height);
    expect(niche.overflow, `${width}x${height}`).toBeLessThanOrEqual(0);
  }
});

// The wide niche's measures on the glass: the zoom (--niche-zoom) is the
// hero's drawn width over its own, and a gloss's size is its computed size
// at that zoom.
async function nicheOnGlass(page) {
  return page.evaluate(() => {
    const hero = document.querySelector(".home-hero");
    const zoom = hero.getBoundingClientRect().width / hero.offsetWidth;
    const bottom = (selector) => document.querySelector(selector).getBoundingClientRect().bottom;
    const notes = [...document.querySelectorAll(".home-hour-link-note")];
    return {
      zoom,
      notes: notes.filter((note) => note.checkVisibility()).length,
      smallestNote: Math.min(...notes.map((note) => parseFloat(getComputedStyle(note).fontSize) * zoom)),
      date: bottom("#home-date-heading"),
      feast: bottom(".feast"),
      pray: bottom(".pray-now"),
      hours: Math.max(...[...document.querySelectorAll(".home-hour-link")].map((link) => link.getBoundingClientRect().bottom)),
      overflow: document.documentElement.scrollWidth - innerWidth,
    };
  });
}

test("the wide niche glosses every hour and keeps the day and the hours above the fold", async ({ page }) => {
  // A laptop's niche once kept only the hour that is now glossed, and a
  // short window lost the evening hours under the fold. Every gloss is at
  // least 12px on the glass, whatever the zoom.
  for (const [width, height] of [[1280, 800], [1180, 820], [1024, 768], [820, 1180], [768, 1024], [701, 900], [1440, 900], [1920, 1080]]) {
    await page.setViewportSize({ width, height });
    for (const date of [testDate, "2026-12-04"]) {
      await openDatedPage(page, `/?date=${date}`);
      const niche = await nicheOnGlass(page);
      const at = `${width}x${height} ${date}`;
      expect(niche.notes, `${at}: every hour glossed`).toBe(7);
      expect(niche.smallestNote, `${at}: glosses legible`).toBeGreaterThanOrEqual(12);
      for (const part of ["date", "feast", "pray", "hours"]) {
        expect(niche[part], `${at}: ${part} above the fold`).toBeLessThanOrEqual(height);
      }
      expect(niche.overflow, at).toBeLessThanOrEqual(0);
    }
  }
});

test("the wide niche scales by one computed zoom, never under its least", async ({ page }) => {
  // One formula, not a table of steps: the height left under the chrome over
  // the niche's own, capped by a share of the width; 0.96 at least on its
  // side (0.9 upright) so a gloss never falls under 12px.
  const zooms = [];
  for (const [width, height, least] of [[1024, 700, 0.96], [1280, 800, 0.96], [1440, 900, 0.96], [1920, 1200, 0.96], [768, 900, 0.9], [820, 1180, 0.9]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, `/?date=${testDate}`);
    const { zoom } = await nicheOnGlass(page);
    expect(zoom, `${width}x${height}`).toBeGreaterThanOrEqual(least - 0.005);
    expect(zoom, `${width}x${height}`).toBeLessThanOrEqual(1.5);
    zooms.push(zoom);
  }
  // A taller screen of one width never gets a smaller niche.
  await page.setViewportSize({ width: 1440, height: 1000 });
  await openDatedPage(page, `/?date=${testDate}`);
  const taller = (await nicheOnGlass(page)).zoom;
  await page.setViewportSize({ width: 1440, height: 1100 });
  await openDatedPage(page, `/?date=${testDate}`);
  expect((await nicheOnGlass(page)).zoom).toBeGreaterThanOrEqual(taller);
  expect(new Set(zooms.map((z) => z.toFixed(2))).size).toBeGreaterThan(2);
});

test("the wide niche sets the hours one to a line where its heaviest day fits them, a glossed table below", async ({ page }) => {
  // Upright from 1000px high; on its side from 1146px, where the list's
  // heaviest day fits at the least zoom (below that it was clamped and ran
  // under the fold).
  const rows = async () =>
    page.evaluate(() => {
      const top = (hour) => Math.round(document.querySelector(`.home-hour-link[data-hour="${hour}"]`).getBoundingClientRect().top);
      return { lauds: top("lauds"), prime: top("prime"), terce: top("terce"), none: top("none") };
    });
  for (const [width, height, list] of [[1280, 800, false], [1180, 820, false], [1728, 1030, false], [1920, 1080, false], [1920, 1200, true], [820, 1180, true]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, `/?date=${testDate}`);
    const at = await rows();
    if (list) {
      expect(at.prime, `${width}x${height}: Prime under Lauds`).toBeGreaterThan(at.lauds + 20);
      expect(at.none, `${width}x${height}: None under Terce`).toBeGreaterThan(at.terce + 40);
    } else {
      expect(Math.abs(at.prime - at.lauds), `${width}x${height}: Lauds and Prime share a row`).toBeLessThanOrEqual(1);
      expect(Math.abs(at.none - at.terce), `${width}x${height}: Terce and None share a row`).toBeLessThanOrEqual(1);
    }
  }
});

test("the fold never cuts through the wide home's colophon, and a fitted niche never scrolls", async ({ page }) => {
  // Where the zoom was held at its least, the colophon's tailpiece or words
  // were sliced by the fold; where the niche fitted, a pixel or three of
  // scroll remained. The colophon is now wholly in view or wholly below.
  for (const [width, height] of [[1280, 800], [1180, 820], [1470, 830], [1440, 900], [1728, 1030], [1680, 1050], [1920, 1080], [820, 1180], [1024, 768]]) {
    await page.setViewportSize({ width, height });
    for (const date of [testDate, "2026-12-13"]) {
      await openDatedPage(page, `/?date=${date}`);
      const at = `${width}x${height} ${date}`;
      const foot = await page.evaluate(() => {
        const footer = document.querySelector("footer").getBoundingClientRect();
        return { top: footer.top, bottom: footer.bottom, scroll: document.documentElement.scrollHeight - innerHeight };
      });
      const whole = foot.bottom <= height + 0.5;
      expect(whole || foot.top >= height - 0.5, `${at}: colophon cut by the fold`).toBe(true);
      if (whole) expect(foot.scroll, `${at}: no stray scroll`).toBeLessThanOrEqual(0);
    }
  }
});

test("a phone on its side keeps the day and the invitation above the fold", async ({ page }) => {
  // The niche's least zoom once left only the date and feast in view on a
  // phone turned sideways; its small words still keep 12px on the glass.
  for (const [width, height] of [[844, 390], [932, 430]]) {
    await page.setViewportSize({ width, height });
    for (const date of [testDate, "2026-12-13"]) {
      await openDatedPage(page, `/?date=${date}`);
      const at = `${width}x${height} ${date}`;
      const view = await page.evaluate(() => {
        const hero = document.querySelector(".home-hero");
        const zoom = hero.getBoundingClientRect().width / hero.offsetWidth;
        const glass = (el) => parseFloat(getComputedStyle(el).fontSize) * zoom * (parseFloat(getComputedStyle(el).zoom) || 1);
        return {
          pray: document.querySelector(".pray-now").getBoundingClientRect().bottom,
          smallest: Math.min(...[...document.querySelectorAll(".home-hour-link-note, .home-hour-group-label, .home-date-weekday")].map(glass)),
        };
      });
      expect(view.pray, `${at}: "Pray" above the fold`).toBeLessThanOrEqual(height);
      expect(view.smallest, `${at}: small words legible`).toBeGreaterThanOrEqual(11.95);
    }
  }
});

test("the zoomed niche's hand-ruled joints fall between the periods", async ({ page }) => {
  // The joints were placed from client rects, measured on the glass, in the
  // zoomed table's own pixels: under zoom they drifted off the joints.
  for (const [width, height] of [[1280, 800], [820, 1180], [1920, 1080]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, `/?date=${testDate}`);
    await expect(page.locator(".home-hour-links.is-ruled")).toHaveCount(1);
    const drift = await page.evaluate(() => {
      const joints = [...document.querySelectorAll(".home-hour-links > svg.home-rules path.r-inner")];
      const groups = [...document.querySelectorAll(".home-hour-group")].slice(1);
      return joints.map((joint, i) => {
        const box = joint.getBoundingClientRect();
        return Math.abs((box.top + box.bottom) / 2 - groups[i].getBoundingClientRect().top);
      });
    });
    expect(drift.length, `${width}x${height}`).toBe(2);
    for (const d of drift) expect(d, `${width}x${height}: joint on the period's edge`).toBeLessThanOrEqual(2);
  }
});

test("the wide home by night is one sky inside the niche and a plain wall round it", async ({ page }) => {
  for (const [width, height] of [[1280, 800], [820, 1180]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, `/?date=${testDate}`, "dark");
    await expect(page.locator(".home-sky svg.home-stars")).toHaveCount(1);
    const night = await page.evaluate(() => ({
      wall: getComputedStyle(document.body, "::before").display,
      sky: getComputedStyle(document.querySelector(".home-sky")).display,
      stars: document.querySelectorAll(".home-sky svg.home-stars path, .home-sky svg.home-stars use").length,
    }));
    expect(night.wall, `${width}x${height}: the margins' starfield is off`).toBe("none");
    expect(night.sky, `${width}x${height}`).not.toBe("none");
    expect(night.stars, `${width}x${height}: the head has its stars`).toBeGreaterThan(5);
  }
});

test("home's day stands clear of the lining's hairline, up the head as at its sides", async ({ page }) => {
  // The ordo's names are long and the head narrows as it rises: a name once
  // ran over the hairline at the jambs and, on tall phones with the versicle
  // under the feast, a date or a name crossed it further up the arch. A probe
  // clipped as the hairline is finds where it runs, so the test holds no
  // figures of the arch.
  for (const [width, height, dates] of [
    [320, 740, ["2026-11-02", "2026-04-18", "2026-06-11"]],
    [390, 844, ["2026-11-02", "2026-04-18", "2026-06-11"]],
    [412, 915, ["2026-10-02", "2026-06-11"]],
    [430, 932, ["2026-11-02", "2026-06-12"]],
  ]) {
    await page.setViewportSize({ width, height });
    for (const date of dates) {
      // Opened as today, so the versicle takes the head's spare height.
      await page.clock.setFixedTime(new Date(`${date}T15:56:00-04:00`));
      await openDatedPage(page, `/?date=${date}`);
      const crossing = await page.evaluate(() => {
        const lining = document.querySelector(".home-lining");
        const hairline = getComputedStyle(lining, "::after");
        const probe = document.createElement("div");
        Object.assign(probe.style, { position: "absolute", top: hairline.top, right: hairline.right, bottom: hairline.bottom, left: hairline.left, clipPath: hairline.clipPath, pointerEvents: "auto" });
        lining.append(probe);
        const on = (x, y) => document.elementsFromPoint(x, y).includes(probe);
        // Each line of the day, grown by its clear air, must not reach the hairline.
        const air = 8;
        const crossing = [];
        const words = document.createTreeWalker(document.querySelector(".home-day-head"), NodeFilter.SHOW_TEXT);
        for (let node = words.nextNode(); node; node = words.nextNode()) {
          if (!node.textContent.trim() || !node.parentElement.checkVisibility()) continue;
          const range = document.createRange();
          range.selectNodeContents(node);
          for (const line of range.getClientRects()) {
            const [left, right, top] = [line.left - air, line.right + air, line.top - air];
            let touches = false;
            for (let x = left; x <= right && !touches; x += 0.5) touches = on(x, top);
            for (let y = top; y <= line.bottom && !touches; y += 0.5) touches = on(left, y) || on(right, y);
            if (touches) crossing.push(node.textContent.trim());
          }
        }
        probe.remove();
        return crossing;
      });
      expect(crossing, `${width}x${height} ${date}`).toEqual([]);
    }
  }
});

test("a feast's familiar name is set apart in italic and never broken", async ({ page }) => {
  // A break inside it would leave half the parenthesis at a line's end.
  for (const [width, height] of [[320, 740], [390, 844], [430, 932]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, "/?date=2026-11-02");
    const alias = await page.locator(".home .feast-alias").evaluate((node) => ({
      text: node.textContent,
      italic: getComputedStyle(node).fontStyle,
      lines: node.getClientRects().length,
    }));
    expect(alias, `${width}x${height}`).toEqual({ text: "(All Souls\u2019 Day)", italic: "italic", lines: 1 });
  }
  // A name without one is set whole.
  await openDatedPage(page, `/?date=${testDate}`);
  await expect(page.locator(".home .feast-alias")).toHaveCount(0);
});

test("a tablet's hour header sets the hours as one rank, with no link stranded", async ({ page }) => {
  // Wide widths hold the same: the prayer's softened field once reached past
  // the column and, with Large text at the 1000px threshold, past the viewport.
  for (const size of ["default", "large"]) {
    for (const width of [701, 820, 999, 1000, 1100, 1280, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      await page.addInitScript((s) => localStorage.setItem("office-text-size", s), size);
      await openDatedPage(page, `/vespers/${testDate}`);
      const header = await page.evaluate(() => {
        // About stands down from a wide hour's header (it has no box there).
        const links = [...document.querySelectorAll(".site-menu nav a")].filter((a) => a.getClientRects().length);
        // Links on one baseline have boxes a pixel or two apart in height
        // (Reminders is set smaller); a new row begins a whole line lower.
        const tops = links.map((a) => a.getBoundingClientRect().top).sort((a, b) => a - b);
        return {
          rows: tops.filter((top, i) => i === 0 || top - tops[i - 1] > 8).length,
          overflow: document.documentElement.scrollWidth - innerWidth,
        };
      });
      expect(header.rows, `${width}px ${size} text`).toBe(1);
      expect(header.overflow, `${width}px ${size} text`).toBe(0);
    }
  }
});

test("forced colours drop the wall for the system canvas", async ({ page }) => {
  await page.emulateMedia({ forcedColors: "active" });
  await page.setViewportSize({ width: 1280, height: 900 });
  for (const path of [`/?date=${testDate}`, "/calendar/2026/03", "/calendar/2026", `/lauds/${testDate}`]) {
    await openDatedPage(page, path);
    const wall = await page.evaluate(() => getComputedStyle(document.documentElement, "::before").content);
    expect(wall, path).toBe("none");
  }
  const band = await page.evaluate(() => getComputedStyle(document.body, "::before").content);
  expect(band, "hour band").toBe("none");
});

test("quick theme changes settle on the last choice with the wall restored", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await openDatedPage(page, `/?date=${testDate}`);
  await page.evaluate(() => {
    const pick = choice => document.querySelector(`.theme-option[data-theme-choice="${choice}"]`).click();
    pick("dark");
    pick("light");
    setTimeout(() => pick("dark"), 50);
  });
  const html = page.locator("html");
  await expect(html).toHaveAttribute("data-theme", "dark");
  await expect(html).not.toHaveClass(/vault-hidden|theme-anim/);
  expect(await page.evaluate(() => localStorage.getItem("office-theme"))).toBe("dark");
  await expect.poll(() => page.evaluate(() => Number(getComputedStyle(document.documentElement, "::before").opacity))).toBe(1);
});

test("the wall fades out before a theme swap and respects reduced motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await openDatedPage(page, `/?date=${testDate}`);
  await page.evaluate(() => {
    window.wallAtThemeSwap = new Promise(resolve => {
      const observer = new MutationObserver(() => {
        if (document.documentElement.dataset.theme !== "dark") return;
        resolve(Number(getComputedStyle(document.documentElement, "::before").opacity));
        observer.disconnect();
      });
      observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    });
  });
  await choosePreference(page, "Apse");
  expect(await page.evaluate(() => window.wallAtThemeSwap)).toBeLessThan(0.1);
  const opacity = () => page.evaluate(() => Number(getComputedStyle(document.documentElement, "::before").opacity));
  await expect.poll(opacity).toBe(1);

  await page.emulateMedia({ reducedMotion: "reduce" });
  await choosePreference(page, "Nave");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  expect(await opacity()).toBe(1);
});

for (const theme of ["light", "dark"]) {
  test(`print removes wall material and uses white paper in ${theme}`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    // Home stands for the threshold pages that share its print rules; the
    // ordo adds its sticky heading, usage its own print sheet, and the hour
    // its softened prayer band.
    for (const path of [`/?date=${testDate}`, "/calendar/2026/03", "/admin/usage?days=7", `/lauds/${testDate}`]) {
      await openDatedPage(page, path, theme);
      await page.emulateMedia({ media: "print" });
      const paper = await page.evaluate(() => {
        const heading = document.querySelector(".month h2");
        return {
          wall: getComputedStyle(document.documentElement, "::before").content,
          background: getComputedStyle(document.documentElement).backgroundColor,
          headingImage: heading ? getComputedStyle(heading).backgroundImage : "none",
          band: getComputedStyle(document.body, "::before").content,
        };
      });
      expect(paper, path).toEqual({ wall: "none", background: "rgb(255, 255, 255)", headingImage: "none", band: "none" });
      await page.emulateMedia({ media: "screen" });
    }
  });
}

test("the frontispiece holds its width whatever the day is called", async ({ page }) => {
  // body is a column flex container, and an auto cross-axis margin suppresses
  // flex stretch — so main needs an explicit width:100% or it becomes
  // shrink-to-fit and the measure caps nothing. Prose hides that (a
  // paragraph's max-content exceeds the cap anyway); the frontispiece does
  // not, and the card collapsed to the width of the day's feast name.
  const days = [
    "2026-08-10", // "St. Lawrence, Martyr"
    "2026-07-13", // no feast name at all
    "2026-11-03", // "Day III within the Octave of All Saints"
  ];
  for (const width of [1280, 1920]) {
    await page.setViewportSize({ width, height: 1000 });
    const widths = [];
    for (const date of days) {
      await openDatedPage(page, `/?date=${date}`);
      widths.push(
        await page.evaluate(() =>
          Math.round(document.querySelector(".home-hero").getBoundingClientRect().width),
        ),
      );
    }
    expect(new Set(widths).size).toBe(1);
    // And it is the declared measure, not whatever the content happened to
    // need: the landscape lancet's 37.5rem, scaled by the niche's zoom.
    const zoom = await page.evaluate(() => {
      const hero = document.querySelector(".home-hero");
      return hero.getBoundingClientRect().width / hero.offsetWidth;
    });
    expect(Math.abs(widths[0] - 37.5 * 16 * zoom)).toBeLessThanOrEqual(1);
  }
});

// The Apse vault paints --ornament through an SVG star-tile mask. Nave leaves
// the tile off and the ink transparent, so a field paints stars only when it
// has both. Takes { mask, ink } read from a computed style.
function vaultPaints({ mask, ink }) {
  const transparent = /^transparent$|^rgba\(\d+, \d+, \d+, 0\)$/.test(String(ink));
  return String(mask).includes("ornaments/vault.svg") && !transparent;
}

// The Nave's wall is powdered with rosettes through the same fields, in the
// lining thinned almost to the ground (a color() with a small alpha). A
// field powders only when it has the rosette tile and some ink.
function powderPaints({ mask, ink }) {
  const transparent = /^transparent$|^rgba\(\d+, \d+, \d+, 0\)$|\/ 0\)$/.test(String(ink));
  return String(mask).includes("ornaments/powder.svg") && !transparent;
}

// Multi-layer backgrounds serialize each layer's position/size (Chromium:
// "50% 0%, 50% 0%, …"). Engines also differ on keywords vs percentages.
// Compare every layer's components rather than the full string.
function phaseLayers(phase) {
  return String(phase)
    .split(",")
    .map((s) => s.replace(/\s+/g, " ").trim().toLowerCase())
    .filter(Boolean);
}

function isTopCenterLayer(p) {
  return (
    p === "50% 0%" ||
    p === "50% 0" ||
    p === "center top" ||
    p === "top center" ||
    p === "50% top" ||
    p === "center 0%" ||
    p === "center 0"
  );
}

function isTopCenterPhase(phase) {
  const layers = phaseLayers(phase);
  return layers.length > 0 && layers.every(isTopCenterLayer);
}

function tileEdgePx(size) {
  // Take the first layer; all vault layers share one tile.
  const first = String(size).split(",")[0].trim();
  const m = first.match(/([\d.]+)px(?:\s+([\d.]+)px)?/);
  if (!m) return null;
  return { w: parseFloat(m[1]), h: parseFloat(m[2] || m[1]) };
}

test("the mobile home vault is one stable full-page layer without scroll", async ({
  page,
}) => {
  const read = async ({ width = 390, height, theme, scheme, asToday = false, date = testDate }) => {
    const context = await page.context().browser().newContext({
      viewport: { width, height },
      isMobile: true,
      hasTouch: true,
      colorScheme: scheme,
    });
    const sheet = await context.newPage();
    if (theme) await sheet.addInitScript((t) => localStorage.setItem("office-theme", t), theme);
    await sheet.goto(`/?date=${date}`);
    const seen = await sheet.evaluate((asToday) => {
      // The fixed test day is historical, so the server adds the recovery
      // line that local today's home never carries; drop it to measure the
      // layout a reader sees on the day itself.
      if (asToday) document.querySelectorAll(".not-today-notice").forEach((notice) => notice.remove());
      const field = getComputedStyle(document.body, "::before");
      const footerElement = document.querySelector("footer");
      // The pointed head's halo is a filter on its outer courses.
      const halo = getComputedStyle(document.querySelector(".home-arch"));
      return {
        stars: { mask: field.maskImage || field.webkitMaskImage, ink: field.backgroundColor },
        position: field.position,
        tileSize: field.maskSize || field.webkitMaskSize,
        phase: field.maskPosition || field.webkitMaskPosition,
        // The phone's home carries no colophon; the wide home keeps it.
        footerShown: getComputedStyle(footerElement).display !== "none",
        // Probe the night token rather than hard-coding #121c28 — the halo must
        // use whatever --bg is, not a particular hex.
        pageBg: getComputedStyle(document.documentElement).backgroundColor,
        cardHalo: halo.filter,
        scrolls: document.documentElement.scrollHeight > window.innerHeight + 1,
        scrollHeight: document.documentElement.scrollHeight,
      };
    }, asToday);
    await context.close();
    return seen;
  };

  const apse = await read({ height: 844, theme: "dark", scheme: "dark" });
  expect(vaultPaints(apse.stars)).toBe(true);
  expect(apse.position).toBe("fixed");
  expect(tileEdgePx(apse.tileSize)).toEqual({ w: 528, h: 528 });
  expect(isTopCenterPhase(apse.phase)).toBe(true);
  expect(apse.footerShown).toBe(false);
  expect(apse.cardHalo).toContain(apse.pageBg);
  expect(apse.scrolls).toBe(false);
  // The night carries no powdering: its field is the vault alone.
  expect(powderPaints(apse.stars)).toBe(false);

  // Nave keeps the same page geometry but paints no vault: its wall is
  // powdered with rosettes instead, in the same fixed field.
  for (const [theme, scheme] of [
    ["light", "light"],
    [null, "light"],
  ]) {
    const nave = await read({ height: 844, theme, scheme });
    expect(vaultPaints(nave.stars)).toBe(false);
    expect(powderPaints(nave.stars)).toBe(true);
    expect(nave.position).toBe("fixed");
    expect(nave.footerShown).toBe(false);
    expect(nave.scrolls).toBe(false);
    expect(nave.scrollHeight).toBe(apse.scrollHeight);
  }

  // On the day itself a phone's home fits its viewport whole, every hour in
  // reach without scrolling, on a 375x667 screen as well, even on the year's
  // longest heading (Gaudete with three commemorations and abstinence).
  for (const date of [testDate, "2026-12-13"]) {
    const small = await read({ width: 375, height: 667, theme: "light", scheme: "light", asToday: true, date });
    expect(small.scrolls).toBe(false);
  }

  // Representative phone corners (short, mid, tall). Tile size and top-centre
  // origin must not depend on viewport; a full width×height matrix is CI cost
  // without extra signal once those two invariants hold.
  for (const [width, height] of [
    [320, 667],
    [390, 844],
    [430, 932],
  ]) {
    const field = await read({ width, height, theme: "dark", scheme: "dark" });
    const bare = await read({ width, height, theme: "light", scheme: "light" });
    expect(vaultPaints(field.stars)).toBe(true);
    expect(field.position).toBe("fixed");
    expect(tileEdgePx(field.tileSize)).toEqual({ w: 528, h: 528 });
    expect(isTopCenterPhase(field.phase)).toBe(true);
    expect(field.scrollHeight).toBe(bare.scrollHeight);
  }
});

test("the mobile home vault survives browser-back viewport changes", async ({ page }) => {
  await openDatedPage(page, `/?date=${testDate}`, "dark");
  await page.locator(".pray-now").click();
  await expect(page.locator("body")).toHaveClass(/page-hour/);

  // Mobile browser chrome can shorten the effective viewport before restoring
  // a history entry. The vault must not be conditional on the 800px height the
  // home page happened to have when it was first painted.
  await page.setViewportSize({ width: 390, height: 740 });
  await page.goBack();
  await expect(page.locator("body")).toHaveClass(/page-home/);

  const field = await page.evaluate(() => {
    const style = getComputedStyle(document.body, "::before");
    return [style.content, style.maskImage || style.webkitMaskImage, style.maskPosition || style.webkitMaskPosition];
  });
  expect(field[0]).not.toBe("none");
  expect(field[1]).not.toBe("none");
  expect(isTopCenterPhase(field[2])).toBe(true);
});

test("the home vault is lit from the frontispiece in Apse only", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const gilding = () => page.evaluate(() => getComputedStyle(document.body, "::before").backgroundImage);
  await openDatedPage(page, `/?date=${testDate}`, "dark");
  expect(await gilding()).toContain("radial-gradient");
  const field = await page.evaluate(() => {
    const style = getComputedStyle(document.body, "::before");
    return { mask: style.maskImage || style.webkitMaskImage, size: style.maskSize || style.webkitMaskSize };
  });
  expect(field.mask).toContain("leaf.png");
  expect(field.size.startsWith("704px 704px")).toBe(true);
  await openDatedPage(page, `/?date=${testDate}`, "light");
  expect(await gilding()).toBe("none");
});

test("the inscription band carries the frontispiece heading in both themes", async ({ page }) => {
  await openDatedPage(page, `/?date=${testDate}`);

  const read = () =>
    page.evaluate(() => {
      const h = document.querySelector(".home-prayer-card h2");
      const card = document.querySelector(".home-hero");
      const hs = getComputedStyle(h);
      const hr = h.getBoundingClientRect();
      const cr = card.getBoundingClientRect();
      return {
        ground: hs.backgroundColor,
        ink: hs.color,
        // Full-bleed: the course reaches the frame, minus the card's border.
        bleed: Math.round(cr.width - hr.width) <= 4,
      };
    });

  const nave = await read();
  expect(nave.ground).not.toBe("rgba(0, 0, 0, 0)");
  expect(nave.bleed).toBe(true);

  await choosePreference(page, "Apse");
  // app.js dips the Apse vault invisible before it applies data-theme (so the
  // vault's background-image swaps while unseen instead of popping), which
  // holds this attribute back by ~100ms; the painted course then separately
  // crossfades for 200ms once it lands. toHaveAttribute retries, so it
  // covers the first wait; poll for the rendered colour below rather than
  // sampling the Nave end of that second transition on a fast single-worker
  // CI run.
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect.poll(async () => (await read()).ground).not.toBe(nave.ground);
  const apse = await read();
  expect(apse.ground).not.toBe("rgba(0, 0, 0, 0)");
  expect(apse.ground).not.toBe(nave.ground);
  expect(apse.bleed).toBe(true);

  // The band must never appear on a prayer page.
  await page.goto(`/lauds/${testDate}`);
  expect(await page.locator(".elements .home-prayer-card").count()).toBe(0);
});

test("the inscription band keeps the season with the rest of the gilding", async ({ page }) => {
  // Leaf lettering is gilding, so it goes quiet in Lent, veils in Passiontide
  // and warms in Paschaltide like the ✦ and the drop caps. The timber course
  // and the rubric-red ✠ do not move — the church veils its images, not its
  // rubrics. Both themes read one pair of leaf colours, because the ground is
  // dark in each.
  for (const theme of ["light", "dark"]) {
    const ink = {};
    const ground = {};
    const cross = {};
    for (const [season, date] of [
      ["ordinary", "2026-10-07"],
      ["quiet", testDate],
      ["passiontide", "2026-04-08"],
      ["eastertide", "2026-04-20"],
    ]) {
      const context = await page.context().browser().newContext();
      const sheet = await context.newPage();
      await sheet.addInitScript((t) => localStorage.setItem("office-theme", t), theme);
      await sheet.goto(`/?date=${date}`);
      const band = await sheet.evaluate(() => {
        const style = getComputedStyle(document.querySelector(".home-prayer-card h2"));
        return { ink: style.color, ground: style.backgroundColor };
      });
      ink[season] = band.ink;
      ground[season] = band.ground;
      await sheet.goto(`/vespers/${date}`);
      cross[season] = await sheet.locator(".cross").first().evaluate((node) => getComputedStyle(node).color);
      await context.close();
    }
    expect(new Set([ink.ordinary, ink.quiet, ink.passiontide, ink.eastertide]).size).toBe(4);
    for (const season of ["quiet", "passiontide", "eastertide"]) {
      expect(ground[season]).toBe(ground.ordinary);
      expect(cross[season]).toBe(cross.ordinary);
    }
  }
});

test("the hours' sun and moon and the Apse's gilt tituli keep the season", async ({ page }) => {
  // The sun and moon are images, and Apse's tituli are gilt, so they go
  // quiet, veil and warm with the drop caps. Nave's tituli are painted red
  // ochre and stay; Apse's moon is silver and keeps its own ink at Eastertide.
  const ink = {};
  for (const theme of ["light", "dark"]) {
    for (const [season, date] of [["ordinary", "2026-10-07"], ["quiet", "2026-03-24"], ["passiontide", "2026-04-08"], ["eastertide", "2026-04-20"]]) {
      const context = await page.context().browser().newContext();
      const sheet = await context.newPage();
      await sheet.addInitScript((t) => localStorage.setItem("office-theme", t), theme);
      const sign = async () => sheet.locator(".hour-header .ornament-headpiece > span[data-sign]")
        .evaluate((node) => getComputedStyle(node).backgroundColor);
      await sheet.goto(`/lauds/${date}`);
      const sun = await sign();
      const titulus = await sheet.locator(".antiphon em").first().evaluate((node) => getComputedStyle(node).color);
      await sheet.goto(`/vespers/${date}`);
      ink[`${theme} ${season}`] = { sun, moon: await sign(), titulus };
      await context.close();
    }
    const [ordinary, quiet, veiled, paschal] = ["ordinary", "quiet", "passiontide", "eastertide"].map((s) => ink[`${theme} ${s}`]);
    expect(new Set([ordinary.sun, quiet.sun, veiled.sun, paschal.sun]).size).toBe(4);
    expect(quiet.moon).not.toBe(ordinary.moon);
    expect(veiled.moon).not.toBe(ordinary.moon);
    if (theme === "dark") {
      expect(paschal.moon).toBe(ordinary.moon);
      expect(new Set([ordinary.titulus, quiet.titulus, veiled.titulus, paschal.titulus]).size).toBe(4);
    } else {
      expect(paschal.moon).not.toBe(ordinary.moon);
      for (const s of [quiet, veiled, paschal]) expect(s.titulus).toBe(ordinary.titulus);
    }
  }
});

test("a bare hour sets its capitals in the muted ink, puts out its sun, and darkens the Apse vault under the veil", async ({ page }) => {
  // The Triduum's hours and the Office of the Dead are said without hymn or
  // Gloria Patri, and the page takes off its leaf with them (hour-bare): the
  // drop caps are set in --muted rather than the gilding, the headpiece
  // carries no sun or moon and the title's rule closes over its place (the
  // little hours keep their cross), and on the Apse a veiled bare hour's
  // vault has no stars — the Triduum's, not All Souls', whose stars stay.
  const rgb = (hex) => {
    const n = parseInt(hex.slice(1), 16);
    return `rgb(${n >> 16}, ${(n >> 8) & 255}, ${n & 255})`;
  };
  const read = async (path, theme) => {
    await openDatedPage(page, path, theme);
    return page.evaluate(() => {
      const body = getComputedStyle(document.body);
      const head = document.querySelector(".hour-header");
      const sign = head.querySelector(".ornament-headpiece > span[data-sign]");
      const cross = head.querySelector(".ornament-headpiece > span:not([data-sign])");
      return {
        bare: document.body.classList.contains("hour-bare"),
        cap: getComputedStyle(document.querySelector(".psalm-verses .verse:first-child"), "::first-letter").color,
        muted: body.getPropertyValue("--muted").trim(),
        ornament: body.getPropertyValue("--ornament").trim(),
        sign: sign && getComputedStyle(sign).display,
        cross: cross && getComputedStyle(cross).display,
        rule: getComputedStyle(head).getPropertyValue("--title-rule-break").trim(),
        vault: body.getPropertyValue("--apse-vault").trim(),
        // The ending's field: home's sky tile by night, the powdering by day.
        field: getComputedStyle(document.querySelector(".hour-epilogue"), "::before").maskImage,
        // What the ending's two fields paint: under the veil by night, nothing at all.
        fieldInks: [
          getComputedStyle(document.querySelector(".hour-epilogue"), "::before").backgroundColor,
          getComputedStyle(document.querySelector("footer"), "::after").backgroundColor,
        ],
      };
    });
  };
  for (const theme of ["light", "dark"]) {
    const goodFriday = await read("/lauds/2026-04-10", theme);
    expect(goodFriday.bare).toBe(true);
    expect(goodFriday.cap).toBe(rgb(goodFriday.muted));
    expect(goodFriday.sign).toBe("none");
    expect(goodFriday.rule).toBe("0px");
    expect(goodFriday.vault).toBe("none");
    expect(goodFriday.field.includes("sky-tile.svg")).toBe(false);
    const clear = (ink) => ink === "transparent" || /rgba\(.*,\s*0\)$|\/ 0\)$/.test(ink);
    if (theme === "dark") for (const ink of goodFriday.fieldInks) expect(clear(ink), ink).toBe(true);
    // Prime keeps its cross, and the rule its break.
    const prime = await read("/prime/2026-04-10", theme);
    expect(prime.bare).toBe(true);
    expect(prime.cross).not.toBe("none");
    expect(prime.rule).not.toBe("0px");
    if (theme === "dark") for (const ink of prime.fieldInks) expect(clear(ink), ink).toBe(true);
    // All Souls is bare without the veil: the Apse's stars stay gold.
    const allSouls = await read("/lauds/2026-11-02", theme);
    expect(allSouls.bare).toBe(true);
    expect(allSouls.cap).toBe(rgb(allSouls.muted));
    expect(allSouls.sign).toBe("none");
    expect(allSouls.vault === "none").toBe(theme === "light");
    // A Lenten feria keeps its leaf, quiet; Holy Saturday's Vespers is Easter's.
    for (const path of ["/lauds/2026-03-11", "/vespers/2026-04-11"]) {
      const lit = await read(path, theme);
      expect(lit.bare).toBe(false);
      expect(lit.cap).toBe(rgb(lit.ornament));
      expect(lit.sign).not.toBe("none");
      expect(lit.rule).toBe("1.25rem");
      expect(lit.vault === "none").toBe(theme === "light");
      expect(lit.field.includes(theme === "light" ? "powder.svg" : "sky-tile.svg")).toBe(true);
    }
  }
});

test("Vespers of the Dead on the eve of All Souls goes bare where it begins", async ({ page }) => {
  // Diurnal pp. 641–642: Vespers of All Saints keep their gilding and frame;
  // from "Vespers of the Dead" (section-bare) the caps take the muted ink.
  const rgb = (hex) => {
    const n = parseInt(hex.slice(1), 16);
    return `rgb(${n >> 16}, ${(n >> 8) & 255}, ${n & 255})`;
  };
  for (const theme of ["light", "dark"]) {
    await openDatedPage(page, "/vespers/2026-11-01", theme);
    const got = await page.evaluate(() => {
      const body = getComputedStyle(document.body);
      const heading = document.querySelector(".elements > .section-heading.section-bare");
      const verses = [...document.querySelectorAll(".psalm-verses .verse:first-child")];
      const after = (v) => heading.compareDocumentPosition(v) & Node.DOCUMENT_POSITION_FOLLOWING;
      const cap = (v) => getComputedStyle(v, "::first-letter").color;
      return {
        bare: document.body.classList.contains("hour-bare"),
        heading: heading && heading.textContent,
        before: verses.filter((v) => !after(v)).map(cap),
        within: verses.filter(after).map(cap),
        muted: body.getPropertyValue("--muted").trim(),
        ornament: body.getPropertyValue("--ornament").trim(),
      };
    });
    expect(got.bare).toBe(false);
    expect(got.heading).toBe("Vespers of the Dead");
    expect(got.before.length).toBeGreaterThan(0);
    expect(got.within.length).toBe(6); // five psalms and the Magnificat
    expect(got.before.every((c) => c === rgb(got.ornament))).toBe(true);
    expect(got.within.every((c) => c === rgb(got.muted))).toBe(true);
  }
});

test("the Apse lifts the hour's colour band as home lifts its card, and home spaces its fasting badges", async ({ page }) => {
  const band = async (path, theme) => {
    await openDatedPage(page, path, theme);
    return page.locator(".color-band").evaluate((node) => getComputedStyle(node).backgroundColor);
  };
  // Violet (an Ember day) and red (an Ember day in Pentecost) sink into the night unlifted.
  expect(await band("/lauds/2026-09-16", "light")).toBe("rgb(106, 58, 138)");
  expect(await band("/lauds/2026-09-16", "dark")).toBe("rgb(141, 98, 173)");
  expect(await band("/lauds/2026-06-03", "light")).toBe("rgb(176, 42, 36)");
  expect(await band("/lauds/2026-06-03", "dark")).toBe("rgb(201, 80, 63)");
  // White is the vestment's linen in both themes.
  expect(await band("/lauds/2026-12-25", "dark")).toBe("rgb(232, 224, 204)");
  // Ash Wednesday's two badges stand a space apart.
  await openDatedPage(page, "/?date=2026-02-25");
  expect(await page.locator(".penitential").evaluate((node) => node.textContent.trim().replace(/\s+/g, " "))).toBe("Fasting Abstinence");
});

// The lozenge carries the day's liturgical colour; the colour is the same in
// either theme, so one theme suffices.
test("hour titles set their sign in the rule and the day's colour in the lozenge", async ({ page }) => {
  const title = async () => page.evaluate(() => {
    const heading = document.querySelector(".hour-header h1");
    const headpiece = document.querySelector(".hour-header .ornament-headpiece");
    return {
      lozenge: getComputedStyle(heading, "::after").backgroundColor,
      sign: headpiece.querySelector("span").dataset.sign || "cross",
    };
  });
  // Lauds keeps the sun, Vespers and Compline the moon, the little hours
  // the cross.
  await openDatedPage(page, "/lauds/2026-09-28");
  const green = await title();
  expect(green.sign).toBe("sun");
  await openDatedPage(page, "/lauds/2026-06-29");
  const red = await title();
  expect(red.lozenge).toBe("rgb(176, 42, 36)");
  expect(red.lozenge).not.toBe(green.lozenge);
  await openDatedPage(page, "/terce/2026-03-31");
  expect((await title()).sign).toBe("cross");
  await openDatedPage(page, "/vespers/2026-03-31");
  expect((await title()).sign).toBe("moon");
  await expect(page.locator(".hour-header .ornament-headpiece > span")).toBeVisible();
});

// The headpiece's box is one height whatever sign it carries, so the title
// stands at the same place on every hour of a day.
test("hour titles stand at one height under the sun, the moon and the cross", async ({ page }) => {
  const titleTop = async (hour) => {
    await openDatedPage(page, `/${hour}/2026-03-31`);
    return page.evaluate(() => document.querySelector(".hour-header h1").getBoundingClientRect().top);
  };
  const lauds = await titleTop("lauds");
  expect(Math.abs((await titleTop("terce")) - lauds)).toBeLessThan(0.5);
  expect(Math.abs((await titleTop("vespers")) - lauds)).toBeLessThan(0.5);
});

// The date and the season share one muted line, so its separator never ends
// a line; the day's name stands under them in the text's ink, a size up.
test("the hour's date line never strands its separator", async ({ page }) => {
  await openDatedPage(page, "/lauds/2026-03-31");
  const parts = page.locator(".hour-meta-part");
  await expect(parts).toHaveCount(3);
  await expect(page.locator(".hour-meta-sep")).toHaveCount(1);
  const rows = async () => parts.evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().top)));
  for (const width of [320, 390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    const [date, season, feast] = await rows();
    expect(date).toBe(season);
    expect(feast).toBeGreaterThan(date);
    await expect(page.locator(".hour-meta-sep")).toBeVisible();
  }
  const ink = await page.evaluate(() => {
    const feast = getComputedStyle(document.querySelector(".hour-meta-feast"));
    const meta = getComputedStyle(document.querySelector(".hour-meta"));
    return { feast: feast.color, text: getComputedStyle(document.body).color, larger: parseFloat(feast.fontSize) > parseFloat(meta.fontSize) };
  });
  expect(ink.feast).toBe(ink.text);
  expect(ink.larger).toBe(true);
});

// On a first-class day the frame narrows the measure: each disclosure under
// the title keeps one line, its caret beside its words, and where the two do
// not fit side by side they stand one above the other.
test("the hour's disclosures never break inside themselves", async ({ page }) => {
  for (const [width, size] of [[390, "default"], [375, "default"], [320, "default"], [375, "large"]]) {
    await page.setViewportSize({ width, height: 844 });
    await page.addInitScript((s) => localStorage.setItem("office-text-size", s), size);
    await openDatedPage(page, "/vespers/2026-12-25");
    // The fallback face is wider than EB Garamond: measure the face the reader gets.
    await page.evaluate(() => document.fonts.ready);
    const boxes = await page.locator(".hour-context-controls > details > summary:visible").evaluateAll((els) =>
      els.map((el) => {
        const r = el.getBoundingClientRect();
        const range = document.createRange();
        range.selectNodeContents(el);
        const tops = [...range.getClientRects()].filter((q) => q.width > 1).map((q) => q.top);
        // Spans of other faces sit a pixel or two apart on one line; a second line is a line below.
        const lines = 1 + tops.filter((t) => t - Math.min(...tops) > 8).length;
        return { left: r.left, right: r.right, height: r.height, lines: Math.min(lines, 2) };
      }),
    );
    for (const box of boxes) {
      expect(box.lines, `${width} ${size}`).toBe(1);
      expect(box.height).toBeGreaterThanOrEqual(44);
      expect(box.left).toBeGreaterThanOrEqual(0);
      expect(box.right).toBeLessThanOrEqual(width);
    }
  }
});

// A psalm's Latin incipit shares the label's line with a dot between them,
// or stands on its own line beneath without the dot: the dot never ends a
// line alone. Sunday Lauds has Psalm 93's long title, stacked on a phone, and
// the Song of the Three Children, whose name and incipit may outrun even a
// laptop's measure (Chromium's, not WebKit's): its name stays whole and the
// incipit goes beneath.
test("psalm titles stack their Latin incipit without a stranded dot", async ({ page }) => {
  await openDatedPage(page, "/lauds/2026-11-01");
  const labels = page.locator(".item-label:has(.label-sep)");
  expect(await labels.count()).toBeGreaterThan(3);
  // Only the shown prayer form's labels have a box to measure.
  const check = async () => labels.evaluateAll((els) => els.filter((el) => el.getBoundingClientRect().height > 0).map((el) => {
    const sep = el.querySelector(".label-sep");
    const incipit = el.querySelector(".psalm-incipit");
    const stacked = el.classList.contains("stacked");
    const sepShown = getComputedStyle(sep).display !== "none";
    const lines = Math.round(incipit.getBoundingClientRect().top) > Math.round(el.getBoundingClientRect().top) + 2;
    return { stacked, psalm: Boolean(el.closest(".psalm")), ok: stacked ? !sepShown && lines : sepShown && !lines };
  }));
  const phone = await check();
  expect(phone.every((l) => l.ok)).toBe(true);
  expect(phone.some((l) => l.stacked)).toBe(true);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.waitForFunction(() => !document.querySelector(".psalm > .item-label.stacked"));
  const laptop = await check();
  expect(laptop.every((l) => l.ok)).toBe(true);
  expect(laptop.filter((l) => l.psalm).every((l) => !l.stacked)).toBe(true);
});

// Every threshold between the office's parts carries the painted cross,
// including the Chapter's and the Hymn's headings, which open their own
// element rather than the list.
test("Chapter and Hymn headings carry the section cross", async ({ page }) => {
  await openDatedPage(page, "/lauds/2026-03-31");
  const marks = await page.evaluate(() => {
    const mark = (sel) => {
      const el = document.querySelector(sel);
      const cs = getComputedStyle(el, "::before");
      return { height: parseFloat(cs.height), mask: cs.maskImage || cs.webkitMaskImage };
    };
    return {
      responsory: mark(".elements > .section-heading:not(:first-child)"),
      chapter: mark(".elements > .chapter > .section-heading"),
      hymn: mark(".elements > .hymn > .section-heading"),
    };
  });
  expect(marks.chapter.height).toBeGreaterThan(5);
  expect(marks.chapter).toEqual(marks.responsory);
  expect(marks.hymn).toEqual(marks.responsory);
});

// iOS Safari sends a click only to what it judges tappable; a tap on the
// page's text or the header's bare bar produces none, so the overlays must not
// wait on a click to hear a tap outside them. Mimic that here by swallowing
// every click whose target is not a control before the page's listeners see it.
test("menus put away on a tap iOS Safari sends no click for", async ({ page }) => {
  await page.addInitScript(() => {
    window.addEventListener("click", (e) => {
      if (!e.target.closest("a, button, input, select, textarea, label, summary")) {
        e.stopImmediatePropagation();
      }
    }, true);
  });
  await openDatedPage(page, `/lauds?date=${testDate}`);

  const menu = page.locator(".site-menu");
  await page.getByText("Menu", { exact: true }).tap();
  await expect(menu).toHaveAttribute("open", "");
  // Beside the word, on the header's bar: still outside the menu's summary.
  // WebKit moves this tap's click onto the summary; it must not reopen it.
  const shell = await page.locator(".site-nav-shell").boundingBox();
  await page.touchscreen.tap(shell.x + shell.width - 2, shell.y + shell.height / 2);
  await expect(menu).not.toHaveAttribute("open", "");
  await page.getByText("Menu", { exact: true }).tap();
  await expect(menu).toHaveAttribute("open", "");
  await page.locator("footer > p").tap();
  await expect(menu).not.toHaveAttribute("open", "");
  // The word itself still closes it as well as opening it.
  await page.getByText("Menu", { exact: true }).tap();
  await page.getByText("Menu", { exact: true }).tap();
  await expect(menu).not.toHaveAttribute("open", "");

  // A wide iPad's Settings is the same kind of overlay.
  await page.setViewportSize({ width: 1024, height: 768 });
  const settings = page.locator(".site-settings");
  await settings.locator("summary").tap();
  await expect(settings).toHaveAttribute("open", "");
  await page.locator("footer > p").tap();
  await expect(settings).not.toHaveAttribute("open", "");
});

// The header's overlays are one state: at most one is open, and crossing the
// breakpoint puts it away, so a Settings opened wide does not come back after a
// trip through the phone layout (iOS and Android hold the same Overlay).
test("crossing the breakpoint puts the header's overlay away", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await openDatedPage(page, `/lauds?date=${testDate}`);
  const menu = page.locator(".site-menu");
  const settings = page.locator(".site-settings");
  await settings.locator("summary").click();
  await expect(settings).toHaveAttribute("open", "");

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(settings).not.toHaveAttribute("open", "");
  await expect(menu).not.toHaveAttribute("open", "");
  await page.getByText("Menu", { exact: true }).click();
  await expect(menu).toHaveAttribute("open", "");

  // Wide again: the menu is the desktop nav, and Settings stays shut.
  await page.setViewportSize({ width: 1024, height: 768 });
  await expect(menu).toHaveAttribute("open", "");
  await expect(settings).not.toHaveAttribute("open", "");
  await expect(page.locator(".settings-prefs")).toBeHidden();
  // Escape still finds the overlay open now, whichever it is.
  await settings.locator("summary").click();
  await page.keyboard.press("Escape");
  await expect(settings).not.toHaveAttribute("open", "");
  await expect(settings.locator("summary")).toBeFocused();
});

test("desktop navigation and frontispiece remain composed", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/?date=${testDate}`);

  await expect(page.locator(".site-menu")).toHaveAttribute("open", "");
  // The frontispiece is home's hour directory; the header does not repeat it.
  const primary = page.getByRole("navigation", { name: "Primary" });
  await expect(primary.getByRole("link", { name: "Ordo", exact: true })).toBeVisible();
  await expect(primary.locator('[data-nav="hour"]')).toHaveCount(0);
  await expect(page.locator(".menu-prefs")).toBeHidden();
  // The preferences wait under Settings at the end of the header; the page
  // ends in its footer line alone.
  const settings = page.locator(".site-settings");
  await expect(settings).not.toHaveAttribute("open", "");
  await expect(page.locator(".settings-prefs")).toBeHidden();
  await expect(page.locator("footer").getByRole("button")).toHaveCount(0);
  await settings.locator("summary").click();
  await expect(page.locator(".settings-prefs").getByRole("button", { name: "Apse", exact: true })).toBeVisible();
  // An overlay: a preference chosen inside leaves it open, a tap outside or Escape puts it away.
  await page.locator(".settings-prefs").getByRole("button", { name: "Nave", exact: true }).click();
  await expect(settings).toHaveAttribute("open", "");
  // It opens below the header and lies over the page, not under the niche.
  const [header, panel] = await Promise.all([
    page.locator(".site-header").boundingBox(),
    page.locator(".settings-prefs").boundingBox(),
  ]);
  expect(panel.y).toBeGreaterThanOrEqual(header.y + header.height - 1);
  const onTop = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.closest(".settings-prefs") !== null, {
    x: panel.x + panel.width / 2,
    y: panel.y + panel.height - 8,
  });
  expect(onTop).toBe(true);
  await page.locator("footer").click();
  await expect(settings).not.toHaveAttribute("open", "");
  await settings.locator("summary").click();
  await page.keyboard.press("Escape");
  await expect(settings).not.toHaveAttribute("open", "");
  await expect(settings.locator("summary")).toBeFocused();
  await expect(page.getByRole("heading", { name: "Morning", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Evening", exact: true })).toBeVisible();
  await expect(page.locator(".home-hour-links.is-ruled")).toHaveCount(1);
  const rules = await page.evaluate(() => {
    const isVisibleRule = (width, style, color) => {
      const alpha = color.match(/^rgba?\([^,]+,[^,]+,[^,]+(?:,\s*([^)]+))?\)$/)?.[1];
      const alphaValue = alpha === undefined ? 1 : Number.parseFloat(alpha);
      return (
        parseFloat(width) >= 1 &&
        style === "solid" &&
        color !== "transparent" &&
        alphaValue > 0
      );
    };
    const rule = (el, side) => {
      const style = getComputedStyle(el);
      return isVisibleRule(
        style.getPropertyValue(`border-${side}-width`),
        style.getPropertyValue(`border-${side}-style`),
        style.getPropertyValue(`border-${side}-color`),
      );
    };
    const directory = document.querySelector(".home-hour-links");
    const label = document.querySelector(".home-hour-group-label");
    const dividedHour = document.querySelector(".home-hour-group-morning .home-hour-link + .home-hour-link");
    const finalGroup = document.querySelector(".home-hour-group:last-of-type");
    const point = getComputedStyle(dividedHour, "::before");
    return {
      // The table is ruled by hand (app.js), so its CSS borders stand down:
      // a frame and the joints between bands, drawn as the painter's lines.
      ruled: directory.classList.contains("is-ruled"),
      ruledLines: directory.querySelectorAll(":scope > svg.home-rules path").length > 0,
      directoryLeft: rule(directory, "left"),
      directoryRight: rule(directory, "right"),
      finalGroupBottom: rule(finalGroup, "bottom"),
      // The periods are the margin's index with no cell or upright of their
      // own, and neighbouring hours are parted by a gilt point, not a rule.
      labelRight: rule(label, "right"),
      dividedHourLeft: rule(dividedHour, "left"),
      giltPoint: point.content !== "none" && parseFloat(point.width) < 5,
    };
  });
  expect(rules).toEqual({
    ruled: true,
    ruledLines: true,
    directoryLeft: false,
    directoryRight: false,
    finalGroupBottom: false,
    labelRight: false,
    dividedHourLeft: false,
    giltPoint: true,
  });
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1),
  ).toBe(false);
});

test("desktop frontispiece fits its breakpoint and reader sizes", async ({ page }) => {
  await page.addInitScript(() => localStorage.removeItem("office-text-size"));

  for (const width of [701, 1024, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, `/?date=${testDate}`);

    for (const size of ["default", "large", "small"]) {
      if (size !== "default") {
        await choosePreference(page, size === "large" ? "Larger text" : "Smaller text");
      }

      const geometry = await page.evaluate(() => {
        const hero = document.querySelector(".home-hero").getBoundingClientRect();
        const targetHeights = Array.from(document.querySelectorAll(".home-hour-link")).map(
          (link) => link.getBoundingClientRect().height,
        );
        return {
          overflows: document.documentElement.scrollWidth > window.innerWidth + 1,
          heroContained: hero.left >= 0 && hero.right <= window.innerWidth,
          shortestTarget: Math.min(...targetHeights),
        };
      });
      expect(geometry.overflows, `${width}px/${size} should not overflow`).toBe(false);
      expect(geometry.heroContained, `${width}px/${size} should contain the frame`).toBe(true);
      expect(geometry.shortestTarget, `${width}px/${size} hour targets`).toBeGreaterThanOrEqual(44);

      if (size === "large") {
        await choosePreference(page, "Default text size");
      }
    }
  }
});

test("appearance and text size choices persist across prayer navigation", async ({ page }) => {
  await page.goto(`/?date=${testDate}`);

  // A passive visit must not invent a stored preference.
  expect(await page.evaluate(() => localStorage.getItem("office-text-size"))).toBeNull();
  await expect(page.locator("html")).not.toHaveAttribute("data-text-size", /./);

  await choosePreference(page, "Apse");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await choosePreference(page, "Larger text");
  await expect(page.locator("html")).toHaveAttribute("data-text-size", "large");

  await page.goto(`/lauds/${testDate}`);
  await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-text-size", "large");
  // Both copies (menu and Settings) reflect the stored choice.
  const large = page.locator('.text-size-option[data-text-size-choice="large"]');
  await expect(large).toHaveCount(2);
  for (const button of await large.all()) await expect(button).toHaveAttribute("aria-pressed", "true");
});

test("larger text grows the prayer without breaking the phone layout", async ({ page }) => {
  await openDatedPage(page, `/lauds/${testDate}`);

  const prayerSize = () =>
    page.evaluate(() =>
      parseFloat(getComputedStyle(document.querySelector(".elements") || document.body).fontSize),
    );
  const overflows = () =>
    page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1);

  const base = await prayerSize();
  expect(await overflows()).toBe(false);

  await choosePreference(page, "Larger text");
  expect(await prayerSize()).toBeGreaterThan(base);
  expect(await overflows()).toBe(false);

  // Every menu preference keeps a thumb-sized target at the largest setting,
  // and the two rows share columns so each theme sits over a text size.
  await page.locator(".site-menu > summary").click();
  await expect(page.locator(".site-menu")).toHaveAttribute("open", "");
  const measure = () =>
    page.evaluate(() => {
      const rects = (sel) =>
        Array.from(document.querySelectorAll(`.menu-prefs ${sel}`)).map((el) => {
          const r = el.getBoundingClientRect();
          return { left: r.left, width: r.width, height: r.height };
        });
      return {
        themes: rects(".theme-option[data-theme-choice]"),
        sizes: rects(".text-size-option"),
        martyrology: rects(".martyrology-option"),
      };
    });
  // The panel is laid out once the menu has opened (it settles in from a
  // closing transition); measure it then.
  await expect
    .poll(async () => {
      const c = await measure();
      return Math.min(...[...c.themes, ...c.sizes, ...c.martyrology].map((r) => r.height));
    })
    .toBeGreaterThanOrEqual(44);
  const cells = await measure();
  const all = [...cells.themes, ...cells.sizes, ...cells.martyrology];
  expect(all).toHaveLength(8);
  expect(Math.min(...all.map((r) => r.height))).toBeGreaterThanOrEqual(44);
  cells.themes.forEach((r, i) => {
    expect(Math.abs(r.left - cells.sizes[i].left)).toBeLessThan(1);
    expect(Math.abs(r.width - cells.sizes[i].width)).toBeLessThan(1);
  });
  // Off and On sit under the last two themes.
  cells.martyrology.forEach((r, i) => {
    expect(Math.abs(r.left - cells.themes[i + 1].left)).toBeLessThan(1);
    expect(Math.abs(r.width - cells.themes[i + 1].width)).toBeLessThan(1);
  });
  expect(await overflows()).toBe(false);
  await page.locator(".site-menu > summary").click();

  await choosePreference(page, "Smaller text");
  expect(await prayerSize()).toBeLessThan(base);
  expect(await overflows()).toBe(false);

  await choosePreference(page, "Default text size");
  expect(await prayerSize()).toBeCloseTo(base, 1);
  await expect(page.locator("html")).not.toHaveAttribute("data-text-size", /./);
});

test("hour typography keeps the liturgical hierarchy across themes and narrow phone widths", async ({
  page,
}) => {
  const bodySizes = [];

  for (const theme of ["light", "dark"]) {
    for (const width of [320, 390, 540]) {
      await page.setViewportSize({ width, height: 844 });
      await openDatedPage(page, "/vespers/2026-06-18", theme);

      const metrics = await page.evaluate(() => {
        const style = (selector, pseudo) => getComputedStyle(document.querySelector(selector), pseudo);
        const px = (selector, property) => parseFloat(style(selector)[property]);
        const firstLetter = (selector) => {
          const cap = style(selector, "::first-letter");
          return {
            float: cap.float,
            size: parseFloat(cap.fontSize),
            raised: document.querySelector(selector).classList.contains("initial-raised"),
          };
        };
        const hymnLine = style(".hymn-stanza-opening .hymn-line:nth-child(3)");
        return {
          prayer: px(".elements", "fontSize"),
          heading: px(".section-heading", "fontSize"),
          psalmItem: px(".psalm > .item-label", "fontSize"),
          marianItem: px(".marian-antiphon > .item-label", "fontSize"),
          rubric: px(".rubric", "fontSize"),
          chapterRef: px(".chapter-ref", "fontSize"),
          scriptureRef: px(".scripture-ref", "fontSize"),
          rubricStyle: style(".rubric").fontStyle,
          chapterRefStyle: style(".chapter-ref").fontStyle,
          scriptureRefStyle: style(".scripture-ref").fontStyle,
          rubricColor: style(".rubric").color,
          chapterRefColor: style(".chapter-ref").color,
          scriptureRefColor: style(".scripture-ref").color,
          crossColor: style(".cross").color,
          psalmLeading: px(".psalm-verses", "lineHeight"),
          hymnLine: {
            display: hymnLine.display,
            padding: parseFloat(hymnLine.paddingLeft),
            indent: parseFloat(hymnLine.textIndent),
          },
          hymnOpening: (() => {
            const opening = document.querySelector(".hymn-stanza-opening .hymn-line");
            const glyph = (following) => {
              const text = following ? opening.querySelector(".initial-word").firstChild : opening.firstChild;
              const range = document.createRange();
              range.setStart(text, 0);
              range.setEnd(text, 1);
              const { left, right } = range.getBoundingClientRect();
              return { left, right };
            };
            return {
              cap: glyph(false),
              following: glyph(true),
              padding: parseFloat(getComputedStyle(opening).paddingLeft),
              indent: parseFloat(getComputedStyle(opening).textIndent),
            };
          })(),
          caps: [
            ".collect .plain-line",
            ".hymn-stanza-opening .hymn-line",
            ".marian-antiphon .chant-line-opening",
            ".corporate-lord-prayer-officiant",
            ".short-responsory-opening .sigil-text",
          ].map(firstLetter),
          secretCap: firstLetter(".secret-text"),
          overflow: document.documentElement.scrollWidth > window.innerWidth + 1,
        };
      });

      if (theme === "light") bodySizes.push(metrics.prayer);
      const label = `${theme}/${width}px`;
      expect(metrics.overflow, `${label} horizontal overflow`).toBe(false);
      expect(metrics.heading, `${label} section heading`).toBeGreaterThanOrEqual(metrics.prayer * 0.9);
      expect(metrics.heading, `${label} section heading`).toBeLessThanOrEqual(metrics.prayer);
      expect(metrics.psalmItem, `${label} psalm label`).toBeCloseTo(metrics.prayer, 1);
      expect(metrics.marianItem, `${label} unrelated item label`).toBeLessThan(metrics.prayer);
      expect(metrics.rubric, `${label} rubric`).toBeGreaterThanOrEqual(metrics.prayer * 0.84);
      expect(metrics.rubric, `${label} rubric`).toBeLessThan(metrics.prayer);
      expect(metrics.chapterRef, `${label} chapter reference`).toBeLessThan(metrics.prayer);
      expect(metrics.scriptureRef, `${label} scripture reference`).toBeLessThan(metrics.prayer);
      expect(metrics.rubricStyle, `${label} rubric face`).toBe("normal");
      expect(metrics.chapterRefStyle, `${label} chapter reference face`).toBe("normal");
      expect(metrics.scriptureRefStyle, `${label} scripture reference face`).toBe("normal");
      expect(metrics.chapterRefColor, `${label} chapter reference color`).toBe(metrics.rubricColor);
      expect(metrics.scriptureRefColor, `${label} scripture reference color`).toBe(metrics.rubricColor);
      expect(metrics.crossColor, `${label} cross color`).toBe(metrics.rubricColor);
      expect(metrics.psalmLeading / metrics.prayer, `${label} psalm leading`).toBeCloseTo(1.65, 2);
      expect(metrics.hymnLine.display, `${label} hymn line flow`).toBe("block");
      expect(metrics.hymnLine.padding, `${label} hymn continuation inset`).toBeGreaterThan(0);
      expect(metrics.hymnLine.indent, `${label} hymn first-line offset`).toBeLessThan(0);
      expect(metrics.hymnLine.padding + metrics.hymnLine.indent, `${label} hymn stanza edge`).toBeCloseTo(
        0,
        1,
      );
      // The opening cap is floated at the stanza edge, so it must not also
      // take the normal hanging indent. That indentation both offsets the cap
      // from the metrical edge and moves the next glyph beneath it on phones.
      expect(metrics.hymnOpening.padding, `${label} hymn opening padding`).toBe(0);
      expect(metrics.hymnOpening.indent, `${label} hymn opening indent`).toBe(0);
      expect(
        metrics.hymnOpening.following.left,
        `${label} hymn opening glyph clears the drop cap`,
      ).toBeGreaterThanOrEqual(metrics.hymnOpening.cap.right - 0.5);
      for (const cap of metrics.caps) {
        expect(cap.float, `${label} approved opening initial`).toBe(cap.raised ? "none" : "left");
        expect(cap.size, `${label} approved opening initial size`).toBeGreaterThan(
          metrics.prayer * (cap.raised ? 1.5 : 2),
        );
      }
      expect(metrics.secretCap.float, `${label} secret prayer never gets a drop cap`).not.toBe("left");
    }
  }

  // The default eases from 19px on the smallest supported phones to the
  // historical 20px prayer size at the primary 390px design width.
  expect(bodySizes[0]).toBeLessThan(bodySizes[1]);
  expect(bodySizes[1]).toBeCloseTo(bodySizes[2], 1);

  // Individual commemoration headings already name the section. The generic
  // heading was a redundant equal-tier interruption immediately before them.
  await expect(page.getByRole("heading", { name: "Commemorations", exact: true })).toHaveCount(0);
  await expect(
    page.getByRole("heading", {
      name: "Commemoration of St Ephrem the Syrian, Deacon, Confessor & Doctor",
      exact: true,
    }),
  ).toBeVisible();
});

test("themes never change layout", async ({ browser }) => {
  // Light and Apse differ only in colour and material. Every element box and
  // every text line must match, so geometry tests need run in one theme.
  const open = async (theme) => {
    const context = await browser.newContext();
    await context.addInitScript((t) => localStorage.setItem("office-theme", t), theme);
    return { context, sheet: await context.newPage() };
  };
  const fingerprint = async (sheet, path, width) => {
    await sheet.setViewportSize({ width, height: 900 });
    await sheet.goto(path);
    await sheet.evaluate(() => document.fonts.ready);
    // The ordo's faces are font-display: optional, so a face that misses the
    // first paint is replaced by a fallback for the whole visit. Which visits
    // miss is a race, separate for each theme's page. The bold face is not
    // preloaded, and the fasting and abstinence columns are its only use, so
    // at 1280px one theme sometimes set the column's strut in Georgia and
    // the other in Garamond (#798). Measure the cached-font visit, as the
    // visual baselines do.
    if (path.startsWith("/calendar/")) {
      await sheet.reload();
      await sheet.evaluate(() => document.fonts.ready);
    }
    return sheet.evaluate(() => {
      const round = (r) => [r.x, r.y, r.width, r.height].map((v) => Math.round(v * 10)).join(",");
      const range = document.createRange();
      return {
        // Home's night sky is the Apse's material, as the vault is the
        // page's, the lining moves out to the sky's edge, and in the wide
        // niche the panel sits inside the night's slate course where by day
        // it meets the reveal's fillet: painted layers, absolutely placed and
        // aria-hidden, that move no other box.
        boxes: [...document.querySelectorAll("body *")]
          .filter((el) => !el.closest(".home-sky, .home-lining") && !(innerWidth > 700 && el.closest(".home-arch-fill")))
          .map((el) => round(el.getBoundingClientRect())),
        lines: [...document.querySelectorAll(".verse, .plain-line, .hymn-line, .antiphon")].map((el) => {
          range.selectNodeContents(el);
          return [...range.getClientRects()].map(round).join(";");
        }),
      };
    });
  };
  const light = await open("light");
  const dark = await open("dark");
  // A month of the ordo is light enough for both widths; the whole year's
  // 14,000 elements keep to one phone width.
  for (const [path, widths] of [
    ["/vespers/2026-06-18", [320, 1280]],
    ["/lauds/2026-09-13", [320, 1280]],
    [`/?date=${testDate}`, [320, 1280]],
    ["/calendar/2026/03", [320, 1280]],
    ["/calendar/2026", [320, 1280]],
    ["/calendar/2026/all", [390]],
  ]) {
    for (const width of widths) {
      const [a, b] = await Promise.all([fingerprint(light.sheet, path, width), fingerprint(dark.sheet, path, width)]);
      // Compare flat lists and report only the first difference: the whole
      // year has ~14,000 boxes, too many for a deep-equality diff to be useful.
      for (const kind of ["boxes", "lines"]) {
        const at = a[kind].findIndex((value, i) => value !== b[kind][i]);
        const first = at < 0 && a[kind].length === b[kind].length ? null : { at, light: a[kind][at], dark: b[kind][at] };
        expect(first, `${path} ${width}px ${kind}`).toBeNull();
      }
    }
  }
  await light.context.close();
  await dark.context.close();
});

test("short prose openings keep one baseline and adapt to the reading measure", async ({ page }) => {
  await page.route("**/lauds/2026-06-18", async route => {
    const response = await route.fetch();
    await route.fulfill({ response, body: (await response.text()).replace(
      /(<main\b[^>]*>)[\s\S]*?(<\/main>)/,
      '$1<div class="elements"><div class="chapter"><div class="liturgical-block"><p class="plain-line">O God, thou art my God <span class="mediant">*</span> early will I seek thee.</p></div></div></div>$2',
    ) });
  });
  // Light only: themes share one geometry, held page-wide by "themes never change layout".
  const theme = "light";
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, "/lauds/2026-06-18", theme);
  const opening = page.locator(".chapter .plain-line");
  const originalText = await opening.textContent();
  for (const [width, raised] of [[1280, true], [320, false], [430, true]]) {
    await page.setViewportSize({ width, height: 900 });
    await expect.poll(() => opening.evaluate(el => el.classList.contains("initial-raised"))).toBe(raised);
    const geometry = await opening.evaluate(el => {
      const style = getComputedStyle(el);
      const capStyle = getComputedStyle(el, "::first-letter");
      const ctx = document.createElement("canvas").getContext("2d");
      const glyph = (index, font) => {
        const range = document.createRange();
        const node = index ? el.querySelector(".initial-word").firstChild : el.firstChild;
        range.setStart(node, 0);
        range.setEnd(node, 1);
        const rect = range.getBoundingClientRect();
        ctx.font = font;
        return {
          left: rect.left, right: rect.right,
          baseline: rect.top + ctx.measureText("H").fontBoundingBoxAscent,
        };
      };
      return {
        height: el.getBoundingClientRect().height,
        leading: parseFloat(style.lineHeight),
        cap: glyph(0, capStyle.font),
        following: glyph(1, style.font),
        overflow: document.documentElement.scrollWidth > innerWidth + 1,
      };
    });
    expect(geometry.overflow, `${theme}/${width} overflow`).toBe(false);
    expect(geometry.following.left).toBeGreaterThanOrEqual(geometry.cap.right - .5);
    if (raised) {
      expect(Math.abs(geometry.cap.baseline - geometry.following.baseline)).toBeLessThan(1.5);
      expect(geometry.height).toBeCloseTo(geometry.leading, 0);
    } else {
      expect(geometry.height).toBeGreaterThan(geometry.leading * 1.9);
    }
  }
  // At this measure the normal setting fits; Large needs two lines.
  await choosePreference(page, "Larger text");
  await expect(opening).not.toHaveClass(/initial-raised/);
  await choosePreference(page, "Default text size");
  await expect(opening).toHaveClass(/initial-raised/);
  expect(await opening.textContent()).toBe(originalText);
});

// WebKit takes the stack's first available face as the primary font whose
// ascent and descent place every baseline, even a face whose unicode-range
// holds the one glyph ✠; headed by the cross face, Safari sank each line's
// text in its leading and hung the dropped initials under their second line.
// The cross face follows the text faces, and still supplies the cross.
test("the cross face follows the text faces in every stack and still sets the cross", async ({ page }) => {
  const families = (selector) => page.evaluate((sel) => getComputedStyle(document.querySelector(sel)).fontFamily
    .split(",").map((family) => family.trim().replace(/^"|"$/g, "")), selector);
  const crossWidth = (family) => page.evaluate((f) => {
    const span = document.createElement("span");
    span.textContent = "\u2720";
    if (f) span.style.fontFamily = f;
    document.body.appendChild(span);
    const width = span.getBoundingClientRect().width;
    span.remove();
    return width;
  }, family);
  await openDatedPage(page, "/vespers/2026-10-07");
  const prayer = await families("body");
  expect(prayer[0]).toBe("EB Garamond");
  expect(prayer.indexOf("Office Cross")).toBeGreaterThan(0);
  expect(prayer.indexOf("Office Cross")).toBeLessThan(prayer.indexOf("Georgia"));
  expect(await page.evaluate(() => document.fonts.check('1em "Office Cross"', "\u2720"))).toBe(true);
  const inherited = await crossWidth("");
  expect(inherited).toBeCloseTo(await crossWidth('"Office Cross"'), 1);
  expect(inherited).not.toBeCloseTo(await crossWidth('"EB Garamond", serif'), 1);
  await openDatedPage(page, "/calendar/2026/10");
  const ordo = await families(".page-calendar");
  expect(ordo[0]).toBe("Ordo Garamond");
  expect(ordo.indexOf("Ordo Cross")).toBe(ordo.length - 2);
  const detail = await families(".day-office-incipit");
  expect(detail[0]).toBe("Georgia");
  expect(detail.indexOf("Ordo Cross")).toBe(detail.length - 2);
});

test("short psalm openings keep the same initial rank as adjacent psalms", async ({ page }) => {
  // Light only: themes share one geometry, held page-wide by "themes never change layout".
  const theme = "light";
  await openDatedPage(page, "/vespers/2026-09-12", theme);
  const psalms = page.locator(".psalm");
  const opening = psalms.nth(0).locator(".verse").first();
  const following = psalms.nth(1).locator(".verse").first();
  const original = await opening.textContent();
  await expect(psalms.nth(0)).toContainText("Psalm 144b (145b)");
  await expect(psalms.nth(1)).toContainText("Psalm 145 (146)");
  // A one-line verse keeps its full-size initial and natural text flow.
  // Returning to wide after narrow also exercises font-size changes.
  for (const width of [1280, 390, 768, 320, 1280]) {
    await page.setViewportSize({ width, height: 1000 });
    for (const size of ["normal", "large"]) {
      await page.evaluate(value => document.documentElement.setAttribute("data-text-size", value), size);
      // With its "Sit." cue the opening no longer fits one wide line in
      // large type either, so both sizes divide at the mediant. 768px is a
      // knife edge for it: a browser's rounding decides whether its two
      // halves fit one line, so either setting may stand there, as long as
      // it is settled and never raised.
      const expected = width === 1280 ? true : width === 768 ? null : false;
      await expect.poll(() => opening.evaluate(el => el.classList.contains("initial-raised"))).toBe(false);
      if (expected !== null) await expect.poll(() => opening.evaluate(el => el.classList.contains("initial-divided"))).toBe(expected);
      else await expect.poll(() => opening.evaluate(el => el.classList.contains("initial-divided") !== el.classList.contains("initial-elevated"))).toBe(true);
      const divided = await opening.evaluate(el => el.classList.contains("initial-divided"));
      const elevated = width >= 768 && !divided;
      await expect(following).not.toHaveClass(/initial-raised|initial-divided/);
      const geometry = await opening.evaluate(el => {
        const cap = getComputedStyle(el, "::first-letter");
        const range = document.createRange();
        // Measure the verse's own words, past any smaller posture cue.
        const mediant = el.querySelector(".mediant");
        const after = mediant.nextElementSibling?.matches(".posture") ? mediant.nextElementSibling.nextSibling : mediant.nextSibling;
        const start = after.textContent.search(/\S/);
        range.setStart(after, start);
        range.setEnd(after, start + 1);
        const secondHalf = range.getBoundingClientRect();
        range.selectNodeContents(el.querySelector(".initial-word"));
        const firstWord = range.getBoundingClientRect();
        const rect = el.getBoundingClientRect();
        return {
          size: cap.fontSize,
          // Engines without initial-letter (WebKit, Firefox) take the
          // stylesheet's float fallback; matching the next psalm's letter size holds the rank there.
          initial: CSS.supports("initial-letter", "2") ? cap.initialLetter : null,
          height: rect.height,
          leading: parseFloat(getComputedStyle(el).lineHeight),
          halfVerseGap: secondHalf.top - firstWord.top,
          nextTop: el.nextElementSibling.getBoundingClientRect().top,
          bottom: rect.bottom,
        };
      });
      expect(geometry.size).toBe(await following.evaluate(el => getComputedStyle(el, "::first-letter").fontSize));
      if (geometry.initial !== null) expect(geometry.initial).toBe(elevated ? "2 1" : "2");
      expect(geometry.height).toBeGreaterThanOrEqual(geometry.leading - 1);
      expect(geometry.nextTop).toBeGreaterThanOrEqual(geometry.bottom);
      if (width >= 768) {
        expect(await openingTextLines(opening)).toBeCloseTo(divided ? 2 : 1, 1);
        expect(geometry.halfVerseGap).toBeCloseTo(divided ? geometry.leading : 0, 0);
      }
      expect(await opening.textContent()).toBe(original);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    }
  }
});

test("Psalm 63 balances short tails but lets a complete opening stay on one line", async ({ page }) => {
  // Light only: themes share one geometry, held page-wide by "themes never change layout".
  const theme = "light";
  await openDatedPage(page, "/lauds/2026-09-13", theme);
  const psalm = page.locator(".psalm").filter({ hasText: "Psalm 62 (63)" });
  const opening = psalm.locator(".verse").first();
  const original = await opening.textContent();
  for (const size of ["normal", "large"]) {
    await page.evaluate(value => document.documentElement.dataset.textSize = value, size);
    for (const width of [320, 390, 414, 430, 1280, 390]) {
      await page.setViewportSize({ width, height: 900 });
      // The opening carries its "Sit." cue after the mediant. On the
      // narrowest screen the tail wraps naturally and on a desktop the whole
      // opening fits one line; at the phone widths between, a browser's
      // rounding decides whether the tail is short enough to divide for, so
      // either setting may stand, as long as it is settled.
      const expected = width === 320 || width === 1280 ? false : null;
      if (expected !== null) await expect.poll(() => opening.evaluate(el => el.classList.contains("initial-divided"))).toBe(expected);
      // The typesetter runs in the next animation frame after a resize.
      else await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      const divided = await opening.evaluate(el => el.classList.contains("initial-divided"));
      await expect(opening).not.toHaveClass(/initial-raised/);
      const lines = await openingTextLines(opening);
      const elevated = width === 1280;
      expect(lines).toBeCloseTo(elevated ? 1 : 2, 1);
      expect(await opening.evaluate(el => el.classList.contains("initial-elevated"))).toBe(elevated);
      expect(await opening.textContent()).toBe(original);
      // Divided, the "Sit." cue ends the first line beside its mediant: at the
      // head of the second, beside the initial, it read as the half-verse's
      // first word.
      if (divided) {
        const bottoms = await opening.evaluate(el => {
          const cue = el.querySelector(".mediant + .posture");
          const after = cue.nextSibling;
          const range = document.createRange();
          const start = after.textContent.search(/\S/);
          range.setStart(after, start);
          range.setEnd(after, start + 1);
          return {
            leading: parseFloat(getComputedStyle(el).lineHeight),
            first: el.querySelector(".initial-word").getBoundingClientRect().bottom,
            cue: cue.getBoundingClientRect().bottom,
            after: range.getBoundingClientRect().bottom,
          };
        });
        expect(Math.abs(bottoms.cue - bottoms.first)).toBeLessThan(bottoms.leading / 2);
        expect(bottoms.after - bottoms.cue).toBeGreaterThan(bottoms.leading / 2);
      }
      // Repeated measurement (also used for printing/font changes) must not
      // alternate between natural and divided settings at the same measure.
      await page.evaluate(() => {
        window.dispatchEvent(new Event("beforeprint"));
        window.dispatchEvent(new Event("beforeprint"));
      });
      await expect.poll(() => opening.evaluate(el => el.classList.contains("initial-divided"))).toBe(divided);
    }
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.evaluate(() => document.documentElement.dataset.textSize = "normal");
  await expect(opening).not.toHaveClass(/initial-divided/);
  await expectInitialInkClear(page, psalm.locator(".psalm-verses"), ".verse:first-child", `${theme} single-line O clears the numbered verse`);
  await expectInitialInkClear(page, psalm.locator(".psalm-verses"), ".verse:first-child", `${theme} full-size O sits on the first baseline`, ".initial-word");
  await page.setViewportSize({ width: 768, height: 900 });
  const fallback = await fallbackInitials(page);
  await page.evaluate(() => window.dispatchEvent(new Event("beforeprint")));
  await expect(opening).toHaveClass(/initial-elevated/);
  await expectInitialInkClear(page, psalm.locator(".psalm-verses"), ".verse:first-child", `${theme} fallback O sits on the first baseline`, ".initial-word");
  await fallback.evaluate(el => el.remove());
});

test("a half-verse break is rejected when it would add a third line", async ({ page }) => {
  await page.route("**/vespers/2026-06-18", async route => {
    const response = await route.fetch();
    await route.fulfill({ response, body: (await response.text()).replace(
      /(<main\b[^>]*>)[\s\S]*?(<\/main>)/,
      '$1<div class="elements"><div class="psalm"><div class="psalm-verses"><p class="verse">O <span class="mediant">*</span> God, thou art my God early will I seek thee.</p><p class="verse numbered"><span class="verse-num">2</span><span class="verse-body">My soul thirsteth for thee.</span></p></div></div></div>$2',
    ) });
  });
  await page.setViewportSize({ width: 375, height: 900 });
  await openDatedPage(page, "/vespers/2026-06-18");
  const opening = page.locator(".verse").first();
  await expect(opening).not.toHaveClass(/initial-divided|initial-raised/);
  const lineCount = () => opening.evaluate(el => el.getBoundingClientRect().height / parseFloat(getComputedStyle(el).lineHeight));
  expect(await lineCount()).toBeCloseTo(2, 1);
  // Establish that this fixture really would become longer with the break.
  expect(await opening.evaluate(el => {
    el.classList.add("initial-divided");
    return el.getBoundingClientRect().height / parseFloat(getComputedStyle(el).lineHeight);
  })).toBeCloseTo(3, 1);
  await page.evaluate(() => window.dispatchEvent(new Event("beforeprint")));
  await expect(opening).not.toHaveClass(/initial-divided|initial-raised/);
  expect(await lineCount()).toBeCloseTo(2, 1);
});

test("single-line initials clear the following verse across the alphabet and fallback layout", async ({ page }) => {
  const words = ["All", "Blessed", "Come", "Deliver", "Every", "For", "Glory", "Hear", "I will", "Jesus", "King", "Lord", "Make", "Now", "O Lord", "Praise", "Quicken", "Remember", "Save", "The", "Unto", "Vouchsafe", "With", "Xavier", "Ye", "Zion"];
  const fixture = words.map(word => `<div class="psalm"><div class="psalm-verses"><p class="verse">${word} hear our prayer.</p><p class="verse numbered"><span class="verse-num">2</span><span class="verse-body">And let our cry come unto thee.</span></p></div></div>`).join("");
  await page.route("**/vespers/2026-06-18", async route => {
    const response = await route.fetch();
    await route.fulfill({ response, body: (await response.text()).replace(
      /(<main\b[^>]*>)[\s\S]*?(<\/main>)/,
      `$1<div class="elements">${fixture}</div>$2`,
    ) });
  });
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, "/vespers/2026-06-18", "dark");
  for (const size of ["normal", "large"]) {
    await page.evaluate(value => document.documentElement.dataset.textSize = value, size);
    for (const fallback of [false, true]) {
      const override = fallback ? await fallbackInitials(page) : null;
      await page.evaluate(() => window.dispatchEvent(new Event("beforeprint")));
      await expect.poll(() => page.locator(".verse:first-child").evaluateAll(els => els.map(el => el.className))).toEqual(
        Array(words.length).fill(expect.stringMatching(/^(?!.*\binitial-raised\b).*\binitial-elevated\b/)),
      );
      const lines = await openingsTextLines(page, ".verse:first-child");
      expect(lines).toHaveLength(words.length);
      for (const count of lines) expect(count).toBeCloseTo(1, 1);
      await expectInitialInkClear(page, page.locator(".elements"), ".verse:first-child", `${size}/${fallback} alphabet clears numbered verses`);
      if (override) await override.evaluate(el => el.remove());
    }
  }
});

test("prose openings across the alphabet preserve words and clear their initials", async ({ page }) => {
  const words = ["All", "Blessed", "Come", "Deliver", "Every", "For", "Glory", "Hear", "I will", "Jesus", "King", "Lord", "Make", "Now", "O Lord", "Praise", "Quicken", "Remember", "Save", "The", "Unto", "Vouchsafe", "With", "Xavier", "Ye", "Zion"];
  const sentence = " hear our prayer, and let our cry come unto thee. Be merciful unto us, O Lord, and guide our steps in the way of peace.";
  const fixture = words.map(word => `<div class="chapter"><div class="liturgical-block"><p class="plain-line">${word}${sentence}</p></div></div>`).join("");
  await page.route("**/vespers/2026-06-18", async route => {
    const response = await route.fetch();
    await route.fulfill({ response, body: (await response.text()).replace(
      /(<main\b[^>]*>)[\s\S]*?(<\/main>)/,
      `$1<div class="elements">${fixture}</div>$2`,
    ) });
  });
  // Light only: themes share one geometry, held page-wide by "themes never change layout".
  const theme = "light";
  await openDatedPage(page, "/vespers/2026-06-18", theme);
  for (const width of [320, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    const openings = page.locator(".chapter .plain-line");
    // The letters' optical profiles are pinned by the alphabet snapshot; here
    // every opening keeps its words, its small caps and clear ink.
    for (let i = 0; i < words.length; i++) {
      const opening = openings.nth(i);
      await expect(opening).not.toHaveClass(/initial-raised/);
      expect(await opening.textContent()).toBe(words[i] + sentence);
      expect(await opening.locator(".initial-word").evaluate(el => getComputedStyle(el).fontVariantCaps)).toBe("all-small-caps");
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    await expectInitialInkClear(page, page.locator(".elements"), ".chapter .plain-line:first-child", `${theme}/${width} alphabet ink clearance`);
  }
});

test("Prime hymn initial clears its second metrical line on narrow pages", async ({ page }) => {
  const primeHours = [
    { hour: "prime", date: "2026-03-15", label: "Sunday" },
    { hour: "prime", date: "2026-06-18", label: "feria" },
    // "Proclaims the morn's approaching ray:" turns over on a phone.
    { hour: "lauds", date: "2026-10-13", label: "Lauds" },
  ];

  for (const { hour, date, label: hymn } of primeHours) {
    // Light only: themes share one geometry, held page-wide by "themes never change layout".
    const theme = "light";
    for (const width of [320, 390]) {
      await page.setViewportSize({ width, height: 844 });
      await openDatedPage(page, `/${hour}/${date}`, theme);

      const geometry = await page.evaluate(() => {
        const [opening, secondLine] = document.querySelectorAll(
          ".hymn-stanza-opening .hymn-line",
        );
        const firstGlyph = (line) => {
          const node = [...line.childNodes].find(
            (n) => n.nodeType === Node.TEXT_NODE && n.textContent.trim(),
          );
          const range = document.createRange();
          range.setStart(node, 0);
          range.setEnd(node, 1);
          const { left, right, top, bottom } = range.getBoundingClientRect();
          return { left, right, top, bottom };
        };
        return {
          cap: firstGlyph(opening),
          secondLine: firstGlyph(secondLine),
          openingLeft: opening.getBoundingClientRect().left,
          rem: parseFloat(getComputedStyle(document.documentElement).fontSize),
        };
      });
      // Its rows after the first (rowStartsIn leaves out the line's first letter).
      const secondRows = await page.locator(".hymn-stanza-opening .hymn-line").nth(1).evaluate(rowStartsIn);

      const label = `${hymn}/${theme}/${width}px`;
      // Its own turnover steps in from where the line starts, beside the
      // initial or at the stanza edge; it never falls back under the letter.
      const rows = [geometry.secondLine.left, ...secondRows.slice(1)];
      for (const left of rows.slice(1)) {
        expect(left - rows[0], `${label} second line's turnover steps in`).toBeCloseTo(0.9 * geometry.rem, 0);
      }
      const yOverlap =
        geometry.secondLine.top < geometry.cap.bottom - 0.5 &&
        geometry.secondLine.bottom > geometry.cap.top + 0.5;
      if (yOverlap) {
        await expectInitialInkClear(page, page.locator(".hymn-stanza-opening").first(), ".hymn-line:first-child", `${label} second-line ink clears cap`);
      } else {
        // First metrical line wrapped through both drop-cap rows; line 2
        // returns to the stanza edge, not the ordinary hang inset.
        expect(geometry.secondLine.left, `${label} second line at stanza edge`).toBeCloseTo(
          geometry.openingLeft,
          0,
        );
      }
    }
  }
});

test("posture cues sit in the psalm verses as red rubric runs", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/lauds/2026-04-23");
  const psalm = page.locator(".psalm").nth(1);
  const verses = psalm.locator(".psalm-verses .verse");
  // Parish booklet, Psalm 92 at Lauds: Sit. after the first mediant, Stand.
  // after the last, Bow. at Glory be, Stand upright. at As it was.
  await expect(verses.first().locator(".mediant + .posture")).toHaveText("Sit.");
  await expect(verses.last().locator(".mediant + .posture")).toHaveText("Stand.");
  await expect(psalm.locator(".gloria-patri .source-line > .posture")).toHaveText(["Bow.", "Stand upright."]);
  const style = await psalm.locator(".posture").first().evaluate((cue) => {
    const probe = document.createElement("span");
    probe.className = "rubric";
    cue.parentElement.appendChild(probe);
    const rubric = getComputedStyle(probe).color;
    probe.remove();
    const verse = cue.closest(".verse");
    return {
      color: getComputedStyle(cue).color,
      rubric,
      smaller: parseFloat(getComputedStyle(cue).fontSize) < parseFloat(getComputedStyle(verse).fontSize),
    };
  });
  expect(style.color).toBe(style.rubric);
  expect(style.smaller).toBe(true);
});

test("hymn-embedded kneeling rubric is an instruction, not a Latin title", async ({ page }) => {
  const instruction = "The first stanza of the following hymn is said kneeling.";
  for (const theme of ["light", "dark"]) {
    for (const width of [390, 1280]) {
      await page.setViewportSize({ width, height: 900 });
      await openDatedPage(page, "/vespers/2026-09-08", theme);

      const hymn = page.locator(".hymn");
      await expect(hymn.locator(".hymn-title")).toHaveText("Ave, maris stella");
      const rubric = hymn.locator(".hymn-verses .hymn-rubric");
      await expect(rubric).toHaveText(instruction);
      // The hymn's score, shown with Chant on, says it before the stanza too.
      await expect(hymn.locator(".chant-hymn .hymn-rubric")).toHaveText(instruction);
      await expect(rubric).not.toContainText("/:");
      await expect(hymn.locator(".hymn-latin")).toHaveCount(0);
      await expect(hymn.locator(".hymn-stanza-opening .hymn-line").first()).toContainText(
        "Star of ocean fairest",
      );

      const geometry = await page.evaluate(() => {
        const rubric = document.querySelector(".hymn .hymn-verses .hymn-rubric");
        const title = document.querySelector(".hymn .hymn-title");
        const verses = document.querySelector(".hymn .hymn-verses");
        const opening = document.querySelector(".hymn-stanza-opening .hymn-line");
        const rs = getComputedStyle(rubric);
        const ts = getComputedStyle(title);
        const rubricBox = rubric.getBoundingClientRect();
        const versesBox = verses.getBoundingClientRect();
        return {
          color: rs.color,
          titleColor: ts.color,
          fontStyle: rs.fontStyle,
          textAlign: rs.textAlign,
          overflow: rubricBox.left < versesBox.left - 1 || rubricBox.right > versesBox.right + 1,
          pageOverflow: document.documentElement.scrollWidth > window.innerWidth + 1,
          capFloat: getComputedStyle(opening, "::first-letter").float,
        };
      });
      const label = `${theme}/${width}px`;
      expect(geometry.fontStyle, `${label} rubric face`).toBe("normal");
      expect(geometry.textAlign, `${label} rubric alignment`).toBe("center");
      expect(geometry.color, `${label} rubric is not the muted incipit colour`).not.toBe(
        geometry.titleColor,
      );
      expect(geometry.overflow, `${label} rubric stays in the verse column`).toBe(false);
      expect(geometry.pageOverflow, `${label} horizontal overflow`).toBe(false);
      expect(geometry.capFloat, `${label} opening drop cap`).toBe("left");
    }
  }
});

test("Marian antiphon initial clears its second chant line", async ({ page }) => {
  // Salve Regina (Ordinary Time) is the long English form. The opening pair
  // shares one block so a two-line drop cap can float beside both source
  // lines; hanging indent on that block used to clip the gilt M and pull
  // the second line under it.
  for (const width of [320, 390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, "/vespers/2026-07-31", "light");

    const geometry = await page.evaluate(() => {
      const opening = document.querySelector(".marian-antiphon .chant-line-opening");
      const later = document.querySelector(
        ".marian-antiphon .liturgical-block > .chant-line:not(.chant-line-opening)",
      );
      // First source line text is before the <br>; the second is its own
      // block after it, so its turnover hangs under it.
      const firstText = [...opening.childNodes].find(
        (n) => n.nodeType === Node.TEXT_NODE && n.textContent.trim(),
      );
      const secondText = [...(opening.querySelector(":scope > br + .chant-line-second")?.childNodes ?? [])].find(
        (n) => n.nodeType === Node.TEXT_NODE && n.textContent.trim(),
      ) ?? null;
      const glyph = (node, start = 0, end = 1) => {
        if (!node) return null;
        const range = document.createRange();
        range.setStart(node, start);
        range.setEnd(node, Math.min(end, node.textContent.length));
        const { left, right, top, bottom } = range.getBoundingClientRect();
        return { left, right, top, bottom };
      };
      return {
        cap: glyph(firstText, 0, 1),
        following: glyph(opening.querySelector(".initial-word").firstChild),
        second: glyph(secondText, 0, 1),
        secondSnippet: secondText?.textContent?.slice(0, 24) ?? "",
        openingLeft: opening.getBoundingClientRect().left,
        // The two line boxes a two-line initial stands in. A range's box for
        // the cap is its font's em box, which WebKit's float fallback hangs
        // below the second line although the letter stands on that baseline.
        // Only the fallback needs the bound; initial-letter's box is the cap's own.
        capRowsBottom: CSS.supports("initial-letter", "2")
          ? NaN
          : opening.getBoundingClientRect().top + 2 * parseFloat(getComputedStyle(opening).lineHeight),
        openingPadding: parseFloat(getComputedStyle(opening).paddingLeft),
        openingIndent: parseFloat(getComputedStyle(opening).textIndent),
        laterPadding: later ? parseFloat(getComputedStyle(later).paddingLeft) : 0,
        laterIndent: later ? parseFloat(getComputedStyle(later).textIndent) : 0,
        label: document.querySelector(".marian-antiphon .item-label")?.textContent ?? "",
        float: getComputedStyle(opening, "::first-letter").float,
        rem: parseFloat(getComputedStyle(document.documentElement).fontSize),
      };
    });
    // The second source line's turnover hangs under it, as a later line's
    // does, rather than returning to the margin under the initial.
    const secondRows = await page.locator(".marian-antiphon .chant-line-second").evaluate(rowStartsIn);
    for (const left of secondRows.slice(1)) {
      expect(left - geometry.second.left, `${width}px second source line's turnover steps in`).toBeCloseTo(1.2 * geometry.rem, 0);
    }

    const label = `${width}px`;
    expect(geometry.label, `${label} seasonal Marian`).toMatch(/Salve Regina/i);
    expect(geometry.float, `${label} opening drop cap float`).toBe("left");
    expect(geometry.openingPadding, `${label} opening padding`).toBe(0);
    expect(geometry.openingIndent, `${label} opening indent`).toBe(0);
    // Later discrete chant lines keep the hanging indent for wraps.
    expect(geometry.laterPadding, `${label} later continuation inset`).toBeGreaterThan(0);
    expect(geometry.laterIndent, `${label} later first-line offset`).toBeLessThan(0);
    expect(geometry.cap, `${label} drop cap glyph`).not.toBeNull();
    expect(geometry.following, `${label} rest of first word`).not.toBeNull();
    expect(geometry.second, `${label} second source line`).not.toBeNull();
    expect(geometry.secondSnippet, `${label} second source text`).toMatch(/Mary our comfort/i);
    expect(
      geometry.following.left,
      `${label} first-line rest clears the drop cap`,
    ).toBeGreaterThanOrEqual(geometry.cap.right - 0.5);
    // On a wide measure the second source line sits beside the cap. On a
    // phone the first source line may wrap through both drop-cap line boxes,
    // so the second source line starts below at the opening block's left edge.
    const capBottom = Number.isFinite(geometry.capRowsBottom)
      ? Math.min(geometry.cap.bottom, geometry.capRowsBottom)
      : geometry.cap.bottom;
    const yOverlap =
      geometry.second.top < capBottom - 0.5 &&
      geometry.second.bottom > geometry.cap.top + 0.5;
    if (yOverlap) {
      expect(
        geometry.second.left,
        `${label} second-line glyph clears the drop cap`,
      ).toBeGreaterThanOrEqual(geometry.cap.right - 0.5);
    } else {
      expect(geometry.second.left, `${label} second line at opening edge`).toBeCloseTo(
        geometry.openingLeft,
        0,
      );
    }
    // A clipped gilt initial paints a short box; a full two-line M is taller
    // than one body line.
    expect(
      geometry.cap.bottom - geometry.cap.top,
      `${label} drop cap not clipped mid-glyph`,
    ).toBeGreaterThan((geometry.second.bottom - geometry.second.top) * 1.5);
  }
});

test("Litany speaker marks share one spoken-text edge across All lines", async ({ page }) => {
  // "All:" is wider than ℣./℟. It must hang into the margin rather than
  // widen every sigil in its liturgical-block — otherwise the Kyrie triad
  // sits further in than the preceding O Christ exchange (and the ℟. of
  // the corporate Lord's Prayer below).
  for (const width of [320, 390, 920]) {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, "/prime/2026-03-15", "light");

    const geometry = await page.evaluate(() => {
      const section = [...document.querySelectorAll("h2")].find((h) =>
        h.textContent.includes("Litany"),
      );
      const rows = [];
      let allSigilLeft = null;
      for (let el = section.nextElementSibling; el && el.tagName !== "H2"; el = el.nextElementSibling) {
        for (const line of el.querySelectorAll?.(".versicle-line, .response-line, .all-line") ?? []) {
          const sigil = line.querySelector(".sigil");
          const text = line.querySelector(".sigil-text");
          rows.push({
            sigil: sigil?.textContent ?? "",
            textLeft: text.getBoundingClientRect().left,
            text: (text.textContent || "").slice(0, 36),
          });
          if (sigil?.classList.contains("sigil-all")) {
            allSigilLeft = sigil.getBoundingClientRect().left;
          }
        }
      }
      return {
        rows,
        allSigilLeft,
        overflow: document.documentElement.scrollWidth > window.innerWidth + 1,
      };
    });

    expect(geometry.rows.length, `${width}px litany dialogue lines`).toBeGreaterThanOrEqual(5);
    expect(
      geometry.rows.some((row) => row.sigil === "All:"),
      `${width}px includes All speaker mark`,
    ).toBe(true);
    expect(geometry.overflow, `${width}px horizontal overflow`).toBe(false);
    expect(geometry.allSigilLeft, `${width}px All: stays on-screen`).toBeGreaterThanOrEqual(0);

    const edge = geometry.rows[0].textLeft;
    for (const row of geometry.rows) {
      expect(row.textLeft, `${width}px ${row.sigil} ${row.text}`).toBeCloseTo(edge, 0);
    }
  }
});

test("print keeps the designed 11pt prayer size at a desktop viewport", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.emulateMedia({ media: "print" });
  await openDatedPage(page, `/lauds/${testDate}`);

  const printStyles = await page.evaluate(() => {
    const body = getComputedStyle(document.body);
    const elements = getComputedStyle(document.querySelector(".elements"));
    return {
      bodyFont: parseFloat(body.fontSize),
      prayerFont: parseFloat(elements.fontSize),
      prayerMaxWidth: elements.maxWidth,
      headerDisplay: getComputedStyle(document.querySelector("header")).display,
      sessionSummaryDisplay: getComputedStyle(
        document.querySelector(".session-prayers > summary"),
      ).display,
    };
  });

  // CSS px are 96/in; 11pt therefore computes to 14.666…px.
  expect(printStyles.bodyFont).toBeCloseTo(44 / 3, 1);
  expect(printStyles.prayerFont).toBeCloseTo(printStyles.bodyFont, 1);
  // Paper keeps a book measure (30em ≈ the screen's ~70 characters), not
  // the full sheet width, which ran to 120-odd characters a line.
  expect(parseFloat(printStyles.prayerMaxWidth)).toBeCloseTo(30 * printStyles.bodyFont, 0);
  expect(printStyles.headerDisplay).toBe("none");
  expect(printStyles.sessionSummaryDisplay).toBe("none");
  await expect(page.locator(".session-prayers .liturgical-block").first()).toBeVisible();
});

test("the prayers before and after the Office keep the reader's choice for their hour", async ({ page }) => {
  const before = (hour, date) => page.goto(`/${hour}/${date}`).then(() => page.locator('details.session-prayers[data-prayers$="/Prayers Before the Office"]'));
  const after = () => page.locator('details.session-prayers[data-prayers$="/Prayers After the Office"]');

  // Sext's open closed, Lauds's open, until the reader chooses.
  await expect(await before("sext", testDate)).not.toHaveAttribute("open");
  await page.locator("details.session-prayers > summary", { hasText: "Prayers Before the Office" }).click();
  await expect(await before("sext", "2026-03-22")).toHaveAttribute("open");
  // Another hour keeps its own default.
  await expect(await before("terce", "2026-03-22")).not.toHaveAttribute("open");

  // Lauds's prayers after the Office closed, then left out in the Triduum,
  // keep the choice for when they return.
  await before("lauds", testDate);
  await page.locator("details.session-prayers > summary", { hasText: "Prayers After the Office" }).click();
  await before("lauds", "2026-04-10");
  await expect(after()).toHaveCount(0);
  await expect(await before("lauds", "2026-04-20")).toHaveAttribute("open");
  await expect(after()).not.toHaveAttribute("open");
});

// Where the browser cannot ease a height to auto (WebKit), app.js turns the
// disclosures' fold in script. A disclosure folding shut is still [open]:
// pressed again it opens, and that is the choice kept; the ordo's "Show
// office details" is not undone by a day that was folding; the hour's date
// in the header turns the picker on the fold too. Reduced motion cuts.
test("disclosures folded in script turn back when pressed mid-fold", async ({ page, browserName }) => {
  test.skip(browserName !== "webkit", "Chromium eases the fold in CSS (interpolate-size); the script's fold is for the other engines");
  await page.emulateMedia({ reducedMotion: "no-preference" });
  const ready = () => page.waitForFunction(() => document.documentElement.classList.contains("motion-ready"));

  await openDatedPage(page, `/lauds/${testDate}`);
  await ready();
  await page.evaluate(() => localStorage.removeItem("office-prayers-open"));
  const prayers = page.locator('details.session-prayers[data-prayers$="/Prayers Before the Office"]');
  await expect(prayers).toHaveAttribute("open");
  await prayers.locator("summary").evaluate((summary) => {
    summary.click();
    summary.click();
  });
  await expect(prayers).not.toHaveClass(/is-folding/);
  await expect(prayers).toHaveAttribute("open");
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem("office-prayers-open")))).toEqual({ "lauds/Prayers Before the Office": true });

  await openDatedPage(page, "/calendar/2026/03");
  await ready();
  const day = page.locator(".day-disclosures details.day-office-details").first();
  await day.locator("summary").click();
  await expect(day).toHaveAttribute("open");
  await page.evaluate(() => {
    document.querySelector(".day-disclosures details.day-office-details > summary").click();
    document.querySelector(".calendar-expand").click();
  });
  await expect(page.locator(".calendar-expand")).toHaveAttribute("aria-pressed", "true");
  await page.waitForTimeout(500);
  await expect(day).toHaveAttribute("open");
  await expect(page.locator(".day-disclosures details:not([open])")).toHaveCount(0);

  await openDatedPage(page, `/sext/${testDate}`);
  await ready();
  const turning = () => page.evaluate(() => [...document.querySelectorAll("#hour-date-nav > :not(summary)")].some((part) => part.getAnimations().length > 0));
  await page.locator("button.hour-meta-date").click();
  expect(await turning()).toBe(true);
  await expect(page.locator("#hour-date-nav")).toHaveAttribute("open");
  await expect(page.locator("button.hour-meta-date")).toHaveAttribute("aria-expanded", "true");
  await page.locator("button.hour-meta-date").click();
  await expect(page.locator("#hour-date-nav")).not.toHaveAttribute("open");
  await expect(page.locator("button.hour-meta-date")).toHaveAttribute("aria-expanded", "false");

  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.locator("button.hour-meta-date").click();
  expect(await turning()).toBe(false);
  await expect(page.locator("#hour-date-nav")).toHaveAttribute("open");
});

// "Change date" turns the invitation and the hours into the calendar in
// their place, under the band. The head gives up only the difference, or,
// where it has not the height, keeps its own while the page rises with the
// card's foot; either way the calendar ends wholly in view and the card's
// foot rests on its gutter. Folding brings the hours back as they were.
test("home's Change date turns the hours into the calendar in their place", async ({ page }) => {
  for (const [width, height] of [[412, 915], [375, 667], [320, 568]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, `/?date=${testDate}`);
    const summary = page.locator(".home-hero details.home-date-nav > summary");
    const hours = page.locator(".home-hours-face");
    const sizes = () => page.evaluate(() => {
      const hero = document.querySelector(".home-hero");
      const chooser = hero.querySelector(".home-chooser").getBoundingClientRect();
      return {
        head: hero.querySelector(".home-summary").getBoundingClientRect().height,
        foot: hero.getBoundingClientRect().bottom,
        top: chooser.top,
        bottom: chooser.bottom,
        scroll: window.scrollY,
        screen: window.innerHeight,
        band: hero.querySelector(".home-prayer-card h2").getBoundingClientRect().bottom,
      };
    });
    // The head as the page settles it: app.js's load-time fitting (the arch
    // in pixels, the versicle) done, and the same over two looks.
    await page.waitForFunction(() => document.documentElement.classList.contains("motion-ready"));
    let shut = await sizes();
    await expect.poll(async () => {
      const again = await sizes();
      const same = Math.abs(again.head - shut.head) < 0.5;
      shut = again;
      return same;
    }, { message: `${width}x${height}: the head at rest` }).toBe(true);

    await summary.click();
    const picker = page.getByRole("group", { name: "Choose a date" });
    await expect(picker).toBeVisible();
    await expect(hours).toBeHidden();
    await expect(hours).toHaveAttribute("inert", "");
    await expect(page.locator(".home-stage")).not.toHaveClass(/is-turning/);
    await expect.poll(async () => {
      const now = await sizes();
      return now.foot <= now.screen - 15 && now.foot > now.screen - 33;
    }, { message: `${width}x${height}: the card's foot at rest on its gutter` }).toBe(true);
    const open = await sizes();
    expect(Math.abs(open.top - open.band), `${width}x${height}: the calendar under the band`).toBeLessThan(24);
    expect(open.top, `${width}x${height}: the calendar's head in view`).toBeGreaterThanOrEqual(0);
    if (open.scroll > 0) {
      expect(Math.abs(open.head - shut.head), `${width}x${height}: scrolled, the arch keeps its height`).toBeLessThan(1);
    } else {
      expect(Math.abs(open.foot - shut.foot), `${width}x${height}: unscrolled, the card keeps its length`).toBeLessThan(1);
    }

    await summary.click();
    await expect(hours).toBeVisible();
    await expect(picker).toBeHidden();
    await expect(page.locator(".home-hero .home-summary")).not.toHaveAttribute("style", /min-height/);
    await expect(page.locator(".home-stage")).not.toHaveClass(/is-turning/);
    const folded = await sizes();
    expect(Math.abs(folded.head - shut.head), `${width}x${height}: the head as it was`).toBeLessThan(1);
  }
});

test("dated hour navigation keeps the selected liturgical day", async ({ page }) => {
  await openDatedPage(page, `/lauds/${testDate}`);

  await page.getByRole("button", { name: "Sunday, March 15, 2026, change date" }).click();
  await page.getByRole("link", { name: "Previous day" }).click();

  await expect(page).toHaveURL(/\/lauds\/2026-03-14$/);
  await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
});

test("the date picker is set in the room, not the platform", async ({ page }) => {
  // Today is Wednesday the 18th; the page is Sunday the 15th.
  await page.clock.setFixedTime(new Date("2026-03-18T10:00:00-04:00"));
  await openDatedPage(page, `/lauds/${testDate}`);
  // The header's date is the way in: set as its text, with no "Change date"
  // label, but a button to assistive tech that says what it does.
  await expect(page.getByText("Change date", { exact: true })).toBeHidden();
  const date = page.getByRole("button", { name: "Sunday, March 15, 2026, change date" });
  await expect(date).toHaveClass(/hour-meta-date/);
  await expect(date).toHaveAttribute("aria-expanded", "false");
  await date.click();
  await expect(date).toHaveAttribute("aria-expanded", "true");
  const picker = page.getByRole("group", { name: "Choose a date" });
  await expect(picker).toBeVisible();
  // The native field stays in the page for no-script visitors only.
  await expect(page.locator(".date-jump")).toBeHidden();

  const grid = picker.getByRole("grid", { name: "March 2026" });
  const chosen = grid.getByRole("link", { name: "Sunday, March 15, 2026" });
  await expect(chosen.locator("xpath=..")).toHaveAttribute("aria-selected", "true");
  await expect(grid.getByRole("link", { name: "Wednesday, March 18, 2026" })).toHaveAttribute("aria-current", "date");
  await expect(grid.locator("[aria-selected=true]")).toHaveCount(1);
  await expect(grid.locator("[aria-current=date]")).toHaveCount(1);
  // Six weeks always, so paging months never moves what lies below.
  await expect(grid.locator("tbody tr")).toHaveCount(6);
  const height = await picker.evaluate((node) => node.getBoundingClientRect().height);
  await picker.getByRole("button", { name: "Next month" }).click();
  await expect(picker.getByRole("grid", { name: "April 2026" })).toBeVisible();
  expect(await picker.evaluate((node) => node.getBoundingClientRect().height)).toBe(height);
  await picker.getByRole("button", { name: "Previous month" }).click();

  // One day in the tab order; arrows, Page Down and Home move it.
  await expect(grid.locator("a[tabindex='0']")).toHaveCount(1);
  await chosen.focus();
  await page.keyboard.press("ArrowRight");
  await expect(grid.getByRole("link", { name: "Monday, March 16, 2026" })).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await expect(grid.getByRole("link", { name: "Monday, March 23, 2026" })).toBeFocused();
  await page.keyboard.press("PageDown");
  const april = picker.getByRole("grid", { name: "April 2026" });
  await expect(april.getByRole("link", { name: "Thursday, April 23, 2026" })).toBeFocused();
  await page.keyboard.press("Home");
  await expect(april.getByRole("link", { name: "Sunday, April 19, 2026" })).toBeFocused();

  // The title turns to the year's months; a month returns to its days.
  await picker.getByRole("button", { name: /^April 2026/ }).click();
  const months = picker.getByRole("grid", { name: "Months" });
  await expect(months).toBeVisible();
  await expect(april).toBeHidden();
  await months.getByRole("button", { name: "December 2026" }).click();
  const december = picker.getByRole("grid", { name: "December 2026" });
  await expect(december.getByRole("link", { name: "Saturday, December 19, 2026" })).toBeFocused();

  // Escape closes the disclosure back to the date that opened it.
  await page.keyboard.press("Escape");
  await expect(picker).toBeHidden();
  await expect(date).toBeFocused();
  await expect(date).toHaveAttribute("aria-expanded", "false");

  // Reopening starts from the page's own day, and a day is a link to it.
  await date.click();
  await expect(picker.getByRole("grid", { name: "March 2026" })).toBeVisible();
  await picker.getByRole("link", { name: "Tuesday, March 17, 2026" }).click();
  await expect(page).toHaveURL(/\/lauds\/2026-03-17$/);
});

test("the date picker keeps to 1950-2150", async ({ page }) => {
  const picker = page.getByRole("group", { name: "Choose a date" });
  for (const [path, month, closed, open] of [
    ["/lauds/2150-12-15", "December 2150", "Next month", "Previous month"],
    ["/lauds/1950-01-10", "January 1950", "Previous month", "Next month"],
    // Outside the span, the picker opens at the nearer end.
    ["/lauds/2200-06-01", "December 2150", "Next month", "Previous month"],
  ]) {
    await openDatedPage(page, path);
    await page.locator("button.hour-meta-date").click();
    await expect(picker.getByRole("grid", { name: month })).toBeVisible();
    await expect(picker.getByRole("button", { name: closed })).toBeDisabled();
    await expect(picker.getByRole("button", { name: open })).toBeEnabled();
  }
  // Keys stop at the end rather than leaving the span.
  await page.goto("/lauds/2150-12-15");
  await page.locator("button.hour-meta-date").click();
  // Let the opening's toggle run first: it resets the roving focus to the
  // page's day, as a reader's own keypress always follows it.
  await expect(picker.getByRole("grid", { name: "December 2150" })).toBeVisible();
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => setTimeout(resolve))));
  await picker.getByRole("link", { name: "Thursday, December 31, 2150" }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(picker.getByRole("link", { name: "Thursday, December 31, 2150" })).toBeFocused();
  await expect(page.locator(".date-jump")).toHaveAttribute("max", "2150-12-31");
});

test("home's date picker opens the chosen day's home", async ({ page }) => {
  await openDatedPage(page, `/?date=${testDate}`);
  await page.getByText("Change date", { exact: true }).click();
  const picker = page.getByRole("group", { name: "Choose a date" });
  await picker.getByRole("button", { name: "Previous month" }).click();
  await picker.getByRole("link", { name: "Saturday, February 14, 2026" }).click();
  await expect(page).toHaveURL(/\/\?date=2026-02-14$/);
});

test("without JavaScript the native date field still jumps", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  await page.goto(`${baseURL}/lauds/${testDate}`);
  await page.getByText("Change date", { exact: true }).click();
  await expect(page.locator(".date-picker")).toHaveCount(0);
  await page.getByLabel("Jump to date").fill("2026-03-20");
  await page.getByRole("button", { name: "Go" }).click();
  await expect(page).toHaveURL(/\/lauds.*2026-03-20/);
  await context.close();
});

test("a day's commemorations open its office details, each named once", async ({ page }) => {
  // Holy Saturday keeps St Leo at no hour, so the details name him on a row of
  // the day's own; St Paul, kept at Lauds and Vespers on 18 January, is named
  // only there, with his antiphon.
  await openDatedPage(page, "/calendar/2026/04");
  const leo = page.locator("#d-2026-04-11 .day-office-digest .day-office-hour").first();
  await expect(leo.locator("dt")).toHaveText("Day");
  await expect(leo.locator(".day-office-comm")).toHaveText(/^Com\. St Leo the Great/);
  await openDatedPage(page, "/calendar/2026/01");
  const digest = page.locator("#d-2026-01-18 .day-office-digest");
  await expect(digest.locator("dt", { hasText: /^Day$/ })).toHaveCount(0);
  await expect(digest.locator(".day-office-comm", { hasText: "St Paul" })).toHaveCount(2);
  await expect(digest).not.toContainText("Commemoration of");
});

test("ordo disclosures are deliberate and survive a change in screen width", async ({ page }) => {
  await openDatedPage(page, "/calendar/2026/03");
  const day = page.locator("#d-2026-03-01");
  const details = day.locator(".day-office-details");
  await expect(details).not.toHaveAttribute("open", "");
  await details.getByText("Office details", { exact: true }).click();
  await expect(day.locator(".day-office-digest")).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 900 });
  await expect(details).toHaveAttribute("open", "");
  await expect(page.locator("#d-2026-03-02 .day-office-details")).not.toHaveAttribute("open", "");
  await page.getByRole("button", { name: "Show office details" }).click();
  await expect(page.locator(".day-disclosures details:not([open])")).toHaveCount(0);
  await expect(day.locator(".day-office-comm").first()).toBeVisible();
  await page.getByRole("button", { name: "Hide office details" }).click();
  await expect(page.locator(".day-disclosures details[open]")).toHaveCount(0);
});

// The fasting, abstinence and rank marks each spell themselves out, under a
// mouse and under a finger alike, without the tap opening the day.
test("ordo marks name themselves on hover and on a tap", async ({ browser, baseURL }) => {
  const tipOf = (locator) => locator.evaluate((el) => {
    const tip = getComputedStyle(el, "::after");
    if (tip.display === "none") return null;
    const box = el.getBoundingClientRect();
    return { text: tip.content.replace(/^"|"$/g, ""), left: box.right - parseFloat(tip.width), width: document.documentElement.clientWidth };
  });
  // Ash Wednesday 2026 (Julian paschalion) fasts, abstains and ranks first class.
  for (const hasTouch of [true, false]) {
    const viewport = hasTouch ? { width: 320, height: 700 } : { width: 1280, height: 900 };
    const context = await browser.newContext({ baseURL, hasTouch, isMobile: hasTouch, viewport });
    const page = await context.newPage();
    await openDatedPage(page, "/calendar/2026/02");
    const row = page.locator("#d-2026-02-25");
    const scope = hasTouch ? row.locator(".day-mobile-flags") : row.locator("td.day-penitential-flag, td.day-rank");
    const marks = scope.locator("abbr");
    await expect(marks).toHaveCount(3);
    const expected = ["Day of Fasting", "Day of Abstinence", "Double of the 1st Class"];
    for (let i = 0; i < 3; i++) {
      const mark = marks.nth(i);
      if (hasTouch) await mark.tap(); else await mark.hover();
      const tip = await tipOf(mark);
      expect(tip?.text, `${hasTouch ? "tap" : "hover"} ${expected[i]}`).toBe(expected[i]);
      expect(tip.left).toBeGreaterThanOrEqual(0);
    }
    await expect(page).toHaveURL(/\/calendar\/2026\/02$/);
    await context.close();
  }
});

// Delay the font itself: delaying app.js alone misses a late face rewrapping
// the title and every feast above a deep link. All geometric readings happen
// in-page, so Playwright's font-waiting screenshot helper cannot mask the swap.
for (const [width, size, hash, path] of [
  [320, "default", "", "/calendar/2026/09"],
  [390, "default", "", "/calendar/2026/09"],
  [430, "large", "", "/calendar/2026/09"],
  [390, "default", "#d-2026-09-12", "/calendar/2026/09"],
  [1280, "default", "#d-2026-09-12", "/calendar/2026/09"],
  [390, "default", "#d-2026-09-12", "/calendar/2026/all"],
  [390, "default", "", "/calendar/2026"],
]) {
  test(`ordo keeps its layout with late fonts at ${width}px, ${size}, ${path}${hash}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.addInitScript((textSize) => localStorage.setItem("office-text-size", textSize), size);
    let releaseFonts;
    const ready = new Promise((resolve) => { releaseFonts = resolve; });
    await page.route("**/*.woff2*", async (route) => { await ready; await route.continue(); });
    await page.exposeBinding("releaseOrdoFonts", () => releaseFonts());
    await page.addInitScript(() => {
      // The first month (or the frontispiece's Tabula) and the linked day.
      const first = () => document.querySelector(".month, .tabula");
      const day = () => document.getElementById("d-2026-09-12") || document.querySelector(".tabula-tables");
      const geometry = () => ({
        header: document.querySelector(".calendar-header").getBoundingClientRect().height,
        first: first().getBoundingClientRect().top + scrollY,
        day: day().getBoundingClientRect().top,
        height: document.documentElement.scrollHeight,
        scrollY,
        width: document.documentElement.scrollWidth,
      });
      document.addEventListener("DOMContentLoaded", () => {
        setTimeout(async () => {
          window.ordoBeforeFonts = geometry();
          await window.releaseOrdoFonts();
          await document.fonts.ready;
          setTimeout(() => { window.ordoAfterFonts = geometry(); }, 150);
        }, 350);
      }, { once: true });
    });
    // Release the fonts from inside the page so tracing cannot insert a
    // font-waiting snapshot between navigation and the release action.
    await page.goto(`${path}${hash}`);
    await page.waitForFunction(() => window.ordoAfterFonts);
    const { before, after } = await page.evaluate(() => ({ before: window.ordoBeforeFonts, after: window.ordoAfterFonts }));
    expect(before.width).toBe(width);
    expect(after).toEqual(before);
    if (hash) await expect(page.locator(hash)).toBeInViewport();
  });
}

test("ordo small labels share a readable size and today's marker adds no height", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-07-29T23:59:00-04:00") });
  await openDatedPage(page, "/calendar/2026/07#d-2026-07-29");
  const labels = await page.locator("#d-2026-07-29 .day-mobile-weekday, #d-2026-07-29 .day-mobile-flags, #d-2026-07-29 .day-disclosures summary")
    .evaluateAll((items) => items.map((item) => ({ size: parseFloat(getComputedStyle(item).fontSize), family: getComputedStyle(item).fontFamily })));
  expect(labels.length).toBeGreaterThanOrEqual(3);
  for (const label of labels) {
    expect(label.size).toBeGreaterThanOrEqual(12);
    expect(label.family).toContain("Georgia");
  }
  const marker = (date) => page.locator(`#d-${date} .day-mobile-date`).evaluate((node) => {
    const style = getComputedStyle(node, "::after");
    return { text: style.content, visibility: style.visibility };
  });
  expect(await marker("2026-07-29")).toEqual({ text: '"Today"', visibility: "visible" });
  expect((await marker("2026-07-30")).visibility).toBe("hidden");
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  await page.clock.fastForward("02:00");
  expect((await marker("2026-07-29")).visibility).toBe("hidden");
  expect((await marker("2026-07-30")).visibility).toBe("visible");
  expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(height);
});

test("ordo hover shading is reserved for a mouse", async ({ browser, baseURL }) => {
  for (const hasTouch of [true, false]) {
    const context = await browser.newContext({ baseURL, hasTouch, isMobile: hasTouch, viewport: { width: 390, height: 844 } });
    const page = await context.newPage();
    await page.goto("/calendar/2026/03#d-2026-03-01");
    await page.evaluate(() => document.fonts.ready);
    const row = page.locator("#d-2026-03-01");
    if (hasTouch) await row.locator(".day-office-details summary").tap();
    else await row.hover();
    const background = await row.locator(".day-feast").evaluate((node) => getComputedStyle(node).backgroundImage);
    if (hasTouch) expect(background).toBe("none");
    else expect(background).toContain("linear-gradient");
    await context.close();
  }
});

test("ordo first layout and day anchors stay put when the deferred app loads", async ({ page }) => {
  for (const width of [390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    let release;
    const ready = new Promise((resolve) => { release = resolve; });
    await page.route("**/static/app.js*", async (route) => { await ready; await route.continue(); });
    await page.goto("/calendar/2026/09#d-2026-09-14", { waitUntil: "commit" });
    const row = page.locator("#d-2026-09-14");
    await expect(row).toBeAttached();
    // Wait for the faces the first layout asked for. Not document.fonts.ready:
    // WebKit holds it until the document has loaded, which the held app.js forbids.
    await page.evaluate(() => {
      void document.body.offsetHeight;
      return Promise.all([...document.fonts].filter(face => face.status === "loading").map(face => face.loaded.catch(() => {})));
    });
    await expect(page.locator(".ordo-continuation")).toBeAttached();
    const geometry = () => page.evaluate(() => ({
      height: document.documentElement.scrollHeight,
      dayTop: document.getElementById("d-2026-09-14").getBoundingClientRect().top + scrollY,
      header: document.querySelector(".calendar-header").getBoundingClientRect().height,
    }));
    const before = await geometry();
    await expect(page.locator(".day-disclosures details[open]")).toHaveCount(0);
    release();
    await page.waitForLoadState("load");
    await expect(page.locator(".calendar-expand")).toBeVisible();
    expect(await geometry()).toEqual(before);
    await expect(row).toBeInViewport();
    await page.unroute("**/static/app.js*");
  }
});

test("ordo month navigation and full details work without JavaScript", async ({ browser, baseURL }) => {
  // In the pinned browser a no-script page cannot be inspected once it has
  // navigated, so, as in the other no-script tests, a click is checked by
  // the URL it reaches and each page read is opened in a tab of its own.
  const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
  const open = async (path) => {
    const page = await context.newPage();
    await page.goto(`${baseURL}${path}`);
    return page;
  };
  // The frontispiece's strip leads to the month's own page.
  let page = await open("/calendar/2026");
  await page.getByRole("navigation", { name: "Jump to month" }).getByRole("link", { name: /^March\b/ }).click();
  await expect(page).toHaveURL(/\/calendar\/2026\/03$/);
  page = await open("/calendar/2026/03");
  const day = page.locator("#d-2026-03-01");
  await expect(day).toBeInViewport();
  await day.locator(".day-office-details > summary").click();
  await expect(day.locator(".day-office-digest")).toBeVisible();
  await expect(day.locator(".day-office-comm").first()).toBeVisible();
  await page.getByRole("navigation", { name: "Other months" }).getByRole("link", { name: /April/ }).click();
  await expect(page).toHaveURL(/\/calendar\/2026\/04$/);
  // The whole year jumps within itself.
  page = await open("/calendar/2026/all");
  await page.getByRole("navigation", { name: "Jump to month" }).getByRole("link", { name: /^November\b/ }).click();
  await expect(page.locator("#d-2026-11-01")).toBeInViewport();
  page = await open("/calendar/2026/03");
  await page.locator("#d-2026-03-01 .day-feast-name").click();
  await expect(page).toHaveURL(/date=2026-03-01/);
  await context.close();
});

// One theme: overflow and touch targets are geometry, which themes share
// (held page-wide by "themes never change layout").
for (const width of [320, 390, 768, 1280]) {
  test(`ordo navigation fits ${width}px with full touch targets`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, "/calendar/2026/09#d-2026-09-14");
    await expect(page.locator('.month-jump [aria-current="page"]')).toHaveText("Sep");
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(width);
    for (const selector of [".year-nav a", ".month-jump a", ".calendar-expand", ".ordo-continuation a"]) {
      const boxes = await page.locator(selector).evaluateAll((items) => items.map((item) => ({ width: item.getBoundingClientRect().width, height: item.getBoundingClientRect().height })));
      for (const box of boxes) {
        expect(box.width).toBeGreaterThanOrEqual(44);
        expect(box.height).toBeGreaterThanOrEqual(44);
      }
    }
  });
}

test("ordo print reveals the office digest without changing screen disclosures", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, "/calendar/2026/03");
  const day = page.locator("#d-2026-03-01");
  await expect(day.locator(".day-office-digest")).toBeHidden();
  await page.emulateMedia({ media: "print" });
  // Rendered, though its <details> stays closed: Playwright's WebKit counts
  // anything in a closed <details> as hidden, so ask the engine itself.
  const rendered = locator => locator.evaluate(el => el.checkVisibility() && el.getBoundingClientRect().height > 0);
  await expect.poll(() => rendered(day.locator(".day-office-digest"))).toBe(true);
  await expect.poll(() => rendered(day.locator(".day-office-comm").first())).toBe(true);
  await expect(page.locator(".calendar-tools")).toBeHidden();
  await expect(page.locator(".ordo-continuation")).toBeHidden();
  await page.emulateMedia({ media: "screen" });
  await expect(day.locator(".day-office-digest")).toBeHidden();
});

for (const theme of ["light", "dark"]) {
  test(`ordo navigation and day rows are accessible in ${theme}`, async ({ page }) => {
    await openDatedPage(page, "/calendar/2026/01", theme);
    // A representative month covers the repeated table and disclosure markup.
    const results = await new AxeBuilder({ page })
      .include(".calendar-header").include(".month-jump").include(".calendar-tools").include("#january").include(".ordo-continuation")
      .withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze();
    expect(violationFingerprints(results)).toEqual([]);
    await openDatedPage(page, "/calendar/2026", theme);
    const frontispiece = await new AxeBuilder({ page })
      .include(".calendar-header").include(".month-jump").include(".tabula")
      .withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze();
    expect(violationFingerprints(frontispiece)).toEqual([]);
  });
}

test("ordo Today leads back to the current year from an archive", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-09-12T12:00:00-04:00") });
  for (const path of ["/calendar/2025", "/calendar/2025/09", "/calendar/2026/08", "/calendar/2025/all"]) {
    await page.goto(path);
    await expect(page.locator("#calendar-today-link"), path).toHaveAttribute("href", "/calendar/2026/09#d-2026-09-12");
  }
  // Today's own month jumps to its row.
  await page.goto("/calendar/2026/09");
  await expect(page.locator("#calendar-today-link")).toHaveAttribute("href", "#d-2026-09-12");
  // The strip marks this month in its own year alone.
  await page.goto("/calendar/2026");
  await expect(page.locator(".month-jump .is-today-month")).toHaveText("Sep");
  await expect(page.locator(".month-jump .is-today-month")).toHaveAttribute("aria-label", "September, this month");
  await page.goto("/calendar/2025");
  await expect(page.locator(".month-jump .is-today-month")).toHaveCount(0);
});

test("the strip's month marker moves with the month at midnight", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-09-30T23:59:00-04:00") });
  await openDatedPage(page, "/calendar/2026/09");
  const september = page.locator('.month-jump [data-month="2026-09"]');
  const october = page.locator('.month-jump [data-month="2026-10"]');
  await expect(september).toHaveClass(/is-today-month/);
  await page.clock.fastForward("02:00");
  await expect(september).not.toHaveClass(/is-today-month/);
  await expect(september).toHaveAttribute("aria-label", "September");
  await expect(october).toHaveClass(/is-today-month/);
  await expect(october).toHaveAttribute("aria-label", "October, this month");
  await expect(page.locator(".month-jump .is-today-month")).toHaveCount(1);
});

test("Tabula dates stay whole at the narrowest width and largest text", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 844 });
  await page.addInitScript(() => localStorage.setItem("office-text-size", "large"));
  await openDatedPage(page, "/calendar/2026");
  const dates = await page.locator(".tabula-tables dd").evaluateAll((items) => items.map((dd) => ({
    text: dd.textContent,
    lines: dd.getClientRects().length && Math.round(dd.getBoundingClientRect().height / parseFloat(getComputedStyle(dd).lineHeight)),
    right: Math.round(dd.getBoundingClientRect().right),
    edge: Math.round(dd.parentElement.getBoundingClientRect().right),
  })));
  expect(dates.length).toBe(11);
  for (const date of dates) {
    expect(date.lines, date.text).toBe(1);
    expect(date.right, date.text).toBe(date.edge);
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);
});

test("links into the year from before month pages still reach their day", async ({ page }) => {
  // The year page replaces itself while it parses, so its own load never comes; WebKit has been
  // seen to leave the goto waiting on it. The URL and viewport checks wait for the month page.
  await page.goto("/calendar/2026#d-2026-09-12", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/calendar\/2026\/09#d-2026-09-12$/);
  await expect(page.locator("#d-2026-09-12")).toBeInViewport();
  await page.goto("/calendar/2026?form=priest#march", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/calendar\/2026\/03\?form=priest$/);
  // The frontispiece itself, and its own anchors, stay put.
  await page.goto("/calendar/2026#tabula-heading");
  await expect(page).toHaveURL(/\/calendar\/2026#tabula-heading$/);
});

test("the foreground Ordo moves rather than duplicates its today marker at midnight", async ({
  page,
}) => {
  await page.clock.install({ time: new Date("2026-07-29T23:59:00-04:00") });
  await openDatedPage(page, "/calendar/2026/07");

  const oldToday = page.locator("#d-2026-07-29");
  const newToday = page.locator("#d-2026-07-30");
  await expect(oldToday).toHaveClass(/is-today/);
  await expect(oldToday).toHaveAttribute("aria-current", "date");

  await page.clock.fastForward("02:00");

  await expect(oldToday).not.toHaveClass(/is-today/);
  await expect(oldToday).not.toHaveAttribute("aria-current", "date");
  await expect(newToday).toHaveClass(/is-today/);
  await expect(newToday).toHaveAttribute("aria-current", "date");
  await expect(page.locator(".month-table tr.is-today")).toHaveCount(1);
});

test("an office left open in the foreground offers today after midnight", async ({ page }) => {
  const today = await serverTodaySlug(page);
  const midnight = await page.evaluate((slug) => {
    // This executes in Playwright's configured America/New_York timezone, so
    // local midnight and the next date stay correct in EST, EDT, and at a
    // year boundary without baking in an offset.
    const before = new Date(`${slug}T23:59:00`);
    const after = new Date(before.getTime() + 2 * 60 * 1000);
    const dateSlug = (d) =>
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    return { before: before.getTime(), tomorrow: dateSlug(after) };
  }, today);

  await page.clock.install({ time: new Date(midnight.before) });
  await openDatedPage(page, `/lauds/${today}`);

  await expect(page.locator(".not-today-notice")).toHaveCount(0);
  await page.clock.fastForward("02:00");

  const notice = page.locator(".not-today-notice");
  await expect(notice).toBeVisible();
  await expect(notice).toHaveAttribute("role", "status");
  await expect(notice.getByRole("link", { name: "Go to today" })).toHaveAttribute(
    "href",
    `/lauds/${midnight.tomorrow}`,
  );
});

test("reminder choices update the subscription URL", async ({ page }) => {
  await page.goto("/reminders");

  const copy = page.getByRole("button", { name: "Copy link" });
  const subscribe = page.locator("#reminder-webcal");
  await expect(copy).toBeVisible();
  await expect(subscribe).toHaveAttribute("href", /^webcal:/);
  await expect(page.getByLabel("Time for Lauds")).toBeEnabled();
  await expect(page.getByLabel("Time for Lauds")).toHaveAttribute("required", "");
  await expect(page.getByLabel("Time for Prime")).toBeDisabled();
  await expect(page.getByLabel("Time for Prime")).not.toHaveAttribute("required");

  for (const checkbox of await page.locator('input[name="hour"]').all()) {
    await checkbox.uncheck();
  }
  await expect(copy).toBeDisabled();
  await expect(subscribe).toHaveAttribute("aria-disabled", "true");
  await expect(subscribe).not.toHaveAttribute("href");
  await expect(subscribe).toHaveAttribute("tabindex", "-1");
  await expect(page.locator("#reminder-url")).toHaveText("No hour is chosen yet.");
  await expect(page.locator("#reminder-rule")).toHaveText("No hour is chosen yet.");

  await page.locator('input[name="hour"][value="lauds"]').check();
  await expect(copy).toBeEnabled();
  await expect(subscribe).toHaveAttribute("aria-disabled", "false");
  await expect(subscribe).toHaveAttribute("href", /^webcal:/);
  await expect(subscribe).not.toHaveAttribute("tabindex");
  await expect(page.getByLabel("Time for Lauds")).toBeEnabled();
  await page.getByLabel("Time for Lauds").fill("07:30");
  await page.getByLabel("Time for Lauds").press("Tab");

  const feedURL = page.locator("#reminder-url");
  await expect(feedURL).toContainText("lauds=07%3A30");
  await expect(feedURL).toContainText("tz=America%2FNew_York");

  await page.getByLabel("Time for Lauds").fill("");
  await page.getByLabel("Time for Lauds").press("Tab");
  await expect(copy).toBeDisabled();
  await expect(subscribe).not.toHaveAttribute("href");
  await expect(feedURL).toHaveText("Choose a time for each chosen hour.");
  await expect(page.locator("#reminder-rule")).toHaveText("Choose a time for each chosen hour.");

  await page.getByLabel("Time for Lauds").fill("07:30");
  await page.getByLabel("Time for Lauds").press("Tab");
  await expect(copy).toBeEnabled();

  for (const checkbox of await page.locator('input[name="day"]').all()) {
    await checkbox.uncheck();
  }
  await expect(copy).toBeDisabled();
  await expect(subscribe).not.toHaveAttribute("href");
  await expect(feedURL).toHaveText("No day is chosen yet.");
  await expect(page.locator("#reminder-rule")).toHaveText("No day is chosen yet.");

  await page.locator('input[name="day"][value="sun"]').check();
  await expect(copy).toBeEnabled();
  await expect(subscribe).toHaveAttribute("href", /^webcal:/);
  await expect(feedURL).toContainText("days=sun");
});

test("reminder copy failure reveals the calendar address", async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
  });
  await page.goto("/reminders");

  await page.getByRole("button", { name: "Copy link" }).click();

  await expect(page.locator("#reminder-copy-note")).toHaveText(
    "Copy unavailable. The calendar address is shown below.",
  );
  await expect(page.locator(".reminder-address")).toHaveAttribute("open", "");
  await expect(page.locator("#reminder-url")).toBeVisible();
});

// The rule, written out: the sentences of presentation::reminders' table tests, from app.js's
// copy of the grammar and the words the page is sent.
test("the reminder rule says back the choices", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/reminders");
  const rule = page.locator("#reminder-rule");
  const said = async () => (await rule.textContent()).replace(/\u00a0/g, " ");
  const day = (name) => page.locator(".reminder-day", { hasText: name }).locator("input");
  await expect.poll(said).toBe(
    "Lauds at 6:45 AM, Vespers at 6:00 PM, and Compline at 9:00 PM, every day, ten minutes before.",
  );
  await page.locator('input[name="hour"][value="compline"]').uncheck();
  await expect.poll(said).toBe("Lauds at 6:45 AM and Vespers at 6:00 PM, every day, ten minutes before.");
  await page.locator('input[name="hour"][value="vespers"]').uncheck();
  await day("Sat").uncheck();
  await day("Sun").uncheck();
  await expect.poll(said).toBe("Lauds at 6:45 AM, on weekdays, ten minutes before.");
  await day("Sat").check();
  await day("Sun").check();
  await day("Fri").uncheck();
  await page.selectOption('select[name="alarm"]', "none");
  await expect.poll(said).toBe("Lauds at 6:45 AM, except Fridays, with no alert.");
  for (const name of ["Mon", "Tue", "Thu", "Sat"]) {
    await day(name).uncheck();
  }
  await day("Fri").check();
  await page.selectOption('select[name="alarm"]', "0");
  await expect.poll(said).toBe("Lauds at 6:45 AM, on Wednesdays, Fridays, and Sundays, at the hour.");
  await day("Wed").check();
  await day("Thu").check();
  await expect.poll(said).toBe("Lauds at 6:45 AM, except Mondays, Tuesdays, and Saturdays, at the hour.");
  for (const box of await page.locator('input[name="day"]').all()) {
    await box.uncheck();
  }
  await expect.poll(said).toBe("No day is chosen yet.");
  await page.locator('input[name="hour"][value="lauds"]').uncheck();
  await expect.poll(said).toBe("No hour or day is chosen yet.");
});

test("a tap on an unchosen hour's time chooses the hour", async ({ page }) => {
  await page.goto("/reminders");
  await expect(page.getByLabel("Time for Prime")).toBeDisabled();
  await page.locator('[data-reminder-hour="prime"] .reminder-time').click();
  await expect(page.locator('input[name="hour"][value="prime"]')).toBeChecked();
  await expect(page.getByLabel("Time for Prime")).toBeEnabled();
});

test("copying the link changes its words in place for a moment", async ({ page, context, browserName }) => {
  test.skip(browserName !== "chromium", "clipboard permissions are Chromium's");
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/reminders");
  const copy = page.getByRole("button", { name: "Copy link" });
  // Where the link stands on the page, whatever the scroll.
  const place = () => copy.evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x, y: r.y + window.scrollY, width: r.width, height: r.height };
  });
  await copy.scrollIntoViewIfNeeded();
  const before = await place();
  await copy.click();
  await expect(copy).toHaveClass(/is-copied/);
  await expect(page.locator("#reminder-copied")).toHaveText("Copied.");
  expect(await place()).toEqual(before);
  await expect(copy).not.toHaveClass(/is-copied/, { timeout: 4000 });
});

test("quiet mobile controls retain full thumb targets", async ({ page }) => {
  for (const [path, selectors] of [
    [
      `/?date=${testDate}`,
      [".site-brand", ".home-date-link", ".not-today-link", ".home-date-nav > summary"],
    ],
    [
      `/lauds/${testDate}`,
      [
        ".site-brand",
        ".hour-meta-date",
        ".session-prayers > summary",
        ".report-issue a:visible",
      ],
    ],
    [
      "/calendar/2026/03",
      [".year-nav a:not([hidden])", ".month-jump a", ".day-disclosures summary", ".ordo-continuation a"],
    ],
    [
      "/reminders",
      [
        ".reminder-hour-name",
        '.reminder-hour-row input[type="time"]',
        ".reminder-day",
        ".reminder-alarm select",
        ".reminder-subscribe",
        ".reminder-copy",
        ".reminder-address > summary",
        ".reminder-help > summary",
      ],
    ],
  ]) {
    await openDatedPage(page, path);
    for (const selector of selectors) {
      const targets = page.locator(selector);
      const count = await targets.count();
      expect(count, `${path} should expose ${selector}`).toBeGreaterThan(0);
      for (let i = 0; i < Math.min(count, 12); i++) {
        // A target whose thumb box is drawn round it (an absolute ::before, as
        // the header's links and the ordo's phone rows have) is measured by it.
        const box = await targets.nth(i).evaluate((el) => {
          const r = el.getBoundingClientRect();
          if (r.width === 0 && r.height === 0) return null;
          const b = getComputedStyle(el, "::before");
          if (b.content === "none" || b.position !== "absolute") return { width: r.width, height: r.height };
          const px = (v) => (v.endsWith("px") ? parseFloat(v) : 0);
          const width = Math.max(r.width, r.width - px(b.left) - px(b.right));
          const height = Math.max(r.height, r.height - px(b.top) - px(b.bottom));
          return { width, height };
        });
        expect(box, `${path} ${selector} should be laid out`).not.toBeNull();
        expect(box.height, `${path} ${selector} target height`).toBeGreaterThanOrEqual(44);
        expect(box.width, `${path} ${selector} target width`).toBeGreaterThanOrEqual(44);
      }
    }
  }
});

for (const { name, path, theme, knownViolations } of [
  {
    name: "home in the Nave theme",
    path: `/?date=${testDate}`,
    theme: "light",
    knownViolations: [],
  },
  {
    name: "Lauds in the Apse theme",
    path: `/lauds/${testDate}`,
    theme: "dark",
    knownViolations: [],
  },
  {
    name: "Reminders in the Apse theme",
    path: "/reminders",
    theme: "dark",
    knownViolations: [],
  },
  {
    name: "About in the Nave theme",
    path: "/about",
    theme: "light",
    knownViolations: [],
  },
]) {
  test(`${name} stays within the accessibility baseline`, async ({ page }) => {
    await openDatedPage(page, path, theme);

    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
      .analyze();

    expect(violationFingerprints(results)).toEqual(knownViolations);
  });
}

// Usage metrics: exercise the normal CI behavior-test entry point.

// Reporting days are Eastern, so "today" must be computed there and not from
// the runner's clock or a literal that would rot into the archive tomorrow.
function easternDay(offsetDays = 0) {
  const now = new Date(Date.now() + offsetDays * 864e5);
  return now.toLocaleDateString("en-CA", { timeZone: "America/New_York" });
}

// A beacon body is the page scope followed by the dimensions describing how
// that page was rendered; most of these tests care only about the scope.
const scopes = (events) => events.map(body => body.split(" ")[0]);

// Any key or pointer press engages the page. A bare Shift does so without a
// click that may land on a link (the home hour directory fills the middle of
// a phone screen) and start a navigation that races the test's next goto.
const engage = (page) => page.keyboard.press("Shift");

// A person, not the default HeadlessChrome agent the server drops as a bot.
const HUMAN_UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
  "(KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36";

test("engaged visits to a current page send an event, passive and background ones do not", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });

  // Rendering alone is not use: a scraper that renders and leaves must not count.
  await page.goto(`/lauds/${easternDay()}`);
  await page.waitForTimeout(1500);
  expect(events).toEqual([]);

  // A touch is engagement.
  await page.mouse.click(200, 300);
  await expect.poll(() => events.length).toBe(1);
  expect(scopes(events)).toEqual(["lauds"]);

  // A background fetch of another hour is not a visit to it.
  await page.evaluate(async () => {
    // Drain it: an abandoned body would be cancelled by the next navigation.
    await (await fetch(`/vespers/${document.body.getAttribute("data-usage-when")}`)).text();
    document.dispatchEvent(new Event("visibilitychange"));
  });
  expect(scopes(events)).toEqual(["lauds"]);

  await page.goto(`/?date=${easternDay()}`);
  await engage(page);
  await expect.poll(() => events.length).toBe(2);
  expect(scopes(events)[1]).toBe("site");

  // The dashboard itself is never counted.
  await page.goto("/admin/usage?days=7");
  await engage(page);
  await expect(page.getByRole("heading", { name: "Daily usage", exact: true })).toBeVisible();
  expect(events.length).toBe(2);
});

test("the dated archive is freely readable but never counted", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  // Deep past, far future, and a distant ordo year: all render, none report.
  for (const path of ["/lauds/2019-03-04", "/?date=2045-06-01", "/calendar/2050", "/vespers/2031-12-25"]) {
    await page.goto(path);
    await engage(page);
    await page.waitForTimeout(300);
    await expect(page.locator("body")).toBeVisible();
  }
  expect(events).toEqual([]);

  // Yesterday and tomorrow are ordinary use, not archive.
  for (const day of [easternDay(-1), easternDay(1)]) {
    await page.goto(`/lauds/${day}`);
    await engage(page);
  }
  await expect.poll(() => events.length).toBe(2);
});

test("the current ordo page is tracked in its own column, not just the site total", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  const year = new Date().getFullYear();
  await page.goto(`/calendar/${year}/01`);
  await engage(page);
  await expect.poll(() => events.length).toBe(1);
  expect(scopes(events)).toEqual(["ordo"]);
});

test("generating a reminder feed link is tracked separately from viewing the page", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  // A stub so the copy actually "succeeds" without a real clipboard permission.
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: () => Promise.resolve() },
    });
  });
  await page.goto("/reminders");
  await engage(page);
  // Merely opening and engaging with the page reports "site", never "reminders".
  await expect.poll(() => scopes(events)).toEqual(["site"]);

  await page.getByRole("button", { name: "Copy link" }).click();
  await expect.poll(() => scopes(events)).toEqual(["site", "reminders"]);
});

// The default project emulates a phone, which is what most readers use.
test("the beacon reports the appearance the page was read in", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  const read = async (label) => {
    events.length = 0;
    await page.goto(`/vespers/${easternDay()}`);
    await engage(page);
    await expect.poll(() => events.length, { message: label }).toBe(1);
    // Leave for a page that never reports before changing the device: a
    // beacon still settling would otherwise re-report in the new appearance.
    await page.goto("/privacy");
    // Whether today's Vespers has a score, and so reports chant:, is the day's.
    return events[0].replace(/ chant:\w+/, "");
  };

  // Device appearance, no stored choice: what is on screen is what counts.
  expect(await read("light phone")).toBe("vespers appearance:nave screen:mobile prayer-form:private visit:first client:browser");
  await page.emulateMedia({ colorScheme: "dark" });
  expect(await read("dark phone")).toBe("vespers appearance:apse screen:mobile prayer-form:private visit:first client:browser");

  // An explicit choice overrides the device, so someone reading the Nave on a
  // dark-mode phone counts as Nave.
  await page.evaluate(() => localStorage.setItem("office-theme", "light"));
  expect(await read("chosen Nave on a dark phone")).toBe("vespers appearance:nave screen:mobile prayer-form:private visit:first client:browser");
  await page.evaluate(() => localStorage.removeItem("office-theme"));
});

test("the installed web app reports itself apart from a browser tab", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  // Safari's home-screen app says so on navigator; others match display-mode: standalone.
  await page.addInitScript(() => Object.defineProperty(navigator, "standalone", { configurable: true, value: true }));
  await page.goto(`/?date=${easternDay()}`);
  await engage(page);
  await expect.poll(() => events.length).toBe(1);
  expect(events[0]).toBe("site appearance:nave screen:mobile visit:first client:pwa");
});

// The device keeps only the day it first opened the Office; the beacon says first or returning.
test("a browser is new on the day it first opens the Office and returning after", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  const visit = async () => {
    events.length = 0;
    await page.goto(`/?date=${easternDay()}`);
    await engage(page);
    await expect.poll(() => events.length).toBe(1);
    return events[0].match(/visit:(\w+)/)[1];
  };
  // Any number of pages on the first day are new; an earlier first day is returning.
  expect(await visit()).toBe("first");
  expect(await visit()).toBe("first");
  await page.evaluate(() => localStorage.setItem("office-first-counted", "2026-01-01"));
  expect(await visit()).toBe("returning");
  // A reader from before this was counted, with a saved setting, is not mistaken for a new one.
  await page.evaluate(() => { localStorage.clear(); localStorage.setItem("office-theme", "dark"); });
  expect(await visit()).toBe("returning");
});

// Home offers the introduction beside Change date for a reader's first week, in the season's
// place; after that, and for a reader from before first days were kept, About is in the menu.
test("home introduces the Office to a reader in their first week", async ({ page }) => {
  // Kept off the real usage store, which the deduplication test counts from in parallel.
  await page.route("**/api/usage", route => route.fulfill({ status: 204 }));
  const introduction = page.locator(".home-introduction");
  const home = `/?date=${easternDay()}`;
  await page.goto(home);
  await expect(introduction).toBeVisible();
  await expect(introduction).toHaveAttribute("href", "/about");
  await page.evaluate((day) => localStorage.setItem("office-first-counted", day), easternDay(-6));
  await page.goto(home);
  await expect(introduction).toBeVisible();
  // Seven whole New York days on, the week is over; a first day ahead of today never counts.
  for (const offset of [-7, 1]) {
    await page.evaluate((day) => localStorage.setItem("office-first-counted", day), easternDay(offset));
    await page.goto(home);
    await expect(introduction).toBeHidden();
  }
  await page.evaluate(() => localStorage.setItem("office-first-counted", "before"));
  await page.goto(home);
  await expect(introduction).toBeHidden();
  await page.getByText("Menu", { exact: true }).click();
  await page.getByRole("link", { name: "About", exact: true }).click();
  await expect(page).toHaveURL(/\/about$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("About the Office");
  await expect(page.locator(".about-hour dt a")).toHaveText(["Lauds", "Prime", "Terce", "Sext", "None", "Vespers", "Compline"]);
});

// About's table marks the hour it is now as home's directory does, and the page ends with
// home's invitation, words and link alike; both follow the clock across an office boundary.
test("About lights the hour it is now and ends with home's invitation", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-03-15T16:59:00-04:00") });
  await openDatedPage(page, "/about");
  const current = page.locator(".about-hour.is-current");
  const door = page.locator(".about-door .pray-now");
  await expect(current).toHaveCount(1);
  await expect(current).toHaveAttribute("data-hour", "none");
  await expect(current.locator("dt a")).toHaveAttribute("aria-current", "time");
  await expect(door).toHaveText("Pray None");
  await expect(door).toHaveAttribute("href", "/none/2026-03-15");
  // The door comes after the last paragraph, and is home's ruled tablet.
  await expect(page.locator(".about > :last-child")).toHaveClass(/about-door/);
  await expect(door).toHaveClass(/is-ruled/);
  // A Prime gloss on one line: the seven rows stand alike.
  const heights = await page.locator(".about-hour").evaluateAll(rows => rows.map(r => Math.round(r.getBoundingClientRect().height)));
  expect(Math.max(...heights) - Math.min(...heights)).toBeLessThanOrEqual(1);

  await page.clock.fastForward("02:00");
  await expect(current).toHaveAttribute("data-hour", "vespers");
  await expect(door).toHaveText("Pray Vespers");
  await expect(page.locator('.about-hour dt a[aria-current="time"]')).toHaveCount(1);
});

test("About lights no hour from midnight to 2am, as home does", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-03-15T01:00:00-04:00"));
  await openDatedPage(page, "/about");
  await expect(page.locator(".about-hour.is-current")).toHaveCount(0);
  await expect(page.locator('.about-hour dt a[aria-current="time"]')).toHaveCount(0);
  const door = page.locator(".about-door .pray-now");
  await expect(door).toHaveText("Pray Compline");
  await expect(door).toHaveAttribute("href", "/compline/2026-03-14");
});

test.describe("on a screen with a mouse", () => {
  test.use({ isMobile: false, hasTouch: false, viewport: { width: 1280, height: 900 } });

  test("the beacon separates a desk from a narrowed window", async ({ page }) => {
    const events = [];
    await page.route("**/api/usage", async route => {
      events.push(route.request().postData());
      await route.fulfill({ status: 204 });
    });
    const read = async (label) => {
      events.length = 0;
      await page.goto(`/vespers/${easternDay()}`);
      await engage(page);
      await expect.poll(() => events.length, { message: label }).toBe(1);
      // Leave for a page that never reports before changing the device: a
      // beacon still settling would otherwise re-report in the new appearance.
      await page.goto("/privacy");
      return events[0].replace(/ chant:\w+/, "");
    };

    expect(await read("wide window")).toBe("vespers appearance:nave screen:desktop prayer-form:private visit:first client:browser");
    // A desktop window dragged narrow gets the phone layout, and is counted
    // as the layout it is actually being read in.
    await page.setViewportSize({ width: 390, height: 900 });
    expect(await read("narrow window")).toBe("vespers appearance:nave screen:mobile prayer-form:private visit:first client:browser");
  });
});

// The cookie round trip is the whole basis of deduplication, so exercise it
// against the real endpoint rather than a stubbed one.
test("real events deduplicate per browser and exclude crawlers", async ({ browser, baseURL }) => {
  const today = async (page) => {
    await page.goto("/admin/usage?days=7");
    const row = page.locator("tbody tr").filter({ has: page.locator(".usage-total") }).first();
    return Number(await row.locator("td.usage-total").innerText());
  };

  const readerCtx = await browser.newContext({ baseURL, userAgent: HUMAN_UA, timezoneId: "America/New_York" });
  const reader = await readerCtx.newPage();
  const before = await today(reader);

  // One browser reading several hours is one visitor, however many pages.
  for (const hour of ["lauds", "prime", "vespers"]) {
    await reader.goto(`/${hour}/${easternDay()}`);
    await engage(reader);
    await reader.waitForTimeout(300);
  }
  expect(await today(reader)).toBe(before + 1);

  // A crawler with a fresh profile per URL adds nothing at all.
  for (const path of [`/lauds/${easternDay()}`, `/?date=${easternDay()}`]) {
    const botCtx = await browser.newContext({
      baseURL, timezoneId: "America/New_York",
      userAgent: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
    });
    const bot = await botCtx.newPage();
    await bot.goto(path);
    await engage(bot);
    await bot.waitForTimeout(300);
    await botCtx.close();
  }
  expect(await today(reader)).toBe(before + 1);

  await readerCtx.close();
});

test("service worker does not cache the usage report", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ serviceWorkers: "allow", baseURL });
  const page = await context.newPage();
  await page.goto("/admin/usage?days=7");
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready;
    if (!navigator.serviceWorker.controller) {
      await new Promise(resolve => navigator.serviceWorker.addEventListener("controllerchange", resolve, { once: true }));
    }
  });
  await page.reload();
  expect(await page.evaluate(async () => {
    const names = await caches.keys();
    for (const name of names) {
      const keys = await (await caches.open(name)).keys();
      if (keys.some(key => new URL(key.url).pathname === "/admin/usage")) return true;
    }
    return false;
  })).toBe(false);
  await context.setOffline(true);
  await expect(page.goto("/admin/usage?days=7")).rejects.toThrow();
  await context.close();
});

test("the Martyrology at Prime is a Settings choice, off by default, that holds offline", async ({ browser, baseURL, browserName }) => {
  test.skip(browserName === "webkit", "Playwright's WebKit fails any navigation once context.setOffline(true) is set, even one its service worker would answer; service-worker offline emulation works only in Chromium");
  const context = await browser.newContext({ serviceWorkers: "allow", baseURL });
  try {
    const page = await context.newPage();
    // Keep the September 8 reading, but visit its next occurrence:
    // precache intentionally prunes historical pages as the real clock moves.
    const now = new Date();
    const year = now.getFullYear() + (now >= new Date(now.getFullYear(), 8, 8) ? 1 : 0);
    const prime = `/prime/${year}-09-07`;
    const reading = page.getByRole("heading", { name: "Martyrology for tomorrow, September 8", exact: true });
    const rubric = page.locator(".elements").getByText("this may laudably be done");
    await page.goto(prime);
    await page.evaluate(async () => {
      await navigator.serviceWorker.ready;
      if (!navigator.serviceWorker.controller) {
        await new Promise(resolve => navigator.serviceWorker.addEventListener("controllerchange", resolve, { once: true }));
      }
    });
    await page.reload();
    // Off by default: the rubric stands, and nothing is stored.
    await expect(rubric).toBeVisible();
    await expect(reading).toBeHidden();
    expect(await page.evaluate(() => localStorage.getItem("office-martyrology"))).toBeNull();
    for (const button of await page.locator('.martyrology-option[data-martyrology-choice="off"]').all()) {
      await expect(button).toHaveAttribute("aria-pressed", "true");
    }

    await choosePreference(page, "On", ".martyrology-option");
    await expect(reading).toBeVisible();
    await expect(rubric).toBeHidden();
    await expect(page.locator(".elements")).not.toContainText("Thomas of Villanova");
    // Remembered, and the same page serves both settings, offline too.
    await context.setOffline(true);
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-martyrology", "on");
    await expect(reading).toBeVisible();
    await choosePreference(page, "Off", ".martyrology-option");
    await expect(rubric).toBeVisible();
    await expect(reading).toBeHidden();
    expect(await page.evaluate(() => localStorage.getItem("office-martyrology"))).toBe("off");
  } finally {
    await context.close();
  }
});

test("closing antiphons share one left edge whatever their length", async ({ page }) => {
  // Vespers of 2026-09-22 closes psalms with both one-line and wrapping antiphons.
  for (const width of [390, 1280]) {
    await page.setViewportSize({ width, height: 844 });
    await openDatedPage(page, "/vespers/2026-09-22");
    const lefts = await page.locator(".elements > :is(.psalm, .canticle) + .antiphon").evaluateAll((antiphons) =>
      antiphons.map((antiphon) => Math.round(antiphon.getBoundingClientRect().left)),
    );
    expect(lefts.length, `${width}px closing antiphons`).toBeGreaterThan(3);
    expect(new Set(lefts).size, `${width}px closing antiphon edges ${lefts.join(", ")}`).toBe(1);
  }
});

test("long opening verses return to the numbered text edge below the initial", async ({ page }) => {
  for (const width of [320, 390, 540]) {
    await page.setViewportSize({ width, height: 844 });
    await openDatedPage(page, "/vespers/2026-06-18");
    for (const size of ["normal", "large"]) {
      await page.evaluate((value) => document.documentElement.setAttribute("data-text-size", value), size);
      const geometry = await page.locator(".psalm-verses").first().evaluate((psalm) => {
        const opening = psalm.querySelector(".verse");
        const leading = parseFloat(getComputedStyle(opening).lineHeight);
        const walker = document.createTreeWalker(opening, NodeFilter.SHOW_TEXT);
        const lines = new Map();
        let node;
        let first = true;
        while ((node = walker.nextNode())) {
          for (let i = 0; i < node.length; i++) {
            if (first) { first = false; continue; } // The initial has its own ink box.
            if (/\s/.test(node.textContent[i])) continue;
            const range = document.createRange();
            range.setStart(node, i);
            range.setEnd(node, i + 1);
            const rect = range.getBoundingClientRect();
            // Mediant has an optical vertical offset; it is not a new line.
            if (node.parentElement.closest(".mediant")) continue;
            // A smaller posture cue has its own glyph top on the same line.
            const y = [...lines.keys()].find((top) => Math.abs(top - rect.top) < leading / 2) ?? Math.round(rect.top);
            lines.set(y, Math.min(lines.get(y) ?? Infinity, rect.left));
          }
        }
        return {
          lines: [...lines.entries()].sort((a, b) => a[0] - b[0]).map((line) => line[1]),
          edge: psalm.querySelector(".verse-body").getBoundingClientRect().left,
          overflow: document.documentElement.scrollWidth > innerWidth,
        };
      });
      expect(geometry.overflow).toBe(false);
      if (width <= 390) expect(geometry.lines.length).toBeGreaterThan(2);
      for (const left of geometry.lines.slice(2)) {
        expect(Math.abs(left - geometry.edge), `${width}px ${size} continuation alignment`).toBeLessThan(1);
      }
    }
  }
});

// The print diurnal's rule, as the psalms keep it above: below a dropped
// initial, a prose opening turns over at the spoken-text edge its ℟ shares,
// not back at the margin under the letter. 1 Cor. 6:20 is the reported case.
// One test per width: the twenty-four layouts measured in one test outran its
// timeout on a loaded CI runner. Each page is opened once per width.
const proseOpenings = {
  "/none/2026-10-13": [
    { block: ".chapter .liturgical-block", opening: ".plain-line:first-child" },
  ],
  "/lauds/2026-10-13": [
    { block: ".chapter .liturgical-block", opening: ".plain-line:first-child" },
    { block: ".collect .liturgical-block", opening: ".plain-line:first-child" },
    { block: ".corporate-lord-prayer", opening: ".corporate-lord-prayer-officiant" },
  ],
};
for (const width of [320, 390, 1280]) {
  test(`prose openings turn over at the response's text edge below the initial at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    for (const [path, openings] of Object.entries(proseOpenings)) {
      await openDatedPage(page, path);
      for (const { block, opening } of openings) for (const fallback of [false, true]) {
        const tag = fallback ? await fallbackInitials(page) : null;
        await page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))));
        const target = page.locator(block).first();
        const label = `${path} ${block} ${width}px ${fallback ? "fallback" : "native"}`;
        await expect(target.locator(opening)).not.toHaveClass(/initial-raised/);
        const rows = await target.locator(opening).evaluate(rowStartsIn);
        const edge = await target.locator(".response-line .sigil-text").first().evaluate((el) => el.getBoundingClientRect().left);
        if (width <= 390) expect(rows.length, `${label} turns over`).toBeGreaterThan(2);
        // Beside a narrow initial a line meets the same edge; never short of it.
        for (const left of rows.slice(0, 2)) expect(left, `${label} beside the initial`).toBeGreaterThan(edge - 1);
        for (const left of rows.slice(2)) expect(Math.abs(left - edge), `${label} turnover at the text edge`).toBeLessThan(1);
        await expectInitialInkClear(page, target, opening, `${label} ink clears the initial`);
        if (tag) await tag.evaluate((el) => el.remove());
      }
    }
  });
}

test("wide and narrow initials clear text in native and fallback layouts", async ({ page }) => {
  for (const width of [320, 390]) {
    await page.setViewportSize({ width, height: 844 });
    await openDatedPage(page, "/vespers/2026-06-18");
    for (const size of ["normal", "large"]) {
      await page.evaluate(value => document.documentElement.setAttribute("data-text-size", value), size);
      for (const fallback of [false, true]) {
        const override = fallback ? await fallbackInitials(page) : null;
        for (const initial of ["W", "I"]) {
          const geometry = await page.locator(".psalm-verses").first().evaluate((psalm, letter) => {
            const opening = psalm.querySelector(".verse");
            // Deliberate layout fixture: exercise the extremes of the font's
            // initial widths without depending on a particular day's psalms.
            opening.textContent = letter + "ith all my heart I will give thanks unto the Lord, and tell of all his wonderful works. With all my heart I will give thanks unto the Lord.";
            opening.dataset.initial = letter;
            const node = opening.firstChild;
            const word = node.splitText(1);
            word.splitText(3);
            const incipit = document.createElement("span");
            incipit.className = "initial-word";
            opening.insertBefore(incipit, word);
            incipit.appendChild(word);
            const glyph = (text, index) => {
              const range = document.createRange();
              range.setStart(text, index);
              range.setEnd(text, index + 1);
              const { left, right, top, bottom } = range.getBoundingClientRect();
              return { left, right, top, bottom };
            };
            const cap = glyph(node, 0);
            const following = glyph(word, 0);
            return {
              cap, following,
              last: glyph(opening.lastChild, opening.lastChild.length - 1),
              next: psalm.querySelector(".verse.numbered").getBoundingClientRect().top,
              gloria: psalm.parentElement.querySelector(".gloria-patri").getBoundingClientRect().left +
                parseFloat(getComputedStyle(psalm.parentElement.querySelector(".gloria-patri")).paddingLeft),
              edge: psalm.querySelector(".verse-body").getBoundingClientRect().left,
              overflow: document.documentElement.scrollWidth > innerWidth,
            };
          }, initial);
          const label = `${width}/${size}/${fallback ? "fallback" : "native"}/${initial}`;
          expect(geometry.overflow, label).toBe(false);
          expect(geometry.following.left, label).toBeGreaterThanOrEqual(geometry.cap.right - .5);
          expect(geometry.next, label).toBeGreaterThan(geometry.cap.bottom);
          expect(geometry.next, label).toBeGreaterThan(geometry.last.top);
          expect(Math.abs(geometry.gloria - geometry.edge), label).toBeLessThan(1);
        }
        if (override) await override.evaluate(node => node.remove());
      }
    }
  }
});

async function choosePrayerForm(page, value) {
  const selector = page.locator('.leader-selector');
  if ((await selector.getAttribute('open')) === null) await selector.locator('summary').click();
  await selector.locator(`input[value="${value}"]`).check();
  await expect(page.locator('html')).toHaveAttribute('data-leader', value);
}

test('prayer forms switch complete sequences and keep reports and print consistent', async ({ page, context }) => {
  await page.goto(`/compline/${testDate}`);
  const prayers = page.locator('.elements');
  await expect(page.locator('.leader-selector > summary')).toHaveText(/^Prayer form:\s+Private$/i, { useInnerText: true });
  expect((await prayers.innerText()).match(/I confess to God Almighty/gi)).toHaveLength(1);
  await context.setOffline(true);
  await choosePrayerForm(page, 'deacon');
  expect((await prayers.innerText()).match(/I confess to God Almighty/gi)).toHaveLength(1);
  expect(await prayers.innerText()).toContain('Sir, ask a blessing');
  expect(await prayers.innerText()).not.toContain('Lord, grant a blessing');
  expect(await prayers.innerText()).not.toContain('your sins');
  await expect(page.locator('.prayer-speaker:visible')).toHaveText(['All']);
  await choosePrayerForm(page, 'priest');
  expect((await prayers.innerText()).match(/I confess to God Almighty/gi)).toHaveLength(2);
  expect(await prayers.innerText()).toContain('thee, father');
  expect(await prayers.innerText()).toContain('Sir, ask a blessing');
  expect(await prayers.innerText()).toContain('remission of your sins');
  const speakers = ['Priest', 'People', 'Priest', 'People', 'Priest', 'People', 'Priest', 'People'];
  await expect(page.locator('.prayer-speaker:visible')).toHaveText(speakers);
  await expect(page.locator('.prayer-turn:visible').nth(2)).toContainText('Amen.');
  const report = page.locator('.report-issue:visible a');
  const body = new URL(await report.getAttribute('href')).searchParams.get('body');
  expect(body).toContain('?form=priest');
  expect(body).toContain('Priest');
  await page.emulateMedia({ media: 'print' });
  await expect(page.locator('.prayer-speaker:visible')).toHaveText(speakers);
  expect((await prayers.innerText()).match(/I confess to God Almighty/gi)).toHaveLength(2);
  await expect(page.locator('.leader-selector')).not.toBeVisible();
  await page.emulateMedia({ media: 'screen' });
  await choosePrayerForm(page, 'private');
  expect((await prayers.innerText()).match(/I confess to God Almighty/gi)).toHaveLength(1);
  expect(await prayers.innerText()).not.toContain('The Lord be with you');
});

test('prayer form persists and explicit review links override it without changing another tab', async ({ page, context }) => {
  await page.goto(`/compline/${testDate}`);
  await choosePrayerForm(page, 'priest');
  await page.reload();
  await expect(page.locator('.leader-selector > summary')).toHaveText(/^Prayer form:\s+Priest$/i, { useInnerText: true });
  const other = await context.newPage();
  // Avoid Chromium cross-document transition stalls with multiple test tabs.
  await other.emulateMedia({ reducedMotion: 'reduce' });
  await other.goto(`/lauds/${testDate}?form=deacon`);
  await expect(other.locator('html')).toHaveAttribute('data-leader', 'deacon');
  expect(await other.evaluate(() => localStorage.getItem('office-prayer-form'))).toBe('priest');
  await other.locator('.next-hour').click();
  await expect(other).toHaveURL(/\/prime\/.*form=deacon/);
  await other.bringToFront();
  await expect(other.locator('html')).toHaveAttribute('data-leader', 'deacon');
  await choosePrayerForm(other, 'private');
  await expect(page.locator('html')).toHaveAttribute('data-leader', 'priest');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-leader', 'private');
  await other.close();
});

test('prayer-form metrics record the rendered form and subsequent switches only on office pages', async ({ page }) => {
  const bodies = [];
  await page.route('**/api/usage', async route => {
    bodies.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  const today = await serverTodaySlug(page);
  bodies.length = 0;
  await page.goto(`/lauds/${today}`);
  await page.locator('.hour-header h1').click();
  await expect.poll(() => bodies.some(body => body.includes('prayer-form:private'))).toBe(true);
  await choosePrayerForm(page, 'priest');
  await expect.poll(() => bodies.some(body => body.includes('prayer-form:priest'))).toBe(true);
  await choosePrayerForm(page, 'deacon');
  await expect.poll(() => bodies.some(body => body.includes('prayer-form:deacon'))).toBe(true);
  const count = bodies.length;
  await choosePrayerForm(page, 'private');
  await page.waitForTimeout(150);
  expect(bodies).toHaveLength(count);
  await page.goto('/');
  await page.locator('.site-brand').click();
  await expect.poll(() => bodies.some(body => body.startsWith('site '))).toBe(true);
  expect(bodies.filter(body => body.startsWith('site ')).every(body => !body.includes('prayer-form:'))).toBe(true);
});

test('an hour with a score reports whether its chant notation is shown, and no other page does', async ({ page }) => {
  const bodies = [];
  await page.route('**/api/usage', async route => {
    bodies.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  const today = await serverTodaySlug(page);
  // Compline closes with a scored Marian antiphon on every day but the Triduum's.
  await page.goto(`/compline/${today}`);
  const scored = await page.locator('[data-chant-variant="on"]').count() > 0;
  await page.locator('.hour-header h1').click();
  await expect.poll(() => bodies.some(body => body.startsWith('compline '))).toBe(true);
  expect(bodies[0].includes(' chant:off ')).toBe(scored);
  expect(bodies[0]).not.toContain('chant:on');
  if (scored) {
    await choosePreference(page, 'On', '.chant-option');
    await expect.poll(() => bodies.some(body => body.includes(' chant:on '))).toBe(true);
  }
  await page.goto('/');
  await page.locator('.site-brand').click();
  await expect.poll(() => bodies.some(body => body.startsWith('site '))).toBe(true);
  expect(bodies.filter(body => body.startsWith('site ')).every(body => !body.includes('chant:'))).toBe(true);
});

test('cached prayer pages include every form and preserve an explicit form through redirects', async ({ browser, baseURL, browserName }) => {
  test.skip(browserName === "webkit", "Playwright's WebKit fails any navigation once context.setOffline(true) is set, even one its service worker would answer; service-worker offline emulation works only in Chromium");
  const context = await browser.newContext({ serviceWorkers: 'allow', baseURL });
  try {
    const page = await context.newPage();
    const today = await serverTodaySlug(page);
    await page.goto(`/compline/${today}`);
    await page.evaluate(async () => {
      await navigator.serviceWorker.ready;
      if (!navigator.serviceWorker.controller) await new Promise(resolve => navigator.serviceWorker.addEventListener('controllerchange', resolve, { once: true }));
    });
    await page.reload();
    await context.setOffline(true);
    await page.goto(`/compline?date=${today}&form=priest`);
    await expect(page.locator('html')).toHaveAttribute('data-leader', 'priest');
    expect((await page.locator('.elements').innerText()).match(/I confess to God Almighty/gi)).toHaveLength(2);
    await choosePrayerForm(page, 'private');
    expect((await page.locator('.elements').innerText()).match(/I confess to God Almighty/gi)).toHaveLength(1);
  } finally { await context.close(); }
});

test("the ordo's current month and frontispiece are saved for offline use", async ({ browser, baseURL, browserName }) => {
  test.skip(browserName === "webkit", "Playwright's WebKit fails any navigation once context.setOffline(true) is set, even one its service worker would answer; service-worker offline emulation works only in Chromium");
  const context = await browser.newContext({ serviceWorkers: 'allow', baseURL });
  try {
    const page = await context.newPage();
    const today = await serverTodaySlug(page);
    const month = `/calendar/${today.slice(0, 4)}/${today.slice(5, 7)}`;
    const year = `/calendar/${today.slice(0, 4)}`;
    await page.goto(`/lauds/${today}`);
    await page.evaluate(async () => {
      await navigator.serviceWorker.ready;
      if (!navigator.serviceWorker.controller) await new Promise(resolve => navigator.serviceWorker.addEventListener('controllerchange', resolve, { once: true }));
    });
    // The month is in the install shell; the frontispiece follows in the top-up.
    await expect.poll(() => page.evaluate(async (paths) => {
      for (const path of paths) if (!(await caches.match(path))) return false;
      return true;
    }, [month, year]), { timeout: 15000 }).toBe(true);
    await context.setOffline(true);
    await page.goto('/calendar');
    await expect(page).toHaveURL(new RegExp(`${month}#d-${today}$`));
    await expect(page.locator(`#d-${today}`)).toBeVisible();
    await page.goto(year);
    await expect(page.getByRole('heading', { name: 'Tabula Temporaria', exact: true })).toBeVisible();
  } finally { await context.close(); }
});

test('prayer-form controls fit both themes and narrow or wide reading', async ({ page }) => {
  await page.goto(`/compline/${testDate}`);
  for (const width of [320, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    for (const theme of ['light', 'dark']) {
      await page.evaluate(value => document.documentElement.setAttribute('data-theme', value), theme);
      const selector = page.locator('.leader-selector');
      await selector.evaluate(element => { element.open = true; });
      for (const name of ['Praying privately', 'With others, led by a deacon', 'With others, led by a priest']) {
        await expect(selector.getByRole('radio', { name, exact: true })).toBeVisible();
      }
      // The picker lends its parts to the controls' grid, so its open panel is what is measured.
      const bounds = await selector.locator('fieldset').boundingBox();
      expect(bounds.x).toBeGreaterThanOrEqual(0);
      expect(bounds.x + bounds.width).toBeLessThanOrEqual(width);
      const labels = await selector.locator('label').evaluateAll(elements => elements.map(element => element.getBoundingClientRect().height));
      expect(labels.every(height => height >= 44)).toBe(true);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      const results = await new AxeBuilder({ page }).include('.leader-selector').analyze();
      expect(results.violations).toEqual([]);
    }
  }
});

test('prayer forms fall back safely with unavailable storage or scripting', async ({ page, browser, baseURL }) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, 'localStorage', { get() { throw new Error('storage disabled'); } });
  });
  await page.goto(`/compline/${testDate}`);
  await expect(page.locator('html')).toHaveAttribute('data-leader', 'private');
  await choosePrayerForm(page, 'deacon');
  await expect(page).toHaveURL(/form=deacon/);
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-leader', 'deacon');

  const plain = await browser.newContext({ baseURL, javaScriptEnabled: false });
  try {
    const reader = await plain.newPage();
    await reader.goto(`/compline/${testDate}?form=priest`);
    await expect(reader.locator('.leader-selector > summary')).toHaveText(/^Prayer form:\s+Private$/i, { useInnerText: true });
    await reader.locator('.leader-selector > summary').click();
    await expect(reader.locator('.leader-selector input').first()).toBeDisabled();
    await expect(reader.locator('noscript p')).toContainText('The private form is shown');
    expect((await reader.locator('.elements').innerText()).match(/I confess to God Almighty/gi)).toHaveLength(1);
  } finally { await plain.close(); }
});

test('private greetings are not repeated after preces when switching offline or printing', async ({ page, context }) => {
  for (const [path, privateCount] of [
    ['/compline/2026-03-10', 2],
    ['/prime/2026-03-10', 2],
    ['/lauds/2026-11-02', 1],
    ['/vespers/2026-11-01', 3],
    ['/compline/2026-11-01', 1],
  ]) {
    await context.setOffline(false);
    await page.goto(`${path}?form=private`);
    const prayers = page.locator('.elements');
    const countPrivateResponses = async () => {
      const text = (await prayers.innerText()).replace(/[℣℟]\./g, '').replace(/\s+/g, ' ');
      return (text.match(/O Lord, hear my prayer\. And let my cry come unto thee\./g) || []).length;
    };
    expect(await countPrivateResponses()).toBe(privateCount);
    const omittedGreeting = page.locator('.leader-slot[data-leader-slot="greeting"][data-leaders="deacon priest"]');
    await expect(omittedGreeting).toHaveCount(1);
    await expect(omittedGreeting).not.toBeVisible();
    await context.setOffline(true);
    for (const form of ['deacon', 'priest']) {
      await choosePrayerForm(page, form);
      expect(await countPrivateResponses()).toBe(1);
      expect(await prayers.innerText()).toContain('The Lord be with you');
      await expect(omittedGreeting).toBeVisible();
    }
    await choosePrayerForm(page, 'private');
    expect(await countPrivateResponses()).toBe(privateCount);
    await page.emulateMedia({ media: 'print' });
    expect(await countPrivateResponses()).toBe(privateCount);
    await expect(omittedGreeting).not.toBeVisible();
    await page.emulateMedia({ media: 'screen' });
  }
});

// Diurnal p. 313: the Our Father is entirely silent; the collect's body is
// spoken, and its conclusion is silent. These cues must survive both themes.
test("Triduum distinguishes silent prayers and the collect conclusion", async ({ page }) => {
  for (const theme of ["light", "dark"]) {
    await openDatedPage(page, "/lauds/2026-04-09?form=priest", theme);
    const collect = page.locator(".collect");
    await expect(collect).toHaveCount(1);
    await expect(collect.locator(".spoken-text")).toContainText("Almighty God");
    await expect(collect.locator(".secret-text").first()).toContainText("Who with thee");
    await expect(page.locator(".secret-text").filter({ hasText: /^Our Father/ })).toHaveCount(1);
    const colors = await collect.evaluate(el => ({
      spoken: getComputedStyle(el.querySelector(".spoken-text")).color,
      silent: getComputedStyle(el.querySelector(".secret-text")).color,
    }));
    expect(colors.silent).not.toBe(colors.spoken);
  }
  await page.goto("/compline/2026-04-11?form=priest");
  await expect(page.locator(".collect .secret-text")).toHaveCount(0);
});

// A newcomer's word in a rubric opens its note above the word, so the note
// never covers the prayer it explains, and closes by a second tap, a tap
// elsewhere, or Escape.
test("a rubric's secretly opens its note above the word", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  for (const theme of ["light", "dark"]) {
    await openDatedPage(page, `/lauds/${testDate}`, theme);
    const rubric = page.locator("p.rubric", { hasText: "are said secretly." }).first();
    const term = rubric.getByRole("button", { name: "secretly" });
    await expect(rubric).toHaveText("The Our Father and Hail Mary are said secretly.");
    await expect(term).toHaveAttribute("aria-expanded", "false");
    // Closed, the note is an empty live region clipped out of sight.
    const note = page.locator("#gloss-note");
    await expect(note).toHaveClass(/closed/);

    await term.scrollIntoViewIfNeeded();
    await term.click();
    await expect(term).toHaveAttribute("aria-expanded", "true");
    await expect(note).not.toHaveClass(/closed/);
    await expect(note).toContainText("Only the first words are said aloud.");
    const [t, n] = [await term.boundingBox(), await note.boundingBox()];
    expect(n.y + n.height).toBeLessThanOrEqual(t.y);
    expect(n.x).toBeGreaterThanOrEqual(0);
    expect(n.x + n.width).toBeLessThanOrEqual(390);

    await term.click();
    await expect(note).toHaveClass(/closed/);
    await term.click();
    await page.keyboard.press("Escape");
    await expect(note).toHaveClass(/closed/);
    await expect(term).toBeFocused();
    // A tap on the prayer's plain text, which iOS answers with no click.
    await term.click();
    await expect(note).not.toHaveClass(/closed/);
    const prayer = await page.locator(".secret-text").first().boundingBox();
    await page.touchscreen.tap(prayer.x + 20, prayer.y + prayer.height / 2);
    await expect(note).toHaveClass(/closed/);
    await expect(term).toHaveAttribute("aria-expanded", "false");
  }
});

// The term stays inline: as a <button> it laid out as inline-block, and the
// rubric's closing stop wrapped alone onto the next line.
test("a glossed term keeps the rubric's stop on its line", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openDatedPage(page, "/lauds/2026-04-09");
  const rubric = page.locator("p.rubric", { hasText: "are said secretly." }).first();
  await expect(rubric.getByRole("button", { name: "secretly" })).toBeVisible();
  const tops = await rubric.evaluate((p) => {
    const term = p.querySelector(".gloss-term");
    const stop = term.nextSibling;
    const range = document.createRange();
    range.setStart(stop, 0);
    range.setEnd(stop, 1);
    const rects = term.getClientRects();
    return { term: rects[rects.length - 1].top, stop: range.getBoundingClientRect().top, text: stop.data };
  });
  expect(tops.text).toBe(".");
  expect(Math.abs(tops.stop - tops.term)).toBeLessThan(2);
});

// Compline's Our Father before the Confession has no voiced words, so its
// rubric takes the whole-prayer note, not the incipit's.
test("entirely in secret takes its own note", async ({ page }) => {
  await openDatedPage(page, `/compline/${testDate}`);
  await page.getByRole("button", { name: "entirely in secret" }).click();
  await expect(page.locator("#gloss-note")).toContainText("The whole prayer, shown in grey, is said silently.");
});

test("home keeps feast and octave above the recovery link, including after midnight", async ({ page }) => {
  await openDatedPage(page, "/?date=2026-12-25");
  const order = () => page.locator(".home-day-head").evaluate(el => {
    const notice = el.querySelector(".not-today-notice");
    return [...el.querySelectorAll("h1, .feast, .octave-note")].every(item =>
      Boolean(item.compareDocumentPosition(notice) & Node.DOCUMENT_POSITION_FOLLOWING) &&
      item.getBoundingClientRect().bottom <= notice.getBoundingClientRect().top);
  });
  expect(await order()).toBe(true);
  await expect(page.locator(".color-band")).toHaveCount(0);
  await expect(page.getByText("Liturgical color:", { exact: false })).toHaveCount(1);

  const today = await serverTodaySlug(page);
  const beforeMidnight = await page.evaluate(slug => new Date(`${slug}T23:59:00`).getTime(), today);
  await page.clock.install({ time: new Date(beforeMidnight) });
  await page.goto(`/?date=${today}`);
  await expect(page.locator(".not-today-notice")).toHaveCount(0);
  await page.clock.fastForward("02:00");
  await expect(page.locator(".not-today-notice")).toHaveAttribute("role", "status");
  expect(await order()).toBe(true);
});

test("home's versicle takes only the height its head has to spare", async ({ page }) => {
  // Each day is opened as the reader's today, so no recovery link joins the head.
  const open = async (date, width, height) => {
    await page.setViewportSize({ width, height });
    await page.clock.setFixedTime(new Date(`${date}T15:56:00-04:00`));
    await page.goto(`/?date=${date}`);
    await page.evaluate(() => document.fonts.ready);
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    return page.evaluate(() => ({
      shown: document.querySelector(".home-versicle")?.checkVisibility() ?? null,
      scrolls: document.documentElement.scrollHeight > innerHeight,
    }));
  };
  // St Placidus's two lines fit a 412×735 phone without moving the hours, and
  // so, with no colophon under the arch, do the Holy Angels' four.
  expect(await open("2026-10-05", 412, 735)).toEqual({ shown: true, scrolls: false });
  expect(await open("2026-10-02", 412, 735)).toEqual({ shown: true, scrolls: false });
  // On a narrow phone, where the day's lines wrap, they do not, so the head
  // goes without them; a taller phone has room.
  expect(await open("2026-10-02", 320, 740)).toEqual({ shown: false, scrolls: false });
  expect(await open("2026-10-02", 430, 932)).toEqual({ shown: true, scrolls: false });
  // A 667px phone has none to spare.
  expect(await open("2026-10-05", 375, 667)).toEqual({ shown: false, scrolls: false });
  // A Lenten feria is stripped: no versicle at all.
  expect((await open("2026-03-11", 412, 735)).shown).toBe(null);
});

test("first-class hours stand in a frame that leaves the measure and ends at the prayer", async ({ page }) => {
  for (const [width, height] of [[390, 844], [1280, 900]]) {
    await page.setViewportSize({ width, height });
    await openDatedPage(page, "/lauds/2026-12-25");
    await expect(page.locator("body")).toHaveClass(/rank-first-class/);
    const frame = await page.locator(".office-hour").evaluate(el => {
      const before = getComputedStyle(el, "::before");
      const after = getComputedStyle(el, "::after");
      const elements = el.querySelector(".elements").getBoundingClientRect();
      const epilogue = el.querySelector(".hour-epilogue").getBoundingClientRect();
      const grid = getComputedStyle(el).gridTemplateColumns.split(" ").map(parseFloat);
      return {
        drawn: before.borderLeftStyle === "double" && after.borderLeftStyle === "double",
        // Each line sits in its own grid column, and the inner column is the full measure.
        side: grid[0], measure: grid[1], elementsWidth: elements.width,
        gap: getComputedStyle(el).columnGap,
        epilogueTop: epilogue.top, elementsBottom: elements.bottom,
      };
    });
    expect(frame.drawn).toBe(true);
    expect(frame.gap).toBe("0px");
    // The phone gives the frame a little of its measure; the desktop's measure stays untouched.
    expect(frame.elementsWidth).toBeLessThanOrEqual(frame.measure + 0.5);
    expect(frame.side).toBe(width < 920 ? 12 : 8);
    await openDatedPage(page, "/lauds/2026-10-07");
    const feria = await page.locator(".elements").evaluate(el => el.getBoundingClientRect().width);
    expect(frame.elementsWidth).toBeCloseTo(width < 920 ? feria - 24 : feria, 0);
    await openDatedPage(page, "/lauds/2026-12-25");
    // The phone's lines are drawn out past their columns to the screen's edge.
    const lines = await page.locator(".office-hour").evaluate(el => {
      const r = el.getBoundingClientRect();
      const before = getComputedStyle(el, "::before");
      const after = getComputedStyle(el, "::after");
      return { left: before.translate, right: after.translate, width: r.width };
    });
    expect(lines.left).toBe(width < 920 ? "-20px" : "none");
    expect(lines.right).toBe(width < 920 ? "20px" : "none");
  }
  // A feria has no frame.
  await openDatedPage(page, "/lauds/2026-10-07");
  await expect(page.locator("body")).not.toHaveClass(/rank-first-class/);
  expect(await page.locator(".office-hour").evaluate(el => getComputedStyle(el, "::before").content)).toBe("none");
});

test("Compline openings preserve words and align response columns around the blessing", async ({ page }) => {
  for (const width of [320, 390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, "/compline/2026-12-25");
    await expect(page.locator(".elements > .section-heading").first()).toHaveText("Opening");
    // The opening capital stays full size; only the rest of the run is small caps.
    const confession = page.locator(".opening-small-caps:visible").filter({ hasText: /^CONFESS$/ });
    await expect(confession).toHaveCount(1);
    expect(await confession.evaluate(el => getComputedStyle(el).fontVariantCaps)).toBe("all-small-caps");
    expect(await confession.evaluate(el => el.parentElement.textContent.startsWith("I CONFESS to God"))).toBe(true);
    expect(await confession.evaluate(el => el.previousSibling.textContent)).toBe("I ");
    const lines = page.locator(".elements .sigil:visible:not(.sigil-word):not(.sigil-all)");
    const edges = await lines.evaluateAll(els => els.slice(0, 5).map(el => el.getBoundingClientRect().left));
    for (const edge of edges) expect(edge).toBeCloseTo(edges[0], 0);
    const blessing = page.locator(".sigil-word:visible").first();
    expect(await blessing.evaluate(el => el.getBoundingClientRect().left)).toBeGreaterThanOrEqual(0);
    const blessingText = await blessing.evaluate(el => {
      const rect = el.nextElementSibling.getBoundingClientRect();
      const reference = el.parentElement.previousElementSibling.querySelector(".sigil-text").getBoundingClientRect();
      return { left: rect.left, right: rect.right, width: rect.width, reference: reference.left };
    });
    expect(blessingText.left).toBeCloseTo(blessingText.reference, 0);
    expect(blessingText.width).toBeGreaterThan(200);
    expect(blessingText.right).toBeLessThanOrEqual(width);
  }
});

test("Office prayer instructions retain spacing and Marian collects share initial treatment", async ({ page }) => {
  for (const theme of ["light", "dark"]) {
    await openDatedPage(page, "/vespers/2026-09-22", theme);
    const rubric = page.locator(".session-prayers-content > .rubric").last();
    // A stanza's step of the prayer's line (the rhythm's space between elements), on screen and paper.
    const steps = () => rubric.evaluate(el => (el.getBoundingClientRect().top - el.previousElementSibling.getBoundingClientRect().bottom)
      / parseFloat(getComputedStyle(el).getPropertyValue("--line")));
    expect(await steps()).toBeCloseTo(0.45, 1);
    const collect = page.locator(".marian-antiphon .collect .plain-line").first();
    await expect(collect).toHaveAttribute("data-initial", "A");
    await expect(collect.locator(".initial-word")).toHaveText("lmighty");
    await page.emulateMedia({ media: "print" });
    await expect(rubric).toBeVisible();
    expect(await steps()).toBeCloseTo(0.45, 1);
    await page.emulateMedia({ media: "screen" });
  }
});

test("error pages display hostile input as text", async ({ page }) => {
  const payload = '<img src=x onerror="window.unsafeMarkup=true"> + " & / café';
  const response = await page.goto(`/?date=${encodeURIComponent(payload)}`);
  expect(response.status()).toBe(400);
  await expect(page.locator(".not-found-msg")).toHaveText(
    `Invalid date ${JSON.stringify(payload)} — please use YYYY-MM-DD format.`,
  );
  await expect(page.locator(".not-found img")).toHaveCount(0);
  expect(await page.evaluate(() => window.unsafeMarkup)).toBeUndefined();
  await expect(page.locator(".not-found-home a")).toHaveAttribute("href", "/");
});

test("escaped page links retain navigation and report query values", async ({ page }) => {
  await page.goto("/lauds/2026-06-07");
  await expect(page.locator(".next-hour")).toHaveAttribute("href", "/prime/2026-06-07");
  const report = page.getByRole("link", { name: "Report a problem", exact: true }).first();
  const url = new URL(await report.getAttribute("href"));
  expect(url.hostname).toBe("github.com");
  expect(url.searchParams.get("body")).toContain("2026-06-07");
  expect(url.searchParams.get("body")).toContain("lauds");
  await page.locator(".next-hour").click();
  await expect(page).toHaveURL(/\/prime\/2026-06-07/);
});

test("a feast's page leads back to the search that found it", async ({ page }) => {
  await page.goto("/calendar/find?q=raphael");
  await page.locator(".find-name").first().click();
  await expect(page).toHaveURL(/\/feast\//);
  await expect(page.locator(".find-back")).toHaveAttribute("href", "/calendar/find?q=raphael");
  await page.locator(".find-back").click();
  await expect(page).toHaveURL(/\/calendar\/find\?q=raphael$/);

  // Reached otherwise, it leads to the empty search.
  await page.goto("/feast/st-raphael-of-brooklyn");
  await expect(page.locator(".find-back")).toHaveAttribute("href", "/calendar/find");
});

test("a feast's page leads back to the day or the ordo it was opened from", async ({ page }) => {
  await openDatedPage(page, "/?date=2026-02-27");
  await page.locator(".home .feast-link").first().click();
  await expect(page).toHaveURL(/\/feast\/st-raphael-of-brooklyn\?from=2026-02-27$/);
  await expect(page.locator(".find-back")).toHaveText(/February 27$/);
  await page.locator(".find-back").click();
  await expect(page).toHaveURL(/\/\?date=2026-02-27$/);

  await page.goto("/feast/st-raphael-of-brooklyn?from=ordo-2026-02-27");
  await expect(page.locator(".find-back")).toHaveText(/Ordo, February 2026$/);
  await expect(page.locator(".find-back")).toHaveAttribute("href", "/calendar/2026/02#d-2026-02-27");
});

test("a feast's head is home's day block turned about, ruled with home's pen", async ({ page }) => {
  await page.goto("/feast/annunciation");
  const head = page.locator(".feast-head");
  await expect(head).toHaveClass(/is-first/);
  // The date is the titulus over the name, read after it.
  const box = (sel) => page.locator(sel).first().evaluate((n) => n.getBoundingClientRect().top);
  expect(await box(".feast-titulus")).toBeLessThan(await box(".feast-head h1"));
  await expect(page.locator(".feast-titulus")).toHaveText("March 25");
  // Each painted line is a pen stroke once script has run; the CSS borders stand down.
  for (const host of [".feast-head", ".feast-standing", ".feast-texts > h2", ".feast-years .find-years"]) {
    await expect(page.locator(host).first()).toHaveClass(/is-ruled/);
  }
  expect(await page.locator(".feast-years .find-years > svg.feast-rules path").count()).toBe(9);
  // A greater double carries the cross in the lining; a monastic observance's head is bare.
  await page.goto("/feast/st-raphael-of-brooklyn");
  await expect(page.locator(".feast-head")).toHaveClass(/is-feast/);
  await page.goto("/feast/solemnity-st-benedict");
  await expect(page.locator(".feast-head")).toHaveClass(/is-plain/);
  await expect(page.locator(".feast-head > svg.feast-rules")).toHaveCount(0);
});

test("a feast's rules follow the reader's text size, and a feast never kept has no empty standing", async ({ page }) => {
  await page.goto("/feast/st-raphael-of-brooklyn");
  const frame = () => page.locator(".feast-standing").evaluate((n) => {
    const ys = [...n.querySelectorAll(":scope > svg.feast-rules path")].map((p) => p.getBBox()).map((b) => b.y + b.height);
    return { height: n.offsetHeight, bottom: Math.max(...ys) };
  });
  await page.evaluate(() => document.documentElement.setAttribute("data-text-size", "large"));
  await expect.poll(async () => { const f = await frame(); return Math.abs(f.bottom - f.height) < 3; }).toBe(true);

  await page.goto("/feast/comm-12-25-st-anastasia-virgin-and-martyr");
  await expect(page.locator(".feast-standing")).toHaveCount(0);
});

test("a feast's rules are never drawn across an unstyled page", async ({ page }) => {
  // WebKit can run the deferred script before a slow stylesheet applies; the
  // rules then wait for the page's load rather than measuring bare markup.
  await page.route(/style\.css/, async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 400));
    await route.continue();
  });
  await page.addInitScript(() => {
    document.addEventListener("DOMContentLoaded", () => {
      window.__early = {
        rules: document.querySelectorAll("svg.feast-rules path").length,
        styled: Boolean(getComputedStyle(document.documentElement).getPropertyValue("--bg")),
      };
    });
  });
  await page.goto("/feast/christmas");
  // Chromium holds the script for the stylesheet; WebKit may not, and then
  // nothing is ruled yet.
  const early = await page.evaluate(() => window.__early);
  expect(early.styled || early.rules === 0, JSON.stringify(early)).toBe(true);
  const fits = () => page.locator(".feast-standing").evaluate((n) => {
    const ys = [...n.querySelectorAll(":scope > svg.feast-rules path")].map((p) => p.getBBox()).map((b) => b.y + b.height);
    return ys.length > 0 && Math.abs(Math.max(...ys) - n.offsetHeight) < 3;
  });
  await expect.poll(fits).toBe(true);
});

test("a feast's page leads with its standing, then its texts, then its years, beside them when wide", async ({ page }) => {
  await page.goto("/feast/st-raphael-of-brooklyn");
  const top = (sel) => page.locator(sel).first().evaluate((n) => n.getBoundingClientRect().top);
  const left = (sel) => page.locator(sel).first().evaluate((n) => n.getBoundingClientRect().left);
  // A phone: one column, the standing, then the texts with the collect first, then the years.
  const [standing, texts, years] = [await top(".feast-standing"), await top(".feast-texts"), await top(".feast-years")];
  expect(standing).toBeLessThan(texts);
  expect(texts).toBeLessThan(years);
  await expect(page.locator(".feast-text h3").first()).toHaveText(/^Collect/);
  await expect(page.locator(".feast-standing .feast-next")).toBeVisible();
  // A year kept as usual says nothing more; one he is only commemorated says so.
  await expect(page.locator(".feast-years .find-year-status b", { hasText: /^Kept$/ })).toHaveCount(0);
  expect(await page.locator(".feast-years .find-year-status b", { hasText: "Commemorated" }).count()).toBeGreaterThan(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);

  // A tablet already has the rail, so the prayer keeps its measure.
  await page.setViewportSize({ width: 820, height: 1180 });
  expect(await left(".feast-standing")).toBeGreaterThan(await left(".feast-texts") + 300);
  // A wide screen: the standing and the years make a rail to the right of the texts.
  await page.setViewportSize({ width: 1280, height: 900 });
  expect(await left(".feast-standing")).toBeGreaterThan(await left(".feast-texts") + 300);
  // (The standing's frame sits 3px in, its outer rule in the margin.)
  expect(Math.abs(await left(".feast-years") - await left(".feast-standing"))).toBeLessThanOrEqual(4);
  expect(await top(".feast-standing")).toBeLessThan(await top(".feast-years"));
  expect(Math.abs(await top(".feast-standing") - await top(".feast-texts"))).toBeLessThan(24);
});

test("Chant On notes every verse of a whole hymn, its mode's figure once over the initial", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 900 });
  await openDatedPage(page, "/lauds/2026-10-28");
  const scores = page.locator(".hymn .chant-hymn");
  await expect(scores).toBeHidden();

  // The engine measures the lyrics with the app's own Garamond's metrics, fetched with it.
  const metrics = page.waitForResponse((r) => r.url().includes("eb-garamond-lyrics.nmet") && r.ok());
  await choosePreference(page, "On", ".chant-option");
  await expect(page.locator("html")).toHaveAttribute("data-chant", "on");
  await metrics;
  await expect(scores).toBeVisible();
  expect(await scores.locator(".chant-stanza").count()).toBeGreaterThan(2);
  // The mode once, as a figure the engine sets over the first verse's initial.
  await expect(scores.locator("svg [class$='-annotation']")).toHaveCount(1);
  await expect(scores.locator(".chant-stanza").first().locator("svg [class$='-annotation']")).toHaveText(/^[IVX]+$/);

  await choosePreference(page, "Off", ".chant-option");
  await expect(scores).toBeHidden();
});

test("a chant score fills its column at every width and keeps its own when the engine can't load", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("office-chant", "on"));
  // The hour's last score, off the screen: one on it keeps its first drawing until it leaves.
  const fit = () => page.evaluate(() => {
    const score = [...document.querySelectorAll(".chant-score")].filter((e) => e.offsetParent).pop();
    const box = score.querySelector(".chant-width");
    return {
      column: score.clientWidth,
      score: box.getBoundingClientRect().width,
      laidOut: !box.querySelector("svg").getAttribute("class").endsWith(box.getAttribute("data-chant-prefix")),
      // Laid out here, or engraved by the server at the column the page last reported.
      atColumn: !box.querySelector("svg").getAttribute("class").endsWith(box.getAttribute("data-chant-prefix")) ||
        Math.abs(parseFloat(box.dataset.chantUnits) - (score.clientWidth / parseFloat(getComputedStyle(score).fontSize)) * 2.45 * 6) < 0.5,
      overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    };
  });
  for (const width of [320, 390, 430, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, "/compline/2026-10-27");
    await expect.poll(async () => (await fit()).atColumn, { message: `${width}px` }).toBe(true);
    const f = await fit();
    expect(Math.abs(f.score - f.column), `${width}px: ${JSON.stringify(f)}`).toBeLessThan(1);
    expect(f.overflow, `${width}px`).toBe(0);
  }
  // A narrower window lays it out again.
  await page.setViewportSize({ width: 360, height: 900 });
  await expect.poll(async () => { const f = await fit(); return Math.abs(f.score - f.column) < 1; }).toBe(true);

  // Without the engine the page's own score stands, inside the column.
  await page.route(/\/static\/neuma\.mjs/, (route) => route.abort());
  await openDatedPage(page, "/compline/2026-10-27");
  await page.waitForTimeout(300);
  const fallback = await fit();
  expect(fallback.laidOut).toBe(false);
  expect(fallback.score).toBeLessThanOrEqual(fallback.column + 0.5);
  expect(fallback.overflow).toBe(0);
  // Back online, the next resize fetches it again and lays the score out.
  await page.unroute(/\/static\/neuma\.mjs/);
  await page.setViewportSize({ width: 380, height: 900 });
  await expect.poll(async () => (await fit()).laidOut).toBe(true);

  // A browser too old for the engine never fetches it.
  const fetched = [];
  page.on("request", (r) => { if (r.url().includes("/static/neuma.mjs")) fetched.push(r.url()); });
  await page.addInitScript(() => { HTMLScriptElement.supports = () => false; });
  await openDatedPage(page, "/compline/2026-10-27");
  await page.waitForTimeout(300);
  expect((await fit()).laidOut).toBe(false);
  expect(fetched).toEqual([]);
});

// Every frame's heights of the scores shown, from the page's first frame. A long page may be
// painted before it is all parsed, so a frame can show fewer scores than the next: each score is
// compared with itself.
async function logScoreHeights(page) {
  await page.addInitScript(() => {
    window.__scores = [];
    const loop = () => {
      if (document.body) {
        const heights = [...document.querySelectorAll(".chant-score")].filter((e) => e.offsetParent).map((e) => Math.round(e.getBoundingClientRect().height));
        if (heights.length) window.__scores.push(heights.join("/"));
      }
      if (performance.now() < 10000) requestAnimationFrame(loop);
    };
    requestAnimationFrame(loop);
  });
}

test("a chant score is drawn at its final height from the first frame once the column is known", async ({ page, context }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.setItem("office-chant", "on"));
  await logScoreHeights(page);
  // A first visit lays the score out at the column and keeps the column's width for the server.
  await openDatedPage(page, "/terce/2026-11-13");
  await expect.poll(async () => (await context.cookies()).find((c) => c.name === "chant-width")?.value).toMatch(/^\d+\.\d{4}$/);
  const column = await page.evaluate(() => {
    const score = [...document.querySelectorAll(".chant-score")].find((e) => e.offsetParent);
    return (Math.round((score.clientWidth / parseFloat(getComputedStyle(score).fontSize)) * 16) / 16).toFixed(4);
  });
  expect((await context.cookies()).find((c) => c.name === "chant-width").value).toBe(column);

  // The next page arrives engraved at it: no score changes its height, and the engine is not fetched.
  const fetched = [];
  page.on("request", (r) => { if (r.url().includes("/static/neuma.mjs")) fetched.push(r.url()); });
  for (const path of ["/compline/2026-11-13", "/terce/2026-11-13"]) {
    await openDatedPage(page, path);
    await page.waitForTimeout(800);
    const frames = await page.evaluate(() => window.__scores);
    const changed = frames.reduce((seen, frame) => {
      frame.split("/").forEach((h, i) => (seen[i] ||= new Set()).add(h));
      return seen;
    }, []).some((heights) => heights.size > 1);
    expect(changed, `${path}: ${frames[0]} then ${frames[frames.length - 1]}`).toBe(false);
    const fit = await page.evaluate(() => [...document.querySelectorAll(".chant-score")].filter((e) => e.offsetParent)
      .map((e) => e.clientWidth - e.querySelector(".chant-width").getBoundingClientRect().width));
    for (const gap of fit) expect(Math.abs(gap), path).toBeLessThan(1);
  }
  expect(fetched).toEqual([]);
});

test("a score laid out anew on arriving never moves the words being read", async ({ page, context }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.setItem("office-chant", "on"));
  // The page's faces fetched beforehand (the engine not), and no column reported: without scroll
  // anchoring a face arriving moves the text too, and this test is about the scores.
  await page.route("**/static/neuma.mjs*", (route) => route.abort());
  await page.goto("/compline/2026-11-13");
  await page.evaluate(() => document.fonts.ready);
  await expect.poll(async () => (await context.cookies()).some((c) => c.name === "chant-width")).toBe(true);
  await page.goto("about:blank");
  await page.unroute("**/static/neuma.mjs*");
  await context.clearCookies();
  await logScoreHeights(page);
  // A reader brought back below the scores, in a browser that does not anchor scrolling, at a
  // width the page was not engraved at.
  await page.addInitScript(() => {
    document.addEventListener("DOMContentLoaded", () => {
      document.documentElement.style.overflowAnchor = "none";
      const last = [...document.querySelectorAll(".chant-score")].filter((e) => e.offsetParent).pop();
      window.scrollTo(0, last.getBoundingClientRect().bottom + scrollY + 300);
      const probe = [...document.querySelectorAll(".elements p")].find((p) => p.offsetParent && p.getBoundingClientRect().top >= 100);
      window.__probe = [];
      // Watched once the faces are in, with the scores' heights beside it.
      const loop = () => {
        if (document.fonts.status === "loaded") {
          const scores = [...document.querySelectorAll(".chant-score")].filter((e) => e.offsetParent).map((e) => Math.round(e.getBoundingClientRect().height)).join("/");
          window.__probe.push({ top: probe.getBoundingClientRect().top, scores });
        }
        if (performance.now() < 10000) requestAnimationFrame(loop);
      };
      requestAnimationFrame(loop);
    });
  });
  await page.goto("/compline/2026-11-13");
  // The scores above were laid out again at the column while the line was watched...
  await expect.poll(() => page.evaluate(() => { const f = window.__probe; return f.length > 0 && f[0].scores !== f[f.length - 1].scores; })).toBe(true);
  await page.waitForTimeout(300);
  // ...and the line being read stood still.
  const probe = await page.evaluate(() => window.__probe.map((f) => f.top));
  expect(Math.max(...probe) - Math.min(...probe)).toBeLessThan(1);

});

test("a score on the screen keeps its first drawing until it leaves the screen", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.setItem("office-chant", "on"));
  // The first score in the middle of the screen as the page arrives, at a width it was not engraved at.
  await page.addInitScript(() => {
    document.addEventListener("DOMContentLoaded", () => {
      const score = [...document.querySelectorAll(".chant-score")].find((e) => e.offsetParent);
      scrollTo(0, score.getBoundingClientRect().top + scrollY - 200);
    });
  });
  const metrics = page.waitForResponse((r) => r.url().includes("eb-garamond-lyrics.nmet") && r.ok());
  await page.goto("/compline/2026-11-13");
  await metrics;
  const score = page.locator(".chant-score:visible").first();
  const height = () => score.evaluate((e) => e.getBoundingClientRect().height);
  const first = await height();
  await page.waitForTimeout(1000);
  expect(await height()).toBe(first);
  // Scrolled past, it is drawn at its column, and the words on the screen stay where they stand.
  await page.evaluate(() => scrollBy(0, 1500));
  await page.evaluate(() => {
    const probe = [...document.querySelectorAll(".elements p")].find((p) => p.offsetParent && p.getBoundingClientRect().top >= 100);
    probe.dataset.probe = "1";
    window.__probeTop = probe.getBoundingClientRect().top;
  });
  await expect.poll(height).not.toBe(first);
  expect(Math.abs(await page.locator("[data-probe]").evaluate((p) => p.getBoundingClientRect().top - window.__probeTop))).toBeLessThan(1);
});

test("changing the text size keeps the line the reader is at", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.removeItem("office-text-size"));
  await openDatedPage(page, "/vespers/2026-11-13");
  await page.evaluate(() => scrollTo(0, 3000));
  const at = () => page.evaluate(() => {
    // The first block on the screen, from the same blocks officeHoldPlace holds: a heading above
    // the first paragraph is the one held, and the paragraph under it may move with the heading.
    const blocks = ".elements p, .elements h2, .elements h3, .elements summary, .elements .chant-score";
    const p = [...document.querySelectorAll(blocks)].find((e) => { const r = e.getBoundingClientRect(); return r.height && r.bottom > 0; });
    p.dataset.held = "1";
    const r = p.getBoundingClientRect();
    return { top: r.top, height: r.height };
  });
  const before = await at();
  // The menu's Text row, tapped where the reader is (a click here would scroll to the header).
  await page.evaluate(() => document.querySelector('.text-size-option[data-text-size-choice="large"]').click());
  await expect(page.locator("html")).toHaveAttribute("data-text-size", "large");
  const after = await page.locator("[data-held]").evaluate((p) => { const r = p.getBoundingClientRect(); return { top: r.top, height: r.height }; });
  expect(after.height).toBeGreaterThan(before.height);
  // The share of the line's block above the top of the screen is the share it was.
  const want = before.top < 0 ? (before.top / before.height) * after.height : before.top;
  expect(Math.abs(after.top - want), JSON.stringify({ before, after })).toBeLessThan(2);
});

test("closing the prayers after the Office at the page's end leaves its summary where it was", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.removeItem("office-prayers-open"));
  await openDatedPage(page, "/lauds/2026-11-13");
  const after = page.locator('details.session-prayers[data-prayers$="/Prayers After the Office"]');
  await expect(after).toHaveAttribute("open", "");
  const summary = after.locator(":scope > summary");
  // The summary near the top of the screen, with less than a screen of page below it.
  await page.evaluate(() => {
    const s = [...document.querySelectorAll("details.session-prayers > summary")].pop();
    scrollTo(0, s.getBoundingClientRect().top + scrollY - 80);
  });
  const top = await summary.evaluate((s) => s.getBoundingClientRect().top);
  await summary.click();
  await expect(after).not.toHaveAttribute("open", "");
  await page.waitForTimeout(400);
  expect(Math.abs((await summary.evaluate((s) => s.getBoundingClientRect().top)) - top)).toBeLessThan(1);
  // Scrolling back up gives the room back, without moving what is on the screen.
  await scrollHourBy(page, -2000);
  await expect.poll(() => page.evaluate(() => document.documentElement.style.minHeight)).toBe("");
});

test("the prayed watch measures the hour's words again only when they move", async ({ page }) => {
  await page.addInitScript(() => {
    window.__measured = 0;
    const rects = Range.prototype.getClientRects;
    Range.prototype.getClientRects = function () { window.__measured++; return rects.call(this); };
  });
  await page.clock.install({ time: new Date("2026-10-07T21:00:00") });
  await page.goto("/compline/2026-10-07");
  await page.clock.runFor(1000);
  const once = await page.evaluate(() => window.__measured);
  expect(once).toBeGreaterThan(100);
  for (let i = 0; i < 3; i++) {
    await scrollHourBy(page, 400);
    await page.clock.runFor(1000);
  }
  expect(await page.evaluate(() => window.__measured)).toBe(once);
  // A new text size moves every word: they are measured again, once (the page's other
  // measures made at its load aside).
  await page.evaluate(() => document.documentElement.setAttribute("data-text-size", "large"));
  await page.clock.runFor(2000);
  const again = (await page.evaluate(() => window.__measured)) - once;
  expect(again).toBeGreaterThan(0.9 * once);
  expect(again).toBeLessThanOrEqual(once);
});

// Home marks the reader's own hour before its first paint, as the theme is
// set: not the server's hour, corrected once the deferred script runs.
test("home marks the device's hour before its script runs", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-03-15T10:00:00-04:00"));
  let release;
  const ready = new Promise((resolve) => { release = resolve; });
  await page.route("**/static/app.js*", async (route) => { await ready; await route.continue(); });
  await page.goto(`/?date=${testDate}`, { waitUntil: "commit" });
  await expect(page.locator(".pray-now")).toHaveText("Pray Terce");
  await expect(page.locator(".pray-now")).toHaveAttribute("href", `/terce/${testDate}`);
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveAttribute("data-hour", "terce");
  await expect(page.locator(".home-hour-link.is-current")).toHaveCount(1);
  release();
  await page.waitForLoadState("load");
  await expect(page.locator(".pray-now")).toHaveText("Pray Terce");
  await page.unroute("**/static/app.js*");
});

// A press is seen: a tap marks the control at once, and one that opens a page
// keeps the mark until the page goes; an ordo row is pressed as one control.
// Reported from the page, which is leaving when the mark matters.
test("a tapped ordo row and a tapped hour show their press at once", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true });
  const page = await context.newPage();
  const seen = [];
  await page.exposeFunction("reportPress", (note) => seen.push(note));
  await page.goto("/calendar/2026/09");
  await page.evaluate(() => {
    new MutationObserver((records) => records.forEach((r) => {
      if (r.target.classList.contains("is-pressed")) window.reportPress(r.target.id || r.target.className);
    })).observe(document.body, { subtree: true, attributes: true, attributeFilter: ["class"] });
  });
  const row = page.locator("#d-2026-09-20");
  await row.scrollIntoViewIfNeeded();
  const box = await row.boundingBox();
  await page.touchscreen.tap(box.x + box.width * 0.7, box.y + box.height - 6);
  await page.waitForURL(/\?date=2026-09-20/);
  expect(seen).toContain("d-2026-09-20");
  // After app.js's own listener: keep the page, and say how the hour stood.
  await page.evaluate(() => document.addEventListener("click", (e) => {
    const link = e.target.closest(".home-hour-link");
    if (!link) return;
    e.preventDefault();
    window.reportPress((link.classList.contains("is-pressed") ? "pressed " : "plain ") + getComputedStyle(link).transitionDuration);
  }));
  seen.length = 0;
  await page.locator(".home-hour-link[data-hour='vespers']").tap();
  await expect.poll(() => seen.find((note) => note.startsWith("pressed") || note.startsWith("plain"))).toMatch(/^pressed 0s/);
  await context.close();
});

test("a link chosen in the open phone menu puts the menu away before the page goes", async ({ page }) => {
  await page.goto(`/vespers/${testDate}`);
  await page.getByText("Menu", { exact: true }).click();
  await expect(page.locator(".site-menu-panel")).toBeVisible();
  const seen = [];
  await page.exposeFunction("reportMenu", (note) => seen.push(note));
  await page.evaluate(() => document.addEventListener("click", () => {
    window.reportMenu(getComputedStyle(document.querySelector(".site-menu-panel")).display);
  }));
  await page.locator('.site-menu nav a[data-hour="lauds"]').click();
  await page.waitForURL(/\/lauds\//);
  expect(seen).toEqual(["none"]);
});

test("the brand on today's home stays put", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".site-brand")).toHaveAttribute("aria-current", "page");
  let navigations = 0;
  page.on("framenavigated", () => { navigations += 1; });
  await page.locator(".site-brand").click();
  await page.waitForTimeout(300);
  expect(navigations).toBe(0);
});

// A day asked for comes to rest with the day before it in view under the
// pinned month heading, and its gold lets go once the reader scrolls.
test("the ordo shows the day before a day asked for, and lets the asked day go", async ({ page }) => {
  for (const [width, day, before] of [[390, "14", "13"], [1280, "22", "21"]]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/calendar/2026/09#d-2026-09-${day}`);
    await page.evaluate(() => document.fonts.ready);
    const place = await page.evaluate(([day, before]) => ({
      heading: document.getElementById(`d-2026-09-${day}`).closest(".month").querySelector("h2").getBoundingClientRect().bottom,
      before: document.getElementById(`d-2026-09-${before}`).getBoundingClientRect().top,
    }), [day, before]);
    expect(place.before, `${width}px`).toBeGreaterThanOrEqual(place.heading - 2);
    const ground = () => page.locator(`#d-2026-09-${day} > td`).last().evaluate((td) => getComputedStyle(td).backgroundColor);
    expect(await ground()).not.toBe("rgba(0, 0, 0, 0)");
    // The reader's own scroll (mobile WebKit has no wheel to send).
    await page.evaluate(() => { window.dispatchEvent(new WheelEvent("wheel", { deltaY: 240 })); window.scrollBy(0, 240); });
    await expect.poll(ground).toBe("rgba(0, 0, 0, 0)");
  }
});
