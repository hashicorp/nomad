// @ts-check
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import {
  click,
  currentURL,
  visit,
  triggerEvent,
  findAll,
} from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Layout from 'nomad-ui/tests/pages/layout';
import percySnapshot from '@percy/ember';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

module('Acceptance | keyboard', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  module('modal', function () {
    test('Opening and closing shortcuts modal with key commands', async function (assert) {
      assert.expect(4);
      await visit('/');
      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      await percySnapshot(assert);
      await a11yAudit(assert);
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      assert.notOk(Layout.keyboard.modalShown);
    });

    test('closing shortcuts modal by clicking dismiss', async function (assert) {
      await visit('/');
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      assert.dom('button.dismiss').isFocused();
      await click('button.dismiss');
      assert.notOk(Layout.keyboard.modalShown);
    });

    test('closing shortcuts modal by clicking outside', async function (assert) {
      await visit('/');
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      await click('.page-layout');
      assert.notOk(Layout.keyboard.modalShown);
    });
  });

  module('Enable/Disable', function (enableDisableHooks) {
    enableDisableHooks.beforeEach(function () {
      window.localStorage.clear();
    });

    test('Shortcuts work by default and stops working when disabled', async function (assert) {
      await visit('/');

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'c' });
      assert.equal(
        currentURL(),
        `/clients`,
        'end up on the clients page after typing g c'
      );
      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      assert.dom('[data-test-enable-shortcuts-toggle]').hasClass('is-active');
      await click('[data-test-enable-shortcuts-toggle]');
      assert
        .dom('[data-test-enable-shortcuts-toggle]')
        .doesNotHaveClass('is-active');
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      assert.notOk(Layout.keyboard.modalShown);
      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'j' });
      assert.equal(
        currentURL(),
        `/clients`,
        'typing g j did not bring you back to the jobs page, since shortcuts are disabled'
      );
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      await click('[data-test-enable-shortcuts-toggle]');
      assert.dom('[data-test-enable-shortcuts-toggle]').hasClass('is-active');
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'j' });
      assert.equal(
        currentURL(),
        `/jobs`,
        'typing g j brings me to the jobs page after re-enabling shortcuts'
      );
    });
  });

  module('Local storage bind/rebind', function (rebindHooks) {
    rebindHooks.beforeEach(function () {
      window.localStorage.clear();
    });

    test('You can rebind shortcuts', async function (assert) {
      await visit('/');

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'c' });
      assert.equal(
        currentURL(),
        `/clients`,
        'end up on the clients page after typing g c'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'j' });
      assert.equal(
        currentURL(),
        `/jobs`,
        'end up on the clients page after typing g j'
      );

      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);

      await click('[data-test-command-label="Go to Clients"] button.re-bind');

      triggerEvent('.page-layout', 'keydown', { key: 'r' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'f' });
      triggerEvent('.page-layout', 'keydown', { key: 'l' });
      await triggerEvent('.page-layout', 'keydown', { key: 'Enter' });
      assert
        .dom('[data-test-command-label="Go to Clients"] button.re-bind')
        .hasText('r o f l');

      assert.equal(
        currentURL(),
        `/jobs`,
        'typing g c does not do anything, since I re-bound the shortcut'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'r' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'f' });
      await triggerEvent('.page-layout', 'keydown', { key: 'l' });

      assert.equal(
        currentURL(),
        `/clients`,
        'typing the newly bound shortcut brings me to clients'
      );

      await click('[data-test-command-label="Go to Clients"] button.re-bind');

      triggerEvent('.page-layout', 'keydown', { key: 'n' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'p' });
      triggerEvent('.page-layout', 'keydown', { key: 'e' });
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      assert
        .dom('[data-test-command-label="Go to Clients"] button.re-bind')
        .hasText(
          'r o f l',
          'text unchanged when I hit escape during recording'
        );

      await click(
        '[data-test-command-label="Go to Clients"] button.reset-to-default'
      );
      assert
        .dom('[data-test-command-label="Go to Clients"] button.re-bind')
        .hasText('g c', 'Resetting to default rebinds the shortcut');
    });

    test('Rebound shortcuts persist from localStorage', async function (assert) {
      window.localStorage.setItem(
        'keyboard.command.Go to Clients',
        JSON.stringify(['b', 'o', 'o', 'p'])
      );
      await visit('/');

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'c' });
      assert.equal(
        currentURL(),
        `/jobs`,
        'After a refresh with a localStorage-found binding, a default key binding doesnt do anything'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'b' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      await triggerEvent('.page-layout', 'keydown', { key: 'p' });
      assert.equal(
        currentURL(),
        `/clients`,
        'end up on the clients page after typing the localstorage-bound shortcut'
      );

      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      assert
        .dom('[data-test-command-label="Go to Clients"] button.re-bind')
        .hasText('b o o p');
    });
  });
});
