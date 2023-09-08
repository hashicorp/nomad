/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Component | gauge-chart', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    this.subject = this.owner.factoryFor('component:gauge-chart');
  });

  test('percent is a function of value and total OR complement', function (assert) {
    const chart = this.subject.create();
    chart.setProperties({
      value: 5,
      total: 10,
    });

    assert.equal(chart.percent, 0.5);

    chart.setProperties({
      total: null,
      complement: 15,
    });

    assert.equal(chart.percent, 0.25);
  });
});
