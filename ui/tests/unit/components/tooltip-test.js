/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import setupGlimmerComponentFactory from 'nomad-ui/tests/helpers/glimmer-factory';

module('Unit | Component | tooltip', function (hooks) {
  setupTest(hooks);
  setupGlimmerComponentFactory(hooks, 'tooltip');

  test('long texts are ellipsised in the middle', function (assert) {
    const tooltip = this.createComponent({
      text: 'reeeeeeeeeeeeeeeeeally long text',
    });
    assert.equal(tooltip.text, 'reeeeeeeeeeeeee...long text');
  });
});
