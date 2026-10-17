import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  fullyParallel: false,
  timeout: 120000,
  use: {
    baseURL: "http://localhost:18080",
    viewport: { width: 1440, height: 1000 },
    trace: "retain-on-failure",
  },
  globalSetup: "./e2e/setup.ts",
  globalTeardown: "./e2e/teardown.ts",
  reporter: "list",
});
