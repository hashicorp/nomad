import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import sinon from 'sinon';

module('Unit | Component | scale-events-chart', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.refTime = new Date();
    this.clock = sinon.useFakeTimers(this.refTime);
  });

  hooks.afterEach(function() {
    this.clock.restore();
    delete this.refTime;
  });

  test('the current date is appended as a datum for the line chart to render', function(assert) {
    const chart = this.owner.factoryFor('component:scale-events-chart').create();
    const events = [
      { time: new Date('2020-08-02T04:06:00'), count: 2, hasCount: true },
      { time: new Date('2020-08-01T04:06:00'), count: 2, hasCount: true },
    ];

    chart.set('events', events);

    assert.equal(chart.data.length, events.length + 1);
    assert.deepEqual(chart.data.slice(0, events.length), events.sortBy('time'));

    const appendedDatum = chart.data[chart.data.length - 1];
    assert.equal(appendedDatum.count, events.sortBy('time').lastObject.count);
    assert.equal(+appendedDatum.time, +this.refTime);
  });

  test('if the earliest annotation is outside the domain of the events, the earliest annotation time is added as a datum for the line chart to render', function(assert) {
    const chart = this.owner.factoryFor('component:scale-events-chart').create();
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

    chart.set('events', annotationOutside);

    assert.equal(chart.data.length, annotationOutside.length + 1);
    assert.deepEqual(
      chart.data.slice(1, annotationOutside.length),
      annotationOutside.filterBy('hasCount')
    );

    const appendedDatum = chart.data[0];
    assert.equal(appendedDatum.count, annotationOutside[1].count);
    assert.equal(+appendedDatum.time, +annotationOutside[0].time);

    chart.set('events', annotationInside);

    assert.equal(chart.data.length, annotationOutside.length);
    assert.deepEqual(
      chart.data.slice(0, annotationOutside.length - 1),
      annotationOutside.filterBy('hasCount')
    );
  });
});
