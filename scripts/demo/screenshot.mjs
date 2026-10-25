// Drives the real Runyx web UI with Playwright against a real `runyx
// serve` + real llama-server + real model, and saves the screenshots and
// a short video the demo.yml workflow publishes. No mocked data: every
// screenshot reflects what the running daemon actually returns.
import { chromium } from "playwright";
import fs from "node:fs";
import path from "node:path";

const BASE_URL = process.env.RUNYX_BASE_URL ?? "http://127.0.0.1:11535";
const OUT_DIR = process.env.RUNYX_OUT_DIR ?? "demo-output";
const SHOTS_DIR = path.join(OUT_DIR, "screenshots");
const VIDEO_DIR = path.join(OUT_DIR, "video");
const PROMPT = process.env.RUNYX_PROMPT ?? "Explain in one sentence what you are.";

// Keeps the demo deterministic and bounded in time: a short, capped,
// temperature-0 reply instead of however long the real default
// generation would take on a CI CPU. Chat.svelte only honors these when
// present on the page URL, so a normal user's conversation is never
// affected. See docs/DECISIONS.md.
const DEMO_MAX_TOKENS = process.env.RUNYX_DEMO_MAX_TOKENS ?? "64";
const DEMO_TEMPERATURE = process.env.RUNYX_DEMO_TEMPERATURE ?? "0";
const DEMO_URL = `${BASE_URL}/?demoMaxTokens=${DEMO_MAX_TOKENS}&demoTemp=${DEMO_TEMPERATURE}`;

// How long to wait for the streaming cursor to disappear. Generous: even
// a capped, deterministic reply can be slow on a CI CPU running the
// deliberately-de-optimized "safe" kernel (see RUNYX_LLAMA_CPP_SAFE_CPU_KERNEL
// in demo.yml) — this is a ceiling against a genuine hang, not a budget
// we expect to need in full.
const CURSOR_TIMEOUT_MS = Number(process.env.RUNYX_DEMO_CURSOR_TIMEOUT_MS ?? "300000");

// A minimal valid 1x1 PNG, used to exercise the real attachment-upload
// pipeline without shipping a binary fixture in the repo.
const TINY_PNG_B64 =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=";

function ensureDir(p) {
  fs.mkdirSync(p, { recursive: true });
}

async function killAnimations(page) {
  await page.addStyleTag({
    content: `*, *::before, *::after {
      animation-duration: 0s !important;
      animation-delay: 0s !important;
      transition-duration: 0s !important;
      transition-delay: 0s !important;
      caret-color: transparent !important;
    }`,
  });
}

async function setPrefs(page, theme, reduceMotion) {
  await page.evaluate(
    ({ theme, reduceMotion }) => {
      try {
        localStorage.setItem("runyx.theme", theme);
        localStorage.setItem("runyx.reduceMotion", reduceMotion ? "1" : "0");
      } catch {
        /* ignore */
      }
    },
    { theme, reduceMotion }
  );
}

async function gotoReady(page) {
  await page.goto(DEMO_URL, { waitUntil: "load" });
  await killAnimations(page);
  // Not `nav`: the sidebar is deliberately display:none below 760px
  // (phone width), so waiting for it to be *visible* would never
  // resolve on the phone pass. `main` (the content area) is always
  // rendered regardless of viewport.
  await page.waitForSelector("main", { timeout: 15000 });
}

async function openTab(page, label) {
  await page.locator(".tab", { hasText: label }).click();
  await page.waitForTimeout(300);
  await killAnimations(page);
}

async function sendAndWait(page, text, timeoutMs = CURSOR_TIMEOUT_MS) {
  await page.locator("textarea").first().fill(text);
  await page.locator(".send").first().click();
  // Streaming started: a cursor marker appears, then disappears once the
  // reply is persisted back as a normal message bubble.
  await page.waitForSelector(".cursor", { timeout: 15000 }).catch(() => {});
  await page.waitForSelector(".cursor", { state: "detached", timeout: timeoutMs });
  await page.waitForTimeout(400); // let the DOM settle after the final refresh
}

async function shoot(page, file) {
  ensureDir(path.dirname(file));
  await page.screenshot({ path: file });
}

async function newChat(page) {
  await page.locator(".quick-link", { hasText: "New chat" }).click();
  await page.waitForTimeout(300);
}

async function runThemePass(browser, theme) {
  const dir = path.join(SHOTS_DIR, theme);
  ensureDir(dir);

  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    reducedMotion: "reduce",
    colorScheme: theme,
  });
  const page = await context.newPage();
  page.on("dialog", async (d) => {
    console.error(`[${theme}] unexpected alert(): ${d.message()}`);
    await d.accept();
  });

  await gotoReady(page);
  await setPrefs(page, theme, true);
  await page.reload({ waitUntil: "load" });
  await killAnimations(page);
  await page.waitForTimeout(300);

  // 1. Home (greeting + centered composer + shortcuts, or the "pick a
  // model" invite if nothing is installed yet).
  await shoot(page, path.join(dir, "01-home.png"));

  // 2. Ctrl+K command palette.
  await page.keyboard.down("Control");
  await page.keyboard.press("k");
  await page.keyboard.up("Control");
  await page.waitForSelector(".palette", { timeout: 5000 });
  await shoot(page, path.join(dir, "02-command-palette.png"));
  await page.keyboard.press("Escape");
  await page.waitForTimeout(150);

  // 3. A real conversation with a completed real model response, sent
  // from the home screen's centered composer.
  await sendAndWait(page, PROMPT);
  await shoot(page, path.join(dir, "03-conversation-response.png"));

  // 4. An attachment added to a message (before it's sent).
  const pngPath = path.join(OUT_DIR, "tmp-attachment.png");
  ensureDir(OUT_DIR);
  fs.writeFileSync(pngPath, Buffer.from(TINY_PNG_B64, "base64"));
  await page.locator('button[title="Attach a file"]').click();
  await page.setInputFiles('input[type="file"]', pngPath);
  await page.waitForSelector(".chip", { timeout: 10000 });
  await shoot(page, path.join(dir, "04-attachment.png"));
  await page.locator(".send").first().click();
  await page.waitForSelector(".cursor", { state: "detached", timeout: CURSOR_TIMEOUT_MS }).catch(() => {});
  await page.waitForTimeout(300);

  // 5. Models page: Installed / Recommended / Browse all.
  await openTab(page, "Models");
  await shoot(page, path.join(dir, "05-models.png"));

  // 6. Agent page, then mobile pairing with its QR code.
  await openTab(page, "Agent");
  await shoot(page, path.join(dir, "06-agent.png"));
  await page.locator("button", { hasText: "Pair a device" }).click();
  await page.waitForTimeout(200);
  await page.locator(".page button", { hasText: "Generate QR code" }).click();
  await page.waitForSelector("img.qr", { timeout: 10000 }).catch(() => {});
  await page.waitForTimeout(300);
  await shoot(page, path.join(dir, "07-pairing-qrcode.png"));

  // 7. Settings (appearance).
  await openTab(page, "Chat");
  await page.waitForTimeout(150);
  await page.locator(".quick-link", { hasText: "Settings" }).click();
  await page.waitForTimeout(200);
  await shoot(page, path.join(dir, "08-settings.png"));

  // 8. Sidebar collapsed, showing the device card icon-only.
  await openTab(page, "Chat");
  await page.waitForTimeout(150);
  await page.locator(".collapse-btn").click();
  await page.waitForTimeout(250);
  await shoot(page, path.join(dir, "09-sidebar-collapsed.png"));

  await context.close();
}

async function runPhonePass(browser) {
  const dir = SHOTS_DIR;
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  await gotoReady(page);
  await setPrefs(page, "light", true);
  await page.reload({ waitUntil: "load" });
  await killAnimations(page);
  await page.waitForTimeout(300);

  await shoot(page, path.join(dir, "phone-01-home.png"));

  // No sidebar at this width (it's display:none below 760px) and so no
  // "New chat" quick-link to click — send straight from the home
  // screen's composer, same as the desktop passes do; that implicitly
  // creates the conversation.
  await sendAndWait(page, PROMPT);
  await shoot(page, path.join(dir, "phone-02-chat.png"));

  await context.close();
}

async function recordVideo(browser) {
  ensureDir(VIDEO_DIR);
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    reducedMotion: "reduce",
    recordVideo: { dir: VIDEO_DIR, size: { width: 1440, height: 900 } },
  });
  const page = await context.newPage();
  await gotoReady(page);
  await setPrefs(page, "light", true);
  await page.reload({ waitUntil: "load" });
  await killAnimations(page);
  await newChat(page);
  await page.waitForTimeout(500);
  await sendAndWait(page, PROMPT);
  await page.waitForTimeout(800);

  const video = page.video();
  await context.close();
  if (video) {
    const savedPath = await video.path();
    const finalPath = path.join(VIDEO_DIR, "conversation.webm");
    fs.renameSync(savedPath, finalPath);
    console.log(`video saved to ${finalPath}`);
  }
}

async function main() {
  ensureDir(OUT_DIR);
  const launchOpts = {};
  if (process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH) {
    launchOpts.executablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
  }
  const browser = await chromium.launch(launchOpts);
  const errors = [];

  try {
    for (const theme of ["light", "dark"]) {
      console.log(`== theme: ${theme} ==`);
      await runThemePass(browser, theme);
    }
    console.log("== phone viewport ==");
    await runPhonePass(browser);
    console.log("== conversation video ==");
    await recordVideo(browser);
  } catch (err) {
    errors.push(err);
    console.error(err);
  } finally {
    await browser.close();
  }

  if (errors.length > 0) {
    process.exit(1);
  }
}

main();
