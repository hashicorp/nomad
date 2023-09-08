/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | trim-path', function (hooks) {
  setupRenderingTest(hooks);

  test('it doesnt mess with internal slashes', async function (assert) {
    this.set('inputValue', 'a/b/c/d');
    await render(hbs`{{trim-path this.inputValue}}`);
    assert.dom(this.element).hasText('a/b/c/d');
  });
  test('it will remove a prefix slash', async function (assert) {
    this.set('inputValue', '/a/b/c/d');
    await render(hbs`{{trim-path this.inputValue}}`);
    assert.dom(this.element).hasText('a/b/c/d');
  });
  test('it will remove a suffix slash', async function (assert) {
    this.set('inputValue', 'a/b/c/d/');
    await render(hbs`{{trim-path this.inputValue}}`);
    assert.dom(this.element).hasText('a/b/c/d');
  });
  test('it will remove both at once', async function (assert) {
    this.set('inputValue', '/a/b/c/d/');
    await render(hbs`{{trim-path this.inputValue}}`);
    assert.dom(this.element).hasText('a/b/c/d');
  });
});
