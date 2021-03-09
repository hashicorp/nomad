import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { run } from '@ember/runloop';
import d3 from 'd3-selection';
import d3Scale from 'd3-scale';
import d3Axis from 'd3-axis';
import d3Array from 'd3-array';
import d3Format from 'd3-format';
import d3TimeFormat from 'd3-time-format';
import styleString from 'nomad-ui/utils/properties/glimmer-style-string';
import uniquely from 'nomad-ui/utils/properties/uniquely';

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

const defaultXScale = (data, yAxisOffset, xProp, timeseries) => {
  const scale = timeseries ? d3Scale.scaleTime() : d3Scale.scaleLinear();
  const domain = data.length ? d3Array.extent(data, d => d[xProp]) : [0, 1];

  scale.rangeRound([10, yAxisOffset]).domain(domain);

  return scale;
};

const defaultYScale = (data, xAxisOffset, yProp) => {
  let max = d3Array.max(data, d => d[yProp]) || 1;
  if (max > 1) {
    max = nice(max);
  }

  return d3Scale
    .scaleLinear()
    .rangeRound([xAxisOffset, 10])
    .domain([0, max]);
};

export default class LineChart extends Component {
  /** Args
    data = null;
    xProp = null;
    yProp = null;
    curve = 'linear';
    title = 'Line Chart';
    description = null;
    timeseries = false;
    chartClass = 'is-primary';
    activeAnnotation = null;
    onAnnotationClick() {}
    xFormat;
    yFormat;
    xScale;
    yScale;
  */

  @tracked width = 0;
  @tracked height = 0;
  @tracked isActive = false;
  @tracked activeDatum = null;
  @tracked tooltipPosition = null;
  @tracked element = null;
  @tracked ready = false;

  @uniquely('title') titleId;
  @uniquely('desc') descriptionId;

  get xProp() {
    return this.args.xProp || 'time';
  }
  get yProp() {
    return this.args.yProp || 'value';
  }
  get data() {
    return this.args.data || [];
  }
  get curve() {
    return this.args.curve || 'linear';
  }
  get chartClass() {
    return this.args.chartClass || 'is-primary';
  }

  @action
  xFormat(timeseries) {
    if (this.args.xFormat) return this.args.xFormat;
    return timeseries ? d3TimeFormat.timeFormat('%b %d, %H:%M') : d3Format.format(',');
  }

  @action
  yFormat() {
    if (this.args.yFormat) return this.args.yFormat;
    return d3Format.format(',.2~r');
  }

  get activeDatumLabel() {
    const datum = this.activeDatum;

    if (!datum) return undefined;

    const x = datum[this.xProp];
    return this.xFormat(this.args.timeseries)(x);
  }

  get activeDatumValue() {
    const datum = this.activeDatum;

    if (!datum) return undefined;

    const y = datum[this.yProp];
    return this.yFormat()(y);
  }

  @styleString
  get tooltipStyle() {
    return this.tooltipPosition;
  }

  get xScale() {
    const fn = this.args.xScale || defaultXScale;
    return fn(this.data, this.yAxisOffset, this.xProp, this.args.timeseries);
  }

  get xRange() {
    const { xProp, data } = this;
    const range = d3Array.extent(data, d => d[xProp]);
    const formatter = this.xFormat(this.args.timeseries);

    return range.map(formatter);
  }

  get yRange() {
    const yProp = this.yProp;
    const range = d3Array.extent(this.data, d => d[yProp]);
    const formatter = this.yFormat();

    return range.map(formatter);
  }

  get yScale() {
    const fn = this.args.yScale || defaultYScale;
    return fn(this.data, this.xAxisOffset, this.yProp);
  }

  get xAxis() {
    const formatter = this.xFormat(this.args.timeseries);

    return d3Axis
      .axisBottom()
      .scale(this.xScale)
      .ticks(5)
      .tickFormat(formatter);
  }

  get yTicks() {
    const height = this.xAxisOffset;
    const tickCount = Math.ceil(height / 120) * 2 + 1;
    const domain = this.yScale.domain();
    const ticks = lerp(domain, tickCount);
    return domain[1] - domain[0] > 1 ? nice(ticks) : ticks;
  }

  get yAxis() {
    const formatter = this.yFormat();

    return d3Axis
      .axisRight()
      .scale(this.yScale)
      .tickValues(this.yTicks)
      .tickFormat(formatter);
  }

  get yGridlines() {
    // The first gridline overlaps the x-axis, so remove it
    const [, ...ticks] = this.yTicks;

    return d3Axis
      .axisRight()
      .scale(this.yScale)
      .tickValues(ticks)
      .tickSize(-this.yAxisOffset)
      .tickFormat('');
  }

  get xAxisHeight() {
    // Avoid divide by zero errors by always having a height
    if (!this.element) return 1;

    const axis = this.element.querySelector('.x-axis');
    return axis && axis.getBBox().height;
  }

  get yAxisWidth() {
    // Avoid divide by zero errors by always having a width
    if (!this.element) return 1;

    const axis = this.element.querySelector('.y-axis');
    return axis && axis.getBBox().width;
  }

  get xAxisOffset() {
    return Math.max(0, this.height - this.xAxisHeight);
  }

  get yAxisOffset() {
    return Math.max(0, this.width - this.yAxisWidth);
  }

  @action
  onInsert(element) {
    this.element = element;
    this.updateDimensions();

    const canvas = d3.select(this.element.querySelector('.hover-target'));
    const updateActiveDatum = this.updateActiveDatum.bind(this);

    const chart = this;
    canvas.on('mouseenter', function() {
      const mouseX = d3.mouse(this)[0];
      chart.latestMouseX = mouseX;
      updateActiveDatum(mouseX);
      run.schedule('afterRender', chart, () => (chart.isActive = true));
    });

    canvas.on('mousemove', function() {
      const mouseX = d3.mouse(this)[0];
      chart.latestMouseX = mouseX;
      updateActiveDatum(mouseX);
    });

    canvas.on('mouseleave', () => {
      run.schedule('afterRender', this, () => (this.isActive = false));
      this.activeDatum = null;
    });
  }

  updateActiveDatum(mouseX) {
    const { xScale, xProp, yScale, yProp, data } = this;

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

    this.activeDatum = datum;
    this.tooltipPosition = {
      left: xScale(datum[xProp]),
      top: yScale(datum[yProp]) - 10,
    };
  }

  // The renderChart method should only ever be responsible for runtime calculations
  // and appending d3 created elements to the DOM (such as axes).
  renderChart() {
    // There is nothing to do if the element hasn't been inserted yet
    if (!this.element) return;

    // Create the axes to get the dimensions of the resulting
    // svg elements
    this.mountD3Elements();

    run.next(() => {
      // Since each axis depends on the dimension of the other
      // axis, the axes themselves are recomputed and need to
      // be re-rendered.
      this.mountD3Elements();
      this.ready = true;
      if (this.isActive) {
        this.updateActiveDatum(this.latestMouseX);
      }
    });
  }

  mountD3Elements() {
    if (!this.isDestroyed && !this.isDestroying) {
      d3.select(this.element.querySelector('.x-axis')).call(this.xAxis);
      d3.select(this.element.querySelector('.y-axis')).call(this.yAxis);
      d3.select(this.element.querySelector('.y-gridlines')).call(this.yGridlines);
    }
  }

  annotationClick(annotation) {
    this.args.onAnnotationClick && this.args.onAnnotationClick(annotation);
  }

  @action
  updateDimensions() {
    const $svg = this.element.querySelector('svg');

    this.height = $svg.clientHeight;
    this.width = $svg.clientWidth;
    this.renderChart();
  }
}
