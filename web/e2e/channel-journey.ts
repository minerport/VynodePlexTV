import { expect, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
export async function channelJourney(page: Page) {
  await page
    .getByRole("navigation")
    .getByRole("link", { name: "Channels", exact: true })
    .click();
  await page.getByRole("button", { name: "Create channel" }).click();
  await page
    .getByLabel("Channel name", { exact: true })
    .fill("Handpicked Test");
  await expect(
    page.getByRole("button", { name: /Test Movie.*Select/ }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Select this page", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "2 selected · ordered membership" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Save channel", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Handpicked Test", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Create channel" }).click();
  await page.getByLabel("Channel name", { exact: true }).fill("Show Test");
  await page
    .getByRole("combobox", { name: "Channel type", exact: true })
    .selectOption("SHOW");
  await page
    .getByRole("button", { name: "Choose tv show", exact: true })
    .click();
  await page.getByRole("button", { name: /Test Show.*seasons/ }).click();
  await expect(
    page.getByRole("heading", { name: "1 playable items", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Save channel", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Show Test", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Create channel" }).click();
  await page
    .getByLabel("Channel name", { exact: true })
    .fill("Collection Test");
  await page
    .getByRole("combobox", { name: "Channel type", exact: true })
    .selectOption("COLLECTION");
  await page
    .getByRole("button", { name: "Choose plex collection", exact: true })
    .click();
  await page
    .getByRole("button", { name: /Test Universe/ })
    .first()
    .click();
  await page.getByRole("button", { name: "Save channel", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Collection Test", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Create channel" }).click();
  await page
    .getByLabel("Channel name", { exact: true })
    .fill("Nested Rule Test");
  await page
    .getByRole("combobox", { name: "Channel type", exact: true })
    .selectOption("RULE");
  await page.getByRole("button", { name: "Add group", exact: false }).click();
  await page
    .getByRole("combobox", { name: "Group logic", exact: true })
    .last()
    .selectOption("NOT");
  await page
    .getByRole("combobox", { name: "Field", exact: true })
    .last()
    .selectOption("year");
  await page.getByLabel("Value", { exact: true }).fill("1990");
  await expect(
    page.getByRole("heading", { name: "1 playable items", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Save channel", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Nested Rule Test", exact: true }),
  ).toBeVisible();
  const card = page.locator(".channel-card").filter({
    has: page.getByRole("heading", { name: "Nested Rule Test", exact: true }),
  });
  await card.getByRole("button", { name: "Duplicate", exact: true }).click();
  const copy = page.locator(".channel-card").filter({
    has: page.getByRole("heading", {
      name: "Nested Rule Test Copy",
      exact: true,
    }),
  });
  await copy.getByRole("button", { name: "Disable", exact: true }).click();
  await expect(
    copy.getByRole("button", { name: "Enable", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/channels-desktop.png",
    fullPage: true,
  });
  execFileSync(
    "docker",
    [
      "compose",
      "-p",
      "afterglow-test",
      "-f",
      "compose.test.yaml",
      "restart",
      "app",
    ],
    { cwd: "..", stdio: "inherit" },
  );
  await expect
    .poll(async () => {
      try {
        return (await page.request.get("/api/v1/ready")).status();
      } catch {
        return 0;
      }
    })
    .toBe(200);
  await page.reload();
  await expect(
    copy.getByRole("button", { name: "Enable", exact: true }),
  ).toBeVisible();
  await copy.getByRole("button", { name: "Delete", exact: true }).click();
  await page
    .getByRole("button", { name: "Confirm delete channel", exact: true })
    .click();
  await expect(copy).toHaveCount(0);
  await page.setViewportSize({ width: 768, height: 1024 });
  await page.screenshot({
    path: "test-results/channels-tablet.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.setViewportSize({ width: 1440, height: 1000 });
}
