/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import moment from 'moment';
import d3Format from 'd3-format';
import d3TimeFormat from 'd3-time-format';
import setupGlimmerComponentFactory from 'nomad-ui/tests/helpers/glimmer-factory';

module('Unit | Component | stats-time-series', function (hooks) {
  setupTest(hooks);
  setupGlimmerComponentFactory(hooks, 'stats-time-series');

  const ts = (offset, resolution = 'm') =>
    moment().subtract(offset, resolution).toDate();

  const wideData = [
    { timestamp: ts(20), percent: 0.5 },
    { timestamp: ts(18), percent: 0.5 },
    { timestamp: ts(16), percent: 0.4 },
    { timestamp: ts(14), percent: 0.3 },
    { timestamp: ts(12), percent: 0.9 },
    { timestamp: ts(10), percent: 0.3 },
    { timestamp: ts(8), percent: 0.3 },
    { timestamp: ts(6), percent: 0.4 },
    { timestamp: ts(4), percent: 0.5 },
    { timestamp: ts(2), percent: 0.6 },
    { timestamp: ts(0), percent: 0.6 },
  ];

  const narrowData = [
    { timestamp: ts(20, 's'), percent: 0.5 },
    { timestamp: ts(18, 's'), percent: 0.5 },
    { timestamp: ts(16, 's'), percent: 0.4 },
    { timestamp: ts(14, 's'), percent: 0.3 },
    { timestamp: ts(12, 's'), percent: 0.9 },
    { timestamp: ts(10, 's'), percent: 0.3 },
  ];

  const unboundedData = [
    { timestamp: ts(20, 's'), percent: -0.5 },
    { timestamp: ts(18, 's'), percent: 1.5 },
  ];

  const nullData = [
    { timestamp: ts(20, 's'), percent: null },
    { timestamp: ts(18, 's'), percent: null },
  ];

  test('xFormat is time-formatted for hours, minutes, and seconds', function (assert) {
    assert.expect(11);

    const chart = this.createComponent({ data: wideData });

    wideData.forEach((datum) => {
      assert.equal(
        chart.xFormat(datum.timestamp),
        d3TimeFormat.timeFormat('%H:%M:%S')(datum.timestamp)
      );
    });
  });

  test('yFormat is percent-formatted', function (assert) {
    assert.expect(11);

    const chart = this.createComponent({ data: wideData });

    wideData.forEach((datum) => {
      assert.equal(
        chart.yFormat(datum.percent),
        d3Format.format('.1~%')(datum.percent)
      );
    });
  });

  test('x scale domain is at least five minutes', function (assert) {
    const chart = this.createComponent({ data: narrowData });

    assert.equal(
      +chart.xScale(narrowData, 0).domain()[0],
      +moment(Math.max(...narrowData.mapBy('timestamp')))
        .subtract(5, 'm')
        .toDate(),
      'The lower bound of the xScale is 5 minutes ago'
    );
  });

  test('x scale domain is greater than five minutes when the domain of the data is larger than five minutes', function (assert) {
    const chart = this.createComponent({ data: wideData });

    assert.equal(
      +chart.xScale(wideData, 0).domain()[0],
      Math.min(...wideData.mapBy('timestamp')),
      'The lower bound of the xScale is the oldest timestamp in the dataset'
    );
  });

  test('y scale domain is typically 0 to 1 (0 to 100%)', function (assert) {
    const chart = this.createComponent({ data: wideData });

    assert.deepEqual(
      [
        Math.min(...wideData.mapBy('percent')),
        Math.max(...wideData.mapBy('percent')),
      ],
      [0.3, 0.9],
      'The bounds of the value prop of the dataset is narrower than 0 - 1'
    );

    assert.deepEqual(
      chart.yScale(wideData, 0).domain(),
      [0, 1],
      'The bounds of the yScale are still 0 and 1'
    );
  });

  test('the extent of the y domain overrides the default 0 to 1 domain when there are values beyond these bounds', function (assert) {
    const chart = this.createComponent({ data: unboundedData });

    assert.deepEqual(
      chart.yScale(unboundedData, 0).domain(),
      [-0.5, 1.5],
      'The bounds of the yScale match the bounds of the unbounded data'
    );

    chart.args.data = [unboundedData[0]];

    assert.deepEqual(
      chart.yScale(chart.args.data, 0).domain(),
      [-0.5, 1],
      'The upper bound is still the default 1, but the lower bound is overridden due to the unbounded low value'
    );

    chart.args.data = [unboundedData[1]];

    assert.deepEqual(
      chart.yScale(chart.args.data, 0).domain(),
      [0, 1.5],
      'The lower bound is still the default 0, but the upper bound is overridden due to the unbounded high value'
    );
  });

  test('when there are only empty frames in the data array, the default y domain is used', function (assert) {
    const chart = this.createComponent({ data: nullData });

    assert.deepEqual(
      chart.yScale(nullData, 0).domain(),
      [0, 1],
      'The bounds are 0 and 1'
    );
  });
});
