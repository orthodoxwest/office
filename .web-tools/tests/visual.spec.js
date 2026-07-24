import { expect, test } from "@playwright/test";

const testDate = "2026-03-15";

test.skip(process.platform !== "linux", "Visual baselines use the pinned Linux CI image.");

async function openForSnapshot(page, path, theme) {
  await page.addInitScript((storedTheme) => {
    localStorage.setItem("office-theme", storedTheme);
  }, theme);
  await page.goto(path);
  await page.evaluate(() => document.fonts.ready);
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
}

test("mobile Ordo — light", async ({ page }) => {
  await openForSnapshot(page, "/calendar/2026", "light");
  await expect(page.getByRole("heading", { name: "2026 Ordo", exact: true })).toBeVisible();
  await expect(page).toHaveScreenshot("ordo-light.png");
});
