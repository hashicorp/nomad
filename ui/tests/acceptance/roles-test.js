/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { visit, findAll, find, click, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { allScenarios } from '../../mirage/scenarios/default';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import AccessControl from 'nomad-ui/tests/pages/access-control';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

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

    assert.dom('[data-test-role-row').exists({ count: server.db.roles.length });

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
});
