// @ts-check
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { click, currentURL, visit, triggerEvent } from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Layout from 'nomad-ui/tests/pages/layout';
import percySnapshot from '@percy/ember';

module('Acceptance | keyboard', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  module('modal', function () {
    test('Opening and closing shortcuts modal with key commands', async function (assert) {
      await visit('/');
      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      await percySnapshot(assert);
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

  module('Enable/Disable', function () {
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
});
