/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember-a11y-testing/a11y-audit-called */
import { module, test } from 'qunit';
import { click, visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Layout from 'nomad-ui/tests/pages/layout';

let managementToken;

module('Acceptance | global header', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('it diplays no links', async function (assert) {
    server.create('agent');

    await visit('/');

    assert.false(Layout.navbar.end.vaultLink.isVisible);
    assert.false(Layout.navbar.end.vaultLink.isVisible);
  });

  test('it diplays both links', async function (assert) {
    server.create('agent', 'withConsulLink', 'withVaultLink');

    await visit('/');

    assert.true(Layout.navbar.end.vaultLink.isVisible);
    assert.true(Layout.navbar.end.vaultLink.isVisible);
  });

  test('it diplays Consul link', async function (assert) {
    server.create('agent', 'withConsulLink');

    await visit('/');

    assert.true(Layout.navbar.end.consulLink.isVisible);
    assert.equal(Layout.navbar.end.consulLink.text, 'Consul');
    assert.equal(Layout.navbar.end.consulLink.link, 'http://localhost:8500/ui');
  });

  test('it diplays Vault link', async function (assert) {
    server.create('agent', 'withVaultLink');

    await visit('/');

    assert.true(Layout.navbar.end.vaultLink.isVisible);
    assert.equal(Layout.navbar.end.vaultLink.text, 'Vault');
    assert.equal(Layout.navbar.end.vaultLink.link, 'http://localhost:8200/ui');
  });

  test('it diplays SignIn', async function (assert) {
    managementToken = server.create('token');

    window.localStorage.clear();

    await visit('/');
    assert.true(Layout.navbar.end.signInLink.isVisible);
    assert.false(Layout.navbar.end.profileDropdown.isVisible);
  });

  test('it diplays a Profile dropdown', async function (assert) {
    managementToken = server.create('token');

    window.localStorage.nomadTokenSecret = managementToken.secretId;

    await visit('/');
    assert.true(Layout.navbar.end.profileDropdown.isVisible);
    assert.false(Layout.navbar.end.signInLink.isVisible);
    await Layout.navbar.end.profileDropdown.open();

    await click('.dropdown-options .ember-power-select-option:nth-child(1)');
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Authroization link takes you to the tokens page'
    );

    await Layout.navbar.end.profileDropdown.open();
    await click('.dropdown-options .ember-power-select-option:nth-child(2)');
    assert.equal(window.localStorage.nomadTokenSecret, null, 'Token is wiped');
    assert.equal(currentURL(), '/jobs', 'After signout, back on the jobs page');
  });
});
