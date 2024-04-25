/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { fillIn, find, triggerEvent } from '@ember/test-helpers';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

const DEBOUNCE_MS = 500;

module('Integration | Component | job-search-box', function (hooks) {
  setupRenderingTest(hooks);

  test('debouncer debounces appropriately', async function (assert) {
    assert.expect(5);

    let message = '';

    this.set('externalAction', (value) => {
      message = value;
    });

    await render(
      hbs`<Hds::SegmentedGroup as |S|><JobSearchBox @onSearchTextChange={{this.externalAction}} @S={{S}} /></Hds::SegmentedGroup>`
    );
    await componentA11yAudit(this.element, assert);

    const element = find('input');
    await fillIn('input', 'test1');
    assert.equal(message, 'test1', 'Initial typing');
    element.value += ' wont be ';
    triggerEvent('input', 'input');
    assert.equal(
      message,
      'test1',
      'Typing has happened within debounce window'
    );
    element.value += 'seen ';
    triggerEvent('input', 'input');
    await delay(DEBOUNCE_MS - 100);
    assert.equal(
      message,
      'test1',
      'Typing has happened within debounce window, albeit a little slower'
    );
    element.value += 'until now.';
    triggerEvent('input', 'input');
    await delay(DEBOUNCE_MS + 100);
    assert.equal(
      message,
      'test1 wont be seen until now.',
      'debounce window has closed'
    );
  });
});

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
