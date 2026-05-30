// Capture console screenshots for the docs/README.
//
// Usage (with a runtime already running, e.g. `make run`):
//   npx --yes playwright@1.47 install chromium
//   URL=http://localhost:8080 node scripts/screenshots.mjs
//
// It signs up a throwaway account via the API, injects the session token, then
// captures each console view into docs/assets/screenshots/*.png.
import { chromium } from "playwright";
import fs from "node:fs";
import path from "node:path";

const URL = process.env.URL || "http://localhost:8080";
const OUT = path.resolve("docs/assets/screenshots");
fs.mkdirSync(OUT, { recursive: true });

const views = [
  ["overview", "Overview"],
  ["runtimes", "Runtimes"],
  ["catalog", "Catalog"],
  ["sandboxes", "Sandboxes"],
  ["models", "Models"],
  ["agents", "Jobs"],
  ["policies", "Policies"],
  ["audit", "Audit"],
  ["settings", "Settings"],
  ["install", "Add a runtime"],
  ["matrixshell", "MatrixShell"],
];

async function token() {
  const email = `shots+${Date.now()}@example.com`;
  const r = await fetch(`${URL}/v1/auth/signup`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "Maya Chen", email, password: "screenshots1" }),
  });
  if (!r.ok) throw new Error(`signup failed: ${r.status}`);
  return (await r.json()).token;
}

const tok = await token();
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 });
await ctx.addInitScript((t) => localStorage.setItem("matrixcloud_token", t), tok);
const page = await ctx.newPage();

for (const [route, label] of views) {
  await page.goto(`${URL}/`, { waitUntil: "networkidle" });
  // Click the matching nav item (or top action) by visible label.
  const target = page.getByRole("button", { name: label, exact: false }).first();
  try { await target.click({ timeout: 2000 }); } catch { /* overview is default */ }
  await page.waitForTimeout(900);
  const file = path.join(OUT, `${route}.png`);
  await page.screenshot({ path: file });
  console.log("captured", file);
}

await browser.close();
console.log(`\nDone → ${OUT}`);
