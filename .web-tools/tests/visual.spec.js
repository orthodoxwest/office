import { expect, test } from "@playwright/test";

const testDate = "2026-03-15";

test.skip(!process.env.CI, "Visual baselines run in the pinned CI browser container.");

async function openForSnapshot(page, path, theme) {
  await page.addInitScript((storedTheme) => {
    localStorage.setItem("office-theme", storedTheme);
  }, theme);
  await page.goto(path);
  await page.evaluate(() => document.fonts.ready);
  // Ordo intentionally keeps a fallback when its font arrives late. Record
  // the cached-font appearance deterministically; UX tests cover a cold font.
  if (path.startsWith("/calendar/")) {
    await page.reload();
    await page.evaluate(() => document.fonts.ready);
  }
}

for (const theme of ["light", "dark"]) {
  test(`mobile home — ${theme}`, async ({ page }) => {
    await openForSnapshot(page, `/?date=${testDate}`, theme);
    await expect(page.locator(".home")).toBeVisible();
    await expect(page).toHaveScreenshot(`home-${theme}.png`, { fullPage: true });
  });

  test(`mobile Lauds — ${theme}`, async ({ page }) => {
    await openForSnapshot(page, `/lauds/${testDate}`, theme);
    await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
    await expect(page).toHaveScreenshot(`lauds-${theme}.png`);
  });

  test(`desktop Lauds — ${theme}`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await openForSnapshot(page, `/lauds/${testDate}`, theme);
    await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
    await expect(page).toHaveScreenshot(`lauds-desktop-${theme}.png`);
  });

  test(`desktop home — ${theme}`, async ({ page }) => {
    // The project stays in its pinned touch-emulated Chromium profile; this
    // guards desktop responsive composition, not mouse/pointer behavior.
    await page.setViewportSize({ width: 1280, height: 900 });
    await openForSnapshot(page, `/?date=${testDate}`, theme);
    await expect(page.locator(".home")).toBeVisible();
    await expect(page).toHaveScreenshot(`home-desktop-${theme}.png`, { fullPage: true });
  });

  test(`mobile Reminders — ${theme}`, async ({ page }) => {
    await openForSnapshot(page, "/reminders", theme);
    await expect(page.getByRole("heading", { name: "Set prayer reminders", exact: true })).toBeVisible();
    await expect(page).toHaveScreenshot(`reminders-${theme}.png`, { fullPage: true });
  });
}

for (const theme of ["light", "dark"]) {
  for (const width of [390, 1280]) {
    test(`Ordo at ${width}px — ${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
      await page.clock.install({ time: new Date("2026-01-04T12:00:00-05:00") });
      await openForSnapshot(page, "/calendar/2026", theme);
      await expect(page.getByRole("heading", { name: "2026 Ordo", exact: true })).toBeVisible();
      await expect(page).toHaveScreenshot(`ordo-${width === 1280 ? "desktop-" : ""}${theme}.png`);
      if (width === 390) {
        const today = page.locator("#d-2026-01-04");
        await today.locator(".day-office-details summary").click();
        await today.evaluate((node) => node.scrollIntoView({ block: "start" }));
        await expect(today.locator(".day-office-digest")).toBeVisible();
        await expect(page).toHaveScreenshot(`ordo-details-${theme}.png`);
      }
    });
  }
}

test("mobile menu — light", async ({ page }) => {
  await openForSnapshot(page, `/?date=${testDate}`, "light");
  await page.getByText("Menu", { exact: true }).click();
  await expect(page.getByRole("navigation", { name: "Primary" })).toBeVisible();
  await expect(page).toHaveScreenshot("menu-open-light.png");
});

test("mobile hour ending — dark", async ({ page }) => {
  await openForSnapshot(page, `/lauds/${testDate}`, "dark");
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect(page.locator(".hour-epilogue")).toBeVisible();
  await expect(page).toHaveScreenshot("hour-ending-dark.png");
});

test("desktop prayer transition — light", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await openForSnapshot(page, `/lauds/${testDate}`, "light");
  await page
    .getByRole("heading", { name: "The Short Responsory", exact: true })
    .scrollIntoViewIfNeeded();
  // Keep the responsory-to-hymn transition in frame without cutting the
  // hymn's two-line opening initial off at the bottom of the viewport.
  await page.evaluate(() => window.scrollBy(0, 120));
  await expect(page).toHaveScreenshot("lauds-transition-desktop-light.png");
});

for (const theme of ["light", "dark"]) {
  test(`complete initial alphabet — ${theme}`, async ({ page }) => {
    const words = ["All", "Blessed", "Come", "Deliver", "Every", "For", "Glory", "Hear", "I will", "Jesus", "King", "Lord", "Make", "Now", "O Lord", "Praise", "Quicken", "Remember", "Save", "The", "Unto", "Vouchsafe", "With", "Xavier", "Ye", "Zion"];
    const fixture = words.map(word => `<div class="chapter"><div class="liturgical-block"><p class="plain-line">${word} hear our prayer, and let our cry come unto thee. Be merciful unto us, O Lord, and guide our steps in the way of peace.</p></div></div>`).join("");
    await page.route(`**/vespers/${testDate}`, async route => {
      const response = await route.fetch();
      await route.fulfill({ response, body: (await response.text()).replace(
        /(<main\b[^>]*>)[\s\S]*?(<\/main>)/,
        `$1<div class="elements">${fixture}</div>$2`,
      ) });
    });
    await page.setViewportSize({ width: 1280, height: 1000 });
    await openForSnapshot(page, `/vespers/${testDate}`, theme);
    await page.addStyleTag({ content: `
      main { max-width: 1000px; }
      .elements { max-width: none; display: grid; grid-template-columns: 1fr 1fr; gap: 24px 40px; }
      .chapter { margin: 0; }
      header, nav, .hour-progress { visibility: hidden; }
    ` });
    await expect(page.locator(".initial-word")).toHaveCount(26);
    await expect(page.locator(".elements")).toHaveScreenshot(`initial-alphabet-${theme}.png`);
  });
}
