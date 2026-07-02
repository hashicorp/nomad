/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import setupGlimmerComponentFactory from 'nomad-ui/tests/helpers/glimmer-factory';

module('Unit | Component | gauge-chart', function (hooks) {
  setupTest(hooks);
  setupGlimmerComponentFactory(hooks, 'gauge-chart');

  test('percent is a function of value and total OR complement', function (assert) {
    const chart = this.createComponent({
      value: 5,
      total: 10,
    });

    assert.deepEqual(chart.percent, 0.5);

    chart.args.total = null;
    chart.args.complement = 15;

    assert.deepEqual(chart.percent, 0.25);
  });
});
