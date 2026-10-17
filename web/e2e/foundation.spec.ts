import { test, expect } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { channelJourney } from "./channel-journey";

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
  const nav = page.getByRole("navigation", { name: "Main navigation" });
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
    page.getByRole("heading", { name: "Your first channel starts here." }),
  ).toBeVisible();
  await nav.getByRole("link", { name: "Plex Setup", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Your library. Center stage." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Add Plex server" }).click();
  await page.getByLabel("Server URL").fill("http://plex-fixture:32400");
  await page
    .getByLabel("Plex token", { exact: true })
    .fill(process.env.PLEX_FIXTURE_TOKEN!);
  await page
    .getByRole("button", { name: "Test connection", exact: true })
    .click();
  await expect(page.getByText("Connected to Test Cinema")).toBeVisible();
  await page
    .getByRole("button", { name: "Connect server", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Test Cinema", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Libraries", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Test Movies", exact: true }),
  ).toBeVisible({ timeout: 15000 });
  for (const name of ["Test Movies", "Test TV"]) {
    const row = page
      .locator(".library-row")
      .filter({ has: page.getByRole("heading", { name, exact: true }) });
    await row
      .getByRole("button", { name: "Select library", exact: true })
      .click();
    await expect(
      row.getByRole("button", { name: "Selected", exact: true }),
    ).toBeVisible();
  }
  await page
    .getByRole("button", { name: "Sync selected", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Media browser", exact: true })
    .click();
  await expect
    .poll(
      async () => {
        const response = await page.request.get("/api/v1/media");
        return (await response.json()).total;
      },
      { timeout: 20000 },
    )
    .toBe(4);
  await page.getByRole("button", { name: "Jobs", exact: true }).click();
  await expect(page.getByText("SUCCEEDED").first()).toBeVisible();
  await page
    .getByRole("button", { name: "Media browser", exact: true })
    .click();
  await expect(page.getByText("Test Movie", { exact: true })).toBeVisible();
  await expect(page.locator(".media-card")).toHaveCount(2);
  await page.locator(".media-card").filter({ hasText: "Test Show" }).click();
  await page.locator(".media-card").filter({ hasText: "Season One" }).click();
  await page.locator(".media-card").filter({ hasText: "Test Episode" }).click();
  await expect(
    page.getByRole("region", { name: "Media details" }),
  ).toContainText("Test Episode");
  await page
    .getByRole("navigation", { name: "TV library hierarchy" })
    .getByRole("button", { name: "Test Show", exact: true })
    .click();
  await expect(page.locator(".media-card")).toContainText("Season One");
  await page
    .getByRole("navigation", { name: "TV library hierarchy" })
    .getByRole("button", { name: "Movies & shows" })
    .click();
  await expect(page.locator(".media-card")).toHaveCount(2);
  const movie = page.locator(".media-card").filter({ hasText: "Test Movie" });
  await expect
    .poll(() =>
      movie
        .locator("img")
        .evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBeGreaterThan(0);
  await movie.click();
  await expect(
    page.getByRole("region", { name: "Media details" }),
  ).toBeVisible();
  await expect
    .poll(() =>
      page
        .getByAltText("Background artwork")
        .evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBeGreaterThan(0);
  await page.screenshot({
    path: "test-results/plex-media.png",
    fullPage: true,
  });
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
  await nav.getByRole("link", { name: "Plex Setup", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Test Cinema", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Test", exact: true }).click();
  await expect
    .poll(
      async () => {
        const response = await page.request.get("/api/v1/jobs");
        const jobs = await response.json();
        return jobs.find(
          (job: { type: string; status: string }) =>
            job.type === "PLEX_HEALTH_CHECK",
        )?.status;
      },
      { timeout: 15000 },
    )
    .toBe("SUCCEEDED");
  await page
    .getByRole("button", { name: "Sync selected", exact: true })
    .click();
  await expect
    .poll(
      async () => {
        const response = await page.request.get("/api/v1/jobs");
        const jobs = await response.json();
        return jobs.filter(
          (job: { type: string; status: string }) =>
            job.type === "PLEX_LIBRARY_SYNC" &&
            ["RUNNING", "QUEUED"].includes(job.status),
        ).length;
      },
      { timeout: 15000 },
    )
    .toBe(0);
  const inventory = await (await page.request.get("/api/v1/media")).json();
  expect(inventory.total).toBe(4);
  await page
    .getByRole("button", { name: "Media browser", exact: true })
    .click();
  await expect(page.getByText("Test Movie", { exact: true })).toBeVisible();
  await expect
    .poll(() =>
      page
        .locator(".media-card img")
        .first()
        .evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBeGreaterThan(0);
  await nav.getByRole("link", { name: "Settings", exact: true }).click();
  await channelJourney(page);
  await nav.getByRole("link", { name: "Settings", exact: true }).click();
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
