import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

import sinon from 'sinon';

import { triggerCopyError, triggerCopySuccess } from 'ember-cli-clipboard/test-support';

module('Integration | Component | copy-button', function(hooks) {
  setupRenderingTest(hooks);

  test('it shows the copy icon by default', async function(assert) {
    await render(hbs`{{copy-button class='copy-button'}}`);

    assert.dom('.copy-button .icon-is-copy-action').exists();
  });

  test('it shows the success icon on success and resets afterward', async function(assert) {
    const clock = sinon.useFakeTimers();

    await render(hbs`{{copy-button class='copy-button'}}`);

    await click('.copy-button button');
    await triggerCopySuccess('.copy-button button');

    assert.dom('.copy-button .icon-is-copy-success').exists();

    clock.runAll();

    assert.dom('.copy-button .icon-is-copy-success').doesNotExist();
    assert.dom('.copy-button .icon-is-copy-action').exists();

    clock.restore();
  });

  test('it shows the error icon on error', async function(assert) {
    await render(hbs`{{copy-button class='copy-button'}}`);

    await click('.copy-button button');
    await triggerCopyError('.copy-button button');

    assert.dom('.copy-button .icon-is-alert-triangle').exists();
  });
});
