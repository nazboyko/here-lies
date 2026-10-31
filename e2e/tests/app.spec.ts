import { expect, test, type Page, type Request } from "@playwright/test";
import { networkInterfaces } from "node:os";
import { readFileSync, statSync } from "node:fs";
import path from "node:path";

const photos = path.resolve(__dirname, "../../testdata/photos");
const photo = (name: string) => path.join(photos, name);
const out = path.resolve(__dirname, "../out");

function lanIP(): string | undefined {
  for (const list of Object.values(networkInterfaces())) {
    for (const a of list ?? []) {
      if (a.family === "IPv4" && !a.internal && !a.address.startsWith("169.254.")) return a.address;
    }
  }
}

// bury uploads a photo through the real file input and waits for the
// obituary card or the still-alive notice.
async function bury(page: Page, file: string): Promise<"obituary" | "notice"> {
  await page.locator("#photo-file").setInputFiles(file);
  const card = page.locator("#card");
  const notice = page.locator(".notice");
  await expect(card.or(notice)).toBeVisible();
  return (await card.count()) > 0 ? "obituary" : "notice";
}

async function noHorizontalScroll(page: Page) {
  const { scroll, inner } = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    inner: window.innerWidth,
  }));
  expect(scroll).toBeLessThanOrEqual(inner);
}

test.describe("desktop", () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test("a dead thing gets an obituary that cites what was seen, and downloads as a PNG", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Here Lies" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Choose a file" })).toBeVisible();

    const progress = page.locator(".progress");
    const kind = bury(page, photo("03-worn-out-shoes.jpg"));
    await expect(progress).toContainText(/Examining the remains|Writing the obituary/);
    expect(await kind).toBe("obituary");

    const card = page.locator("#card");
    await expect(card.locator(".name")).not.toBeEmpty();
    await expect(card.locator(".years")).toHaveText(/^(est\. .+|Unknown) – \w+ \d+, \d{4}$/);
    await expect(card).toContainText("Cause of death:");

    // Every life sentence on screen points at a detail listed under "How does it know?".
    const sentences = card.locator(".life span");
    const n = await sentences.count();
    expect(n).toBeGreaterThanOrEqual(1);
    for (let i = 0; i < n; i++) {
      await expect(sentences.nth(i)).not.toBeEmpty();
      const ids = ((await sentences.nth(i).getAttribute("data-cites")) ?? "").split(" ").filter(Boolean);
      expect(ids.length, `life sentence ${i + 1} cites no detail`).toBeGreaterThanOrEqual(1);
      for (const id of ids) {
        const item = page.locator(`.details-list li[data-id="${id}"]`);
        await expect(item, `life sentence ${i + 1} cites ${id}`).toHaveCount(1);
        await expect(item).toContainText(`life, sentence ${i + 1}`);
      }
    }
    // The footer prints the real detail texts.
    const items = page.locator(".details-list li");
    expect(await items.count()).toBeGreaterThanOrEqual(2);
    for (const text of await items.locator("> div").evaluateAll((els) =>
      els.map((e) => (e.firstChild?.textContent ?? "").trim()),
    )) {
      await expect(card.locator(".seen")).toContainText(text.slice(1));
    }

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.getByRole("button", { name: "Download card" }).click(),
    ]);
    expect(download.suggestedFilename()).toMatch(/^here-lies-[a-z0-9-]+\.png$/);
    const file = path.join(out, download.suggestedFilename());
    await download.saveAs(file);
    const head = readFileSync(file).subarray(0, 8);
    expect(head.equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]))).toBe(true);
    expect(statSync(file).size).toBeGreaterThan(30 * 1024);

    await page.getByRole("button", { name: "Bury another" }).click();
    await expect(card).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Choose a file" })).toBeFocused();
  });

  test("an animal is still alive", async ({ page }) => {
    await page.goto("/");
    expect(await bury(page, photo("12-cat.jpg"))).toBe("notice");
    await expect(page.getByRole("status")).toContainText("Still alive.");
    await expect(page.getByRole("status")).toContainText("Give it a treat instead.");
    await expect(page.locator("#card")).toHaveCount(0);
    await page.getByRole("button", { name: "Try another photo" }).click();
    await expect(page.getByRole("status")).toHaveCount(0);
  });

  test("a healthy plant is still with us", async ({ page }) => {
    await page.goto("/");
    expect(await bury(page, photo("14-healthy-monstera.jpg"))).toBe("notice");
    await expect(page.getByRole("status")).toContainText("This plant is still with us.");
    await expect(page.locator("#card")).toHaveCount(0);
  });

  test("keyboard reaches every action in order", async ({ page }) => {
    await page.goto("/");
    await page.keyboard.press("Tab");
    await expect(page.getByRole("button", { name: "Choose a file" })).toBeFocused();
    expect(await bury(page, photo("04-burnt-toast.jpg"))).toBe("obituary");

    const reached: string[] = [];
    for (let i = 0; i < 8; i++) {
      await page.keyboard.press("Tab");
      reached.push(await page.evaluate(() => (document.activeElement as HTMLElement | null)?.innerText?.trim() ?? ""));
    }
    const order = ["Download card", "Bury another", "What the model saw in the photo"].map((t) =>
      reached.findIndex((r) => r.startsWith(t)),
    );
    expect(order.every((i) => i >= 0), `reached: ${reached.join(" | ")}`).toBe(true);
    expect(order).toEqual([...order].sort((a, b) => a - b));

    // Enter on the summary folds the list, Enter on Bury another starts over.
    await page.locator("summary").focus();
    await page.keyboard.press("Enter");
    await expect(page.locator("details.know")).not.toHaveAttribute("open", "");
    await page.getByRole("button", { name: "Bury another" }).focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("button", { name: "Choose a file" })).toBeFocused();
  });

  test("nothing leaves the machine during a full run", async ({ page }) => {
    const hosts = new Set<string>();
    page.on("request", (r: Request) => hosts.add(new URL(r.url()).hostname));
    await page.goto("/");
    expect(await bury(page, photo("08-burnt-light-bulb.jpg"))).toBe("obituary");
    await Promise.all([page.waitForEvent("download"), page.getByRole("button", { name: "Download card" }).click()]);

    const allowed = new Set(["localhost", "127.0.0.1", lanIP() ?? "127.0.0.1"]);
    const outside = [...hosts].filter((h) => h !== "" && !allowed.has(h));
    expect(outside, `requests left the machine: ${outside.join(", ")}`).toEqual([]);
    expect(hosts.has("localhost")).toBe(true);
  });
});

test.describe("phone", () => {
  test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

  test("upload works at phone width without sideways scrolling", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("button", { name: "Take a photo" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Choose a file" })).toBeVisible();
    await noHorizontalScroll(page);

    expect(await bury(page, photo("01-cracked-phone.jpg"))).toBe("obituary");
    await expect(page.locator("#card .name")).not.toBeEmpty();
    await expect(page.getByRole("button", { name: "Download card" })).toBeVisible();
    await noHorizontalScroll(page);
  });
});

test.describe("problems", () => {
  test("a model that is not pulled says how to get it", async ({ page }) => {
    await page.goto("http://127.0.0.1:8789/");
    await expect(page.getByRole("alert")).toHaveText(
      "The model is not on this computer yet. Get it with: ollama pull missing-model",
    );
    const api = await page.request.post("http://127.0.0.1:8789/api/obituary", {
      multipart: { photo: { name: "toast.jpg", mimeType: "image/jpeg", buffer: readFileSync(photo("04-burnt-toast.jpg")) } },
    });
    expect(api.status()).toBe(503);
    expect((await api.json()).error).toBe("The model is not on this computer yet. Get it with: ollama pull missing-model");

    const response = page.waitForResponse((r) => r.url().endsWith("/api/obituary"));
    await page.locator("#photo-file").setInputFiles(photo("04-burnt-toast.jpg"));
    await response;
    await expect(page.getByRole("alert")).toHaveText(
      "The model is not on this computer yet. Get it with: ollama pull missing-model",
    );
    await expect(page.getByRole("button", { name: "Choose a file" })).toBeEnabled();
    await expect(page.locator("#card")).toHaveCount(0);
  });

  test("Ollama not running says how to start it", async ({ page }) => {
    await page.goto("http://127.0.0.1:8790/");
    await expect(page.getByRole("alert")).toHaveText("Can't reach the model. Start Ollama with: ollama serve");
  });

  test("a file that is not a photo is turned away in one sentence", async ({ page }) => {
    await page.goto("/");
    await page.locator("#photo-file").setInputFiles({
      name: "notes.txt",
      mimeType: "text/plain",
      buffer: Buffer.from("not a photo"),
    });
    await expect(page.getByRole("alert")).toHaveText(
      "This file isn't a photo we can read. Use a JPEG, PNG or WebP picture.",
    );
  });
});

test.describe("LAN", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("the printed LAN address works for a whole run", async ({ page }) => {
    const ip = lanIP();
    test.skip(!ip, "no LAN address on this machine");
    await page.goto(`http://${ip}:8788/`);
    expect(await bury(page, photo("06-broken-drinking-glass.jpg"))).toBe("obituary");
    await expect(page.locator("#card .name")).not.toBeEmpty();
  });
});
