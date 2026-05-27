/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

import sinon from 'sinon';

import {
  triggerCopyError,
  triggerCopySuccess,
} from 'ember-cli-clipboard/test-support';

module('Integration | Component | copy-button', function (hooks) {
  setupRenderingTest(hooks);

  test('it shows the copy icon by default', async function (assert) {
    await render(hbs`<CopyButton />`);
    assert.dom('.copy-button .hds-icon-clipboard-copy').exists();
  });

  test('it shows the success icon on success and resets afterward', async function (assert) {
    const clock = sinon.useFakeTimers({ shouldAdvanceTime: true });

    await render(hbs`<CopyButton @clipboardText="tomster" />`);

    await click('.copy-button button');
    await triggerCopySuccess('.copy-button button');

    assert.dom('[data-test-copy-success]').exists();

    clock.runAll();

    assert.dom('[data-test-copy-success]').doesNotExist();
    assert.dom('.copy-button .hds-icon-clipboard-copy').exists();

    clock.restore();
  });

  test('it shows the error icon on error', async function (assert) {
    await render(hbs`<CopyButton @clipboardText="tomster" />`);

    await click('.copy-button button');
    await triggerCopyError('.copy-button button');

    assert.dom('.copy-button .hds-icon-clipboard-x').exists();
  });
});
