/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import sinon from 'sinon';
import setupGlimmerComponentFactory from 'nomad-ui/tests/helpers/glimmer-factory';

module('Unit | Component | scale-events-chart', function (hooks) {
  setupTest(hooks);
  setupGlimmerComponentFactory(hooks, 'scale-events-chart');

  hooks.beforeEach(function () {
    this.refTime = new Date();
    this.clock = sinon.useFakeTimers(this.refTime);
  });

  hooks.afterEach(function () {
    this.clock.restore();
    delete this.refTime;
  });

  test('the current date is appended as a datum for the line chart to render', function (assert) {
    const events = [
      { time: new Date('2020-08-02T04:06:00'), count: 2, hasCount: true },
      { time: new Date('2020-08-01T04:06:00'), count: 2, hasCount: true },
    ];

    const chart = this.createComponent({ events });

    assert.deepEqual(chart.data.length, events.length + 1);
    assert.deepEqual(chart.data.slice(0, events.length), events.sortBy('time'));

    const appendedDatum = chart.data[chart.data.length - 1];
    assert.deepEqual(
      appendedDatum.count,
      events.sortBy('time').lastObject.count,
    );
    assert.deepEqual(+appendedDatum.time, +this.refTime);
  });

  test('if the earliest annotation is outside the domain of the events, the earliest annotation time is added as a datum for the line chart to render', function (assert) {
    const annotationOutside = [
      { time: new Date('2020-08-01T04:06:00'), hasCount: false, error: true },
      { time: new Date('2020-08-02T04:06:00'), count: 2, hasCount: true },
      { time: new Date('2020-08-03T04:06:00'), count: 2, hasCount: true },
    ];
    const annotationInside = [
      { time: new Date('2020-08-02T04:06:00'), count: 2, hasCount: true },
      { time: new Date('2020-08-02T12:06:00'), hasCount: false, error: true },
      { time: new Date('2020-08-03T04:06:00'), count: 2, hasCount: true },
    ];

    const chart = this.createComponent({ events: annotationOutside });

    assert.deepEqual(chart.data.length, annotationOutside.length + 1);
    assert.deepEqual(
      chart.data.slice(1, annotationOutside.length),
      annotationOutside.filterBy('hasCount'),
    );

    const appendedDatum = chart.data[0];
    assert.deepEqual(appendedDatum.count, annotationOutside[1].count);
    assert.deepEqual(+appendedDatum.time, +annotationOutside[0].time);

    chart.args.events = annotationInside;

    assert.deepEqual(chart.data.length, annotationOutside.length);
    assert.deepEqual(
      chart.data.slice(0, annotationOutside.length - 1),
      annotationOutside.filterBy('hasCount'),
    );
  });
});
