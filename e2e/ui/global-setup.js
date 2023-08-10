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
  await page.fill('input[id="token-input"]', NOMAD_TOKEN);
  await page.click('button:has-text("Set Token")', {strict: true});

  await page.context().storageState({ path: 'storageState.json' });
  await browser.close();
};
