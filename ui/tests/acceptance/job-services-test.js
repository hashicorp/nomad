/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { find, findAll, currentURL, settled } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import { allScenarios } from '../../mirage/scenarios/default';

import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Services from 'nomad-ui/tests/pages/jobs/job/services';

module('Acceptance | job services', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  hooks.beforeEach(async function () {
    allScenarios.servicesTestCluster(server);
    await Services.visit({ id: 'service-haver@default' });
  });

  test('Visiting job services', async function (assert) {
    assert.expect(3);
    assert.dom('.tabs.is-subnav a.is-active').hasText('Services');
    assert.dom('.service-list table').exists();
    await a11yAudit(assert);
  });

  test('it shows both consul and nomad, and both task and group services', async function (assert) {
    assert.dom('table tr[data-test-service-provider="consul"]').exists();
    assert.dom('table tr[data-test-service-provider="nomad"]').exists();
    assert.dom('table tr[data-test-service-level="task"]').exists();
    assert.dom('table tr[data-test-service-level="group"]').exists();
  });

  test('Digging into a service', async function (assert) {
    const expectedNumAllocs = find(
      '[data-test-service-level="group"]'
    ).getAttribute('data-test-num-allocs');
    const serviceName = find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"]'
    ).getAttribute('data-test-service-name');

    await find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"] a'
    ).click();
    await settled();

    assert.ok(
      currentURL().includes(`services/${serviceName}?level=group`),
      'correctly traverses to a service instance list'
    );

    assert.equal(
      findAll('tr[data-test-service-row]').length,
      expectedNumAllocs,
      'Same number of alloc rows as the index shows'
    );
  });
});
