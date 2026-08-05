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

  await page.getByText("Menu", { exact: true }).click();
  await expect(menu).toHaveAttribute("open", "");
  await expect(
    page.getByRole("navigation", { name: "Primary" }).getByRole("link", { name: "Lauds", exact: true }),
  ).toBeVisible();
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
        await page.getByRole("button", { name: size === "large" ? "Larger text" : "Smaller text" }).click();
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
        await page.getByRole("button", { name: "Default text size" }).click();
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
  expect(naveState.borderStyle).toBe("double");

  await page.getByRole("button", { name: "Apse", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const apseState = await invitation.evaluate((element) => ({
    background: getComputedStyle(element).backgroundColor,
    borderStyle: getComputedStyle(element).borderTopStyle,
  }));
  expect(apseState.background).toBe("rgba(0, 0, 0, 0)");
  expect(apseState.borderStyle).toBe("double");
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

test("parish material stays in non-liturgical rooms and off the prayer page", async ({
  page,
}) => {
  await openDatedPage(page, `/?date=${testDate}`);

  const naveMaterial = await page.evaluate(() => ({
    page: getComputedStyle(document.body).backgroundImage,
    inscriptionBand: getComputedStyle(
      document.querySelector(".home-hour-group-label"),
    ).backgroundColor,
  }));
  expect(naveMaterial.page).not.toBe("none");
  expect(naveMaterial.inscriptionBand).not.toBe("rgba(0, 0, 0, 0)");

  await page.getByRole("button", { name: "Apse", exact: true }).click();
  // The Apse vault is a background-image, which can only snap rather than
  // crossfade, so app.js dips it invisible and applies the theme (and swaps
  // this image) only once that dip completes — poll instead of reading the
  // pre-swap Nave material on a fast single-worker CI run.
  const readMaterial = () => page.evaluate(() => getComputedStyle(document.body).backgroundImage);
  await expect.poll(readMaterial).not.toBe(naveMaterial.page);
  const apseMaterial = await readMaterial();
  expect(apseMaterial).not.toBe("none");

  await page.goto(`/lauds/${testDate}`);
  const prayerMaterial = await page.evaluate(() => ({
    page: getComputedStyle(document.body).backgroundImage,
    prayer: getComputedStyle(document.querySelector(".elements")).backgroundImage,
  }));
  expect(prayerMaterial).toEqual({ page: "none", prayer: "none" });

  // Desktop. The theme persists in localStorage, so each half sets its own
  // explicitly rather than inheriting whatever the previous step left behind.
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/?date=${testDate}`);

  // Nave keeps the still, flat field: the broad wash reads as spotlighting on
  // a wide canvas, and the nave ceiling is plaster, so there is nothing else
  // for it to carry.
  await page.getByRole("button", { name: "Nave", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  expect(await page.evaluate(() => getComputedStyle(document.body).backgroundImage)).toBe(
    "none",
  );

  // Apse gets the vault instead — stars only, never the broad wash.
  await page.getByRole("button", { name: "Apse", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const vault = await page.evaluate(
    () => getComputedStyle(document.body, "::before").backgroundImage,
  );
  expect(vault).not.toBe("none");
  // One diaper cell: a principal star (three gradients) at each rib crossing,
  // a lesser star (two — its core dot drowned under the rays and cost a paint
  // layer) in each panel. The two rib families are linear gradients, counted
  // separately below.
  expect((vault.match(/radial-gradient/g) || []).length).toBe(10);
  expect((vault.match(/linear-gradient/g) || []).length).toBe(2);
});

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
      const ink = style.backgroundImage.match(/color\(srgb ([\d.]+) ([\d.]+) ([\d.]+)/);
      const rgb = style.backgroundColor.match(/\d+/g).slice(0, 3).map(Number);
      return {
        // The vault has its own layer, so this cannot pick up
        // --page-material's broad radials, which live on body and legitimately
        // remain on the phone. One diaper cell is exactly ten radials.
        stars: (style.backgroundImage.match(/radial-gradient/g) || []).length,
        pageIsDark: rgb.reduce((a, b) => a + b, 0) / 3 < 100,
        ink: ink ? ink.slice(1).join(",") : null,
      };
    });
    await context.close();
    return read;
  };

  const home = `/?date=${testDate}`;
  // Gold stars must never land on plaster. The Default-theme/light-device case
  // is the one that bites: :root:not([data-theme="light"]) matches when no
  // choice has been stored, so without a prefers-color-scheme guard the vault
  // would light up over the Nave.
  for (const scheme of ["light", "dark"]) {
    const seen = await vault({ width: 1280, theme: null, scheme, path: home });
    if (!seen.pageIsDark) expect(seen.stars).toBe(0);
  }
  expect((await vault({ width: 1280, theme: "light", scheme: "dark", path: home })).stars).toBe(0);

  // Present behind the Apse home at every width, absent in working rooms.
  expect((await vault({ width: 1280, theme: "dark", scheme: "dark", path: home })).stars).toBe(10);
  expect((await vault({ width: 390, theme: "dark", scheme: "dark", path: home })).stars).toBe(10);
  for (const path of ["/calendar/2026", "/reminders"]) {
    expect((await vault({ width: 1280, theme: "dark", scheme: "dark", path })).stars).toBe(0);
  }

  // Gold leaf is gilding, so the vault keeps the season. This only holds while
  // --apse-vault is declared where the seasonal --ornament lands; hoisting it
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
    // And it is the declared measure, not whatever the content happened to need.
    expect(widths[0]).toBe(38 * 16);
  }
});

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
  const read = async ({ width = 390, height, theme, scheme }) => {
    const context = await page.context().browser().newContext({
      viewport: { width, height },
      isMobile: true,
      hasTouch: true,
      colorScheme: scheme,
    });
    const sheet = await context.newPage();
    if (theme) await sheet.addInitScript((t) => localStorage.setItem("office-theme", t), theme);
    await sheet.goto(`/?date=${testDate}`);
    const seen = await sheet.evaluate(() => {
      const field = getComputedStyle(document.body, "::before");
      const footerElement = document.querySelector("footer");
      const diamond = getComputedStyle(footerElement, "::before");
      const card = getComputedStyle(document.querySelector(".home-hero"));
      return {
        stars: (field.backgroundImage.match(/radial-gradient/g) || []).length,
        position: field.position,
        tileSize: field.backgroundSize,
        phase: field.backgroundPosition,
        diamondVisible: diamond.visibility !== "hidden",
        // Probe the night token rather than hard-coding #121c28 — the halo must
        // use whatever --bg is, not a particular hex.
        pageBg: getComputedStyle(document.body).backgroundColor,
        cardShadow: card.boxShadow,
        scrolls: document.documentElement.scrollHeight > window.innerHeight + 1,
        scrollHeight: document.documentElement.scrollHeight,
      };
    });
    await context.close();
    return seen;
  };

  const apse = await read({ height: 844, theme: "dark", scheme: "dark" });
  expect(apse.stars).toBe(10);
  expect(apse.position).toBe("fixed");
  expect(tileEdgePx(apse.tileSize)).toEqual({ w: 132, h: 132 });
  expect(isTopCenterPhase(apse.phase)).toBe(true);
  expect(apse.diamondVisible).toBe(false);
  expect(apse.cardShadow).toContain(apse.pageBg);
  expect(apse.scrolls).toBe(false);

  // Nave keeps the same page geometry but paints no vault.
  for (const [theme, scheme] of [
    ["light", "light"],
    [null, "light"],
  ]) {
    const nave = await read({ height: 844, theme, scheme });
    expect(nave.stars).toBe(0);
    expect(nave.diamondVisible).toBe(true);
    expect(nave.scrolls).toBe(false);
    expect(nave.scrollHeight).toBe(apse.scrollHeight);
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
    expect(field.stars).toBe(10);
    expect(field.position).toBe("fixed");
    expect(tileEdgePx(field.tileSize)).toEqual({ w: 132, h: 132 });
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
    return [style.content, style.backgroundImage, style.backgroundPosition];
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
      const assurance = document.querySelector(".assurance-panel");
      const footerElement = document.querySelector("footer");
      const field = getComputedStyle(epilogue, "::before");
      const footerField = getComputedStyle(footerElement, "::after");
      const diamond = getComputedStyle(footerElement, "::before");
      const mainBox = main.getBoundingClientRect();
      const prayerBox = prayer.getBoundingClientRect();
      const epilogueBox = epilogue.getBoundingClientRect();
      const footerBox = footerElement.getBoundingClientRect();
      return {
        pageClass: document.body.classList.contains("page-hour"),
        prayerField: getComputedStyle(prayer).backgroundImage,
        fieldLayers: (field.backgroundImage.match(/radial-gradient/g) || []).length,
        footerLayers: (footerField.backgroundImage.match(/radial-gradient/g) || []).length,
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
        phase: [field.backgroundPosition, footerField.backgroundPosition],
        diamond: diamond.content,
        diamondVisibility: diamond.visibility,
        assuranceBackground: getComputedStyle(assurance).backgroundColor,
      };
    });
    await context.close();
    return seen;
  };

  const apse = await read("dark");
  expect(apse.pageClass).toBe(true);
  expect(apse.prayerField).toBe("none");
  expect(apse.fieldLayers).toBe(10);
  expect(apse.footerLayers).toBe(10);
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
  // Kept, not dropped: the diamond's box stays (visibility: hidden) so the
  // footer's own height — and everything below the glyph — never moves when
  // Nave/Apse toggles.
  expect(apse.diamond).toContain("✦");
  expect(apse.diamondVisibility).toBe("hidden");

  const nave = await read("light");
  expect(nave.prayerField).toBe("none");
  expect(nave.fieldLayers).toBe(0);
  expect(nave.footerLayers).toBe(0);
  expect(nave.diamond).toContain("✦");
  expect(nave.diamondVisibility).toBe("visible");
  expect(apse.assuranceBackground).not.toBe(nave.assuranceBackground);

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
    [1280, "/calendar/2026"],
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
    expect(header.links).toBe(9);
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

  await page.getByRole("button", { name: "Apse", exact: true }).click();
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

test("desktop navigation and frontispiece remain composed", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/?date=${testDate}`);

  await expect(page.locator(".site-menu")).toHaveAttribute("open", "");
  await expect(
    page.getByRole("navigation", { name: "Primary" }).getByRole("link", { name: "Vespers", exact: true }),
  ).toBeVisible();
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
      // Deliberately absent. The periods hold 2, 3 and 2 hours, so a rule
      // between hours implies columns that cannot line up across the rows and
      // the directory reads as a mis-set table. The frame, the label column
      // and the band rules carry the structure instead.
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
        await page.getByRole("button", { name: size === "large" ? "Larger text" : "Smaller text" }).click();
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
        await page.getByRole("button", { name: "Default text size" }).click();
      }
    }
  }
});

test("appearance choice persists across prayer navigation", async ({ page }) => {
  await page.goto(`/?date=${testDate}`);

  await page.getByRole("button", { name: "Apse", exact: true }).click();
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

  await page.getByRole("button", { name: "Larger text", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-text-size", "large");

  await page.goto(`/lauds/${testDate}`);
  await expect(page.locator("html")).toHaveAttribute("data-text-size", "large");
  await expect(page.getByRole("button", { name: "Larger text", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
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

  await page.getByRole("button", { name: "Larger text", exact: true }).click();
  expect(await prayerSize()).toBeGreaterThan(base);
  expect(await overflows()).toBe(false);

  // Every footer control keeps a thumb-sized target at the largest setting.
  const heights = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".text-size-option, .theme-option")).map(
      (el) => el.getBoundingClientRect().height,
    ),
  );
  expect(Math.min(...heights)).toBeGreaterThanOrEqual(44);

  await page.getByRole("button", { name: "Smaller text", exact: true }).click();
  expect(await prayerSize()).toBeLessThan(base);
  expect(await overflows()).toBe(false);

  await page.getByRole("button", { name: "Default text size", exact: true }).click();
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
          return { float: cap.float, size: parseFloat(cap.fontSize) };
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
            const text = opening.firstChild;
            const glyph = (start, end) => {
              const range = document.createRange();
              range.setStart(text, start);
              range.setEnd(text, end);
              const { left, right } = range.getBoundingClientRect();
              return { left, right };
            };
            return {
              cap: glyph(0, 1),
              following: glyph(1, 2),
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
        expect(cap.float, `${label} approved opening drop cap`).toBe("left");
        expect(cap.size, `${label} approved opening drop cap size`).toBeGreaterThan(metrics.prayer * 2);
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

test("Prime hymn initial clears its second metrical line on narrow pages", async ({ page }) => {
  const primeHours = [
    { date: "2026-03-15", label: "Sunday" },
    { date: "2026-06-18", label: "feria" },
  ];

  for (const { date, label: hymn } of primeHours) {
    for (const theme of ["light", "dark"]) {
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
          // Cap still occupies this row; the glyph must sit to its right.
          expect(
            geometry.secondLine.left,
            `${label} second-line glyph clears cap`,
          ).toBeGreaterThanOrEqual(geometry.cap.right - 0.5);
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
        following: glyph(firstText, 1, 2),
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
      bannerDisplay: getComputedStyle(document.querySelector(".site-banner")).display,
      sessionSummaryDisplay: getComputedStyle(
        document.querySelector(".session-prayers > summary"),
      ).display,
    };
  });

  // CSS px are 96/in; 11pt therefore computes to 14.666…px.
  expect(printStyles.bodyFont).toBeCloseTo(44 / 3, 1);
  expect(printStyles.prayerFont).toBeCloseTo(printStyles.bodyFont, 1);
  expect(printStyles.prayerMaxWidth).toBe("none");
  expect(printStyles.headerDisplay).toBe("none");
  expect(printStyles.bannerDisplay).toBe("none");
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

test("ordo day details collapse on a phone and stay open on a wide screen", async ({ page }) => {
  await openDatedPage(page, "/calendar/2026");

  const digest = page.locator("#d-2026-03-01 details.day-office-details");
  await expect(digest).not.toHaveAttribute("open", "");
  await expect(page.locator("#d-2026-03-01 .day-office-digest")).toBeHidden();

  await digest.getByText("Office details", { exact: true }).click();
  await expect(page.locator("#d-2026-03-01 .day-office-digest")).toBeVisible();

  // A closed <details> contributes no height, so the wide layout has to open
  // them rather than reveal the contents with CSS.
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, "/calendar/2026");

  await expect(digest).toHaveAttribute("open", "");
  await expect(page.locator("#d-2026-03-01 .day-office-digest")).toBeVisible();
  await expect(page.locator("#d-2026-03-01 .day-commemoration").first()).toBeVisible();
});

test("the foreground Ordo moves rather than duplicates its today marker at midnight", async ({
  page,
}) => {
  await page.clock.install({ time: new Date("2026-07-29T23:59:00-04:00") });
  await openDatedPage(page, "/calendar/2026");

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
        ".assurance-panel > summary",
        ".report-issue a",
      ],
    ],
    [
      "/calendar/2026",
      [".year-nav a:not([hidden])", ".month-jump a", ".day-disclosures summary"],
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
