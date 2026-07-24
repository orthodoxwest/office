import { defineConfig } from "@playwright/test";

const goFlags = [process.env.GOFLAGS, "-buildvcs=false"].filter(Boolean).join(" ");
const baseURL = process.env.PLAYWRIGHT_BASE_URL || "http://127.0.0.1:18159";
const externalServer = Boolean(process.env.PLAYWRIGHT_EXTERNAL_SERVER);

export default defineConfig({
  testDir: "./tests",
  outputDir: "../output/playwright/test-results",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ["line"],
    ["html", { outputFolder: "../output/playwright/report", open: "never" }],
  ],
  expect: {
    toHaveScreenshot: {
      animations: "disabled",
      caret: "hide",
      maxDiffPixelRatio: 0.0005,
    },
  },
  use: {
    baseURL,
    locale: "en-US",
    timezoneId: "America/New_York",
    serviceWorkers: "block",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: externalServer
    ? undefined
    : {
        command: "go run ./cmd/server serve 127.0.0.1:18159",
        cwd: "..",
        env: {
          ...process.env,
          GOFLAGS: goFlags,
        },
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
  projects: [
    {
      name: "mobile-chromium",
      use: {
        browserName: "chromium",
        viewport: { width: 390, height: 844 },
        hasTouch: true,
        isMobile: true,
        colorScheme: "light",
      },
    },
  ],
});
