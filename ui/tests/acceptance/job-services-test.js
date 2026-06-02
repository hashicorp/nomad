/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { find, findAll, currentURL, settled } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import { allScenarios } from '../../mirage/scenarios/default';
import removeRecord from 'nomad-ui/utils/remove-record';

import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Services from 'nomad-ui/tests/pages/jobs/job/services';

module('Acceptance | job services', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  hooks.beforeEach(async function () {
    allScenarios.servicesTestCluster(this.server);
    await Services.visit({ id: 'service-haver@default' });
  });

  test('Visiting job services', async function (assert) {
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
      '[data-test-service-level="group"]',
    ).getAttribute('data-test-num-allocs');
    const serviceName = find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"]',
    ).getAttribute('data-test-service-name');

    await find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"] a',
    ).click();
    await settled();

    assert.ok(
      currentURL().includes(`services/${serviceName}?level=group`),
      'correctly traverses to a service instance list',
    );

    assert.strictEqual(
      findAll('tr[data-test-service-row]').length,
      Number(expectedNumAllocs),
      'Same number of alloc rows as the index shows',
    );
  });

  test('service detail page handles registrations being deleted from under the job', async function (assert) {
    const serviceName = find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"]',
    ).getAttribute('data-test-service-name');
    const initialCount = Number(
      find(
        '[data-test-service-level="group"][data-test-service-provider="nomad"]',
      ).getAttribute('data-test-num-allocs'),
    );

    await find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"] a',
    ).click();
    await settled();

    assert.strictEqual(
      findAll('tr[data-test-service-row]').length,
      initialCount,
      'detail page renders the expected number of instances',
    );

    // Simulate what reloadRelationship + removeRecord do when `nomad service
    // delete` runs on the backend: null out relationships (which removes the
    // service from job.services via inverse) and unload the record. The route
    // observer should pick up the job.services.length change and refresh.
    const store = this.owner.lookup('service:store');
    const toRemove = store
      .peekAll('service')
      .find((s) => s.name === serviceName && s.derivedLevel === 'group');

    if (toRemove) {
      removeRecord(store, toRemove);
    }

    await settled();

    assert.strictEqual(
      findAll('tr[data-test-service-row]').length,
      initialCount - 1,
      'detail page updates after a service registration is removed',
    );
    assert.dom('.empty-message-headline').doesNotExist();
  });

  test('service detail page shows empty state when there are no live instances', async function (assert) {
    const serviceName = find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"]',
    ).getAttribute('data-test-service-name');

    // Remove every live registration for this service before navigating in.
    const store = this.owner.lookup('service:store');
    store
      .peekAll('service')
      .filter((s) => s.name === serviceName && s.derivedLevel === 'group')
      .forEach((s) => removeRecord(store, s));
    await settled();

    await find(
      '[data-test-service-level="group"][data-test-service-provider="nomad"] a',
    ).click();
    await settled();

    assert
      .dom('[data-test-empty-service-instances]')
      .exists('empty state is shown when no live registrations exist');
    assert.dom('tr[data-test-service-row]').doesNotExist();
  });
});
