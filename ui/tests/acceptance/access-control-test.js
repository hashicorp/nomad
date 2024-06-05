/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { currentURL, triggerKeyEvent, click } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Administration from 'nomad-ui/tests/pages/administration';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import { allScenarios } from '../../mirage/scenarios/default';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import percySnapshot from '@percy/ember';

// Several related tests within Access Control are contained in the Tokens, Roles,
// and Policies acceptance tests.

module('Acceptance | access control', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    window.sessionStorage.clear();
    // server.create('token');
    allScenarios.rolesTestCluster(server);
  });

  test('Access Control is only accessible by a management user', async function (assert) {
    assert.expect(7);
    await Administration.visit();

    assert.equal(
      currentURL(),
      '/jobs',
      'redirected to the jobs page if a non-management token on /administration'
    );

    await Administration.visitTokens();
    assert.equal(
      currentURL(),
      '/jobs',
      'redirected to the jobs page if a non-management token on /tokens'
    );

    assert.dom('[data-test-gutter-link="administration"]').doesNotExist();

    await Tokens.visit();
    const managementToken = server.db.tokens.findBy(
      (t) => t.type === 'management'
    );
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();

    assert.dom('[data-test-gutter-link="administration"]').exists();

    await Administration.visit();
    assert.equal(
      currentURL(),
      '/administration',
      'management token can access /administration'
    );

    await a11yAudit(assert);

    await Administration.visitTokens();
    assert.equal(
      currentURL(),
      '/administration/tokens',
      'management token can access /administration/tokens'
    );
  });

  test('Access control does not show Sentinel Policies if they are not present in license', async function (assert) {
    allScenarios.policiesTestCluster(server);
    await Tokens.visit();
    const managementToken = server.db.tokens.findBy(
      (t) => t.type === 'management'
    );
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Administration.visit();
    assert.dom('[data-test-sentinel-policies-card]').doesNotExist();
  });

  test('Access control shows Sentinel Policies if they are present in license', async function (assert) {
    assert.expect(2);
    allScenarios.policiesTestCluster(server, { sentinel: true });
    await Tokens.visit();
    const managementToken = server.db.tokens.findBy(
      (t) => t.type === 'management'
    );
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Administration.visit();

    assert.dom('[data-test-sentinel-policies-card]').exists();
    await percySnapshot(assert);
    await click('[data-test-sentinel-policies-card] a');
    assert.equal(currentURL(), '/administration/sentinel-policies');
  });

  test('Access control index content', async function (assert) {
    await Tokens.visit();
    const managementToken = server.db.tokens.findBy(
      (t) => t.type === 'management'
    );
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();

    await Administration.visit();
    assert.dom('[data-test-tokens-card]').exists();
    assert.dom('[data-test-roles-card]').exists();
    assert.dom('[data-test-policies-card]').exists();
    assert.dom('[data-test-namespaces-card]').exists();

    const numberOfTokens = server.db.tokens.length;
    const numberOfRoles = server.db.roles.length;
    const numberOfPolicies = server.db.policies.length;
    const numberOfNamespaces = server.db.namespaces.length;

    assert
      .dom('[data-test-tokens-card] a')
      .includesText(`${numberOfTokens} Tokens`);
    assert
      .dom('[data-test-roles-card] a')
      .includesText(`${numberOfRoles} Roles`);
    assert
      .dom('[data-test-policies-card] a')
      .includesText(`${numberOfPolicies} Policies`);
    assert
      .dom('[data-test-namespaces-card] a')
      .includesText(`${numberOfNamespaces} Namespaces`);
  });

  test('Access control subnav', async function (assert) {
    await Tokens.visit();
    const managementToken = server.db.tokens.findBy(
      (t) => t.type === 'management'
    );
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();

    await Administration.visit();

    assert.equal(currentURL(), '/administration');

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.equal(
      currentURL(),
      `/administration/tokens`,
      'Shift+ArrowRight takes you to the next tab (Tokens)'
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.equal(
      currentURL(),
      `/administration/roles`,
      'Shift+ArrowRight takes you to the next tab (Roles)'
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.equal(
      currentURL(),
      `/administration/policies`,
      'Shift+ArrowRight takes you to the next tab (Policies)'
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.equal(
      currentURL(),
      `/administration/namespaces`,
      'Shift+ArrowRight takes you to the next tab (Namespaces)'
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.equal(
      currentURL(),
      `/administration`,
      'Shift+ArrowLeft takes you back to the Access Control index page'
    );
  });
});
