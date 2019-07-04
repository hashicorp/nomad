import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

import { triggerCopyError, triggerCopySuccess } from 'ember-cli-clipboard/test-support';

module('Integration | Component | x-copy-button', function(hooks) {
  setupRenderingTest(hooks);

  test('it shows the copy icon by default', async function(assert) {
    await render(hbs`{{x-copy-button class='copy-button'}}`);

    assert.dom('.copy-button .icon-is-copy-action').exists();
  });

  test('it shows the success icon on success', async function(assert) {
    await render(hbs`{{x-copy-button class='copy-button'}}`);

    await click('.copy-button button');
    await triggerCopySuccess('.copy-button button');

    assert.dom('.copy-button .icon-is-copy-success').exists();
  });

  test('it shows the error icon on error', async function(assert) {
    await render(hbs`{{x-copy-button class='copy-button'}}`);

    await click('.copy-button button');
    await triggerCopyError('.copy-button button');

    assert.dom('.copy-button .icon-is-alert-triangle').exists();
  });
});
