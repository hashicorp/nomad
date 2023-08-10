/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL } from '@ember/test-helpers';
import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import ClientMonitor from 'nomad-ui/tests/pages/clients/monitor';
import Layout from 'nomad-ui/tests/pages/layout';

let node;
let managementToken;
let clientToken;

module('Acceptance | client monitor', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    server.create('node-pool');
    node = server.create('node');

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.nomadTokenSecret = managementToken.secretId;

    server.create('agent');
    run.later(run, run.cancelTimers, 500);
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);

    await ClientMonitor.visit({ id: node.id });
    await a11yAudit(assert);
  });

  test('/clients/:id/monitor should have a breadcrumb trail linking back to clients', async function (assert) {
    await ClientMonitor.visit({ id: node.id });

    assert.equal(Layout.breadcrumbFor('clients.index').text, 'Clients');
    assert.equal(
      Layout.breadcrumbFor('clients.client').text,
      `Client ${node.id.split('-')[0]}`
    );

    await Layout.breadcrumbFor('clients.index').visit();
    assert.equal(currentURL(), '/clients');
  });

  test('the monitor page immediately streams agent monitor output at the info level', async function (assert) {
    await ClientMonitor.visit({ id: node.id });

    const logRequest = server.pretender.handledRequests.find((req) =>
      req.url.startsWith('/v1/agent/monitor')
    );
    assert.ok(ClientMonitor.logsArePresent);
    assert.ok(logRequest);
    assert.ok(logRequest.url.includes('log_level=info'));
  });

  test('switching the log level persists the new log level as a query param', async function (assert) {
    await ClientMonitor.visit({ id: node.id });
    await ClientMonitor.selectLogLevel('Debug');
    assert.equal(currentURL(), `/clients/${node.id}/monitor?level=debug`);
  });

  test('when the current access token does not include the agent:read rule, a descriptive error message is shown', async function (assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await ClientMonitor.visit({ id: node.id });
    assert.notOk(ClientMonitor.logsArePresent);
    assert.ok(ClientMonitor.error.isShown);
    assert.equal(ClientMonitor.error.title, 'Not Authorized');
    assert.ok(ClientMonitor.error.message.includes('agent:read'));

    await ClientMonitor.error.seekHelp();
    assert.equal(currentURL(), '/settings/tokens');
  });
});
