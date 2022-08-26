import { chromium } from '@playwright/test';
import { client, NOMAD_TOKEN } from './api-client.js';
import { PROD_NAMESPACE, DEV_NAMESPACE, OPERATOR_POLICY_JSON, DEV_POLICY_JSON, ANON_POLICY_JSON } from './utils/index.js';

async function globalSetup() {
  if (NOMAD_TOKEN === undefined || NOMAD_TOKEN === "") {
    return
  }

  try {
      await client(`/namespace/prod`, {data: PROD_NAMESPACE});
      await client(`/namespace/dev`, {data: DEV_NAMESPACE});
      await client(`/acl/policy/operator`, {data: OPERATOR_POLICY_JSON});
      await client(`/acl/policy/dev`, {data: DEV_POLICY_JSON});
      await client(`/acl/token/operator`, {data: {Name: "Operator", Type: "client", Policies: ["operator"]}});
      await client(`/acl/token/dev`, {data: {Name: "Developer", Type: "client", Policies: ["dev"]}});
    } catch (e) {
      console.error('ERROR:  ', e)
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

export default globalSetup;
