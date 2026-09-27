import { defineConfig } from "@playwright/test";

const goFlags = [process.env.GOFLAGS, "-buildvcs=false"].filter(Boolean).join(" ");
const baseURL = process.env.PLAYWRIGHT_BASE_URL || "http://127.0.0.1:18159";
const externalServer = Boolean(process.env.PLAYWRIGHT_EXTERNAL_SERVER);
// Rust is the default; the retained Go reference is opt-in during cutover.
const serverCommand = process.env.PLAYWRIGHT_SERVER === "go"
  ? "go run ./cmd/server serve 127.0.0.1:18159"
  : "target/release/office serve 127.0.0.1:18159";
// Separate CI invocations must retain both reports and their failure traces.
const outputRoot = process.env.PLAYWRIGHT_SUITE
  ? `../output/playwright/${process.env.PLAYWRIGHT_SUITE}`
  : "../output/playwright";

export default defineConfig({
  testDir: "./tests",
  outputDir: `${outputRoot}/test-results`,
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ["line"],
    ["html", { outputFolder: `${outputRoot}/report`, open: "never" }],
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
        command: `mkdir -p output/playwright && ${serverCommand}`,
        cwd: "..",
        env: {
          ...process.env,
          GOFLAGS: goFlags,
          OFFICE_USAGE_DB: process.env.OFFICE_USAGE_DB || "output/playwright/usage.sqlite",
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
