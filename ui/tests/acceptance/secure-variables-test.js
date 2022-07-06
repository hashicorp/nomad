import {
  currentRouteName,
  currentURL,
  click,
  find,
  findAll,
  typeIn,
} from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import { setupApplicationTest } from 'ember-qunit';
import { module, test } from 'qunit';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import defaultScenario from '../../mirage/scenarios/default';

import Variables from 'nomad-ui/tests/pages/variables';
import Layout from 'nomad-ui/tests/pages/layout';

const SECURE_TOKEN_ID = '53cur3-v4r14bl35';

module('Acceptance | secure variables', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  hooks.beforeEach(async function () {
    server.createList('variable', 3);
  });

  test('it redirects to jobs and hides the gutter link when the token lacks permissions', async function (assert) {
    await Variables.visit();
    assert.equal(currentURL(), '/jobs');
    assert.ok(Layout.gutter.variables.isHidden);
  });

  test('it allows access for management level tokens', async function (assert) {
    defaultScenario(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await Variables.visit();
    assert.equal(currentURL(), '/variables');
    assert.ok(Layout.gutter.variables.isVisible, 'Menu section is visible');
  });

  test('it allows access for list-variables allowed ACL rules', async function (assert) {
    assert.expect(2);
    defaultScenario(server);
    const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
    window.localStorage.nomadTokenSecret = variablesToken.secretId;

    await Variables.visit();
    assert.equal(currentURL(), '/variables');
    assert.ok(Layout.gutter.variables.isVisible);
  });

  test('it correctly traverses to and deletes a variable', async function (assert) {
    assert.expect(13);
    defaultScenario(server);
    const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
    window.localStorage.nomadTokenSecret = variablesToken.secretId;

    await Variables.visit();
    assert.equal(currentURL(), '/variables');
    assert.ok(Layout.gutter.variables.isVisible);

    let abcLink = [...findAll('[data-test-folder-row]')].filter((a) =>
      a.textContent.includes('a/b/c')
    )[0];

    await click(abcLink);

    assert.equal(
      currentURL(),
      '/variables/path/a/b/c',
      'correctly traverses to a deeply nested path'
    );
    assert.equal(
      findAll('[data-test-folder-row]').length,
      2,
      'correctly shows 2 sub-folders'
    );
    assert.equal(
      findAll('[data-test-file-row]').length,
      2,
      'correctly shows 2 files'
    );
    let fooLink = [...findAll('[data-test-file-row]')].filter((a) =>
      a.textContent.includes('foo0')
    )[0];

    assert.ok(fooLink, 'foo0 file is present');

    await click(fooLink);
    assert.equal(
      currentURL(),
      '/variables/var/a/b/c/foo0',
      'correctly traverses to a deeply nested variable file'
    );
    const deleteButton = find('[data-test-delete-button] button');
    assert.dom(deleteButton).exists('delete button is present');

    await click(deleteButton);
    assert
      .dom('[data-test-confirmation-message]')
      .exists('confirmation message is present');

    await click(find('[data-test-confirm-button]'));
    assert.equal(
      currentURL(),
      '/variables/path/a/b/c',
      'correctly returns to the parent path page after deletion'
    );

    assert.equal(
      findAll('[data-test-folder-row]').length,
      2,
      'still correctly shows 2 sub-folders'
    );
    assert.equal(
      findAll('[data-test-file-row]').length,
      1,
      'now correctly shows 1 file'
    );

    fooLink = [...findAll('[data-test-file-row]')].filter((a) =>
      a.textContent.includes('foo0')
    )[0];

    assert.notOk(fooLink, 'foo0 file is no longer present');
  });

  test('it does not allow you to save if you lack Items', async function (assert) {
    defaultScenario(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await Variables.visitNew();
    assert.equal(currentURL(), '/variables/new');
    await typeIn('.path-input', 'foo/bar');
    await click('button[type="submit"]');
    assert.dom('.flash-message.alert-error').exists();
    await click('.flash-message.alert-error .close-button');
    assert.dom('.flash-message.alert-error').doesNotExist();

    await typeIn('.key-value label:nth-child(1) input', 'myKey');
    await typeIn('.key-value label:nth-child(2) input', 'superSecret');
    await click('button[type="submit"]');

    assert.dom('.flash-message.alert-success').exists();
    assert.equal(currentURL(), '/variables/var/foo/bar');
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);
    defaultScenario(server);
    const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
    window.localStorage.nomadTokenSecret = variablesToken.secretId;
    await Variables.visit();
    await a11yAudit(assert);
  });

  module('create flow', function () {
    test('allows a user with correct permissions to create a secure variable', async function (assert) {
      // Arrange Test Set-up
      defaultScenario(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visit();
      // End Test Set-up

      assert
        .dom('[data-test-create-var]')
        .exists(
          'It should display an enabled button to create a secure variable'
        );
      await click('[data-test-create-var]');

      assert.equal(currentRouteName(), 'variables.new');

      await typeIn('[data-test-path-input]', 'foo/bar');
      await typeIn('[data-test-var-key]', 'kiki');
      await typeIn('[data-test-var-value]', 'do you love me');
      await click('[data-test-submit-var]');

      assert.equal(
        currentRouteName(),
        'variables.variable.index',
        'Navigates user back to variables list page after creating variable.'
      );
      assert
        .dom('.flash-message.alert.alert-success')
        .exists('Shows a success toast notification on creation.');
      assert
        .dom('[data-test-var=kiki]')
        .exists('The new variable key should appear in the list.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('prevents users from creating a secure variable without proper permissions', async function (assert) {
      // Arrange Test Set-up
      defaultScenario(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].SecureVariables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list'];
      await Variables.visit();
      // End Test Set-up

      assert
        .dom('[data-test-disabled-create-var]')
        .exists(
          'It should display an disabled button to create a secure variable on the main listings page'
        );

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });
  });

  module('edit flow', function () {
    test('allows a user with correct permissions to edit a secure variable', async function (assert) {
      // Arrange Test Set-up
      defaultScenario(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].SecureVariables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list', 'write'];
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up

      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-edit-button]')
        .exists('The edit button is enabled in the view.');
      await click('[data-test-edit-button]');
      assert.equal(
        currentRouteName(),
        'variables.variable.edit',
        'Clicking the button navigates you to editing view.'
      );

      assert.dom('[data-test-path-input]').isDisabled('Path cannot be edited');

      document.querySelector('[data-test-var-key]').value = ''; // clear current input
      await typeIn('[data-test-var-key]', 'kiki');
      await typeIn('[data-test-var-value]', 'do you love me');
      await click('[data-test-submit-var]');

      assert.equal(
        currentRouteName(),
        'variables.variable.index',
        'Navigates user back to variables list page after creating variable.'
      );
      assert
        .dom('.flash-message.alert.alert-success')
        .exists('Shows a success toast notification on edit.');
      assert
        .dom('[data-test-var=kiki]')
        .exists('The edited variable key should appear in the list.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('prevents users from editing a secure variable without proper permissions', async function (assert) {
      // Arrange Test Set-up
      defaultScenario(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].SecureVariables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list'];
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up

      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-edit-button]')
        .doesNotExist('The edit button is hidden in the view.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });
  });

  module('delete flow', function () {
    test('allows a user with correct permissions to delete a secure variable', async function (assert) {
      // Arrange Test Set-up
      defaultScenario(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].SecureVariables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list', 'destroy'];
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up

      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-delete-button]')
        .exists('The delete button is enabled in the view.');
      await click('[data-test-idle-button]');

      assert
        .dom('[data-test-confirmation-message]')
        .exists('Deleting a variable requires two-step confirmation.');

      await click('[data-test-confirm-button]');

      assert.equal(
        currentRouteName(),
        'variables.index',
        'Navigates user back to variables list page after destroying a variable.'
      );

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });

    test('prevents users from delete a secure variable without proper permissions', async function (assert) {
      // Arrange Test Set-up
      defaultScenario(server);
      server.createList('variable', 3);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      const policy = server.db.policies.find('Variable Maker');
      policy.rulesJSON.Namespaces[0].SecureVariables.Paths.find(
        (path) => path.PathSpec === '*'
      ).Capabilities = ['list'];
      await Variables.visit();
      await click('[data-test-file-row]');
      // End Test Set-up

      assert.equal(currentRouteName(), 'variables.variable.index');
      assert
        .dom('[data-test-delete-button]')
        .doesNotExist('The delete button is hidden in the view.');

      // Reset Token
      window.localStorage.nomadTokenSecret = null;
    });
  });
});
