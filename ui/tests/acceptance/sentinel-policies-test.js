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
    assert.expect(3);
    await a11yAudit(assert);

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

  test('Edit Sentinel Policy: Description and Enforcement Level', async function (assert) {
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
    await click('[data-test-enforcement-level="hard-mandatory"]');
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
    assert
      .dom(policyRow.querySelector('[data-test-sentinel-policy-enforcement]'))
      .hasText('hard-mandatory');
  });

  test('New Sentinel Policy from Scratch', async function (assert) {
    await click('[data-test-create-sentinel-policy]');
    assert.equal(currentURL(), '/administration/sentinel-policies/new');
    await fillIn('[data-test-policy-name-input]', 'new-policy');
    await fillIn('[data-test-policy-description]', 'new description');
    await click('[data-test-enforcement-level="hard-mandatory"]');

    await click('[data-test-save-policy]');
    assert.dom('.flash-message.alert-success').exists('success message shown');

    // Go back to the index
    await Administration.visitSentinelPolicies();
    const policyRow = find(
      '[data-test-sentinel-policy-name="new-policy"]'
    ).closest('[data-test-sentinel-policy-row]');
    assert.dom(policyRow).exists('new policy row exists');
    let rowDescription = policyRow.querySelector(
      '[data-test-sentinel-policy-description]'
    );
    assert.equal(
      rowDescription.textContent.trim(),
      'new description',
      'description matches new policy input'
    );
    assert
      .dom(policyRow.querySelector('[data-test-sentinel-policy-enforcement]'))
      .hasText('hard-mandatory', 'enforcement level matches new policy input');

    await click('[data-test-sentinel-policy-name="new-policy"]');
    await click('[data-test-delete-policy] [data-test-idle-button]');
    await click('[data-test-delete-policy] [data-test-confirm-button]');
    assert.dom('.flash-message.alert-success').exists('success message shown');

    await Administration.visitSentinelPolicies();
    assert
      .dom('[data-test-sentinel-policy-name="new-policy"]')
      .doesNotExist('new policy row is gone');
  });

  test('New Sentinel Policy from Template', async function (assert) {
    assert.expect(5);
    await click('[data-test-create-sentinel-policy-from-template]');
    assert.equal(currentURL(), '/administration/sentinel-policies/gallery');
    await percySnapshot(assert);
    const template = find('[data-test-template-card="no-friday-deploys"]');
    await click(template);
    assert.ok(
      find('[data-test-template-card="no-friday-deploys"]')
        ?.closest('label')
        .classList.contains(
          'hds-form-radio-card--checked',
          'template is selected on click'
        )
    );
    await click('[data-test-apply]');
    assert.equal(
      currentURL(),
      '/administration/sentinel-policies/new?template=no-friday-deploys',
      'New Policy page has query param'
    );

    await percySnapshot(assert);

    assert.dom('[data-test-policy-name-input]').hasValue('no-friday-deploys');
    assert
      .dom('[data-test-policy-description]')
      .hasValue('Ensures that no deploys happen on a Friday');
  });
});
