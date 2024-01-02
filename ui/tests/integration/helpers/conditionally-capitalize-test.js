/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | conditionally-capitalize', function (hooks) {
  setupRenderingTest(hooks);

  test('it capizalizes words correctly with a boolean condition', async function (assert) {
    this.set('condition', true);
    await render(hbs`{{conditionally-capitalize "tester" this.condition}}`);
    assert.dom(this.element).hasText('Tester');
    this.set('condition', false);
    await render(hbs`{{conditionally-capitalize "tester" this.condition}}`);
    assert.dom(this.element).hasText('tester');
  });

  test('it capizalizes words correctly with an existence condition', async function (assert) {
    this.set('condition', {});
    await render(hbs`{{conditionally-capitalize "tester" this.condition}}`);
    assert.dom(this.element).hasText('Tester');
    this.set('condition', null);
    await render(hbs`{{conditionally-capitalize "tester" this.condition}}`);
    assert.dom(this.element).hasText('tester');
  });

  test('it capizalizes words correctly with an numeric condition', async function (assert) {
    this.set('condition', 1);
    await render(hbs`{{conditionally-capitalize "tester" this.condition}}`);
    assert.dom(this.element).hasText('Tester');
    this.set('condition', 0);
    await render(hbs`{{conditionally-capitalize "tester" this.condition}}`);
    assert.dom(this.element).hasText('tester');
  });
});
