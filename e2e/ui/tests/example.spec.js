import { test, expect } from "@playwright/test";

test("authenticated users can see their policies", async ({ page }) => {
  var NOMAD_PROXY_ADDR = process.env.NOMAD_PROXY_ADDR;
  if (NOMAD_PROXY_ADDR == undefined || NOMAD_PROXY_ADDR == "") {
    NOMAD_PROXY_ADDR = "http://localhost:4646";
  }

  await page.goto(NOMAD_PROXY_ADDR + "/ui/settings/tokens");

  // smoke test that we reached the page
  const logo = page.locator("div.navbar-brand");
  await expect(logo).toBeVisible();

  const policies = page.locator("text=Policies");
  await expect(policies).toBeVisible();
});
