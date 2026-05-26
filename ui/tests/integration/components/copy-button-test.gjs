/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render, find, waitUntil } from '@ember/test-helpers';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import CopyButton from 'nomad-ui/components/copy-button';
import sinon from 'sinon';

function stubClipboardWrite(stub) {
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: stub },
  });
}

module('Integration | Component | copy-button', function (hooks) {
  setupRenderingTest(hooks);

  test('it shows the copy icon by default', async function (assert) {
    await render(<template><CopyButton /></template>);

    assert.dom('.copy-button .hds-icon-clipboard-copy').exists();
    await componentA11yAudit(find('.copy-button'), assert);
  });

  test('it shows the success icon on success and resets afterward', async function (assert) {
    const writeText = sinon.stub().resolves();
    stubClipboardWrite(writeText);

    await render(<template><CopyButton @clipboardText="tomster" /></template>);

    await click('.copy-button button');

    assert.dom('[data-test-copy-success]').exists();
    await componentA11yAudit(find('.copy-button'), assert);

    await waitUntil(() => !find('[data-test-copy-success]'), { timeout: 3000 });

    assert.dom('[data-test-copy-success]').doesNotExist();
    assert.dom('.copy-button .hds-icon-clipboard-copy').exists();
    assert.ok(writeText.calledWith('tomster'));
  });

  test('it shows the error icon on error', async function (assert) {
    const writeText = sinon.stub().rejects(new Error('clipboard error'));
    stubClipboardWrite(writeText);

    await render(<template><CopyButton @clipboardText="tomster" /></template>);

    await click('.copy-button button');

    assert.dom('.copy-button .hds-icon-clipboard-x').exists();
    assert.ok(writeText.calledWith('tomster'));
    await componentA11yAudit(find('.copy-button'), assert);
  });
});
