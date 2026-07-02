/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { fillIn, find, render, triggerEvent } from '@ember/test-helpers';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import JobSearchBox from 'nomad-ui/components/job-search-box';

const DEBOUNCE_MS = 500;

module('Integration | Component | job-search-box', function (hooks) {
  setupRenderingTest(hooks);

  test('debouncer debounces appropriately', async function (assert) {
    let message = '';
    const externalAction = (value) => {
      message = value;
    };

    await render(
      <template>
        <JobSearchBox @onSearchTextChange={{externalAction}} />
      </template>,
    );
    await componentA11yAudit(find('[data-test-jobs-search]'), assert);

    const element = find('input');
    await fillIn('input', 'test1');
    assert.deepEqual(message, 'test1', 'Initial typing');

    element.value += ' wont be ';
    triggerEvent('input', 'input');
    assert.deepEqual(
      message,
      'test1',
      'Typing has happened within debounce window',
    );

    element.value += 'seen ';
    triggerEvent('input', 'input');
    await delay(DEBOUNCE_MS - 100);
    assert.deepEqual(
      message,
      'test1',
      'Typing has happened within debounce window, albeit a little slower',
    );

    element.value += 'until now.';
    triggerEvent('input', 'input');
    await delay(DEBOUNCE_MS + 100);
    assert.deepEqual(
      message,
      'test1 wont be seen until now.',
      'debounce window has closed',
    );
  });
});

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
