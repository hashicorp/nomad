import fs from "fs";
import { chromium } from "@playwright/test";
import { client, NOMAD_PROXY_ADDR, NOMAD_ADDR, NOMAD_TOKEN } from "./api-client.js";
import {
  PROD_NAMESPACE,
  DEV_NAMESPACE,
  OPERATOR_POLICY_JSON,
  DEV_POLICY_JSON,
} from "./utils/index.js";

const formatToken = token => `export const token = "${token}"`;

async function globalSetup() {
  if (NOMAD_TOKEN === undefined || NOMAD_TOKEN === "") {
    return;
  }

  try {
    await client(`/namespace/prod`, { data: PROD_NAMESPACE });
    await client(`/namespace/dev`, { data: DEV_NAMESPACE });
    await client(`/acl/policy/operator`, { data: OPERATOR_POLICY_JSON });
    await client(`/acl/policy/dev`, { data: DEV_POLICY_JSON });

    // Create Operator Token and save to local director to use in tests
    const {data: {SecretID: operatorToken}} = await client(`/acl/token`, {
      data: { Name: "Operator", Type: "client", Policies: ["operator"] },
    });
    fs.writeFileSync('input/tokens/operator.js', formatToken(operatorToken));

    // Create Developer Token and save to local director to use in tests
    const {data: {SecretID: devToken}} = await client(`/acl/token`, {
      data: { Name: "Developer", Type: "client", Policies: ["dev"] },
    });
    fs.writeFileSync('input/tokens/dev.js', formatToken(devToken));
  } catch (e) {
    console.error("ERROR:  ", e);
  }

  const browser = await chromium.launch();
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  await page.goto(NOMAD_PROXY_ADDR + "/ui/settings/tokens");
  await page.fill('input[id="token-input"]', NOMAD_TOKEN);
  await page.click('button:has-text("Set Token")', { strict: true });

  await page.context().storageState({ path: "storageState.json" });
  await browser.close();
}

export default globalSetup;
