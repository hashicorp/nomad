/**
 * Copyright IBM Corp. 2015, 2025
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
import faker from 'nomad-ui/mirage/faker';

// Several related tests within Access Control are contained in the Tokens, Roles,
// and Policies acceptance tests.

module('Acceptance | access control', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    faker.seed(1);
    window.localStorage.clear();
    window.sessionStorage.clear();
    allScenarios.rolesTestCluster(this.server);
  });

  test('Access Control is only accessible by a management user', async function (assert) {
    await Administration.visit();

    assert.deepEqual(
      currentURL(),
      '/jobs',
      'redirected to the jobs page if a non-management token on /administration',
    );

    await Administration.visitTokens();
    assert.deepEqual(
      currentURL(),
      '/jobs',
      'redirected to the jobs page if a non-management token on /tokens',
    );

    assert.dom('[data-test-gutter-link="administration"]').doesNotExist();

    const managementToken = this.server.create('token', {
      type: 'management',
      name: 'Management Token',
    });

    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();

    assert.dom('[data-test-gutter-link="administration"]').exists();

    await Administration.visit();
    assert.deepEqual(
      currentURL(),
      '/administration',
      'management token can access /administration',
    );

    await a11yAudit(assert);

    await Administration.visitTokens();
    assert.deepEqual(
      currentURL(),
      '/administration/tokens',
      'management token can access /administration/tokens',
    );
  });

  test('Access control does not show Sentinel Policies if they are not present in license', async function (assert) {
    allScenarios.policiesTestCluster(this.server);
    const managementToken = this.server.create('token', {
      type: 'management',
      name: 'Management Token',
    });
    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Administration.visit();
    assert.dom('[data-test-sentinel-policies-card]').doesNotExist();
  });

  test('Access control shows Sentinel Policies if they are present in license', async function (assert) {
    allScenarios.policiesTestCluster(this.server, { sentinel: true });
    const managementToken = this.server.create('token', {
      type: 'management',
      name: 'Management Token',
    });
    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Administration.visit();

    assert.dom('[data-test-sentinel-policies-card]').exists();
    await percySnapshot(assert);
    await click('[data-test-sentinel-policies-card] a');
    assert.deepEqual(currentURL(), '/administration/sentinel-policies');
  });

  test('Access control index content', async function (assert) {
    const managementToken = this.server.create('token', {
      type: 'management',
      name: 'Management Token',
    });
    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();

    await Administration.visit();
    assert.dom('[data-test-tokens-card]').exists();
    assert.dom('[data-test-roles-card]').exists();
    assert.dom('[data-test-policies-card]').exists();
    assert.dom('[data-test-namespaces-card]').exists();

    const numberOfTokens = this.server.db.tokens.length;
    const numberOfRoles = this.server.db.roles.length;
    const numberOfPolicies = this.server.db.policies.length;
    const numberOfNamespaces = this.server.db.namespaces.length;

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
    const managementToken = this.server.create('token', {
      type: 'management',
      name: 'Management Token',
    });
    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();

    await Administration.visit();

    assert.deepEqual(currentURL(), '/administration');

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.deepEqual(
      currentURL(),
      `/administration/tokens`,
      'Shift+ArrowRight takes you to the next tab (Tokens)',
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.deepEqual(
      currentURL(),
      `/administration/roles`,
      'Shift+ArrowRight takes you to the next tab (Roles)',
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.deepEqual(
      currentURL(),
      `/administration/policies`,
      'Shift+ArrowRight takes you to the next tab (Policies)',
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.deepEqual(
      currentURL(),
      `/administration/namespaces`,
      'Shift+ArrowRight takes you to the next tab (Namespaces)',
    );

    await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
      shiftKey: true,
    });
    assert.deepEqual(
      currentURL(),
      `/administration`,
      'Shift+ArrowLeft takes you back to the Access Control index page',
    );
  });
});
