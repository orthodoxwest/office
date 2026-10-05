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
// close it again so later measurements see the page.
async function choosePreference(page, name) {
  const wide = await page.locator(".site-settings > summary").isVisible();
  const disclosure = page.locator(wide ? ".site-settings" : ".site-menu");
  const summary = disclosure.locator(":scope > summary");
  await summary.click();
  await page.locator(wide ? ".settings-prefs" : ".menu-prefs").getByRole("button", { name, exact: true }).click();
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
  await page.locator("footer").click();
  await expect(menu).not.toHaveAttribute("open", "");
  await page.getByText("Menu", { exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(menu).not.toHaveAttribute("open", "");
  await expect(page.locator(".site-menu > summary")).toBeFocused();

  // Home's colophon sits at the foot of the screen, as on desktop, with the
  // card above it rather than the colophon floating under the card.
  const [card, footer] = await Promise.all([
    page.locator(".home").boundingBox(),
    page.locator("footer").boundingBox(),
  ]);
  expect(footer.y).toBeGreaterThanOrEqual(card.y + card.height);
  const foot = await page.evaluate(
    () => innerHeight - parseFloat(getComputedStyle(document.body).paddingBottom),
  );
  expect(Math.abs(footer.y + footer.height - foot)).toBeLessThan(2);

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
        // date + go-to-today (historical days) + pray + 7 hours + change-date.
        .slice(0, 11)
        .map((element) => {
          if (element.matches(".home-date-link")) return "date";
          // Recovery chrome when the landing day is not local today — intentional
          // after the stale-day fix; sits in the day identity, before Pray now.
          if (element.matches(".not-today-link")) return "go-to-today";
          if (element.matches(".pray-now")) return "pray";
          if (element.matches(".home-hour-link")) return element.getAttribute("data-hour");
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
    "go-to-today",
    "pray",
    "lauds",
    "prime",
    "terce",
    "sext",
    "none",
    "vespers",
    "compline",
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
  await expect(current.getByText("Now", { exact: true })).not.toBeInViewport();
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

  await page.clock.fastForward("02:00");

  await expect(page.locator(".pray-now")).toHaveText("Pray Sext");
  await expect(page.locator('.home-hour-link[aria-current="time"]')).toHaveAttribute(
    "data-hour",
    "sext",
  );
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

test("parish material stays off the mobile prayer page", async ({
  page,
}) => {
  await openDatedPage(page, `/?date=${testDate}`);

  // The limewash wall is one fixed layer on html::before, shared by Nave and
  // Apse and coloured by their tokens.
  const wall = () =>
    page.evaluate(() => {
      const field = getComputedStyle(document.documentElement, "::before");
      return { image: field.backgroundImage, position: field.position, content: field.content };
    });
  const naveWall = await wall();
  expect(naveWall.image).toMatch(/plaster(-wide)?\.jpg/);
  expect(naveWall.position).toBe("fixed");
  const naveMaterial = await page.evaluate(() => ({
    page: getComputedStyle(document.body).backgroundImage,
    inscriptionBand: getComputedStyle(
      document.querySelector(".home-hour-group-label"),
    ).backgroundColor,
  }));
  expect(naveMaterial.inscriptionBand).not.toBe("rgba(0, 0, 0, 0)");

  await choosePreference(page, "Apse");
  // The Apse vault is a background-image, which can only snap rather than
  // crossfade, so app.js dips it invisible and applies the theme (and swaps
  // this image) only once that dip completes — poll instead of reading the
  // pre-swap Nave material on a fast single-worker CI run.
  const readMaterial = () => page.evaluate(() => getComputedStyle(document.documentElement).colorScheme);
  await expect.poll(readMaterial).toBe("dark");
  expect((await wall()).image).toMatch(/plaster(-wide)?\.jpg/);

  await page.goto(`/lauds/${testDate}`);
  const prayerMaterial = await page.evaluate(() => ({
    page: getComputedStyle(document.body).backgroundImage,
    prayer: getComputedStyle(document.querySelector(".elements")).backgroundImage,
    wall: getComputedStyle(document.documentElement, "::before").content,
  }));
  expect(prayerMaterial).toEqual({ page: "none", prayer: "none", wall: "none" });

  // Desktop. The theme persists in localStorage, so each half sets its own
  // explicitly rather than inheriting whatever the previous step left behind.
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/?date=${testDate}`);

  // The wall has no composition to become a spotlight on a wide canvas, so
  // desktop Nave keeps it too; the body itself paints nothing over it.
  await choosePreference(page, "Nave");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  expect(await page.evaluate(() => getComputedStyle(document.body).backgroundImage)).toBe(
    "none",
  );
  expect((await wall()).image).toMatch(/plaster(-wide)?\.jpg/);

  // Apse adds the vault over the wall.
  await choosePreference(page, "Apse");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const readVault = () =>
    page.evaluate(() => {
      const style = getComputedStyle(document.body, "::before");
      return { mask: style.maskImage || style.webkitMaskImage, ink: style.backgroundColor };
    });
  // One tile of hand-set stars as a mask, intersected with the page fade,
  // painted in the gilding. The ink crossfades in with the theme, so poll
  // past its transparent start.
  await expect.poll(async () => vaultPaints(await readVault())).toBe(true);
  const vault = await readVault();
  expect((vault.mask.match(/ornaments\/vault\.svg/g) || []).length).toBe(1);
  expect((vault.mask.match(/linear-gradient/g) || []).length).toBe(1);
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

test("a tablet's hour header sets the hours as one rank, with no link stranded", async ({ page }) => {
  for (const size of ["default", "large"]) {
    for (const width of [701, 820, 959, 960, 1100]) {
      await page.setViewportSize({ width, height: 900 });
      await page.addInitScript((s) => localStorage.setItem("office-text-size", s), size);
      await openDatedPage(page, `/vespers/${testDate}`);
      const header = await page.evaluate(() => {
        const links = [...document.querySelectorAll(".site-menu nav a")];
        return {
          rows: new Set(links.map((a) => Math.round(a.getBoundingClientRect().top))).size,
          overflow: document.documentElement.scrollWidth - innerWidth,
        };
      });
      expect(header.rows, `${width}px ${size} text`).toBe(1);
      expect(header.overflow, `${width}px ${size} text`).toBe(0);
    }
  }
});

test("wide hour plaster softens the prayer without sideways scroll or stretching", async ({ page }) => {
  // The prayer's field once reached 10rem past the column; with Large text
  // at the 1000px threshold that ran past the viewport.
  for (const size of ["small", "default", "large"]) {
    for (const width of [1000, 1280]) {
      await page.setViewportSize({ width, height: 900 });
      await page.addInitScript((s) => localStorage.setItem("office-text-size", s), size);
      await openDatedPage(page, `/lauds/${testDate}`);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth);
      expect(overflow, `${width}px ${size} text`).toBe(0);
    }
  }
  // Landscape desktops take the wide crop, cover-fitted, never a distorting
  // 100% 100%; phones keep the portrait field.
  const wall = () =>
    page.evaluate(() => {
      const field = getComputedStyle(document.documentElement, "::before");
      return { image: field.backgroundImage, size: field.backgroundSize };
    });
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/?date=${testDate}`);
  const desktop = await wall();
  expect(desktop.image).toContain("plaster-wide.jpg");
  expect(desktop.size.split(", ").every((layer) => layer === "cover")).toBe(true);
  await page.setViewportSize({ width: 390, height: 844 });
  await openDatedPage(page, `/?date=${testDate}`);
  expect((await wall()).image).toMatch(/plaster\.jpg/);

  // The prayer's band is the same wall with the softened copy in place of
  // the field and one more veil on top, cover-fitted like the wall so its
  // clouds lie where the wall's do. Phones keep a flat page.
  const band = () =>
    page.evaluate(() => {
      const wall = getComputedStyle(document.documentElement, "::before");
      const band = getComputedStyle(document.body, "::before");
      return {
        content: band.content,
        position: band.position,
        wall: wall.backgroundImage,
        image: band.backgroundImage,
        blend: band.backgroundBlendMode,
        wallBlend: wall.backgroundBlendMode,
        size: band.backgroundSize,
        mask: band.maskImage || band.webkitMaskImage,
      };
    });
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/lauds/${testDate}`);
  const wide = await band();
  expect(wide.content).toBe('""');
  expect(wide.position).toBe("fixed");
  expect(wide.image).toContain("plaster-wide-soft.jpg");
  // Each file carries its own content stamp, so compare the layers unstamped.
  const unstamped = (layers) => layers.replace(/\?v=[0-9a-f]+/g, "");
  expect(unstamped(wide.image).endsWith(unstamped(wide.wall).replace("plaster-wide.jpg", "plaster-wide-soft.jpg"))).toBe(true);
  expect(wide.blend).toBe(`normal, ${wide.wallBlend}`);
  expect(wide.size.split(", ").every((layer) => layer === "cover")).toBe(true);
  expect(wide.mask).toContain("linear-gradient");
  await page.setViewportSize({ width: 390, height: 844 });
  await openDatedPage(page, `/lauds/${testDate}`);
  expect((await band()).content).toBe("none");
});

// The rendered page with its content hidden: its mean colour, that mean's
// worst channel drift from --bg, and the luminance contrast in percent.
// `clip` narrows the sample to one region of the viewport; `block` measures
// contrast over square block means, which keeps clouds but drops the 8-bit
// rounding that otherwise swamps Apse's sub-level variation.
async function sampleWall(page, { clip, block = 1 } = {}) {
  const png = (await page.screenshot(clip ? { clip } : {})).toString("base64");
  return page.evaluate(async ({ b64, block }) => {
    const blob = await (await fetch(`data:image/png;base64,${b64}`)).blob();
    const bitmap = await createImageBitmap(blob);
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
    const context = canvas.getContext("2d");
    context.drawImage(bitmap, 0, 0);
    const { width, height } = bitmap;
    const data = context.getImageData(0, 0, width, height).data;
    const sums = [0, 0, 0];
    const columns = Math.floor(width / block);
    const rows = Math.floor(height / block);
    const blocks = new Float64Array(columns * rows);
    for (let y = 0; y < rows * block; y++) {
      for (let x = 0; x < columns * block; x++) {
        const i = 4 * (y * width + x);
        sums[0] += data[i];
        sums[1] += data[i + 1];
        sums[2] += data[i + 2];
        blocks[Math.floor(y / block) * columns + Math.floor(x / block)] +=
          0.2126 * data[i] + 0.7152 * data[i + 1] + 0.0722 * data[i + 2];
      }
    }
    const n = rows * block * columns * block;
    let luminance = 0;
    let luminance2 = 0;
    for (const sum of blocks) {
      const l = sum / (block * block);
      luminance += l;
      luminance2 += l * l;
    }
    const mean = luminance / blocks.length;
    const bg = getComputedStyle(document.documentElement).backgroundColor.match(/\d+/g).map(Number);
    return {
      mean: sums.map((sum) => sum / n),
      drift: Math.max(...sums.map((sum, i) => Math.abs(sum / n - bg[i]))),
      contrast: (100 * Math.sqrt(luminance2 / blocks.length - mean * mean)) / mean,
    };
  }, { b64: png, block });
}

for (const [theme, minContrast] of [["light", 0.84], ["dark", 1.3]]) {
  test(`the ${theme} wall is visible yet averages the page colour`, async ({ page }) => {
    // Nave once rendered at 0.46% luminance contrast: mean-matched, and
    // invisible. Sample the bare wall (content hidden) and hold both ends:
    // visible, never loud, and averaging --bg so flat fields show no edge.
    for (const width of [390, 1280]) {
      await page.setViewportSize({ width, height: 900 });
      // Home, not the ordo: the wall is one fixed layer on every page, and
      // home loads in a fraction of the ordo's time.
      await openDatedPage(page, `/?date=${testDate}`, theme);
      // body::after is the home niche's room light, not the wall.
      await page.addStyleTag({ content: "body > * { visibility: hidden !important; } body::before, body::after { display: none !important; }" });
      const wall = await sampleWall(page);
      expect(wall.drift, `${width}px mean vs --bg`).toBeLessThan(1.5);
      expect(wall.contrast, `${width}px contrast %`).toBeGreaterThan(minContrast);
      expect(wall.contrast, `${width}px contrast %`).toBeLessThan(5);
    }
  });
}

test("the Nave prayer band is quieter than the wall but keeps its tone", async ({ page }) => {
  // The band's softened copy must hold the wall's mean where it covers it,
  // or its feathered edges show as a lighter or darker column; and it must
  // stay quieter than the wall, or it is no longer a band.
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/lauds/${testDate}`, "light");
  await page.addStyleTag({ content: "body > * { visibility: hidden !important; }" });
  // The band's solid middle, then the same region of the bare wall.
  const middle = { clip: { x: 440, y: 0, width: 400, height: 900 }, block: 4 };
  const band = await sampleWall(page, middle);
  await page.addStyleTag({ content: "body::before { display: none !important; }" });
  const wall = await sampleWall(page, middle);
  band.mean.forEach((channel, i) => expect(Math.abs(channel - wall.mean[i]), "band vs wall mean").toBeLessThan(1));
  expect(band.contrast, "band contrast %").toBeLessThan(wall.contrast * 0.6);
  expect(band.contrast, "band contrast %").toBeGreaterThan(0.1);

  // Apse's wall is already quiet, and a feathered layer over so smooth a
  // dark field bands in software rendering; it has no band, by choice or
  // by default.
  await openDatedPage(page, `/lauds/${testDate}`, "dark");
  expect(await page.evaluate(() => getComputedStyle(document.body, "::before").content), "Apse").toBe("none");
  await page.emulateMedia({ colorScheme: "dark" });
  await openDatedPage(page, `/lauds/${testDate}`, "default");
  expect(await page.evaluate(() => getComputedStyle(document.body, "::before").content), "default dark").toBe("none");
});

// The strongest one-pixel stripe in the rendered page: column means high-passed
// along x and row means along y, each as the worst windowed deviation. A
// feathered flat layer over a smooth wall shows here as software rasterizers
// step its fade in one-level columns or rows.
async function stripes(page) {
  const png = (await page.screenshot()).toString("base64");
  return page.evaluate(async (b64) => {
    const bitmap = await createImageBitmap(await (await fetch(`data:image/png;base64,${b64}`)).blob());
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
    const context = canvas.getContext("2d");
    context.drawImage(bitmap, 0, 0);
    const { width, height } = bitmap;
    const data = context.getImageData(0, 0, width, height).data;
    // Each channel apart: the Apse steps fall in blue as often as not.
    const columns = [0, 1, 2].map(() => new Float64Array(width));
    const rows = [0, 1, 2].map(() => new Float64Array(height));
    for (let y = 0; y < height; y++) {
      for (let x = 0; x < width; x++) {
        for (let c = 0; c < 3; c++) {
          const v = data[4 * (y * width + x) + c];
          columns[c][x] += v / height;
          rows[c][y] += v / width;
        }
      }
    }
    const worst = (profile) => {
      const pass = profile.map((v, i) => {
        let sum = 0;
        let count = 0;
        for (let j = Math.max(0, i - 4); j <= Math.min(profile.length - 1, i + 4); j++, count++) sum += profile[j];
        return v - sum / count;
      });
      let max = 0;
      for (let start = 8; start + 160 <= pass.length - 8; start += 40) {
        const window = pass.slice(start, start + 160);
        const mean = window.reduce((a, b) => a + b) / window.length;
        max = Math.max(max, Math.sqrt(window.reduce((a, b) => a + (b - mean) ** 2, 0) / window.length));
      }
      return max;
    };
    return { columns: Math.max(...columns.map(worst)), rows: Math.max(...rows.map(worst)) };
  }, png);
}

for (const theme of ["light", "dark"]) {
  test(`the ${theme} wide hour's layers leave no stripes where they fade`, async ({ page }) => {
    // The end of Lauds on a wide screen gathers every layer above the wall:
    // the prayer band, the side field and the ending's vault. Hide the
    // text, the stars and the footer's painted tailpiece (which are meant
    // to show), then hold the rest to the bare wall's own grain.
    await page.setViewportSize({ width: 1920, height: 1080 });
    await openDatedPage(page, `/lauds/${testDate}`, theme);
    await page.addStyleTag({
      content: `.elements, .hour-header, .hour-epilogue > *, footer > *, footer::before, header, .hour-scroll-progress { visibility: hidden !important; }
        .hour-epilogue::before, footer::after, .office-hour::before, .office-hour::after { display: none !important; }`,
    });
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    const layered = await stripes(page);
    await page.addStyleTag({
      content: "body, body *, body::before, body::after, body *::before, body *::after { visibility: hidden !important; }",
    });
    const wall = await stripes(page);
    expect(layered.columns, "vertical stripes").toBeLessThan(wall.columns * 1.5 + 0.02);
    expect(layered.rows, "horizontal stripes").toBeLessThan(wall.rows * 1.5 + 0.02);
  });
}

test("thresholds and usage share the hour wall in both themes", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const reference = {};
  for (const path of [`/lauds/${testDate}`, `/?date=${testDate}`, "/calendar/2026/03", "/reminders", "/admin/usage?days=7"]) {
    await page.goto(path);
    for (const theme of ["light", "dark"]) {
      const material = await page.evaluate((theme) => {
        document.documentElement.dataset.theme = theme;
        const wall = getComputedStyle(document.documentElement, "::before");
        const heading = document.querySelector(".month h2");
        const signature = (style) => [style.backgroundImage, style.backgroundBlendMode, style.backgroundSize, style.opacity];
        return {
          wall: signature(wall),
          heading: heading ? signature(getComputedStyle(heading)) : null,
          content: wall.content,
          body: getComputedStyle(document.body).backgroundColor,
        };
      }, theme);
      expect(material.content, path).toBe('""');
      expect(material.body, path).toBe("rgba(0, 0, 0, 0)");
      reference[theme] ??= material.wall;
      expect(material.wall, `${path} ${theme}`).toEqual(reference[theme]);
      if (material.heading) expect(material.heading, theme).toEqual(reference[theme]);
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

test("the apse vault appears only over the night, and veils with the season", async ({
  page,
}) => {
  const vault = async ({ width, theme, scheme, path }) => {
    const context = await page.context().browser().newContext({
      viewport: { width, height: 900 },
      colorScheme: scheme,
      isMobile: width < 700,
    });
    const sheet = await context.newPage();
    if (theme) await sheet.addInitScript((t) => localStorage.setItem("office-theme", t), theme);
    await sheet.goto(path);
    const read = await sheet.evaluate(() => {
      const style = getComputedStyle(document.body, "::before");
      const rgb = getComputedStyle(document.documentElement)
        .backgroundColor.match(/\d+/g)
        .slice(0, 3)
        .map(Number);
      return {
        // The vault has its own layer, so this cannot pick up
        // --page-material's broad radials, which live on body and legitimately
        // remain on the phone.
        vault: { mask: style.maskImage || style.webkitMaskImage, ink: style.backgroundColor },
        pageIsDark: rgb.reduce((a, b) => a + b, 0) / 3 < 100,
      };
    });
    await context.close();
    return { stars: vaultPaints(read.vault), pageIsDark: read.pageIsDark, ink: read.vault.ink };
  };

  const home = `/?date=${testDate}`;
  // Gold stars must never land on plaster. The Default-theme/light-device case
  // is the one that bites: :root:not([data-theme="light"]) matches when no
  // choice has been stored, so without a prefers-color-scheme guard the vault
  // would light up over the Nave.
  for (const scheme of ["light", "dark"]) {
    const seen = await vault({ width: 1280, theme: null, scheme, path: home });
    if (!seen.pageIsDark) expect(seen.stars).toBe(false);
  }
  expect((await vault({ width: 1280, theme: "light", scheme: "dark", path: home })).stars).toBe(false);

  // Present behind the Apse home at every width, absent in working rooms.
  expect((await vault({ width: 1280, theme: "dark", scheme: "dark", path: home })).stars).toBe(true);
  expect((await vault({ width: 390, theme: "dark", scheme: "dark", path: home })).stars).toBe(true);
  for (const path of ["/calendar/2026/03", "/calendar/2026", "/reminders"]) {
    expect((await vault({ width: 1280, theme: "dark", scheme: "dark", path })).stars).toBe(false);
  }

  // Gold leaf is gilding, so the vault keeps the season. This only holds while
  // --apse-ink is declared where the seasonal --ornament lands; hoisting it
  // to :root freezes the stars gold through Passiontide.
  const ink = {};
  for (const [season, date] of [
    ["ordinary", testDate],
    ["passiontide", "2026-04-08"],
    ["eastertide", "2026-04-20"],
  ]) {
    ink[season] = (
      await vault({ width: 1280, theme: "dark", scheme: "dark", path: `/?date=${date}` })
    ).ink;
  }
  expect(ink.ordinary).toBeTruthy();
  expect(ink.passiontide).not.toBe(ink.ordinary);
  expect(ink.eastertide).not.toBe(ink.ordinary);
  expect(ink.eastertide).not.toBe(ink.passiontide);

  // The stars must actually light pixels. Declaring the layers is not enough:
  // `circle 0.6px` is a *radius*, so with the colour stop at 50% the solid core
  // was 0.3px and antialiasing ate nearly all of it — 58 declared stars lit 13
  // pixels across half the viewport and the vault was invisible. Measured on
  // the clean field beside the frontispiece, where nothing else paints.
  const context = await page.context().browser().newContext({
    viewport: { width: 1280, height: 900 },
    colorScheme: "dark",
    deviceScaleFactor: 1,
  });
  const sheet = await context.newPage();
  await sheet.addInitScript(() => localStorage.setItem("office-theme", "dark"));
  await sheet.goto(home);
  const cardLeft = await sheet.evaluate(() =>
    Math.round(document.querySelector(".home-hero").getBoundingClientRect().left),
  );
  const shot = await sheet.screenshot({
    clip: { x: 10, y: 110, width: cardLeft - 30, height: 500 },
  });
  const lit = await sheet.evaluate(async (data) => {
    const img = new Image();
    img.src = "data:image/png;base64," + data;
    await img.decode();
    const canvas = document.createElement("canvas");
    canvas.width = img.width;
    canvas.height = img.height;
    const ctx2d = canvas.getContext("2d");
    ctx2d.drawImage(img, 0, 0);
    const px = ctx2d.getImageData(0, 0, canvas.width, canvas.height).data;
    const counts = {};
    for (let i = 0; i < px.length; i += 4) {
      const key = `${px[i]},${px[i + 1]},${px[i + 2]}`;
      counts[key] = (counts[key] || 0) + 1;
    }
    const bg = Object.entries(counts)
      .sort((a, b) => b[1] - a[1])[0][0]
      .split(",")
      .map(Number);
    let n = 0;
    for (let i = 0; i < px.length; i += 4) {
      const dev =
        Math.abs(px[i] - bg[0]) + Math.abs(px[i + 1] - bg[1]) + Math.abs(px[i + 2] - bg[2]);
      if (dev > 20) n += 1;
    }
    return n;
  }, shot.toString("base64"));
  await context.close();
  expect(lit).toBeGreaterThan(30);
});

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
    // need: clamp(38rem, 10rem + 38vw, 48rem), scaled by the niche's zoom
    // step (1.12 from 1700x1000).
    const zoom = width >= 1700 ? 1.12 : 1;
    const declared = Math.min(48 * 16, Math.max(38 * 16, 10 * 16 + 0.38 * width)) * zoom;
    expect(Math.abs(widths[0] - declared)).toBeLessThanOrEqual(1);
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

function isBottomCenterLayer(p) {
  return (
    p === "50% 100%" ||
    p === "50% 100" ||
    p === "center bottom" ||
    p === "bottom center" ||
    p === "50% bottom" ||
    p === "center 100%" ||
    p === "center 100"
  );
}

function isTopCenterPhase(phase) {
  const layers = phaseLayers(phase);
  return layers.length > 0 && layers.every(isTopCenterLayer);
}

function isBottomCenterPhase(phase) {
  const layers = phaseLayers(phase);
  return layers.length > 0 && layers.every(isBottomCenterLayer);
}

function tileEdgePx(size) {
  // Take the first layer; all vault layers share one tile.
  const first = String(size).split(",")[0].trim();
  const m = first.match(/([\d.]+)px(?:\s+([\d.]+)px)?/);
  if (!m) return null;
  return { w: parseFloat(m[1]), h: parseFloat(m[2] || m[1]) };
}

function nearViewportEdge(value, expected, tol = 2) {
  return Math.abs(value - expected) <= tol;
}

test("the mobile home vault is one stable full-page layer without scroll", async ({
  page,
}) => {
  const read = async ({ width = 390, height, theme, scheme, asToday = false }) => {
    const context = await page.context().browser().newContext({
      viewport: { width, height },
      isMobile: true,
      hasTouch: true,
      colorScheme: scheme,
    });
    const sheet = await context.newPage();
    if (theme) await sheet.addInitScript((t) => localStorage.setItem("office-theme", t), theme);
    await sheet.goto(`/?date=${testDate}`);
    const seen = await sheet.evaluate((asToday) => {
      // The fixed test day is historical, so the server adds the recovery
      // line that local today's home never carries; drop it to measure the
      // layout a reader sees on the day itself.
      if (asToday) document.querySelectorAll(".not-today-notice").forEach((notice) => notice.remove());
      const field = getComputedStyle(document.body, "::before");
      const footerElement = document.querySelector("footer");
      const tailpiece = getComputedStyle(footerElement, "::before");
      // The pointed head's halo is a filter on its outer courses.
      const halo = getComputedStyle(document.querySelector(".home-arch"));
      return {
        stars: { mask: field.maskImage || field.webkitMaskImage, ink: field.backgroundColor },
        position: field.position,
        tileSize: field.maskSize || field.webkitMaskSize,
        phase: field.maskPosition || field.webkitMaskPosition,
        // The footer closes on its painted tailpiece in both themes, with
        // the same geometry, so the footer never moves when the theme
        // changes.
        tailpiece: tailpiece.maskImage || tailpiece.webkitMaskImage,
        tailpieceShown: tailpiece.display !== "none",
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
  expect(apse.tailpieceShown).toBe(true);
  expect(apse.tailpiece).toContain("ornaments/tailpiece.svg");
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
    expect(nave.tailpieceShown).toBe(true);
    expect(nave.scrolls).toBe(false);
    expect(nave.scrollHeight).toBe(apse.scrollHeight);
  }

  // On the day itself a phone's home fits its viewport whole, every hour in
  // reach without scrolling, on a 375x667 screen as well.
  const small = await read({ width: 375, height: 667, theme: "light", scheme: "light", asToday: true });
  expect(small.scrolls).toBe(false);

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

test("the hour vault begins after prayer, spans the footer, and does not move it", async ({ page }) => {
  const read = async (theme, width = 390) => {
    const context = await page.context().browser().newContext({
      viewport: { width, height: 844 },
      isMobile: width <= 700,
      hasTouch: width <= 700,
      colorScheme: theme,
    });
    const sheet = await context.newPage();
    await sheet.addInitScript((storedTheme) => localStorage.setItem("office-theme", storedTheme), theme);
    await sheet.goto(`/lauds/${testDate}`);
    const seen = await sheet.evaluate(() => {
      const main = document.querySelector("main");
      const prayer = document.querySelector(".elements");
      const epilogue = document.querySelector(".hour-epilogue");
      const footerElement = document.querySelector("footer");
      const field = getComputedStyle(epilogue, "::before");
      const footerField = getComputedStyle(footerElement, "::after");
      const tailpiece = getComputedStyle(footerElement, "::before");
      const mainBox = main.getBoundingClientRect();
      const prayerBox = prayer.getBoundingClientRect();
      const epilogueBox = epilogue.getBoundingClientRect();
      const footerBox = footerElement.getBoundingClientRect();
      return {
        pageClass: document.body.classList.contains("page-hour"),
        prayerField: getComputedStyle(prayer).backgroundImage,
        fieldLayers: { mask: field.maskImage || field.webkitMaskImage, ink: field.backgroundColor },
        footerLayers: { mask: footerField.maskImage || footerField.webkitMaskImage, ink: footerField.backgroundColor },
        startsAfterPrayer: epilogueBox.top >= prayerBox.bottom,
        endsWithMain: Math.abs(epilogueBox.bottom - mainBox.bottom) < 0.5,
        joinsFooter:
          Math.abs(footerBox.top + parseFloat(footerField.top) - epilogueBox.bottom) < 0.5,
        fieldEdges: [
          Math.round(epilogueBox.left + parseFloat(field.left)),
          Math.round(epilogueBox.right - parseFloat(field.right)),
        ],
        footerEdges: [
          Math.round(footerBox.left + parseFloat(footerField.left)),
          Math.round(footerBox.right - parseFloat(footerField.right)),
        ],
        horizontalOverflow: document.documentElement.scrollWidth > window.innerWidth + 1,
        phase: [
          field.maskPosition || field.webkitMaskPosition,
          footerField.maskPosition || footerField.webkitMaskPosition,
        ],
        tailpiece: tailpiece.display === "none" ? "" : tailpiece.maskImage || tailpiece.webkitMaskImage,
        endMark: getComputedStyle(epilogue.querySelector(".hour-end-mark")).maskImage || "",
      };
    });
    await context.close();
    return seen;
  };

  const apse = await read("dark");
  expect(apse.pageClass).toBe(true);
  expect(apse.prayerField).toBe("none");
  expect(vaultPaints(apse.fieldLayers)).toBe(true);
  expect(vaultPaints(apse.footerLayers)).toBe(true);
  expect(apse.startsAfterPrayer).toBe(true);
  expect(apse.endsWithMain).toBe(true);
  expect(apse.joinsFooter).toBe(true);
  // 50vw full-bleed can be a hair off with classic scrollbars; allow 2px.
  expect(nearViewportEdge(apse.fieldEdges[0], 0)).toBe(true);
  expect(nearViewportEdge(apse.fieldEdges[1], 390)).toBe(true);
  expect(nearViewportEdge(apse.footerEdges[0], 0)).toBe(true);
  expect(nearViewportEdge(apse.footerEdges[1], 390)).toBe(true);
  expect(apse.horizontalOverflow).toBe(false);
  expect(isBottomCenterPhase(apse.phase[0])).toBe(true);
  expect(isTopCenterPhase(apse.phase[1])).toBe(true);
  // The hour ends on one mark, the consecration cross, in both themes; the
  // footer closes on the same painted tailpiece in both, so its height
  // never moves when Nave/Apse toggles.
  expect(apse.endMark).toContain("ornaments/consecration.svg");
  expect(apse.tailpiece).toContain("ornaments/tailpiece.svg");
  expect(powderPaints(apse.fieldLayers)).toBe(false);

  // By day the same fields after the prayer carry the Nave's powdering
  // instead of the vault, on the same geometry.
  const nave = await read("light");
  expect(nave.prayerField).toBe("none");
  expect(vaultPaints(nave.fieldLayers)).toBe(false);
  expect(vaultPaints(nave.footerLayers)).toBe(false);
  expect(powderPaints(nave.fieldLayers)).toBe(true);
  expect(powderPaints(nave.footerLayers)).toBe(true);
  expect(nave.startsAfterPrayer).toBe(true);
  expect(nave.joinsFooter).toBe(true);
  expect(nave.endMark).toContain("ornaments/consecration.svg");
  expect(nave.tailpiece).toContain("ornaments/tailpiece.svg");

  const desktop = await read("dark", 1280);
  expect(desktop.prayerField).toBe("none");
  expect(nearViewportEdge(desktop.fieldEdges[0], 0)).toBe(true);
  expect(nearViewportEdge(desktop.fieldEdges[1], 1280)).toBe(true);
  expect(nearViewportEdge(desktop.footerEdges[0], 0)).toBe(true);
  expect(nearViewportEdge(desktop.footerEdges[1], 1280)).toBe(true);
  expect(desktop.joinsFooter).toBe(true);
  expect(isBottomCenterPhase(desktop.phase[0])).toBe(true);
  expect(isTopCenterPhase(desktop.phase[1])).toBe(true);
  expect(desktop.horizontalOverflow).toBe(false);
});

test("wide hours set a still vault beside the prayer that comes down to meet the ending", async ({ page }) => {
  // From 1680px the Apse vault stands in the margins while the office is
  // said: fixed, so nothing moves at the edge of sight, and faded in only
  // after the opening. The ending's field is a second fixed layer on the
  // same phase, so the sides stay and run straight down into it.
  const fields = () =>
    page.evaluate(() => {
      const read = (element, pseudo) => {
        const field = getComputedStyle(element, pseudo);
        return {
          content: field.content,
          position: field.position,
          opacity: Number(field.opacity),
          mask: field.maskImage || field.webkitMaskImage,
          size: field.maskSize || field.webkitMaskSize,
          phase: field.maskPosition || field.webkitMaskPosition,
          ink: field.backgroundColor,
        };
      };
      const hour = document.querySelector(".office-hour");
      return {
        sides: read(hour, "::after"),
        ending: read(hour, "::before"),
        epilogue: read(document.querySelector(".hour-epilogue"), "::before"),
        footer: read(document.querySelector("footer"), "::after"),
      };
    });
  await page.setViewportSize({ width: 1920, height: 1080 });
  await openDatedPage(page, `/lauds/${testDate}`, "dark");
  const top = await fields();
  for (const field of [top.sides, top.ending]) {
    expect(field.position).toBe("fixed");
    expect(vaultPaints(field)).toBe(true);
    // From 1800px the vault widens, and every star takes its own share of
    // the leaf.
    expect(field.size.startsWith("832px 832px")).toBe(true);
    expect(field.mask).toContain("leaf.png");
    expect(field.opacity).toBe(0);
  }
  // One phase for both, so the lattices meet star to star.
  expect(top.ending.size).toBe(top.sides.size);
  expect(top.ending.phase).toBe(top.sides.phase);
  // The scrolling fields would meet the fixed ones mid-lattice.
  expect(top.epilogue.content).toBe("none");
  expect(top.footer.content).toBe("none");
  await page.evaluate(() => window.scrollTo(0, innerHeight));
  await expect.poll(async () => (await fields()).sides.opacity).toBe(1);
  expect((await fields()).ending.opacity).toBe(0);
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect.poll(async () => (await fields()).ending.opacity).toBe(1);
  expect((await fields()).sides.opacity).toBe(1);

  // At rest the fixed lattice takes the phase the scrolling fields took
  // from the seam between epilogue and footer, at every text size.
  for (const size of ["default", "large"]) {
    await page.addInitScript((choice) => localStorage.setItem("office-text-size", choice), size);
    await openDatedPage(page, `/lauds/${testDate}`, "dark");
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    const seam = await page.evaluate(() => ({
      phase: getComputedStyle(document.querySelector(".office-hour"), "::before").maskPosition,
      height: innerHeight,
      epilogue: document.querySelector(".hour-epilogue").getBoundingClientRect().bottom,
    }));
    // The tile's offset from the foot, as keywords ("left 50% bottom
    // 211px") or as the spec's offsets from the top ("50% calc(100% -
    // 211px)"), whichever this engine serializes.
    const tile = phaseLayers(seam.phase)[0];
    const offset =
      tile.match(/bottom (-?[\d.]+)px$/) ||
      tile.match(/calc\(100% - (-?[\d.]+)px\)$/) ||
      tile.match(/calc\(-(-?[\d.]+)px \+ 100%\)$/);
    expect(offset, `${size} tile phase ${tile}`).not.toBeNull();
    expect(Math.abs(seam.height - Number(offset[1]) - seam.epilogue), `${size} seam`).toBeLessThan(2);
  }

  // Nave has no vault; narrower screens keep plain margins and the
  // scrolling ending.
  await openDatedPage(page, `/lauds/${testDate}`, "light");
  const nave = await fields();
  expect(vaultPaints(nave.sides)).toBe(false);
  expect(vaultPaints(nave.ending)).toBe(false);
  await page.setViewportSize({ width: 1600, height: 1000 });
  await openDatedPage(page, `/lauds/${testDate}`, "dark");
  const narrower = await fields();
  expect(narrower.sides.content).toBe("none");
  expect(narrower.ending.content).toBe("none");
  expect(vaultPaints(narrower.epilogue)).toBe(true);
  expect(vaultPaints(narrower.footer)).toBe(true);
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

test("the header beam holds one line and one geometry on every page", async ({ page }) => {
  // The nav used to inherit the 46rem prose column, which fits the brand and
  // nine links only if "Reminders" wraps — but the ordo widens to 62rem, so
  // the same header sat on two lines everywhere except there. Both the single
  // line and the shared shell width are the point.
  const shellWidths = [];
  for (const [width, path] of [
    [1024, `/?date=${testDate}`],
    [1280, `/?date=${testDate}`],
    [1280, `/lauds/${testDate}`],
    [1280, "/calendar/2026/03"],
    [1600, "/calendar/2026"],
  ]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(path);
    const header = await page.evaluate(() => {
      const links = [...document.querySelectorAll('nav[aria-label="Primary"] a')];
      return {
        links: links.length,
        rows: new Set(links.map((a) => Math.round(a.getBoundingClientRect().top))).size,
        shell: Math.round(document.querySelector(".site-nav-shell").getBoundingClientRect().width),
      };
    });
    // Hour pages carry the seven hours; elsewhere only Ordo and Reminders.
    expect(header.links).toBe(path.startsWith("/lauds") ? 9 : 2);
    expect(header.rows).toBe(1);
    shellWidths.push(`${width}:${header.shell}`);
  }
  // Same viewport ⇒ same header, whichever room you are in.
  expect(shellWidths[1]).toBe(shellWidths[2]);
  expect(shellWidths[2]).toBe(shellWidths[3]);
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
  // Leaf lettering is gilding, so it veils in Passiontide and warms in
  // Paschaltide like the ✦ and the drop caps. The timber course and the
  // rubric-red ✠ do not move — the church veils its images, not its rubrics.
  // Both themes read one pair of leaf colours, because the ground is dark in
  // each.
  for (const theme of ["light", "dark"]) {
    const ink = {};
    const ground = {};
    const cross = {};
    for (const [season, date] of [
      ["ordinary", testDate],
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
    expect(ink.passiontide).not.toBe(ink.ordinary);
    expect(ink.eastertide).not.toBe(ink.ordinary);
    expect(ink.eastertide).not.toBe(ink.passiontide);
    expect(ground.passiontide).toBe(ground.ordinary);
    expect(ground.eastertide).toBe(ground.ordinary);
    expect(cross.passiontide).toBe(cross.ordinary);
    expect(cross.eastertide).toBe(cross.ordinary);
  }
});

for (const theme of ["light", "dark"]) {
  test(`the painted headpiece keeps one form through the seasons without moving the invitation — ${theme}`, async ({ page }) => {
    // The ordo, reminders and usage headpieces are a cross between two
    // painted rules in every season; home opens on the consecration cross.
    await page.goto("/calendar/2026/04");
    await expect(page.locator(".ornament-foliage")).toHaveCount(0);
    const rules = page.locator(".ornament-headpiece .ornament-sprig-rule");
    expect(await rules.count()).toBe(2);
    // A horizontal SVG stroke has a zero-height bounding box, so assert
    // its visibility directly rather than Playwright's box-based matcher.
    for (const rule of await rules.all()) await expect(rule).toHaveCSS("visibility", "visible");
    await expect(page.locator(".ornament-headpiece > span")).toBeVisible();

    await openDatedPage(page, "/?date=2026-04-08", theme);
    await expect(page.locator(".home-crown")).toBeVisible();
    const invitation = page.locator(".pray-now");
    const veiledBox = await invitation.boundingBox();
    const lining = await invitation.evaluate(el => getComputedStyle(el).borderTopColor);
    // Hold the day's content constant to isolate ornament from different
    // feast names, notices, or commemorations changing the page height. The
    // painted frame does not veil or gild with the season.
    await page.evaluate(() => {
      document.body.classList.replace("season-passiontide", "season-eastertide");
    });
    expect(await invitation.boundingBox()).toEqual(veiledBox);
    expect(await invitation.evaluate(el => getComputedStyle(el).borderTopColor)).toBe(lining);
  });
}

for (const theme of ["light", "dark"]) {
  test(`hour titles set their sign in the rule and the day's colour in the lozenge — ${theme}`, async ({ page }) => {
    const title = async () => page.evaluate(() => {
      const heading = document.querySelector(".hour-header h1");
      const headpiece = document.querySelector(".hour-header .ornament-headpiece");
      const box = headpiece.getBoundingClientRect();
      return {
        lozenge: getComputedStyle(heading, "::after").backgroundColor,
        // The upper rule is 6px deep at the heading's top edge.
        offset: (box.top + box.bottom) / 2 - (heading.getBoundingClientRect().top + 3),
        sprigs: [...headpiece.querySelectorAll(".ornament-sprig")].map(svg => getComputedStyle(svg).display),
        sign: headpiece.querySelector("span").dataset.sign || "cross",
      };
    });
    // Lauds keeps the sun, Vespers and Compline the moon, the little hours
    // the cross; the rule itself is the line, so the sprigs never show.
    await openDatedPage(page, "/lauds/2026-09-28", theme);
    const green = await title();
    expect(Math.abs(green.offset)).toBeLessThan(1);
    expect(green.sprigs).toEqual(["none", "none"]);
    expect(green.sign).toBe("sun");
    await openDatedPage(page, "/lauds/2026-06-29", theme);
    const red = await title();
    expect(red.lozenge).toBe("rgb(176, 42, 36)");
    expect(red.lozenge).not.toBe(green.lozenge);
    await openDatedPage(page, "/terce/2026-03-31", theme);
    const terce = await title();
    expect(terce.sign).toBe("cross");
    expect(Math.abs(terce.offset)).toBeLessThan(1);
    await openDatedPage(page, "/vespers/2026-03-31", theme);
    expect((await title()).sign).toBe("moon");
    await expect(page.locator(".hour-header .ornament-headpiece > span")).toBeVisible();
  });
}

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
    const directory = getComputedStyle(document.querySelector(".home-hour-links"));
    const label = getComputedStyle(document.querySelector(".home-hour-group-label"));
    const dividedHour = getComputedStyle(
      document.querySelector(".home-hour-group-morning .home-hour-link + .home-hour-link"),
    );
    const finalGroup = getComputedStyle(document.querySelector(".home-hour-group:last-child"));
    return {
      directoryLeft: isVisibleRule(
        directory.borderLeftWidth,
        directory.borderLeftStyle,
        directory.borderLeftColor,
      ),
      directoryRight: isVisibleRule(
        directory.borderRightWidth,
        directory.borderRightStyle,
        directory.borderRightColor,
      ),
      labelRight: isVisibleRule(
        label.borderRightWidth,
        label.borderRightStyle,
        label.borderRightColor,
      ),
      // Full-height borders remain absent between the unequal 2/3/2 groups.
      // Short decorative hairlines separate neighbours within each band.
      dividedHourLeft: isVisibleRule(
        dividedHour.borderLeftWidth,
        dividedHour.borderLeftStyle,
        dividedHour.borderLeftColor,
      ),
      finalGroupBottom: isVisibleRule(
        finalGroup.borderBottomWidth,
        finalGroup.borderBottomStyle,
        finalGroup.borderBottomColor,
      ),
    };
  });
  expect(rules).toEqual({
    directoryLeft: true,
    directoryRight: true,
    labelRight: true,
    dividedHourLeft: false,
    finalGroupBottom: true,
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

test("appearance choice persists across prayer navigation", async ({ page }) => {
  await page.goto(`/?date=${testDate}`);

  await choosePreference(page, "Apse");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await page.goto(`/lauds/${testDate}`);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
});

test("text size choice persists across prayer navigation", async ({ page }) => {
  await page.goto(`/?date=${testDate}`);

  // A passive visit must not invent a stored preference.
  expect(await page.evaluate(() => localStorage.getItem("office-text-size"))).toBeNull();
  await expect(page.locator("html")).not.toHaveAttribute("data-text-size", /./);

  await choosePreference(page, "Larger text");
  await expect(page.locator("html")).toHaveAttribute("data-text-size", "large");

  await page.goto(`/lauds/${testDate}`);
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
  const cells = await page.evaluate(() => {
    const rects = (sel) =>
      Array.from(document.querySelectorAll(`.menu-prefs ${sel}`)).map((el) => el.getBoundingClientRect());
    return {
      themes: rects(".theme-option[data-theme-choice]"),
      sizes: rects(".text-size-option"),
      martyrology: rects(".martyrology-option"),
    };
  });
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
    return sheet.evaluate(() => {
      const round = (r) => [r.x, r.y, r.width, r.height].map((v) => Math.round(v * 10)).join(",");
      const range = document.createRange();
      return {
        boxes: [...document.querySelectorAll("body *")].map((el) => round(el.getBoundingClientRect())),
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

test("short psalm openings keep the same initial rank as adjacent psalms", async ({ page }) => {
  // Light only: themes share one geometry, held page-wide by "themes never change layout".
  const theme = "light";
  await openDatedPage(page, "/vespers/2026-09-12", theme);
  const psalms = page.locator(".psalm");
  const opening = psalms.nth(0).locator(".verse").first();
  const following = psalms.nth(1).locator(".verse").first();
  const original = await opening.textContent();
  await expect(psalms.nth(0)).toContainText("Psalm 145b");
  await expect(psalms.nth(1)).toContainText("Psalm 146");
  // A one-line verse keeps its full-size initial and natural text flow.
  // Returning to wide after narrow also exercises font-size changes.
  for (const width of [1280, 390, 768, 320, 1280]) {
    await page.setViewportSize({ width, height: 1000 });
    for (const size of ["normal", "large"]) {
      await page.evaluate(value => document.documentElement.setAttribute("data-text-size", value), size);
      // With its "Sit." cue the opening no longer fits one wide line in
      // large type either, so both sizes divide at the mediant.
      const divided = width === 1280;
      const elevated = width >= 768 && !divided;
      await expect.poll(() => opening.evaluate(el => ({
        divided: el.classList.contains("initial-divided"),
        raised: el.classList.contains("initial-raised"),
      }))).toEqual({ divided, raised: false });
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
          initial: cap.initialLetter,
          height: rect.height,
          leading: parseFloat(getComputedStyle(el).lineHeight),
          halfVerseGap: secondHalf.top - firstWord.top,
          nextTop: el.nextElementSibling.getBoundingClientRect().top,
          bottom: rect.bottom,
        };
      });
      expect(geometry.size).toBe(await following.evaluate(el => getComputedStyle(el, "::first-letter").fontSize));
      expect(geometry.initial).toBe(elevated ? "2 1" : "2");
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
  const psalm = page.locator(".psalm").filter({ hasText: "Psalm 63" });
  const opening = psalm.locator(".verse").first();
  const original = await opening.textContent();
  for (const size of ["normal", "large"]) {
    await page.evaluate(value => document.documentElement.dataset.textSize = value, size);
    for (const width of [320, 390, 414, 430, 1280, 390]) {
      await page.setViewportSize({ width, height: 900 });
      // The opening carries its "Sit." cue after the mediant: at 430px it no
      // longer fits one line and divides there; large type at 390px leaves a
      // three-word tail, which wraps naturally.
      const divided = [414, 430].includes(width) || (width === 390 && size === "normal");
      await expect.poll(() => opening.evaluate(el => el.classList.contains("initial-divided"))).toBe(divided);
      await expect(opening).not.toHaveClass(/initial-raised/);
      const lines = await openingTextLines(opening);
      const elevated = width === 1280;
      expect(lines).toBeCloseTo(elevated ? 1 : 2, 1);
      expect(await opening.evaluate(el => el.classList.contains("initial-elevated"))).toBe(elevated);
      expect(await opening.textContent()).toBe(original);
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

test("short responsories retain a raised initial when their text wraps", async ({ page }) => {
  // Light only: themes share one geometry, held page-wide by "themes never change layout".
  const theme = "light";
  await openDatedPage(page, "/lauds/2026-06-18", theme);
  const opening = page.locator(".short-responsory-opening .sigil-text");
  const original = await opening.textContent();
  for (const width of [1280, 320]) {
    await page.setViewportSize({ width, height: 900 });
    for (const size of ["normal", "large"]) {
      await page.evaluate(value => document.documentElement.setAttribute("data-text-size", value), size);
      const geometry = await opening.evaluate(el => ({
        float: getComputedStyle(el, "::first-letter").float,
        height: el.getBoundingClientRect().height,
        leading: parseFloat(getComputedStyle(el).lineHeight),
      }));
      expect(geometry.float).toBe("none");
      if (width === 320) expect(geometry.height).toBeGreaterThan(geometry.leading * 1.9);
      else expect(geometry.height).toBeCloseTo(geometry.leading, 0);
      expect(await opening.textContent()).toBe(original);
    }
  }
});

test("optical initial profiles preserve words and give subsequent lines their own clearance", async ({ page }) => {
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
    for (let i = 0; i < words.length; i++) {
      const opening = openings.nth(i);
      await expect(opening).not.toHaveClass(/initial-raised/);
      expect(await opening.textContent()).toBe(words[i] + sentence);
      const geometry = await opening.evaluate(el => {
        const glyph = (node, index) => {
          const range = document.createRange();
          range.setStart(node, index);
          range.setEnd(node, index + 1);
          return range.getBoundingClientRect();
        };
        const cap = glyph(el.firstChild, 0);
        const word = el.querySelector(".initial-word");
        const first = glyph(word.firstChild, 0);
        const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
        const rows = new Map();
        let node;
        while ((node = walker.nextNode())) {
          for (let j = 0; j < node.length; j++) {
            if ((node === el.firstChild && j === 0) || /\s/.test(node.textContent[j])) continue;
            const rect = glyph(node, j);
            const y = Math.round(rect.top);
            rows.set(y, Math.min(rows.get(y) ?? Infinity, rect.left));
          }
        }
        return {
          initial: el.dataset.initial,
          standalone: el.dataset.initialStandalone === "true",
          caps: getComputedStyle(word).fontVariantCaps,
          first: first.left,
          capRight: cap.right,
          capLeft: cap.left,
          edge: el.getBoundingClientRect().left,
          rows: [...rows.entries()].sort((a, b) => a[0] - b[0]).map(row => row[1]),
        };
      });
      expect(geometry.caps).toBe("all-small-caps");
      expect(geometry.rows.length).toBeGreaterThanOrEqual(2);
      if (["A", "L"].includes(geometry.initial)) {
        // First-word tucking does not pull the second row under the foot.
        expect(geometry.first).toBeLessThan(geometry.rows[1] - 1);
      }
      if (["F", "P", "T", "V", "W", "Y"].includes(geometry.initial)) {
        expect(geometry.rows[1]).toBeLessThan(geometry.first - 2);
      }
      if (geometry.initial === "Q" && width === 320) {
        // The tail continues below the second baseline; keep the next row
        // beside it before later rows return to the paragraph edge.
        expect(geometry.rows[2]).toBeGreaterThanOrEqual(geometry.capRight - .5);
      }
      if (geometry.standalone) {
        expect(geometry.first).toBeGreaterThan(geometry.capRight + 1);
      }
      if (["O", "T", "C"].includes(geometry.initial)) {
        expect(geometry.capLeft).toBeLessThan(geometry.edge);
      }
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    await expectInitialInkClear(page, page.locator(".elements"), ".chapter .plain-line:first-child", `${theme}/${width} alphabet ink clearance`);
  }
});

test("Prime hymn initial clears its second metrical line on narrow pages", async ({ page }) => {
  const primeHours = [
    { date: "2026-03-15", label: "Sunday" },
    { date: "2026-06-18", label: "feria" },
  ];

  for (const { date, label: hymn } of primeHours) {
    // Light only: themes share one geometry, held page-wide by "themes never change layout".
    const theme = "light";
    for (const width of [320, 390]) {
      await page.setViewportSize({ width, height: 844 });
      await openDatedPage(page, `/prime/${date}`, theme);

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
          secondLineIndent: parseFloat(getComputedStyle(secondLine).textIndent),
          secondLinePadding: parseFloat(getComputedStyle(secondLine).paddingLeft),
        };
      });

      const label = `${hymn}/${theme}/${width}px`;
      expect(geometry.secondLineIndent, `${label} second-line outdent`).toBe(0);
      expect(geometry.secondLinePadding, `${label} second-line hang padding`).toBe(0);
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
      const rubric = hymn.locator(".hymn-rubric");
      await expect(rubric).toHaveText(instruction);
      await expect(rubric).not.toContainText("/:");
      await expect(hymn.locator(".hymn-latin")).toHaveCount(0);
      await expect(hymn.locator(".hymn-stanza-opening .hymn-line").first()).toContainText(
        "Star of ocean fairest",
      );

      const geometry = await page.evaluate(() => {
        const rubric = document.querySelector(".hymn .hymn-rubric");
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
      // First source line text is before the <br>; second is after it.
      const br = [...opening.childNodes].find((n) => n.nodeName === "BR");
      const firstText = [...opening.childNodes].find(
        (n) => n.nodeType === Node.TEXT_NODE && n.textContent.trim(),
      );
      let secondText = null;
      if (br) {
        for (let n = br.nextSibling; n; n = n.nextSibling) {
          if (n.nodeType === Node.TEXT_NODE && n.textContent.trim()) {
            secondText = n;
            break;
          }
        }
      }
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
        openingPadding: parseFloat(getComputedStyle(opening).paddingLeft),
        openingIndent: parseFloat(getComputedStyle(opening).textIndent),
        laterPadding: later ? parseFloat(getComputedStyle(later).paddingLeft) : 0,
        laterIndent: later ? parseFloat(getComputedStyle(later).textIndent) : 0,
        label: document.querySelector(".marian-antiphon .item-label")?.textContent ?? "",
        float: getComputedStyle(opening, "::first-letter").float,
      };
    });

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
    const yOverlap =
      geometry.second.top < geometry.cap.bottom - 0.5 &&
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

test("desktop prayer text keeps a centred book measure and crisp section rhythm", async ({
  page,
}) => {
  for (const width of [920, 1280, 1920]) {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, `/lauds/${testDate}`);

    const geometry = await page.evaluate(() => {
      const elements = document.querySelector(".elements");
      const elementsRect = elements.getBoundingClientRect();
      const heading = [...document.querySelectorAll(".section-heading")].find(
        (node) => node.textContent.trim() === "The Short Responsory",
      );
      const headingRect = heading.getBoundingClientRect();
      const beforeRect = heading.previousElementSibling.getBoundingClientRect();
      const afterRect = heading.nextElementSibling.getBoundingClientRect();
      const headingStyle = getComputedStyle(heading);
      return {
        elementsWidth: elementsRect.width,
        centreOffset: Math.abs(elementsRect.left + elementsRect.width / 2 - window.innerWidth / 2),
        prayerFont: parseFloat(getComputedStyle(elements).fontSize),
        headingLeading:
          parseFloat(headingStyle.lineHeight) / parseFloat(headingStyle.fontSize),
        beforeGap: headingRect.top - beforeRect.bottom,
        afterGap: afterRect.top - headingRect.bottom,
        overflow:
          document.documentElement.scrollWidth - document.documentElement.clientWidth,
      };
    });

    expect(geometry.elementsWidth, `${width}px prayer measure`).toBeGreaterThanOrEqual(580);
    expect(geometry.elementsWidth, `${width}px prayer measure`).toBeLessThanOrEqual(600);
    expect(geometry.centreOffset, `${width}px prayer centring`).toBeLessThanOrEqual(1);
    expect(geometry.prayerFont, `${width}px prayer face`).toBeCloseTo(20, 1);
    expect(geometry.headingLeading, `${width}px heading leading`).toBeCloseTo(1.3, 1);
    expect(geometry.beforeGap, `${width}px space before heading`).toBeGreaterThanOrEqual(36);
    expect(geometry.afterGap, `${width}px space after heading`).toBeGreaterThanOrEqual(24);
    expect(geometry.overflow, `${width}px horizontal overflow`).toBe(0);
  }

  // A tablet keeps the denser setting until there is enough open field to
  // frame the larger desktop page.
  await page.setViewportSize({ width: 768, height: 900 });
  await openDatedPage(page, `/lauds/${testDate}`);
  await expect
    .poll(() =>
      page.locator(".elements").evaluate((node) => parseFloat(getComputedStyle(node).fontSize)),
    )
    .toBeCloseTo(19, 1);
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

test("dated hour navigation keeps the selected liturgical day", async ({ page }) => {
  await openDatedPage(page, `/lauds/${testDate}`);

  await page.getByText("Change date", { exact: true }).click();
  await page.getByRole("link", { name: "Previous day" }).click();

  await expect(page).toHaveURL(/\/lauds\/2026-03-14$/);
  await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
});

test("the date picker is set in the room, not the platform", async ({ page }) => {
  // Today is Wednesday the 18th; the page is Sunday the 15th.
  await page.clock.setFixedTime(new Date("2026-03-18T10:00:00-04:00"));
  await openDatedPage(page, `/lauds/${testDate}`);
  await page.getByText("Change date", { exact: true }).click();
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

  // Escape closes the disclosure back to its label.
  await page.keyboard.press("Escape");
  await expect(picker).toBeHidden();
  await expect(page.locator(".hour-date-nav > summary")).toBeFocused();

  // Reopening starts from the page's own day, and a day is a link to it.
  await page.getByText("Change date", { exact: true }).click();
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
    await page.getByText("Change date", { exact: true }).click();
    await expect(picker.getByRole("grid", { name: month })).toBeVisible();
    await expect(picker.getByRole("button", { name: closed })).toBeDisabled();
    await expect(picker.getByRole("button", { name: open })).toBeEnabled();
  }
  // Keys stop at the end rather than leaving the span.
  await page.goto("/lauds/2150-12-15");
  await page.getByText("Change date", { exact: true }).click();
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
  await expect(day.locator(".day-commemoration").first()).toBeVisible();
  await page.getByRole("button", { name: "Hide office details" }).click();
  await expect(page.locator(".day-disclosures details[open]")).toHaveCount(0);
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
    await page.evaluate(() => document.fonts.ready);
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
  await page.getByRole("navigation", { name: "Jump to month" }).getByRole("link", { name: "March", exact: true }).click();
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
  await page.getByRole("navigation", { name: "Jump to month" }).getByRole("link", { name: "November", exact: true }).click();
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
  await expect(day.locator(".day-office-digest")).toBeVisible();
  await expect(day.locator(".day-commemoration").first()).toBeVisible();
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
  await page.goto("/calendar/2026#d-2026-09-12");
  await expect(page).toHaveURL(/\/calendar\/2026\/09#d-2026-09-12$/);
  await expect(page.locator("#d-2026-09-12")).toBeInViewport();
  await page.goto("/calendar/2026?form=priest#march");
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
  await expect(page.locator("#reminder-url")).toHaveText("Select at least one hour above.");
  await expect(page.locator("#reminder-copied")).toHaveText("Select at least one hour above.");
  await expect(page.locator("#reminder-copied")).toBeVisible();

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
  await expect(feedURL).toHaveText("Choose a time for each selected hour.");
  await expect(page.locator("#reminder-copied")).toHaveText(
    "Choose a time for each selected hour.",
  );

  await page.getByLabel("Time for Lauds").fill("07:30");
  await page.getByLabel("Time for Lauds").press("Tab");
  await expect(copy).toBeEnabled();

  for (const checkbox of await page.locator('input[name="day"]').all()) {
    await checkbox.uncheck();
  }
  await expect(copy).toBeDisabled();
  await expect(subscribe).not.toHaveAttribute("href");
  await expect(feedURL).toHaveText("Select at least one day above.");
  await expect(page.locator("#reminder-copied")).toHaveText("Select at least one day above.");

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

  await expect(page.locator("#reminder-copied")).toHaveText(
    "Copy unavailable. The calendar address is shown below.",
  );
  await expect(page.locator(".reminder-address")).toHaveAttribute("open", "");
  await expect(page.locator("#reminder-url")).toBeVisible();
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
        ".hour-date-nav > summary",
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
        const box = await targets.nth(i).boundingBox();
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
    return events[0];
  };

  // Device appearance, no stored choice: what is on screen is what counts.
  expect(await read("light phone")).toBe("vespers appearance:nave screen:mobile prayer-form:private client:browser");
  await page.emulateMedia({ colorScheme: "dark" });
  expect(await read("dark phone")).toBe("vespers appearance:apse screen:mobile prayer-form:private client:browser");

  // An explicit choice overrides the device, so someone reading the Nave on a
  // dark-mode phone counts as Nave.
  await page.evaluate(() => localStorage.setItem("office-theme", "light"));
  expect(await read("chosen Nave on a dark phone")).toBe("vespers appearance:nave screen:mobile prayer-form:private client:browser");
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
  expect(events[0]).toBe("site appearance:nave screen:mobile client:pwa");
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
      return events[0];
    };

    expect(await read("wide window")).toBe("vespers appearance:nave screen:desktop prayer-form:private client:browser");
    // A desktop window dragged narrow gets the phone layout, and is counted
    // as the layout it is actually being read in.
    await page.setViewportSize({ width: 390, height: 900 });
    expect(await read("narrow window")).toBe("vespers appearance:nave screen:mobile prayer-form:private client:browser");
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

test("the Martyrology at Prime is a Settings choice, off by default, that holds offline", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ serviceWorkers: "allow", baseURL });
  try {
    const page = await context.newPage();
    // Keep the September 8 reading, but visit its next occurrence:
    // precache intentionally prunes historical pages as the real clock moves.
    const now = new Date();
    const year = now.getFullYear() + (now >= new Date(now.getFullYear(), 8, 8) ? 1 : 0);
    const prime = `/prime/${year}-09-07`;
    const reading = page.getByRole("heading", { name: "Martyrology — September 8", exact: true });
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

    await choosePreference(page, "On");
    await expect(reading).toBeVisible();
    await expect(rubric).toBeHidden();
    await expect(page.locator(".elements")).not.toContainText("Thomas of Villanova");
    // Remembered, and the same page serves both settings, offline too.
    await context.setOffline(true);
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-martyrology", "on");
    await expect(reading).toBeVisible();
    await choosePreference(page, "Off");
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
  expect(await prayers.innerText()).toContain('Lord, grant a blessing');
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

test('cached prayer pages include every form and preserve an explicit form through redirects', async ({ browser, baseURL }) => {
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

test("the ordo's current month and frontispiece are saved for offline use", async ({ browser, baseURL }) => {
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
      const bounds = await selector.boundingBox();
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
    const gap = await rubric.evaluate(el => el.getBoundingClientRect().top - el.previousElementSibling.getBoundingClientRect().bottom);
    expect(gap).toBeGreaterThanOrEqual(14);
    const collect = page.locator(".marian-antiphon .collect .plain-line").first();
    await expect(collect).toHaveAttribute("data-initial", "A");
    await expect(collect.locator(".initial-word")).toHaveText("lmighty");
    await page.emulateMedia({ media: "print" });
    await expect(rubric).toBeVisible();
    expect(await rubric.evaluate(el => el.getBoundingClientRect().top - el.previousElementSibling.getBoundingClientRect().bottom)).toBeGreaterThanOrEqual(14);
    await page.emulateMedia({ media: "screen" });
  }
});

// The report line clears its own glyphs, as the footer lettering does, and
// leaves the stars around it: an opaque ground across its touch-target box
// erased a whole row of the vault on phones.
test("Apse clears the report lettering, not a band of the starfield", async ({ page }) => {
  for (const width of [390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await openDatedPage(page, "/vespers/2026-09-22", "dark");
    const lines = await page.locator(".report-issue:visible").evaluateAll(els =>
      els.map(el => {
        const style = getComputedStyle(el);
        return {
          background: style.backgroundColor,
          boxShadow: style.boxShadow,
          textShadow: style.textShadow,
          footerShadow: getComputedStyle(document.querySelector("footer")).textShadow,
        };
      }),
    );
    expect(lines.length).toBe(1);
    for (const line of lines) {
      expect(line.background).toBe("rgba(0, 0, 0, 0)");
      expect(line.boxShadow).toBe("none");
      expect(line.textShadow).not.toBe("none");
      expect(line.textShadow).toBe(line.footerShadow);
    }
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
