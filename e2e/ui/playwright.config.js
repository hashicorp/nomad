// playwright.config.js
// @ts-check
import { devices } from "@playwright/test";
import { devices as replayDevices } from "@replayio/playwright";

const inCI = !!process.env.CI;

/** @type {import('@playwright/test').PlaywrightTestConfig} */
const config = {
  forbidOnly: inCI,
  retries: inCI ? 2 : 0,
  timeout: !inCI ? 30 * 1000 : 10 * 1000,
  workers: !inCI ? 1 : undefined,
  use: {
    trace: "on-first-retry",
    ignoreHTTPSErrors: true,
    launchOptions: {
      slowMo: inCI ? 0 : 500,
    }
  },
  globalSetup: "global-setup.js",
  globalTeardown: "global-teardown.js",
  projects: [
    {
      name: "replay-chromium",
      use: { ...replayDevices["Replay Chromium"] },
    },
    {
      name: "replay-firefox",
      use: { ...replayDevices["Replay Firefox"] },
    },
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
    // disabling firefox temporarily because the container doesn't
    // include it and so it tries to automatically install it and
    // that takes longer than the timeout (30s). once we've got
    // some value out of these tests we can customize the container
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: "webkit",
      use: { ...devices["Desktop Safari"] },
    },
  ],
};

export default config;
