import { test, moduleForComponent } from 'ember-qunit';
import d3Format from 'd3-format';

moduleForComponent('line-chart', 'Unit | Component | line-chart');

const data = [
  { foo: 1, bar: 100 },
  { foo: 2, bar: 200 },
  { foo: 3, bar: 300 },
  { foo: 8, bar: 400 },
  { foo: 4, bar: 500 },
];

test('x scale domain is the min and max values in data based on the xProp value', function(assert) {
  const chart = this.subject();

  chart.setProperties({
    xProp: 'foo',
    data,
  });

  let [xDomainLow, xDomainHigh] = chart.get('xScale').domain();
  assert.equal(
    xDomainLow,
    Math.min(...data.mapBy('foo')),
    'Domain lower bound is the lowest foo value'
  );
  assert.equal(
    xDomainHigh,
    Math.max(...data.mapBy('foo')),
    'Domain upper bound is the highest foo value'
  );

  chart.set('data', [...data, { foo: 12, bar: 600 }]);

  [, xDomainHigh] = chart.get('xScale').domain();
  assert.equal(xDomainHigh, 12, 'When the data changes, the xScale is recalculated');
});

test('y scale domain uses the max value in the data based off of yProp, but is always zero-based', function(assert) {
  const chart = this.subject();

  chart.setProperties({
    yProp: 'bar',
    data,
  });

  let [yDomainLow, yDomainHigh] = chart.get('yScale').domain();
  assert.equal(yDomainLow, 0, 'Domain lower bound is always 0');
  assert.equal(
    yDomainHigh,
    Math.max(...data.mapBy('bar')),
    'Domain upper bound is the highest bar value'
  );

  chart.set('data', [...data, { foo: 12, bar: 600 }]);

  [, yDomainHigh] = chart.get('yScale').domain();
  assert.equal(yDomainHigh, 600, 'When the data changes, the yScale is recalculated');
});

test('the number of yTicks is always odd (to always have a mid-line) and is based off the chart height', function(assert) {
  const chart = this.subject();

  chart.setProperties({
    yProp: 'bar',
    xAxisOffset: 100,
    data,
  });

  assert.equal(chart.get('yTicks').length, 3);

  chart.set('xAxisOffset', 240);
  assert.equal(chart.get('yTicks').length, 5);

  chart.set('xAxisOffset', 241);
  assert.equal(chart.get('yTicks').length, 7);
});

test('the values for yTicks are rounded to whole numbers', function(assert) {
  const chart = this.subject();

  chart.setProperties({
    yProp: 'bar',
    xAxisOffset: 100,
    data,
  });

  assert.deepEqual(chart.get('yTicks'), [0, 250, 500]);

  chart.set('xAxisOffset', 240);
  assert.deepEqual(chart.get('yTicks'), [0, 125, 250, 375, 500]);

  chart.set('xAxisOffset', 241);
  assert.deepEqual(chart.get('yTicks'), [0, 83, 167, 250, 333, 417, 500]);
});

test('the values for yTicks are fractions when the domain is between 0 and 1', function(assert) {
  const chart = this.subject();

  chart.setProperties({
    yProp: 'bar',
    xAxisOffset: 100,
    data: [
      { foo: 1, bar: 0.1 },
      { foo: 2, bar: 0.2 },
      { foo: 3, bar: 0.3 },
      { foo: 8, bar: 0.4 },
      { foo: 4, bar: 0.5 },
    ],
  });

  assert.deepEqual(chart.get('yTicks'), [0, 0.25, 0.5]);
});

test('activeDatumLabel is the xProp value of the activeDatum formatted with xFormat', function(assert) {
  const chart = this.subject();

  chart.setProperties({
    xProp: 'foo',
    yProp: 'bar',
    data,
    activeDatum: data[1],
  });

  assert.equal(
    chart.get('activeDatumLabel'),
    d3Format.format(',')(data[1].foo),
    'activeDatumLabel correctly formats the correct prop of the correct datum'
  );
});

test('activeDatumValue is the yProp value of the activeDatum formatted with yFormat', function(assert) {
  const chart = this.subject();

  chart.setProperties({
    xProp: 'foo',
    yProp: 'bar',
    data,
    activeDatum: data[1],
  });

  assert.equal(
    chart.get('activeDatumValue'),
    d3Format.format(',.2~r')(data[1].bar),
    'activeDatumValue correctly formats the correct prop of the correct datum'
  );
});
