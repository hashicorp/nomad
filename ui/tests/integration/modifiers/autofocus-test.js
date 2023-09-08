/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Modifier | autofocus', function (hooks) {
  setupRenderingTest(hooks);

  test('Basic Usage', async function (assert) {
    await render(hbs`
    <form>
      <label>
        <input data-test-input-1 {{autofocus}} />
      </label>
    </form>`);

    assert
      .dom('[data-test-input-1]')
      .isFocused('Autofocus on an element works');
  });

  test('Multiple foci', async function (assert) {
    await render(hbs`
    <form>
      <label>
        <input data-test-input-1 {{autofocus}} />
      </label>
      <label>
        <input data-test-input-2 {{autofocus}} />
      </label>
    </form>`);

    assert
      .dom('[data-test-input-1]')
      .isNotFocused('With multiple autofocus elements, priors are unfocused');
    assert
      .dom('[data-test-input-2]')
      .isFocused('With multiple autofocus elements, posteriors are focused');
  });

  test('Ignore parameter', async function (assert) {
    await render(hbs`
    <form>
    <label>
        <input data-test-input-1 {{autofocus}} />
      </label>
      <label>
        <input data-test-input-2 {{autofocus}} />
      </label>
      <label>
        <input data-test-input-3 {{autofocus ignore=true}} />
      </label>
      <label>
        <input data-test-input-4 {{autofocus ignore=true}} />
      </label>
    </form>`);

    assert
      .dom('[data-test-input-2]')
      .isFocused('The last autofocus element without ignore is focused');
    assert
      .dom('[data-test-input-3]')
      .isNotFocused('Ignore parameter is observed, prior');
    assert
      .dom('[data-test-input-4]')
      .isNotFocused('Ignore parameter is observed, posterior');
  });
});
