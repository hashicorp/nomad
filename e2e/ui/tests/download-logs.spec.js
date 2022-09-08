import { test, expect } from "@playwright/test";
import { client, NOMAD_ADDR } from "../api-client.js";
import { jobSpec } from "../input/jobs/service-discovery/service-discovery.js";
import { token } from "../input/tokens/operator.js";

test.describe("Download Task Logs", () => {
  test.beforeEach(async ({ page }) => {
    // Go to the starting url before each test.
    await client(`/jobs`, { data: jobSpec });
    await page.goto(NOMAD_ADDR + "/ui/settings/tokens");

    // Set token
    await page
      .locator('[placeholder="XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX"]')
      .fill(token);
    await page.locator("text=Set Token").click();
  });

  test.afterEach(async ({ page }) => {
    // Purge jobs to not leak details out
    await client(`/job/trying-multi-dupes`, { method: "delete" }, [
      "purge=true",
    ]);

    // Clear token
    await page.locator("text=ACL Tokens").click();
    await expect(page).toHaveURL("http://localhost:4200/ui/settings/tokens");
    await page.locator("text=Clear Token").click();
  });

  test("should be able to search for a task group, find a task and download raw file", async ({
    page,
  }) => {
    // Arrange -- Naviagate to Task Logs
    await page.goto(NOMAD_ADDR + "/ui/jobs");
    await Promise.all([
      page.waitForNavigation(),
      page.locator("text=trying-multi-dupes").click(),
    ]);
    await page.locator(".allocation-row").first().click();
    await page.locator("text=http.server").click();
    await Promise.all([
      page.waitForNavigation(),
      page.locator("text=Files").click(),
    ]);
    await page.locator("text=executor.out").click();
    
    // Act - Download Log
    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.locator("text=View Raw File").click(),
    ]);

    // Assert that the file is stored
    const path = await download.path();
    await expect(path).toBeTruthy();
  });
});
