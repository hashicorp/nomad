/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | global-search', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`<GlobalSearch />`);

    assert.dom(this.element).hasText('');

    // Template block usage:
    await render(hbs`
      <GlobalSearch>
        template block text
      </GlobalSearch>
    `);

    assert.dom(this.element).hasText('template block text');
  });
});
