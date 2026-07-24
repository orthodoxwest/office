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

test("desktop navigation remains expanded", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await openDatedPage(page, `/?date=${testDate}`);

  await expect(page.locator(".site-menu")).toHaveAttribute("open", "");
  await expect(
    page.getByRole("navigation", { name: "Primary" }).getByRole("link", { name: "Vespers", exact: true }),
  ).toBeVisible();
});

test("appearance choice persists across prayer navigation", async ({ page }) => {
  await page.goto(`/?date=${testDate}`);

  await page.getByRole("button", { name: "Apse", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await page.goto(`/lauds/${testDate}`);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
});

test("dated hour navigation keeps the selected liturgical day", async ({ page }) => {
  await openDatedPage(page, `/lauds/${testDate}`);

  await page.getByText("Change date", { exact: true }).click();
  await page.getByRole("link", { name: "Previous day" }).click();

  await expect(page).toHaveURL(/\/lauds\/2026-03-14$/);
  await expect(page.getByRole("heading", { name: "Lauds", exact: true })).toBeVisible();
});

test("reminder choices update the subscription URL", async ({ page }) => {
  await page.goto("/reminders");

  for (const checkbox of await page.locator('input[name="hour"]').all()) {
    await checkbox.uncheck();
  }
  await page.locator('input[name="hour"][value="lauds"]').check();
  await page.getByLabel("Time for Lauds").fill("07:30");
  await page.getByLabel("Time for Lauds").press("Tab");

  const feedURL = page.locator("#reminder-url");
  await expect(feedURL).toContainText("lauds=07%3A30");
  await expect(feedURL).toContainText("tz=America%2FNew_York");
});

for (const { name, path, theme, knownViolations } of [
  {
    name: "home in the Nave theme",
    path: `/?date=${testDate}`,
    theme: "light",
    knownViolations: [
      { rule: "color-contrast", target: ".hour-date-nav > summary" },
      { rule: "color-contrast", target: "#home-pray-heading" },
    ],
  },
  {
    name: "Lauds in the Apse theme",
    path: `/lauds/${testDate}`,
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
