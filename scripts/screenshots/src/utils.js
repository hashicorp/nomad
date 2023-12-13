/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

async function error(browser, message = "Something went wrong.") {
  console.error(message);
  await browser.close();
  process.exit();
}

async function capture(page, name, options = {}) {
  console.log(`Capturing ${name}`);
  const dir = process.env.SCREENSHOTS_DIR || "screenshots";
  await page.screenshot(
    Object.assign({ path: `${dir}/${name}.png`, fullPage: true }, options)
  );
}

async function wait(time) {
  return new Promise(resolve => {
    setTimeout(resolve, time);
  });
}

async function click(page, selector, options) {
  const [response] = await Promise.all([
    page.waitForNavigation(),
    page.click(selector, options)
  ]);

  // Allow for render
  await wait(500);

  return response;
}

async function clickX(page, path) {
  const [element] = await page.$x(path);
  const [response] = await Promise.all([
    page.waitForNavigation(),
    element.click()
  ]);

  // Allow for render
  await wait(500);

  return response;
}

async function clickJob(page, type) {
  let jobIndex = await page.$$eval(
    "tr.job-row",
    (rows, type) =>
      rows.findIndex(
        row => row.querySelector("td:nth-child(3)").textContent.trim() === type
      ),
    type
  );
  jobIndex++;

  await clickX(page, `//tr[contains(@class, "job-row")][${jobIndex}]`);
}

async function clickTab(page, label) {
  let tabIndex = await page.$$eval(
    ".tabs.is-subnav a",
    (tabs, label) => tabs.findIndex(tab => tab.textContent.trim() === label),
    label
  );
  tabIndex++;

  await clickX(
    page,
    `//div[contains(@class, "is-subnav")]//ul//li[${tabIndex}]//a`
  );
}

async function clickMainMenu(page, label) {
  await clickX(
    page,
    `//div[contains(@class, "page-column is-left")]//a[contains(text(), "${label}")]`
  );
}

module.exports = {
  error,
  capture,
  wait,
  click,
  clickX,
  clickJob,
  clickTab,
  clickMainMenu
};
