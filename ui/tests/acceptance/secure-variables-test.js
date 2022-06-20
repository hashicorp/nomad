import { module, test } from 'qunit';
import { currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import defaultScenario from '../../mirage/scenarios/default';
import { click, find, findAll, typeIn } from '@ember/test-helpers';

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
});
