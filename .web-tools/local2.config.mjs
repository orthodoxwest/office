// Local run on the pre-installed Chromium (not committed).
import base from "./playwright.config.mjs";
export default { ...base, use: { ...base.use, launchOptions: { executablePath: "/opt/pw-browsers/chromium-1194/chrome-linux/chrome" } } };
