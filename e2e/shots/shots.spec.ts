import { expect, test, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

// Images for the README and the landing page, made with the real app and
// the real model: `make shots` writes them to e2e/out. Cards come from the
// app's own Download card button.

const root = path.resolve(__dirname, "../..");
const photos = path.join(root, "testdata/photos");
mkdirSync(path.join(root, "e2e/out/cards"), { recursive: true });

// Only photos with no person in them, from the published set.
const cardPhotos = [
  "01-cracked-phone.jpg",
  "02-frayed-charger-cable.jpg",
  "03-worn-out-shoes.jpg",
  "04-burnt-toast.jpg",
  "05-broken-mug.jpg",
  "06-broken-drinking-glass.jpg",
  "07-collapsing-jack-o-lanterns.jpg",
  "08-burnt-light-bulb.jpg",
  "09-dead-succulent.jpg",
  "10-warped-cassette-adapter.jpg",
  "11-abandoned-umbrella.jpg",
  "16-intact-mug.jpg",
];

async function upload(page: Page, file: string) {
  await page.locator("#photo-file").setInputFiles(path.join(photos, file));
  await expect(page.locator("#card").or(page.locator(".notice"))).toBeVisible();
  await page.waitForTimeout(1800); // let the ink settle
}

test.describe.configure({ mode: "serial" });
test.use({ viewport: { width: 1280, height: 1000 }, deviceScaleFactor: 2 });

test("cards", async ({ page }) => {
  test.setTimeout(10 * 60_000);
  const cards: Record<string, unknown>[] = [];
  for (const file of cardPhotos) {
    await page.goto("/");
    await upload(page, file);
    const card = page.locator("#card");
    const text = await card.evaluate((el) => ({
      name: el.querySelector(".name")?.textContent,
      years: el.querySelector(".years")?.textContent,
      deck: el.querySelector(".deck")?.textContent,
      paragraphs: [...el.querySelectorAll("p")].map((p) => p.textContent),
    }));
    const details = await page.locator(".details-list li").evaluateAll((els) =>
      els.map((e) => e.textContent),
    );
    const repaired = await page.locator(".repaired li").evaluateAll((els) => els.map((e) => e.textContent));
    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.getByRole("button", { name: "Download card" }).click(),
    ]);
    const name = `card-${file.replace(/\.jpg$/, "")}.png`;
    await download.saveAs(path.join(root, "e2e/out/cards", name));
    cards.push({ file, png: name, ...text, details, repaired });
  }
  writeFileSync(path.join(root, "e2e/out/cards/cards.json"), JSON.stringify(cards, null, 2));
});

test("app on desktop and phone", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 1480 });
  await page.goto("/");
  await page.screenshot({ path: path.join(root, "e2e/out/app-idle.png") });
  await upload(page, "04-burnt-toast.jpg");
  await page.locator(".result").evaluate((el) => window.scrollTo(0, el.getBoundingClientRect().top + window.scrollY - 24));
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(root, "e2e/out/app-desktop.png") });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.screenshot({ path: path.join(root, "e2e/out/app-phone-idle.png") });
  await upload(page, "07-collapsing-jack-o-lanterns.jpg");
  await page.screenshot({ path: path.join(root, "e2e/out/app-phone.png") });
});

test("still-alive notices", async ({ page }) => {
  for (const [file, out] of [
    ["12-cat.jpg", "notice-cat.png"],
    ["14-healthy-monstera.jpg", "notice-plant.png"],
    ["13-dog.jpg", "notice-dog.png"],
  ]) {
    await page.goto("/");
    await upload(page, file);
    const box = await page.locator(".notice").boundingBox();
    await page.screenshot({
      path: path.join(root, "e2e/out", out),
      clip: { x: box!.x - 28, y: box!.y - 24, width: box!.width + 56, height: box!.height + 48 },
    });
  }
});

test("pipeline diagram", async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 600 });
  await page.goto("file://" + path.join(__dirname, "pipeline.html"));
  await page.evaluate(() => document.fonts.ready);
  await page.locator("#frame").screenshot({ path: path.join(root, "e2e/out/pipeline.png") });
});

test("cover", async ({ browser }) => {
  // The DEV cover is exactly 1000 x 420.
  const page = await browser.newPage({ viewport: { width: 1000, height: 420 }, deviceScaleFactor: 1 });
  const first = process.env.COVER_FIRST ?? "card-03-worn-out-shoes.png";
  const second = process.env.COVER_SECOND ?? "card-04-burnt-toast.png";
  await page.goto("file://" + path.join(__dirname, "cover.html"));
  await page.evaluate(
    ([a, b]) => {
      (document.getElementById("first") as HTMLImageElement).src = a;
      (document.getElementById("second") as HTMLImageElement).src = b;
    },
    ["file://" + path.join(root, "e2e/out/cards", first), "file://" + path.join(root, "e2e/out/cards", second)],
  );
  await page.waitForFunction(() => [...document.images].every((i) => i.complete && i.naturalWidth > 0));
  await page.evaluate(() => document.fonts.ready);
  await page.locator("#cover").screenshot({ path: path.join(root, "e2e/out/cover.png") });
  await page.close();
});
