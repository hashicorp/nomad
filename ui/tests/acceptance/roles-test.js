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
import AccessControl from 'nomad-ui/tests/pages/access-control';
import percySnapshot from '@percy/ember';

module('Acceptance | roles', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    window.localStorage.clear();
    window.sessionStorage.clear();
    allScenarios.rolesTestCluster(server);
    await Tokens.visit();
    const managementToken = server.db.tokens.findBy(
      (t) => t.type === 'management'
    );
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await AccessControl.visitRoles();
  });

  hooks.afterEach(async function () {
    await Tokens.visit();
    await Tokens.clear();
  });

  test('Roles index, general', async function (assert) {
    assert.expect(3);
    await a11yAudit(assert);

    assert.equal(currentURL(), '/access-control/roles');

    assert
      .dom('[data-test-role-row]')
      .exists({ count: server.db.roles.length });

    await percySnapshot(assert);
  });

  test('Roles index: deletion', async function (assert) {
    // Delete every role
    assert
      .dom('[data-test-empty-role-list-headline]')
      .doesNotExist('no empty state');
    const roleRows = findAll('[data-test-role-row]');
    for (const row of roleRows) {
      const deleteButton = row.querySelector('[data-test-delete-role]');
      await click(deleteButton);
    }
    // there should be as many success messages as there were roles
    assert
      .dom('.flash-message.alert-success')
      .exists({ count: roleRows.length });

    assert.dom('[data-test-empty-role-list-headline]').exists('empty state');
  });

  test('Roles have policies lists', async function (assert) {
    const role = server.db.roles.findBy((r) => r.name === 'reader');
    const roleRow = find(`[data-test-role-row="${role.name}"]`);
    const rolePoliciesCell = roleRow.querySelector('[data-test-role-policies]');
    const policiesCellTags = rolePoliciesCell
      .querySelector('.tag-group')
      .querySelectorAll('span');
    assert.equal(policiesCellTags.length, 2);
    assert.equal(policiesCellTags[0].textContent.trim(), 'client-reader');
    assert.equal(policiesCellTags[1].textContent.trim(), 'job-reader');

    await click(policiesCellTags[0].querySelector('a'));
    assert.equal(currentURL(), '/access-control/policies/client-reader');
    assert.dom('[data-test-title]').containsText('client-reader');
  });

  test('Edit Role: Name and Description', async function (assert) {
    assert.expect(8);
    const role = server.db.roles.findBy((r) => r.name === 'reader');
    await click('[data-test-role-name="reader"] a');
    assert.equal(currentURL(), `/access-control/roles/${role.id}`);

    assert.dom('[data-test-role-name-input]').hasValue(role.name);
    assert.dom('[data-test-role-description-input]').hasValue(role.description);
    assert.dom('[data-test-role-policies]').exists();

    // Modify the name and description
    await fillIn('[data-test-role-name-input]', 'reader-edited');
    await fillIn('[data-test-role-description-input]', 'edited description');
    await click('button[data-test-save-role]');
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(
      currentURL(),
      `/access-control/roles/${role.name}`,
      'remain on page after save'
    );
    await percySnapshot(assert);

    // Go back to the roles index
    await AccessControl.visitRoles();
    let readerRoleRow = find('[data-test-role-row="reader-edited"]');
    assert.dom(readerRoleRow).exists();
    assert.equal(
      readerRoleRow
        .querySelector('[data-test-role-description]')
        .textContent.trim(),
      'edited description'
    );
  });

  test('Edit Role: Policies', async function (assert) {
    const role = server.db.roles.findBy((r) => r.name === 'reader');
    await click('[data-test-role-name="reader"] a');
    assert.equal(currentURL(), `/access-control/roles/${role.id}`);

    // Policies table is sortable

    const nameCells = findAll('[data-test-policy-name]');
    const nameCellText = nameCells.map((cell) => cell.textContent.trim());
    const sortedNameCellText = nameCellText.slice().sort();
    assert.deepEqual(
      nameCellText,
      sortedNameCellText,
      'Policy names are sorted alphabetically'
    );

    // Click on the second thead tr th to reverse
    assert
      .dom('table[data-test-role-policies] thead tr th:nth-child(2)')
      .hasAttribute('aria-sort', 'ascending');
    // await click('table[data-test-role-policies] thead tr th:nth-child(2)');
    // above didnt work, another way?
    await click('[data-test-role-policies] thead tr th:nth-child(2) button');
    assert
      .dom('table[data-test-role-policies] thead tr th:nth-child(2)')
      .hasAttribute('aria-sort', 'descending');

    const reversedNameCells = findAll('[data-test-policy-name]');
    const reversedNameCellText = reversedNameCells.map((cell) =>
      cell.textContent.trim()
    );
    const reversedSortedNameCellText = nameCellText.slice().sort().reverse();

    assert.deepEqual(
      reversedNameCellText,
      reversedSortedNameCellText,
      'Names are reversed alphabetically after click'
    );

    // Make sure the correct policies are checked
    const rolePolicies = role.policyIds;
    // All possible policies are shown
    const allPolicies = server.db.policies;
    assert.equal(
      findAll('[data-test-role-policies] tbody tr').length,
      allPolicies.length,
      'all policies are shown'
    );

    const checkedPolicyRows = findAll(
      '[data-test-role-policies] tbody tr input:checked'
    );

    assert.equal(
      checkedPolicyRows.length,
      rolePolicies.length,
      'correct number of policies are checked'
    );

    const checkedPolicyNames = checkedPolicyRows.map((row) =>
      row
        .closest('tr')
        .querySelector('[data-test-policy-name]')
        .textContent.trim()
    );

    assert.deepEqual(
      checkedPolicyNames.sort(),
      rolePolicies.sort(),
      'All policies belonging to this role are checked'
    );

    // Try de-selecting all policies and saving
    checkedPolicyRows.forEach((row) => row.click());
    await click('button[data-test-save-role]');
    assert
      .dom('.flash-message.alert-critical')
      .exists('Doesnt let you save with no policies selected');

    // Check all policies
    findAll('[data-test-role-policies] tbody tr input').forEach((row) =>
      row.click()
    );
    await click('button[data-test-save-role]');
    assert.dom('.flash-message.alert-success').exists();

    await AccessControl.visitRoles();
    const readerRoleRow = find('[data-test-role-row="reader"]');
    const readerRolePolicies = readerRoleRow
      .querySelector('[data-test-role-policies]')
      .querySelectorAll('span');
    assert.equal(
      readerRolePolicies.length,
      allPolicies.length,
      'all policies are attached to the role at index level'
    );
  });

  test('Edit Role: Tokens', async function (assert) {
    assert.expect(10);
    const role = server.db.roles.findBy((r) => r.name === 'reader');

    await click('[data-test-role-name="reader"] a');
    assert.equal(currentURL(), `/access-control/roles/${role.id}`);
    assert.dom('table.tokens').exists();

    // "Reader" role has a single token with it applied by default
    assert.dom('[data-test-role-token-row]').exists({ count: 1 });

    // Delete it; should get a nice No Tokens message
    await click('[data-test-delete-token-button]');
    assert.dom('.flash-message.alert-success').exists();
    assert.dom('[data-test-role-token-row]').doesNotExist();
    assert.dom('[data-test-empty-role-list-headline]').exists();
    // Create two test tokens
    await click('[data-test-create-test-token]');
    assert.dom('[data-test-empty-role-list-headline]').doesNotExist();
    await click('[data-test-create-test-token]');
    assert
      .dom('[data-test-role-token-row]')
      .exists({ count: 2 }, 'Test tokens are included on the page');
    assert
      .dom('[data-test-role-token-row]:last-child [data-test-token-name]')
      .hasText(`Example Token for ${role.name}`);

    await percySnapshot(assert);

    await AccessControl.visitTokens();
    assert
      .dom('[data-test-token-name="Example Token for reader"]')
      .exists(
        { count: 2 },
        'The two newly-created tokens are listed on the tokens index page'
      );
  });
  test('Edit Role: Deletion', async function (assert) {
    const role = server.db.roles.findBy((r) => r.name === 'reader');
    await click('[data-test-role-name="reader"] a');
    assert.equal(currentURL(), `/access-control/roles/${role.id}`);
    const deleteButton = find('[data-test-delete-role] button');
    assert.dom(deleteButton).exists('delete button is present');
    await click(deleteButton);
    assert
      .dom('[data-test-confirmation-message]')
      .exists('confirmation message is present');
    await click(find('[data-test-confirm-button]'));
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(currentURL(), '/access-control/roles');
    assert.dom('[data-test-role-row="reader"]').doesNotExist();
  });
  test('New Role', async function (assert) {
    await click('[data-test-create-role]');
    assert.equal(currentURL(), '/access-control/roles/new');
    await fillIn('[data-test-role-name-input]', 'test-role');
    await click('button[data-test-save-role]');
    assert
      .dom('.flash-message.alert-critical')
      .exists('Cannnot save with no policies selected');

    // Select a policy
    await click('[data-test-role-policies] tbody tr input');
    await click('button[data-test-save-role]');
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(currentURL(), '/access-control/roles/1'); // default id created via mirage
    await AccessControl.visitRoles();
    assert.dom('[data-test-role-row="test-role"]').exists();

    // Now, try deleting all policies then doing this again. There'll be a warning on the roles/new page.
    await AccessControl.visitPolicies();
    const policyRows = findAll('[data-test-policy-row]');
    for (const row of policyRows) {
      const deleteButton = row.querySelector('[data-test-delete-policy]');
      await click(deleteButton);
    }
    assert.dom('[data-test-empty-policies-list-headline]').exists();
    await AccessControl.visitRoles();
    await click('[data-test-create-role]');
    assert.dom('.empty-message').exists();
    assert
      .dom('.empty-message-body')
      .containsText('At least one Policy is required to create a Role');
  });
});
