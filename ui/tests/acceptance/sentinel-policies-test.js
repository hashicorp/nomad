/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { findAll, fillIn, find, click, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { allScenarios } from '../../mirage/scenarios/default';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import Administration from 'nomad-ui/tests/pages/administration';
import percySnapshot from '@percy/ember';

module('Acceptance | sentinel policies', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    window.localStorage.clear();
    window.sessionStorage.clear();
    allScenarios.policiesTestCluster(server, { sentinel: true });
    await Tokens.visit();
    const managementToken = server.db.tokens.findBy(
      (t) => t.type === 'management'
    );
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Administration.visitSentinelPolicies();
  });

  hooks.afterEach(async function () {
    await Tokens.visit();
    await Tokens.clear();
  });

  test('Sentinel Policies index, general', async function (assert) {
    // assert.expect(3);
    // await a11yAudit(assert);

    assert.equal(currentURL(), '/administration/sentinel-policies');
    assert
      .dom('[data-test-sentinel-policy-row]')
      .exists({ count: server.db.sentinelPolicies.length });

    await percySnapshot(assert);
  });

  test('Sentinel Policies index: deletion', async function (assert) {
    // Delete every policy
    assert
      .dom('[data-test-empty-sentinel-policy-list-headline]')
      .doesNotExist('no empty state');
    const policyRows = findAll('[data-test-sentinel-policy-row]');

    for (const row of policyRows) {
      const deleteButton = row.querySelector(
        '[data-test-delete-policy] [data-test-idle-button]'
      );
      await click(deleteButton);
      const yesReallyDeleteButton = row.querySelector(
        '[data-test-delete-policy] [data-test-confirm-button]'
      );
      await click(yesReallyDeleteButton);
    }
    // there should be as many success messages as there were policies
    assert
      .dom('.flash-message.alert-success')
      .exists({ count: policyRows.length });

    assert
      .dom('[data-test-empty-sentinel-policy-list-headline]')
      .exists('empty state');
  });

  test('Edit Sentinel Policy: Description', async function (assert) {
    const policy = server.db.sentinelPolicies.findBy(
      (sp) => sp.name === 'policy-1'
    );
    await click('[data-test-sentinel-policy-name="policy-1"]');
    assert.equal(
      currentURL(),
      `/administration/sentinel-policies/${policy.id}`
    );

    assert.dom('[data-test-policy-description]').hasValue(policy.description);

    await fillIn('[data-test-policy-description]', 'edited description');
    await click('button[data-test-save-policy]');
    assert.dom('.flash-message.alert-success').exists();

    // Go back to the index
    await Administration.visitSentinelPolicies();
    const policyRow = find(
      '[data-test-sentinel-policy-name="policy-1"]'
    ).closest('[data-test-sentinel-policy-row]');
    assert.dom(policyRow).exists();
    let rowDescription = policyRow.querySelector(
      '[data-test-sentinel-policy-description]'
    );
    assert.equal(rowDescription.textContent.trim(), 'edited description');
  });

  // TO TEST:
  // - deletion from policy page
  // - modify enforcement level (can make in above test)
  // - create new from scratch
  // - create new from template
});
