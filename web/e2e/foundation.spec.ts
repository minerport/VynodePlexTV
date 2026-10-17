import { test, expect } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";

test("complete owner journey, remote navigation, logout, and container persistence", async ({
  page,
  request,
}) => {
  const password = randomBytes(24).toString("hex");
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  expect((await request.get("/api/v1/health")).ok()).toBe(true);
  expect((await request.get("/api/v1/ready")).ok()).toBe(true);
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Make yourself at home." }),
  ).toBeVisible();
  await page.getByLabel("Username").fill("test-owner");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Create owner account" }).click();
  await expect(page.getByRole("status")).toContainText("Owner account created");
  await page.getByLabel("Password").fill("wrong-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("incorrect");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Great television. Your collection." }),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/live-desktop.png",
    fullPage: true,
  });
  const nav = page.getByRole("navigation");
  await nav.getByRole("link", { name: "Live TV", exact: true }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(
    nav.getByRole("link", { name: "Guide", exact: true }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("heading", { name: "No programming yet" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(
    nav.getByRole("link", { name: "Live TV", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await nav.getByRole("link", { name: "Channels", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Your channel collection is waiting" }),
  ).toBeVisible();
  await nav.getByRole("link", { name: "Plex Setup", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Plex connection is coming next" }),
  ).toBeVisible();
  await nav.getByRole("link", { name: "Settings", exact: true }).click();
  await page.getByLabel("Television name").fill("Evening room");
  await page.getByLabel("Guide timezone").fill("America/New_York");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByRole("status")).toHaveText("Settings saved.");
  await page.getByLabel("Television name").focus();
  await page.keyboard.press("ArrowLeft");
  await expect(page.getByLabel("Television name")).toBeFocused();
  execFileSync(
    "docker",
    ["compose", "-p", "afterglow-test", "-f", "compose.test.yaml", "restart"],
    { cwd: "..", stdio: "inherit" },
  );
  await expect
    .poll(async () => {
      try {
        return (await request.get("/api/v1/ready")).status();
      } catch {
        return 0;
      }
    })
    .toBe(200);
  await page.reload();
  await expect(page.getByLabel("Television name")).toHaveValue("Evening room");
  await expect(page.getByLabel("Guide timezone")).toHaveValue(
    "America/New_York",
  );
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(
    page.getByRole("heading", { name: "Welcome back." }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Welcome back." }),
  ).toBeVisible();
  await page.getByLabel("Username").fill("test-owner");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByLabel("Television name")).toHaveValue("Evening room");
  await nav.getByRole("link", { name: "Live TV", exact: true }).click();
  await page.setViewportSize({ width: 768, height: 1024 });
  await page.screenshot({
    path: "test-results/live-tablet.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: "test-results/live-mobile.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
});
