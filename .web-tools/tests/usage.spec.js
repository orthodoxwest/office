import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("visible office visits send an event, background page fetches do not", async ({ page }) => {
  const events = [];
  await page.route("**/api/usage", async route => {
    events.push(route.request().postData());
    await route.fulfill({ status: 204 });
  });
  await page.goto("/lauds/2026-09-04");
  await expect.poll(() => events.length).toBe(1);
  expect(events).toEqual(["lauds"]);
  await page.evaluate(async () => {
    await fetch("/vespers/2026-09-04");
    document.dispatchEvent(new Event("visibilitychange"));
  });
  expect(events).toEqual(["lauds"]);
  await page.goto("/?date=2026-09-04");
  await expect.poll(() => events.length).toBe(2);
  expect(events[1]).toBe("site");
  await page.goto("/admin/usage?days=7");
  await expect(page.getByRole("heading", { name: "Daily usage", exact: true })).toBeVisible();
  expect(events.length).toBe(2);
});

test("usage report is accessible and fits narrow and wide screens", async ({ page }) => {
  const response = await page.goto("/admin/usage?days=7");
  expect(response.headers()["cache-control"]).toBe("no-store");
  expect(response.headers()["x-robots-tag"]).toContain("noindex");
  await expect(page.locator("tbody tr")).toHaveCount(7);
  for (const theme of ["light", "dark"]) {
    await page.evaluate(theme => document.documentElement.setAttribute("data-theme", theme), theme);
    for (const width of [320, 390, 540, 768, 1280, 1920]) {
      await page.setViewportSize({ width, height: 844 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    }
    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  }
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
