import { test, expect } from "@playwright/test";
import { client, NOMAD_TOKEN, NOMAD_PROXY_ADDR } from "../api-client.js";
import { jobSpec } from "../input/jobs/service-discovery/service-discovery.js";

test.describe("Watchers", () => {
  test.beforeEach(async ({ page }) => {
    // Go to the starting url before each test.
    await client(`/jobs`, { data: jobSpec });
    await page.goto(NOMAD_PROXY_ADDR + "/ui/settings/tokens");

    // Set token
    await page
      .locator('[placeholder="XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX"]')
      .fill(NOMAD_TOKEN);
    await page.locator("text=Set Token").click();
  });

  test.afterEach(async ({ page }) => {
    // Purge jobs to not leak details out
    await client(`/job/trying-multi-dupes`, { method: "delete" }, [
      "purge=true",
    ]);

    await page.goto(NOMAD_PROXY_ADDR + "/ui/settings/tokens");
    // Clear token
    await page.locator("text=ACL Tokens").click();
    await expect(page).toHaveURL(NOMAD_PROXY_ADDR + "/ui/settings/tokens");
    await page.locator("text=Clear Token").click();
  });

  test("should reactively update the page when the server detects a change", async ({
    page,
  }) => {
    // Click text=Jobs
    await Promise.all([
      page.waitForNavigation(),
      page.locator("text=Jobs").click(),
    ]);

    // Click text=trying-multi-dupes
    await Promise.all([
      // Waits for the next request with the specified url
      page.waitForRequest(
        `${NOMAD_PROXY_ADDR}/v1/job/trying-multi-dupes?index=**`
      ),
      // Triggers the request
      page.locator("text=trying-multi-dupes").click(),
    ]);

    await expect(page.locator(".task-group-row >> nth=0 >> td >> nth=1")).toHaveText("3");

    const [req] = await Promise.all([
      client(`/job/trying-multi-dupes`, {
        data: { Job: {...jobSpec.Job, TaskGroups: [{...jobSpec.Job.TaskGroups[0], Count: 7}, ...jobSpec.Job.TaskGroups.slice(1)]} },
      }),
      page.waitForRequest(`${NOMAD_PROXY_ADDR}/v1/job/trying-multi-dupes?index=**`),
    ]);

    await expect(page.locator(".task-group-row >> nth=0 >> td >> nth=1")).toHaveText("7");

    let allocId;
    const setAllocId = (page) => {
      const urlArr = page.split("/");
      allocId = urlArr[urlArr.length - 1];
    };

    await Promise.all([
      // Waits for the next request with the specified url
      page.waitForRequest(`${NOMAD_PROXY_ADDR}/v1/allocation/**?index=**`),
      // Triggers the request
      page.locator(".allocation-row").first().click(),
    ]);

    setAllocId(page.url());

    await expect(page).toHaveURL(
      `${NOMAD_PROXY_ADDR}/ui/allocations/${allocId}`
    );
  });
});
