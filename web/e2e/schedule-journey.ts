import { expect, type Page } from "@playwright/test";
export async function scheduleJourney(page: Page) {
  const session = await (await page.request.get("/api/v1/auth/session")).json();
  const headers = {
    Origin: "http://localhost:18080",
    "X-CSRF-Token": session.csrfToken,
  };
  const channels = await (await page.request.get("/api/v1/channels")).json();
  const c = channels.items.find(
    (c: { enabled: boolean; count: number }) => c.enabled && c.count > 0,
  );
  expect(c).toBeTruthy();
  const base = `/api/v1/channels/${c.id}/schedule`;
  const r = await page.request.post(base + "/generate", { headers, data: {} });
  expect(r.status()).toBe(202);
  const { jobId } = await r.json();
  await expect
    .poll(
      async () =>
        (await (await page.request.get("/api/v1/jobs/" + jobId)).json()).status,
      { timeout: 15000 },
    )
    .toBe("SUCCEEDED");
  const status = await (await page.request.get(base + "/status")).json();
  expect(status.integrity).toEqual([]);
  expect(status.programCount).toBeGreaterThan(0);
  const at = await (await page.request.get(base + "/at")).json();
  expect(at.current.end).toBe(at.next.start);
  await page
    .getByRole("button", { name: "Scheduling studio", exact: true })
    .click();
  await page
    .locator("article")
    .filter({ has: page.getByRole("heading", { name: c.name, exact: true }) })
    .getByRole("button", { name: "Open schedule" })
    .click();
  await expect(page.locator(".schedule-programs li").first()).toBeVisible();
  await page.getByText("Scheduling settings", { exact: true }).click();
  await page.getByLabel("Scheduling mode").selectOption("SMART_RANDOM");
  await page.getByLabel("Movie repeat window (days)").fill("14");
  await page
    .getByRole("button", { name: "Save schedule settings", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Settings saved");
  await page
    .getByRole("button", { name: "Regenerate future schedule", exact: true })
    .click();
  await expect(page.getByRole("alertdialog")).toBeVisible();
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await page
    .getByRole("button", { name: "Repair future", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("queued");
  await page.getByRole("button", { name: "Next day", exact: true }).click();
  await page.screenshot({
    path: "test-results/schedule-preview.png",
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("heading", { name: "Your channels, around the clock." }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("button", { name: "Scheduling studio", exact: true }),
  ).toBeVisible();
}
