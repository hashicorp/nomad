import Component from '@ember/component';
import { computed, observer } from '@ember/object';
import { guidFor } from '@ember/object/internals';
import { run } from '@ember/runloop';
import d3 from 'd3-selection';
import d3Scale from 'd3-scale';
import d3Axis from 'd3-axis';
import d3Array from 'd3-array';
import d3Shape from 'd3-shape';
import d3Format from 'd3-format';
import d3TimeFormat from 'd3-time-format';
import WindowResizable from 'nomad-ui/mixins/window-resizable';
import styleStringProperty from 'nomad-ui/utils/properties/style-string';

// Returns a new array with the specified number of points linearly
// distributed across the bounds
const lerp = ([low, high], numPoints) => {
  const step = (high - low) / (numPoints - 1);
  const arr = [];
  for (var i = 0; i < numPoints; i++) {
    arr.push(low + step * i);
  }
  return arr;
};

// Round a number or an array of numbers
const nice = val => (val instanceof Array ? val.map(nice) : Math.round(val));

export default Component.extend(WindowResizable, {
  classNames: ['chart', 'line-chart'],

  // Public API

  data: null,
  xProp: null,
  yProp: null,
  timeseries: false,
  chartClass: 'is-primary',

  title: 'Line Chart',
  description: null,

  // Private Properties

  width: 0,
  height: 0,

  isActive: false,

  fillId: computed(function() {
    return `line-chart-fill-${guidFor(this)}`;
  }),

  maskId: computed(function() {
    return `line-chart-mask-${guidFor(this)}`;
  }),

  activeDatum: null,

  activeDatumLabel: computed('activeDatum', function() {
    const datum = this.get('activeDatum');

    if (!datum) return;

    const x = datum[this.get('xProp')];
    return this.xFormat(this.get('timeseries'))(x);
  }),

  activeDatumValue: computed('activeDatum', function() {
    const datum = this.get('activeDatum');

    if (!datum) return;

    const y = datum[this.get('yProp')];
    return this.yFormat()(y);
  }),

  // Overridable functions that retrurn formatter functions
  xFormat(timeseries) {
    return timeseries ? d3TimeFormat.timeFormat('%b') : d3Format.format(',');
  },

  yFormat() {
    return d3Format.format(',.2~r');
  },

  tooltipPosition: null,
  tooltipStyle: styleStringProperty('tooltipPosition'),

  xScale: computed('data.[]', 'xProp', 'timeseries', 'yAxisOffset', function() {
    const xProp = this.get('xProp');
    const scale = this.get('timeseries') ? d3Scale.scaleTime() : d3Scale.scaleLinear();
    const data = this.get('data');

    const domain = data.length ? d3Array.extent(this.get('data'), d => d[xProp]) : [0, 1];

    scale.rangeRound([10, this.get('yAxisOffset')]).domain(domain);

    return scale;
  }),

  xRange: computed('data.[]', 'xFormat', 'xProp', 'timeseries', function() {
    const { xProp, timeseries, data } = this.getProperties('xProp', 'timeseries', 'data');
    const range = d3Array.extent(data, d => d[xProp]);
    const formatter = this.xFormat(timeseries);

    return range.map(formatter);
  }),

  yRange: computed('data.[]', 'yFormat', 'yProp', function() {
    const yProp = this.get('yProp');
    const range = d3Array.extent(this.get('data'), d => d[yProp]);
    const formatter = this.yFormat();

    return range.map(formatter);
  }),

  yScale: computed('data.[]', 'yProp', 'xAxisOffset', function() {
    const yProp = this.get('yProp');
    let max = d3Array.max(this.get('data'), d => d[yProp]) || 1;
    if (max > 1) {
      max = nice(max);
    }

    return d3Scale
      .scaleLinear()
      .rangeRound([this.get('xAxisOffset'), 10])
      .domain([0, max]);
  }),

  xAxis: computed('xScale', function() {
    const formatter = this.xFormat(this.get('timeseries'));

    return d3Axis
      .axisBottom()
      .scale(this.get('xScale'))
      .ticks(5)
      .tickFormat(formatter);
  }),

  yTicks: computed('xAxisOffset', function() {
    const height = this.get('xAxisOffset');
    const tickCount = Math.ceil(height / 120) * 2 + 1;
    const domain = this.get('yScale').domain();
    const ticks = lerp(domain, tickCount);
    return domain[1] - domain[0] > 1 ? nice(ticks) : ticks;
  }),

  yAxis: computed('yScale', function() {
    const formatter = this.yFormat();

    return d3Axis
      .axisRight()
      .scale(this.get('yScale'))
      .tickValues(this.get('yTicks'))
      .tickFormat(formatter);
  }),

  yGridlines: computed('yScale', function() {
    // The first gridline overlaps the x-axis, so remove it
    const [, ...ticks] = this.get('yTicks');

    return d3Axis
      .axisRight()
      .scale(this.get('yScale'))
      .tickValues(ticks)
      .tickSize(-this.get('yAxisOffset'))
      .tickFormat('');
  }),

  xAxisHeight: computed(function() {
    // Avoid divide by zero errors by always having a height
    if (!this.element) return 1;

    const axis = this.element.querySelector('.x-axis');
    return axis && axis.getBBox().height;
  }),

  yAxisWidth: computed(function() {
    // Avoid divide by zero errors by always having a width
    if (!this.element) return 1;

    const axis = this.element.querySelector('.y-axis');
    return axis && axis.getBBox().width;
  }),

  xAxisOffset: computed('height', 'xAxisHeight', function() {
    return this.get('height') - this.get('xAxisHeight');
  }),

  yAxisOffset: computed('width', 'yAxisWidth', function() {
    return this.get('width') - this.get('yAxisWidth');
  }),

  line: computed('data.[]', 'xScale', 'yScale', function() {
    const { xScale, yScale, xProp, yProp } = this.getProperties(
      'xScale',
      'yScale',
      'xProp',
      'yProp'
    );

    const line = d3Shape
      .line()
      .defined(d => d[yProp] != null)
      .x(d => xScale(d[xProp]))
      .y(d => yScale(d[yProp]));

    return line(this.get('data'));
  }),

  area: computed('data.[]', 'xScale', 'yScale', function() {
    const { xScale, yScale, xProp, yProp } = this.getProperties(
      'xScale',
      'yScale',
      'xProp',
      'yProp'
    );

    const area = d3Shape
      .area()
      .defined(d => d[yProp] != null)
      .x(d => xScale(d[xProp]))
      .y0(yScale(0))
      .y1(d => yScale(d[yProp]));

    return area(this.get('data'));
  }),

  didInsertElement() {
    this.updateDimensions();

    const canvas = d3.select(this.element.querySelector('.canvas'));
    const updateActiveDatum = this.updateActiveDatum.bind(this);

    const chart = this;
    canvas.on('mouseenter', function() {
      const mouseX = d3.mouse(this)[0];
      chart.set('latestMouseX', mouseX);
      updateActiveDatum(mouseX);
      run.schedule('afterRender', chart, () => chart.set('isActive', true));
    });

    canvas.on('mousemove', function() {
      const mouseX = d3.mouse(this)[0];
      chart.set('latestMouseX', mouseX);
      updateActiveDatum(mouseX);
    });

    canvas.on('mouseleave', () => {
      run.schedule('afterRender', this, () => this.set('isActive', false));
      this.set('activeDatum', null);
    });
  },

  didUpdateAttrs() {
    this.renderChart();
  },

  updateActiveDatum(mouseX) {
    const { xScale, xProp, yScale, yProp, data } = this.getProperties(
      'xScale',
      'xProp',
      'yScale',
      'yProp',
      'data'
    );

    if (!data || !data.length) return;

    // Map the mouse coordinate to the index in the data array
    const bisector = d3Array.bisector(d => d[xProp]).left;
    const x = xScale.invert(mouseX);
    const index = bisector(data, x, 1);

    // The data point on either side of the cursor
    const dLeft = data[index - 1];
    const dRight = data[index];

    let datum;

    // If there is only one point, it's the activeDatum
    if (dLeft && !dRight) {
      datum = dLeft;
    } else {
      // Pick the closer point
      datum = x - dLeft[xProp] > dRight[xProp] - x ? dRight : dLeft;
    }

    this.set('activeDatum', datum);
    this.set('tooltipPosition', {
      left: xScale(datum[xProp]),
      top: yScale(datum[yProp]) - 10,
    });
  },

  updateChart: observer('data.[]', function() {
    this.renderChart();
  }),

  // The renderChart method should only ever be responsible for runtime calculations
  // and appending d3 created elements to the DOM (such as axes).
  renderChart() {
    // There is nothing to do if the element hasn't been inserted yet
    if (!this.element) return;

    // First, create the axes to get the dimensions of the resulting
    // svg elements
    this.mountD3Elements();

    run.next(() => {
      // Then, recompute anything that depends on the dimensions
      // on the dimensions of the axes elements
      this.notifyPropertyChange('xAxisHeight');
      this.notifyPropertyChange('yAxisWidth');

      // Since each axis depends on the dimension of the other
      // axis, the axes themselves are recomputed and need to
      // be re-rendered.
      this.mountD3Elements();
      if (this.get('isActive')) {
        this.updateActiveDatum(this.get('latestMouseX'));
      }
    });
  },

  mountD3Elements() {
    if (!this.get('isDestroyed') && !this.get('isDestroying')) {
      d3.select(this.element.querySelector('.x-axis')).call(this.get('xAxis'));
      d3.select(this.element.querySelector('.y-axis')).call(this.get('yAxis'));
      d3.select(this.element.querySelector('.y-gridlines')).call(this.get('yGridlines'));
    }
  },

  windowResizeHandler() {
    run.once(this, this.updateDimensions);
  },

  updateDimensions() {
    const $svg = this.$('svg');
    const width = $svg.width();
    const height = $svg.height();

    this.setProperties({ width, height });
    this.renderChart();
  },
});
