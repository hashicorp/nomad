import { chromium } from '@playwright/test';
import { execaSync } from 'execa';

async function globalSetup() {
    // Run Nomad Commands
    execaSync('nomad namespace apply -description "Prod" prod')
    execaSync('nomad namespace apply -description "Dev" dev')
    execaSync('nomad acl policy apply -description "Operator" operator ./policies/operator.hcl')
    execaSync('nomad acl policy apply -description "Power User" dev ./policies/developer.hcl')
    execaSync('nomad acl policy apply -description "Anonymous" anon ./policies/anon.hcl')
    execaSync('nomad acl token create -name="E2E Operator" -policy=operator -type=client | tee operator.token')
    execaSync('nomad acl token create -name="E2E Power User" -policy=dev -type=client | tee dev.token')
    execaSync('nomad acl token create -name="E2E Anonymous User" -policy=anon -type=client | tee anon.token')  
  
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

export default globalSetup;
