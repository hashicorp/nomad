/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const puppeteer = require("puppeteer");
const {
  capture,
  wait,
  error,
  click,
  clickJob,
  clickTab,
  clickMainMenu,
  clickX
} = require("./utils");

const HOST = process.env.EMBER_HOST || "http://localhost:4200";
console.log(`Using host ${HOST}...`);

const ANSI_YELLOW = "\x1b[33m%s\x1b[0m";

(async () => {
  const startTime = Date.now();
  console.log("Preparing puppeteer...");

  // Create a new browser and tab
  const browser = await puppeteer.launch({
    // Docker related chrome flags
    args: [
      "--no-sandbox",
      "--disable-setuid-sandbox",
      "--disable-dev-shm-usage",
      "--remote-debugging-port=9222",
    ]
  });
  const page = await browser.newPage();

  // Make sure the page is 4K is high-dpi scaling
  page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
  console.log("Loading Nomad UI...");
  console.log(
    ANSI_YELLOW,
    "\n!! Make sure to use the everyFeature Mirage scenario !!\n"
  );

  try {
    await page.goto(`${HOST}/ui/`);
  } catch (err) {
    await error(
      browser,
      "Could not load the Nomad UI. Is the Ember server running?"
    );
  }

  // Give Mirage a chance to settle
  console.log("Waiting for Mirage...");
  await wait(5000);
  console.log("Starting capture sequence!\n");

  // DEBUG: log the URL on all navigations
  monitorURL(page);

  await capture(page, "jobs-list");

  await clickJob(page, "service");
  await capture(page, "job-detail-service");
  await page.goBack();

  await clickJob(page, "batch");
  await capture(page, "job-detail-batch");
  await page.goBack();

  await clickJob(page, "system");
  await capture(page, "job-detail-system");
  await page.goBack();

  await clickJob(page, "periodic");
  await capture(page, "job-detail-periodic");
  await click(page, "tr.job-row");
  await capture(page, "job-detail-periodic-child");
  await page.goBack();
  await page.goBack();

  await clickJob(page, "parameterized");
  await capture(page, "job-detail-parameterized");
  await click(page, "tr.job-row");
  await capture(page, "job-detail-parameterized-child");
  await page.goBack();
  await page.goBack();

  await clickJob(page, "service");

  await clickTab(page, "Definition");
  await capture(page, "job-detail-tab-definition");
  await page.click(".boxed-section .button.is-light.is-compact.pull-right");
  await capture(page, "job-detail-tab-definition-editing");

  await clickTab(page, "Versions");
  await capture(page, "job-detail-tab-versions");
  await page.click(".timeline-object .button.is-light.is-compact.pull-right");
  await capture(page, "job-detail-tab-versions-expanded");

  await clickTab(page, "Deployments");
  await capture(page, "job-detail-tab-deployments");
  await page.click(".timeline-object .button.is-light.is-compact.pull-right");
  await capture(page, "job-detail-tab-deployments-expanded");

  await clickTab(page, "Allocations");
  await capture(page, "job-detail-tab-allocations");

  await clickTab(page, "Evaluations");
  await capture(page, "job-detail-tab-evaluations");

  await clickMainMenu(page, "Jobs");
  await page.click(".toolbar-item .button.is-primary");
  await capture(page, "job-run-empty");
  // Fill in the code editor somehow
  // Capture the plan stage

  await clickMainMenu(page, "Jobs");
  await clickJob(page, "service");
  await click(page, ".task-group-row");
  await capture(page, "task-group");

  await clickMainMenu(page, "Jobs");
  await clickJob(page, "service");

  const allocCount = await page.$$eval(".allocation-row", s => s.length);
  for (let i = 1; i <= allocCount; i++) {
    await click(page, `.allocation-row:nth-of-type(${i}) a.is-primary`);
    await capture(page, `allocation-${i}`);
    await page.goBack();
    await wait(2000);
  }

  await click(page, ".allocation-row a.is-primary");
  await click(page, ".task-row");
  await capture(page, "task-detail");

  await clickTab(page, "Logs");
  await capture(page, "task-logs");

  await clickMainMenu(page, "Clients");
  await capture(page, "clients-list");

  const clientCount = await page.$$eval(".client-node-row", s => s.length);
  for (let i = 1; i <= clientCount; i++) {
    await click(page, `.client-node-row:nth-of-type(${i})`);
    await capture(page, `client-detail-${i}`);
    await page.goBack();
    await wait(500);
  }

  await clickMainMenu(page, "Servers");
  await capture(page, "servers-list");

  await click(page, `.server-agent-row:nth-of-type(2)`);
  await capture(page, "server-detail");

  await clickX(page, '//a[contains(text(), "ACL Tokens")]');
  await capture(page, "acl-tokens");

  console.log(`All done! ${humanDuration(Date.now() - startTime)}`);
  process.exit();
})();

async function* watchURL(page) {
  while (true) {
    await page.waitForNavigation();
    yield page.url();
  }
}

async function monitorURL(page) {
  for await (let url of watchURL(page)) {
    console.log(`=> ${url}`);
  }
}

function humanDuration(duration) {
  const ms = duration % 1000;
  const s = Math.floor((duration / 1000) % 60);
  const m = Math.floor(duration / 1000 / 60);

  const fs = s < 10 ? `0${s}` : `${s}`;
  const fms = ms < 10 ? `00${ms}` : ms < 100 ? `0${ms}` : `${ms}`;

  if (m) return `${m}m ${fs}s ${fms}ms`;
  else if (s) return `${fs}s ${fms}ms`;
  return `${fms}ms`;
}
