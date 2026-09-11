/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | policy-editor', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    assert.expect(0);
    await render(hbs`<PolicyEditor />`);
  });

  test('Only has editable name if new', async function (assert) {
    const newMockPolicy = {
      isNew: true,
      name: 'New Policy',
    };

    const oldMockPolicy = {
      isNew: false,
      name: 'Old Policy',
    };

    this.set('newMockPolicy', newMockPolicy);
    this.set('oldMockPolicy', oldMockPolicy);

    await render(hbs`<PolicyEditor @policy={{this.newMockPolicy}} />`);
    assert.dom('[data-test-policy-name-input]').exists();
    await render(hbs`<PolicyEditor @policy={{this.oldMockPolicy}} />`);
    assert.dom('[data-test-policy-name-input]').doesNotExist();
  });
});
