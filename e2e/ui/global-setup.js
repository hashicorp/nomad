/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { chromium } = require('@playwright/test');

module.exports = async config => {

  var NOMAD_TOKEN = process.env.NOMAD_TOKEN;
  if (NOMAD_TOKEN === undefined || NOMAD_TOKEN === "") {
    return
  }

  var NOMAD_ADDR = process.env.NOMAD_ADDR;
  if (NOMAD_ADDR == undefined || NOMAD_ADDR == "") {
    NOMAD_ADDR = 'http://localhost:4646';
  }

  const browser = await chromium.launch();
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  await page.goto(NOMAD_ADDR+'/ui/settings/tokens');

  // playwright "locater" reference: https://playwright.dev/docs/locators
  // visiting /ui/settings/tokens without a token gets the "anonymous token"
  // automatically, so we need to sign out before we can sign in
  // with a real token.
  await page.getByRole('button', {name: 'Sign Out'}).click();
  // now input the token and sign in
  await page.getByLabel('Secret ID').fill(NOMAD_TOKEN);
  await page.getByRole('button', {name: 'Sign In'}).click();

  const { storageState } = config.projects[0].use;
  await page.context().storageState({ path: storageState });
  await browser.close();
};
