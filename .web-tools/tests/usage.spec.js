import { expect, test } from "@playwright/test";

// Unequal daily sample sizes catch unweighted averages of daily percentages.
const appearance = [[1, 0], [0, 9], [0, 0], [4, 0], [0, 0], [0, 0], [2, 2]];
async function fixture(page, empty = false) {
  await page.route("**/admin/usage?*", async route => {
    const response = await route.fetch();
    let body = await response.text();
    body = body.replace(/(<script id="usage-trend-data"[^>]*>)([\s\S]*?)(<\/script>)/, (_, open, data, close) => {
      const groups = JSON.parse(data);
      for (const group of groups) {
        group.Points = appearance.map((counts, i) => ({
          Day: `2026-09-${String(8 + i).padStart(2, "0")}`,
          Counts: group.Series.map((_, series) => empty ? 0 : group.Key === "appearance" ? counts[series] : i === 6 ? series + 1 : 0),
        }));
      }
      return open + JSON.stringify(groups) + close;
    });
    await route.fulfill({ response, body });
  });
}

test("trend explorer preserves gaps and computes weekly shares from summed counts", async ({ page }) => {
  await fixture(page);
  await page.goto("/admin/usage?days=7");
  await expect(page.getByRole("heading", { name: "Explore trends", exact: true })).toBeVisible();
  await expect(page.locator("#usage-explore-coverage")).toContainText("4 of 7 days");
  // No connecting line across the two interior gaps.
  const path = await page.locator("#usage-explore-lines path").first().getAttribute("d");
  expect(path.match(/M/g)).toHaveLength(3);
  await page.getByLabel("Group by", { exact: true }).selectOption("week");
  await page.getByRole("button", { name: "Previous time period" }).click();
  await expect(page.locator("#usage-inspect-date")).toContainText("Sep 8, 2026 – Sep 13, 2026");
  await expect(page.locator("#usage-inspect-values")).toContainText("5 browser-days · 35.7%");
  await expect(page.locator("#usage-inspect-values")).toContainText("9 browser-days · 64.3%");
  await expect(page.locator("#usage-inspect-note")).toContainText("3 of 6 days");
  await expect(page.locator("#usage-inspect-note")).toContainText("Partial week");
  await page.getByLabel("Measure", { exact: true }).selectOption("share");
  await expect(page.locator("#usage-explore-max")).toHaveText("100%");
  await page.getByLabel("Inspect time period", { exact: true }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.locator("#usage-inspect-date")).toContainText("Sep 14, 2026 · In progress");
  await expect(page.locator("#usage-inspect-values")).toContainText("2 browser-days · 50.0%");
  await page.getByText("Trend data as a table", { exact: true }).click();
  await expect(page.locator("#usage-explore-table tbody tr")).toHaveCount(2);
  await expect(page.locator("#usage-explore-table tbody tr").last()).toContainText("5 · 35.7%");
});

test("trend controls restore from links and retain choices across reporting windows", async ({ page }) => {
  await fixture(page);
  await page.goto("/admin/usage?days=7&compare=screen&interval=week&measure=share");
  await expect(page.getByLabel("Compare", { exact: true })).toHaveValue("screen");
  await expect(page.getByLabel("Measure", { exact: true })).toHaveValue("share");
  // Cropping starts with the observation, not the preceding empty week.
  await expect(page.locator("#usage-explore-first")).toHaveText("Sep 14, 2026");
  await expect(page.getByLabel("Inspect time period", { exact: true })).toBeDisabled();
  await page.getByRole("link", { name: "30 days", exact: true }).click();
  await expect(page.getByLabel("Compare", { exact: true })).toHaveValue("screen");
  await expect(page.getByLabel("Group by", { exact: true })).toHaveValue("week");
  await page.getByLabel("Start at first observation", { exact: true }).uncheck();
  await expect(page.locator("#usage-explore-first")).toHaveText("Sep 8, 2026");
  await expect(page).toHaveURL(/start=window/);
  for (const group of ["prayer-form", "offices"]) {
    await page.getByLabel("Compare", { exact: true }).selectOption(group);
    await expect(page.locator("#usage-inspect-values > div")).toHaveCount(group === "offices" ? 7 : 3);
  }
});

test("empty trend data shows unavailable shares and remains usable", async ({ page }) => {
  await fixture(page, true);
  await page.goto("/admin/usage?days=7&measure=share");
  await expect(page.locator("#usage-explore-empty")).toBeVisible();
  await expect(page.locator("#usage-inspect-note")).toContainText("Shares are unavailable");
  await expect(page.locator("#usage-inspect-values")).toContainText("0 browser-days · —");
  await expect(page.locator(".usage-explorer")).not.toContainText("NaN");
  await page.getByLabel("Group by", { exact: true }).selectOption("week");
  await page.getByRole("button", { name: "Previous time period" }).click();
  await expect(page.locator("#usage-inspect-note")).toContainText("No observations");
});

test("trend inspection works by touch and fits both themes", async ({ page }) => {
  await fixture(page);
  await page.goto("/admin/usage?days=7");
  for (const theme of ["light", "dark"]) {
    await page.evaluate(value => document.documentElement.dataset.theme = value, theme);
    for (const width of [320, 390, 768, 1280]) {
      await page.setViewportSize({ width, height: 844 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      for (const select of await page.locator(".usage-explore-controls select").all()) {
        expect((await select.boundingBox()).height).toBeGreaterThanOrEqual(44);
      }
    }
  }
  await page.setViewportSize({ width: 390, height: 844 });
  const chart = page.locator("#usage-explore-chart");
  await chart.scrollIntoViewIfNeeded();
  const box = await chart.boundingBox();
  await page.touchscreen.tap(box.x + 8, box.y + box.height / 2);
  await expect(page.locator("#usage-inspect-date")).toHaveText("Sep 8, 2026");
  await expect(page.locator("#usage-inspect-values")).toContainText("1 browser-day · 100.0%");
});

test("the report keeps its original summaries when scripting is unavailable", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ baseURL, javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto("/admin/usage?days=7");
  await expect(page.locator(".usage-explorer")).toBeHidden();
  await expect(page.getByRole("heading", { name: "Nave vs Apse" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Desktop vs Mobile" })).toBeVisible();
  await page.getByText("Daily counts by prayer form", { exact: true }).click();
  await expect(page.getByRole("region", { name: "Daily prayer form counts", exact: true })).toBeVisible();
  await context.close();
});
