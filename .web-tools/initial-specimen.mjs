// Every capital as a psalm's dropped initial on the web, one image per letter, for review beside
// the Android app's InitialSpecimenTest (the same verse and opening words). Recalibrate both when
// the initial's face, sizing or profiles change.
//
//   ./office serve 127.0.0.1:8099 &
//   node .web-tools/initial-specimen.mjs [http://127.0.0.1:8099]   → output/initials/web-A.png …
import { chromium } from "playwright";
import fs from "node:fs";

const base = process.argv[2] || "http://127.0.0.1:8099";
const words = ["And", "Blessed", "Come", "Deliver", "Except", "From", "God", "Have", "In", "Judge", "Keep", "Lord", "My",
  "Not", "O", "Praise", "Quicken", "Rejoice", "Save", "The", "Unto", "Verily", "When", "Xerxes", "Ye", "Zion"];
const tail = " the Lord, O my soul, and all that is within me <span class=\"mediant\">*</span> praise his holy Name, for he is gracious, and his mercy endureth for ever.";
const out = "output/initials";
fs.mkdirSync(out, { recursive: true });

const browser = await chromium.launch();
// The service worker would answer from its cache, past the rewritten page.
const context = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 3, serviceWorkers: "block" });
const page = await context.newPage();
const url = `${base}/lauds/2026-03-15`;
const original = await (await fetch(url)).text();
for (const word of words) {
  const html = original.replace(/<div class="psalm-verses"><p class="verse">[\s\S]*?<\/p>/, `<div class="psalm-verses"><p class="verse">${word}${tail}</p>`);
  await page.route(url, (route) => route.fulfill({ body: html, contentType: "text/html" }));
  await page.goto(url);
  await page.waitForTimeout(400);
  const y = await page.locator(".psalm-verses .verse").first().evaluate((verse) => verse.getBoundingClientRect().top + verse.ownerDocument.defaultView.scrollY);
  await page.screenshot({ path: `${out}/web-${word[0]}.png`, fullPage: true, clip: { x: 0, y: y - 8, width: 390, height: 120 } });
  await page.unroute(url);
}
await browser.close();
