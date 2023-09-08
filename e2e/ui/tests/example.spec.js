/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('@playwright/test');

test('authenticated users can see their policies', async ({ page }) => {

  var NOMAD_ADDR = process.env.NOMAD_ADDR;
  if (NOMAD_ADDR == undefined || NOMAD_ADDR == "") {
    NOMAD_ADDR = 'http://localhost:4646';
  }

  await page.goto(NOMAD_ADDR+'/ui/settings/tokens');

  // smoke test that we reached the page
  const logo = page.locator('div.navbar-brand');
  await expect(logo).toBeVisible();


  const policies = page.locator('text=Policies')
  await expect(policies).toBeVisible();
});
