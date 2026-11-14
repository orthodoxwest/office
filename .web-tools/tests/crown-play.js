// The crown's play, held still for a test. A tap on home's consecration
// cross plays dusk or dawn (app.js, "Home: the crown's play"); these helpers
// start it, stop every clock it runs on once the transition's first frame
// is up (every animation paused, every timer of the play's cleared), and
// set the whole picture to any moment of the timetable by writing each
// animation's currentTime. A frame taken this way is a function of the
// moment asked for and nothing else: not of how long the browser took over
// its stills, nor of when the test got round to looking.
import zlib from "node:zlib";
import { expect } from "@playwright/test";

// presentation::crown: the play's pace and the moment the look turns.
export const PACE = 1.625;
export const TURN = 650;
// The page's rest is no earlier than this (ms from the tap, by the
// timetable); the frames a test may ask for end a little before it, where
// app.js begins to top the picture up from script.
export const LAST = { dusk: 3900, dawn: 3400 };

// Opens home in the look the play leaves, taps the cross, and holds the
// play just after the transition's first frame. No screenshot may be taken
// before this returns: a frame read back while the browser makes the
// transition's stills loses the leaving page's still for the whole play.
export async function holdCrown(page, dir, { date = "2026-03-15" } = {}) {
  await page.addInitScript((stored) => localStorage.setItem("office-theme", stored), dir === "dusk" ? "light" : "dark");
  await page.goto(`/?date=${date}`);
  await page.evaluate(() => document.fonts.ready);
  // Home's paint laid (the wall first), as a reader sees it before the tap.
  await page.waitForFunction(() => !document.documentElement.matches(".wall-first, .wall-coat"));
  const tap = page.locator(".home-crown-tap");
  // Shown once the play is set up, in idle time after load; the stars are
  // fetched, and each has its turn in the play.
  await expect(tap).toBeVisible();
  await expect(page.locator(".home-vault-stars path").first()).toBeAttached();
  await tap.click();
  await page.evaluate(
    () =>
      new Promise((resolve) => {
        const ours = (a) => /^crown-/.test(a.animationName || "") || String((a.effect && a.effect.pseudoElement) || "").startsWith("::view-transition");
        const look = () => {
          const all = document.getAnimations().filter(ours);
          const ref = all.find((a) => a.animationName === "crown-glow-in");
          // The first act is held while the stills are made and resumed in
          // the frame after them: wait for that, and for every start time.
          const ready = ref && document.documentElement.classList.contains("crown-new") && all.every((a) => a.playState !== "paused" && typeof a.startTime === "number");
          if (!ready) {
            requestAnimationFrame(look);
            return;
          }
          // The play's own timers (the top-up before the stills go, the
          // settle, the end) would move the picture under a held frame.
          const top = setTimeout(() => {}, 0);
          for (let id = 0; id <= top; id += 1) clearTimeout(id);
          // Two clocks. What began at the tap (the cross, the light on the
          // horizon) was held through the stills and re-anchored after
          // them, so its start lies a turn and more before the
          // transition's; everything else began with the change of look.
          window.__crownHeld = all.map((a) => ({ a, fromTap: a.startTime < ref.startTime - 500 }));
          all.forEach((a) => a.pause());
          resolve();
        };
        look();
      }),
  );
  expect(await page.evaluate(() => [document.documentElement.getAttribute("data-crown-plain"), document.documentElement.getAttribute("data-crown-early")])).toEqual([null, null]);
}

// Sets the held play to `at`, in the timetable's ms from the tap (after the
// turn). Each part is placed on its own clock, exactly: what began at the
// tap at `at`, what began with the change of look at `at` less the turn
// (both times the pace, which is what the page's own clock runs at).
export async function seekCrown(page, at) {
  await page.evaluate(
    ([tap, turn]) => {
      window.__crownHeld.forEach((p) => {
        p.a.currentTime = p.fromTap ? tap : turn;
      });
      // Two frames: the seek is drawn before anyone looks.
      return new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    },
    [at * PACE, (at - TURN) * PACE],
  );
}

// A PNG's pixels (8-bit RGB or RGBA, not interlaced: what a screenshot is).
export function pixels(png) {
  const width = png.readUInt32BE(16), height = png.readUInt32BE(20), kind = png[25];
  const n = kind === 6 ? 4 : 3;
  if (png[24] !== 8 || (kind !== 2 && kind !== 6) || png[28] !== 0) throw new Error("not a plain 8-bit PNG");
  const chunks = [];
  for (let at = 8; at < png.length; ) {
    const len = png.readUInt32BE(at), type = png.toString("latin1", at + 4, at + 8);
    if (type === "IDAT") chunks.push(png.subarray(at + 8, at + 8 + len));
    at += 12 + len;
  }
  const raw = zlib.inflateSync(Buffer.concat(chunks)), stride = width * n;
  const out = Buffer.alloc(stride * height);
  for (let y = 0; y < height; y += 1) {
    const filter = raw[y * (stride + 1)], from = y * (stride + 1) + 1, to = y * stride;
    for (let i = 0; i < stride; i += 1) {
      const a = i >= n ? out[to + i - n] : 0, b = y ? out[to + i - stride] : 0, c = y && i >= n ? out[to + i - stride - n] : 0;
      let v = raw[from + i];
      if (filter === 1) v += a;
      else if (filter === 2) v += b;
      else if (filter === 3) v += (a + b) >> 1;
      else if (filter === 4) {
        const p = a + b - c, pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
        v += pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
      }
      out[to + i] = v & 255;
    }
  }
  return { width, height, n, data: out };
}

// The luminances (0-255) of a box of the picture, sorted. Boxes are in CSS
// px; the picture may be finer.
export function lumas(img, box, scale = 1) {
  const out = [];
  const x0 = Math.max(0, Math.round(box.x * scale)), x1 = Math.min(img.width, Math.round((box.x + box.width) * scale));
  const y0 = Math.max(0, Math.round(box.y * scale)), y1 = Math.min(img.height, Math.round((box.y + box.height) * scale));
  for (let y = y0; y < y1; y += 1) {
    for (let x = x0; x < x1; x += 1) {
      const i = (y * img.width + x) * img.n;
      out.push(0.2126 * img.data[i] + 0.7152 * img.data[i + 1] + 0.0722 * img.data[i + 2]);
    }
  }
  return out.sort((a, b) => a - b);
}
export const mean = (v) => v.reduce((a, b) => a + b, 0) / v.length;
export const centile = (v, p) => v[Math.min(v.length - 1, Math.floor(p * v.length))];
// WCAG contrast of two luminances given 0-255.
export function contrast(a, b) {
  const rel = (v) => (v / 255 <= 0.03928 ? v / 255 / 12.92 : ((v / 255 + 0.055) / 1.055) ** 2.4);
  const hi = Math.max(a, b), lo = Math.min(a, b);
  return (rel(hi) + 0.05) / (rel(lo) + 0.05);
}
