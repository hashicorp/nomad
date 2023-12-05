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

module('Acceptance | namespaces', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('Namespaces index, general', async function (assert) {
    assert.expect(4);
    allScenarios.namespacesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/namespaces');
    assert.dom('[data-test-gutter-link="access-control"]').exists();
    assert.equal(currentURL(), '/access-control/namespaces');
    assert
      .dom('[data-test-namespace-row]')
      .exists({ count: server.db.namespaces.length });
    await a11yAudit(assert);
    await percySnapshot(assert);
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Prevents namespaes access if you lack a management token', async function (assert) {
    allScenarios.namespacesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[1].secretId;
    await visit('/access-control/namespaces');
    assert.equal(currentURL(), '/jobs?namespace=*');
    assert.dom('[data-test-gutter-link="access-control"]').doesNotExist();
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Creating a new namespace', async function (assert) {
    assert.expect(7);
    allScenarios.namespacesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/namespaces');
    await click('[data-test-create-namespace]');
    assert.equal(currentURL(), '/access-control/namespaces/new');
    await typeIn('[data-test-namespace-name-input]', 'My New Namespace');
    await click('button[data-test-save-namespace]');
    assert
      .dom('.flash-message.alert-critical')
      .exists('Doesnt let you save a bad name');
    assert.equal(currentURL(), '/access-control/namespaces/new');
    document.querySelector('[data-test-namespace-name-input]').value = ''; // clear
    await typeIn('[data-test-namespace-name-input]', 'My-New-Namespace');
    await click('button[data-test-save-namespace]');
    assert.dom('.flash-message.alert-success').exists();

    assert.equal(
      currentURL(),
      '/access-control/namespaces/My-New-Namespace',
      'redirected to the now-created namespace'
    );
    await visit('/access-control/namespaces');
    const newNs = [...findAll('[data-test-namespace-name]')].filter((a) =>
      a.textContent.includes('My-New-Namespace')
    )[0];
    assert.ok(newNs, 'Namespace is in the list');
    await click(newNs);
    assert.equal(currentURL(), '/access-control/namespaces/My-New-Namespace');
    await percySnapshot(assert);
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('New namespaces have quotas and node_pool properties if Ent', async function (assert) {
    assert.expect(2);
    allScenarios.namespacesTestCluster(server, { enterprise: true });
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/namespaces');
    await click('[data-test-create-namespace]');

    // Get the dom node text for the description
    const descriptionText = document.querySelector(
      '[data-test-namespace-editor]'
    ).textContent;

    assert.ok(
      descriptionText.includes('Quota'),
      'Includes Quotas in namespace description'
    );
    assert.ok(
      descriptionText.includes(
        'NodePoolConfiguration',
        'Includes NodePoolConfiguration in namespace description'
      )
    );

    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('New namespaces hide quotas and node_pool properties if CE', async function (assert) {
    assert.expect(2);
    allScenarios.namespacesTestCluster(server, { enterprise: false });
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/namespaces');
    await click('[data-test-create-namespace]');

    // Get the dom node text for the description
    const descriptionText = document.querySelector(
      '[data-test-namespace-editor]'
    ).textContent;

    assert.notOk(descriptionText.includes('Quotas'));
    assert.notOk(descriptionText.includes('NodePoolConfiguration'));

    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Modifying an existing namespace', async function (assert) {
    allScenarios.namespacesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/namespaces');
    await click('[data-test-namespace-row]:first-child a');
    // Table sorts by name by default
    let firstNamespace = server.db.namespaces.sort((a, b) => {
      return a.name.localeCompare(b.name);
    })[0];
    assert.equal(
      currentURL(),
      `/access-control/namespaces/${firstNamespace.name}`
    );
    assert.dom('[data-test-namespace-editor]').exists();
    assert.dom('[data-test-title]').includesText(firstNamespace.name);
    await click('button[data-test-save-namespace]');
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(
      currentURL(),
      `/access-control/namespaces/${firstNamespace.name}`,
      'remain on page after save'
    );
    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Deleting a namespace', async function (assert) {
    assert.expect(11);
    allScenarios.namespacesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/namespaces');

    // Default namespace hides delete button
    const defaultNamespaceLink = [
      ...findAll('[data-test-namespace-name]'),
    ].filter((row) => row.textContent.includes('default'))[0];
    await click(defaultNamespaceLink);

    assert.equal(currentURL(), `/access-control/namespaces/default`);
    let deleteButton = find('[data-test-delete-namespace] button');
    assert
      .dom(deleteButton)
      .doesNotExist('delete button is not present for default');

    // Standard namespace properly deletes
    await visit('/access-control/namespaces');

    let nonDefaultNamespace = server.db.namespaces.findBy(
      (ns) => ns.name != 'default'
    );
    const nonDefaultNsLink = [...findAll('[data-test-namespace-name]')].filter(
      (row) => row.textContent.includes(nonDefaultNamespace.name)
    )[0];
    await click(nonDefaultNsLink);
    assert.equal(
      currentURL(),
      `/access-control/namespaces/${nonDefaultNamespace.name}`
    );
    deleteButton = find('[data-test-delete-namespace] button');
    assert.dom(deleteButton).exists('delete button is present for non-default');
    await click(deleteButton);
    assert
      .dom('[data-test-confirmation-message]')
      .exists('confirmation message is present');
    await click(find('[data-test-confirm-button]'));
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(currentURL(), '/access-control/namespaces');
    assert
      .dom(`[data-test-namespace-name="${nonDefaultNamespace.name}"]`)
      .doesNotExist();

    // Namespace with variables errors properly
    // "with-variables" hard-coded into scenario to be a NS with variables attached
    await visit('/access-control/namespaces/with-variables');
    assert.equal(currentURL(), '/access-control/namespaces/with-variables');
    deleteButton = find('[data-test-delete-namespace] button');
    await click(deleteButton);
    await click(find('[data-test-confirm-button]'));
    assert
      .dom('.flash-message.alert-critical')
      .exists('Doesnt let you delete a namespace with variables');

    assert.equal(currentURL(), '/access-control/namespaces/with-variables');

    // Reset Token
    window.localStorage.nomadTokenSecret = null;
  });

  test('Deleting a namespace failure and return', async function (assert) {
    // This is an indirect test of rollbackWithoutChangedAttrs
    // which allows deletes to fail and rolls back attributes
    // It was added because this path was throwing an error when
    // reloading the Ember model that was attempted to be deleted

    assert.expect(3);
    allScenarios.namespacesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;

    // Attempt a delete on an un-deletable namespace
    await visit('/access-control/namespaces/with-variables');
    let deleteButton = find('[data-test-delete-namespace] button');
    await click(deleteButton);
    await click(find('[data-test-confirm-button]'));

    assert
      .dom('.flash-message.alert-critical')
      .exists('Doesnt let you delete a namespace with variables');
    assert.equal(currentURL(), '/access-control/namespaces/with-variables');

    // Navigate back to the page via the index
    await visit('/access-control/namespaces');

    // Default namespace hides delete button
    const notDeletedNSLink = [...findAll('[data-test-namespace-name]')].filter(
      (row) => row.textContent.includes('with-variables')
    )[0];
    await click(notDeletedNSLink);

    assert.equal(currentURL(), `/access-control/namespaces/with-variables`);
  });
});
