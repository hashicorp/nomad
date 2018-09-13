import { test, moduleForComponent } from 'ember-qunit';
import moment from 'moment';
import d3Format from 'd3-format';
import d3TimeFormat from 'd3-time-format';

moduleForComponent('stats-time-series', 'Unit | Component | stats-time-series');

const ts = (offset, resolution = 'm') =>
  moment()
    .subtract(offset, resolution)
    .toDate();

const wideData = [
  { timestamp: ts(20), value: 0.5 },
  { timestamp: ts(18), value: 0.5 },
  { timestamp: ts(16), value: 0.4 },
  { timestamp: ts(14), value: 0.3 },
  { timestamp: ts(12), value: 0.9 },
  { timestamp: ts(10), value: 0.3 },
  { timestamp: ts(8), value: 0.3 },
  { timestamp: ts(6), value: 0.4 },
  { timestamp: ts(4), value: 0.5 },
  { timestamp: ts(2), value: 0.6 },
  { timestamp: ts(0), value: 0.6 },
];

const narrowData = [
  { timestamp: ts(20, 's'), value: 0.5 },
  { timestamp: ts(18, 's'), value: 0.5 },
  { timestamp: ts(16, 's'), value: 0.4 },
  { timestamp: ts(14, 's'), value: 0.3 },
  { timestamp: ts(12, 's'), value: 0.9 },
  { timestamp: ts(10, 's'), value: 0.3 },
];

test('xFormat is time-formatted for hours, minutes, and seconds', function(assert) {
  const chart = this.subject();

  chart.set('data', wideData);

  wideData.forEach(datum => {
    assert.equal(
      chart.xFormat()(datum.timestamp),
      d3TimeFormat.timeFormat('%H:%M:%S')(datum.timestamp)
    );
  });
});

test('yFormat is percent-formatted', function(assert) {
  const chart = this.subject();

  chart.set('data', wideData);

  wideData.forEach(datum => {
    assert.equal(chart.yFormat()(datum.value), d3Format.format('.1~%')(datum.value));
  });
});

test('x scale domain is at least five minutes', function(assert) {
  const chart = this.subject();

  chart.set('data', narrowData);

  assert.equal(
    +chart.get('xScale').domain()[0],
    +moment(Math.max(...narrowData.mapBy('timestamp')))
      .subtract(5, 'm')
      .toDate(),
    'The lower bound of the xScale is 5 minutes ago'
  );
});

test('x scale domain is greater than five minutes when the domain of the data is larger than five minutes', function(assert) {
  const chart = this.subject();

  chart.set('data', wideData);

  assert.equal(
    +chart.get('xScale').domain()[0],
    Math.min(...wideData.mapBy('timestamp')),
    'The lower bound of the xScale is the oldest timestamp in the dataset'
  );
});

test('y scale domain is always 0 to 1 (0 to 100%)', function(assert) {
  const chart = this.subject();

  chart.set('data', wideData);

  assert.deepEqual(
    [Math.min(...wideData.mapBy('value')), Math.max(...wideData.mapBy('value'))],
    [0.3, 0.9],
    'The bounds of the value prop of the dataset is narrower than 0 - 1'
  );

  assert.deepEqual(
    chart.get('yScale').domain(),
    [0, 1],
    'The bounds of the yScale are still 0 and 1'
  );
});
