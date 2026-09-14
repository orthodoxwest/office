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
  await page.getByText("Weekly", { exact: true }).click();
  await page.getByLabel("Inspect time period", { exact: true }).focus();
  await page.keyboard.press("ArrowLeft");
  await expect(page.locator("#usage-inspect-date")).toContainText("Sep 8, 2026 – Sep 13, 2026");
  await expect(page.locator("#usage-inspect-values")).toContainText("5 browser-days (35.7%)");
  await expect(page.locator("#usage-inspect-values")).toContainText("9 browser-days (64.3%)");
  await expect(page.locator("#usage-inspect-note")).toContainText("3 of 6 days");
  await expect(page.locator("#usage-inspect-note")).toContainText("Partial week");
  await page.getByText("%", { exact: true }).click();
  await expect(page.locator("#usage-explore-max")).toHaveText("100%");
  await expect(page.locator("#usage-inspect-values .usage-value-primary").first()).toHaveText("35.7%");
  await page.getByLabel("Inspect time period", { exact: true }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.locator("#usage-inspect-date")).toContainText("Sep 14, 2026 · In progress");
  await expect(page.locator("#usage-inspect-values")).toContainText("50.0% (2 browser-days)");
  await page.getByText("Trend data as a table", { exact: true }).click();
  await expect(page.locator("#usage-explore-table tbody tr")).toHaveCount(2);
  await expect(page.locator("#usage-explore-table tbody tr").last()).toContainText("35.7% (5)");
});

test("trend controls restore from links and retain choices across reporting windows", async ({ page }) => {
  await fixture(page);
  await page.goto("/admin/usage?days=7&compare=screen&interval=week&measure=share");
  await expect(page.getByLabel("Breakdown", { exact: true })).toHaveValue("screen");
  await expect(page.getByRole("radio", { name: "%", exact: true })).toBeChecked();
  // Cropping starts with the observation, not the preceding empty week.
  await expect(page.locator("#usage-explore-first")).toHaveText("Sep 14, 2026");
  await expect(page.getByLabel("Inspect time period", { exact: true })).toBeDisabled();
  await page.getByRole("link", { name: "30 days", exact: true }).click();
  await expect(page.getByLabel("Breakdown", { exact: true })).toHaveValue("screen");
  await expect(page.getByRole("radio", { name: "Weekly", exact: true })).toBeChecked();
  await page.getByLabel("Breakdown", { exact: true }).selectOption("prayer-form");
  await expect(page.locator("#usage-inspect-values > div")).toHaveCount(3);
  await expect(page.getByLabel("Breakdown", { exact: true }).locator("option")).toHaveCount(3);
  await expect(page).not.toHaveURL(/start=/);
});

test("empty trend data shows unavailable shares and remains usable", async ({ page }) => {
  await fixture(page, true);
  await page.goto("/admin/usage?days=7&measure=share");
  await expect(page.locator("#usage-explore-empty")).toBeVisible();
  await expect(page.locator("#usage-inspect-note")).toContainText("Percentages are unavailable");
  await expect(page.locator("#usage-inspect-values")).toContainText("— (0 browser-days)");
  await expect(page.locator(".usage-explorer")).not.toContainText("NaN");
  await page.getByText("Weekly", { exact: true }).click();
  await page.getByLabel("Inspect time period", { exact: true }).focus();
  await page.keyboard.press("ArrowLeft");
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
      for (const select of await page.locator(".usage-explore-controls select, .usage-segments label").all()) {
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
  await expect(page.locator("#usage-inspect-values")).toContainText("1 browser-day (100.0%)");
});

test("the report keeps its original summaries when scripting is unavailable", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ baseURL, javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto("/admin/usage?days=7");
  await expect(page.locator(".usage-explorer")).toBeHidden();
  await expect(page.locator(".usage-breakdown-fallback")).toBeVisible();
  for (const group of ["Appearance", "Screen", "Prayer form"]) {
    await page.getByText(`${group} daily counts`, { exact: true }).click();
    const table = page.getByRole("region", { name: `${group} daily counts`, exact: true });
    await expect(table).toBeVisible();
    await expect(table.locator("tbody tr")).toHaveCount(7);
  }
  await context.close();
});
