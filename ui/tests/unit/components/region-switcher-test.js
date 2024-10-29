/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Unit | Component | region-switcher', function (hooks) {
  setupRenderingTest(hooks);

  test('displays single region', async function (assert) {
    const system = {
      shouldShowRegions: false,
    };

    this.set('system', system);

    await render(hbs`<RegionSwitcher/>`);

    assert.dom('.is-region').exists();
  });
});
