/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import {
  visit,
  currentURL,
  click,
  typeIn,
  findAll,
  find,
} from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { allScenarios } from '../../mirage/scenarios/default';
import { setupMirage } from 'ember-cli-mirage/test-support';
import percySnapshot from '@percy/ember';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

module('Acceptance | policies', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('Policies index route looks good', async function (assert) {
    assert.expect(4);
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    assert.dom('[data-test-gutter-link="access-control"]').exists();
    assert.equal(currentURL(), '/access-control/policies');
    assert
      .dom('[data-test-policy-row]')
      .exists({ count: server.db.policies.length });
    await a11yAudit(assert);
    await percySnapshot(assert);
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Prevents policies access if you lack a management token', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[1].secretId;
    await visit('/access-control/policies');
    assert.equal(currentURL(), '/jobs');
    assert.dom('[data-test-gutter-link="access-control"]').doesNotExist();
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Modifying an existing policy', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    await click('[data-test-policy-row]:first-child a');
    // Table sorts by name by default
    let firstPolicy = server.db.policies.sort((a, b) => {
      return a.name.localeCompare(b.name);
    })[0];
    assert.equal(currentURL(), `/access-control/policies/${firstPolicy.name}`);
    assert.dom('[data-test-policy-editor]').exists();
    assert.dom('[data-test-title]').includesText(firstPolicy.name);
    await click('button[data-test-save-policy]');
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(
      currentURL(),
      `/access-control/policies/${firstPolicy.name}`,
      'remain on page after save'
    );
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Creating a test token', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    await click('[data-test-policy-name="Variable-Maker"]');
    assert.equal(currentURL(), '/access-control/policies/Variable-Maker');
    await click('[data-test-create-test-token]');
    assert.dom('.flash-message.alert-success').exists();
    assert
      .dom('[data-test-token-name="Example Token for Variable-Maker"]')
      .exists('Test token is created and visible');
    const newTokenRow = [
      ...findAll('[data-test-token-name="Example Token for Variable-Maker"]'),
    ][0].parentElement;
    const newTokenDeleteButton = newTokenRow.querySelector(
      '[data-test-delete-token-button]'
    );
    await click(newTokenDeleteButton);
    assert
      .dom('[data-test-token-name="Example Token for Variable-Maker"]')
      .doesNotExist('Token is deleted');
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Creating a new policy', async function (assert) {
    assert.expect(7);
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    await click('[data-test-create-policy]');
    assert.equal(currentURL(), '/access-control/policies/new');
    await typeIn('[data-test-policy-name-input]', 'My Fun Policy');
    await click('button[data-test-save-policy]');
    assert
      .dom('.flash-message.alert-critical')
      .exists('Doesnt let you save a bad name');
    assert.equal(currentURL(), '/access-control/policies/new');
    document.querySelector('[data-test-policy-name-input]').value = ''; // clear
    await typeIn('[data-test-policy-name-input]', 'My-Fun-Policy');
    await click('button[data-test-save-policy]');
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(
      currentURL(),
      '/access-control/policies/My-Fun-Policy',
      'redirected to the now-created policy'
    );
    await visit('/access-control/policies');
    const newPolicy = [...findAll('[data-test-policy-name]')].filter((a) =>
      a.textContent.includes('My-Fun-Policy')
    )[0];
    assert.ok(newPolicy, 'Policy is in the list');
    await click(newPolicy);
    assert.equal(currentURL(), '/access-control/policies/My-Fun-Policy');
    await percySnapshot(assert);
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Deleting a policy', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    let firstPolicy = server.db.policies.sort((a, b) => {
      return a.name.localeCompare(b.name);
    })[0];

    const firstPolicyName = firstPolicy.name;
    const firstPolicyLink = [...findAll('[data-test-policy-name]')].filter(
      (row) => row.textContent.includes(firstPolicyName)
    )[0];
    await click(firstPolicyLink);
    assert.equal(currentURL(), `/access-control/policies/${firstPolicyName}`);

    const deleteButton = find('[data-test-delete-policy] button');
    assert.dom(deleteButton).exists('delete button is present');
    await click(deleteButton);
    assert
      .dom('[data-test-confirmation-message]')
      .exists('confirmation message is present');
    await click(find('[data-test-confirm-button]'));

    assert.dom('.flash-message.alert-success').exists();
    assert.equal(currentURL(), '/access-control/policies');
    assert.dom(`[data-test-policy-name="${firstPolicyName}"]`).doesNotExist();
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Policies Index', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    // Table contains every policy in db
    assert
      .dom('[data-test-policy-row]')
      .exists({ count: server.db.policies.length });

    assert.dom('[data-test-empty-policies-list-headline]').doesNotExist();

    // Deleting all policies results in a message
    const policyRows = findAll('[data-test-policy-row]');
    for (const row of policyRows) {
      const deleteButton = row.querySelector('[data-test-delete-policy]');
      await click(deleteButton);
    }
    assert.dom('[data-test-empty-policies-list-headline]').exists();
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });
});
