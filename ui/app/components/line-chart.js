/* eslint-disable ember/no-observers */
import Component from '@ember/component';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import { observes } from '@ember-decorators/object';
import { computed as overridable } from 'ember-overridable-computed';
import { guidFor } from '@ember/object/internals';
import { run } from '@ember/runloop';
import { htmlSafe } from '@ember/template';
import d3 from 'd3-selection';
import d3Scale from 'd3-scale';
import d3Axis from 'd3-axis';
import d3Array from 'd3-array';
import d3Shape from 'd3-shape';
import d3Format from 'd3-format';
import d3TimeFormat from 'd3-time-format';
import WindowResizable from 'nomad-ui/mixins/window-resizable';
import styleStringProperty from 'nomad-ui/utils/properties/style-string';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

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

const iconFor = {
  error: 'cancel-circle-fill',
  info: 'info-circle-fill',
};

const iconClassFor = {
  error: 'is-danger',
  info: '',
};

@classic
@classNames('chart', 'line-chart')
@classNameBindings('annotations.length:with-annotations')
export default class LineChart extends Component.extend(WindowResizable) {
  // Public API

  data = null;
  annotations = null;
  activeAnnotation = null;
  onAnnotationClick() {}
  xProp = null;
  yProp = null;
  curve = 'linear';
  timeseries = false;
  chartClass = 'is-primary';

  title = 'Line Chart';

  @overridable(function() {
    return null;
  })
  description;

  // Private Properties

  width = 0;
  height = 0;

  isActive = false;

  @computed()
  get fillId() {
    return `line-chart-fill-${guidFor(this)}`;
  }

  @computed()
  get maskId() {
    return `line-chart-mask-${guidFor(this)}`;
  }

  activeDatum = null;

  @computed('activeDatum')
  get activeDatumLabel() {
    const datum = this.activeDatum;

    if (!datum) return undefined;

    const x = datum[this.xProp];
    return this.xFormat(this.timeseries)(x);
  }

  @computed('activeDatum')
  get activeDatumValue() {
    const datum = this.activeDatum;

    if (!datum) return undefined;

    const y = datum[this.yProp];
    return this.yFormat()(y);
  }

  @computed('curve')
  get curveMethod() {
    const mappings = {
      linear: 'curveLinear',
      stepAfter: 'curveStepAfter',
    };
    assert(`Provided curve "${this.curve}" is not an allowed curve type`, mappings[this.curve]);
    return mappings[this.curve];
  }

  // Overridable functions that retrurn formatter functions
  xFormat(timeseries) {
    return timeseries ? d3TimeFormat.timeFormat('%b %d, %H:%M') : d3Format.format(',');
  }

  yFormat() {
    return d3Format.format(',.2~r');
  }

  tooltipPosition = null;
  @styleStringProperty('tooltipPosition') tooltipStyle;

  @computed('xAxisOffset')
  get chartAnnotationBounds() {
    return {
      height: this.xAxisOffset,
    };
  }
  @styleStringProperty('chartAnnotationBounds') chartAnnotationsStyle;

  @computed('data.[]', 'xProp', 'timeseries', 'yAxisOffset')
  get xScale() {
    const xProp = this.xProp;
    const scale = this.timeseries ? d3Scale.scaleTime() : d3Scale.scaleLinear();
    const data = this.data;

    const domain = data.length ? d3Array.extent(this.data, d => d[xProp]) : [0, 1];

    scale.rangeRound([10, this.yAxisOffset]).domain(domain);

    return scale;
  }

  @computed('data.[]', 'xFormat', 'xProp', 'timeseries')
  get xRange() {
    const { xProp, timeseries, data } = this;
    const range = d3Array.extent(data, d => d[xProp]);
    const formatter = this.xFormat(timeseries);

    return range.map(formatter);
  }

  @computed('data.[]', 'yFormat', 'yProp')
  get yRange() {
    const yProp = this.yProp;
    const range = d3Array.extent(this.data, d => d[yProp]);
    const formatter = this.yFormat();

    return range.map(formatter);
  }

  @computed('data.[]', 'yProp', 'xAxisOffset')
  get yScale() {
    const yProp = this.yProp;
    let max = d3Array.max(this.data, d => d[yProp]) || 1;
    if (max > 1) {
      max = nice(max);
    }

    return d3Scale
      .scaleLinear()
      .rangeRound([this.xAxisOffset, 10])
      .domain([0, max]);
  }

  @computed('xScale')
  get xAxis() {
    const formatter = this.xFormat(this.timeseries);

    return d3Axis
      .axisBottom()
      .scale(this.xScale)
      .ticks(5)
      .tickFormat(formatter);
  }

  @computed('xAxisOffset')
  get yTicks() {
    const height = this.xAxisOffset;
    const tickCount = Math.ceil(height / 120) * 2 + 1;
    const domain = this.yScale.domain();
    const ticks = lerp(domain, tickCount);
    return domain[1] - domain[0] > 1 ? nice(ticks) : ticks;
  }

  @computed('yScale')
  get yAxis() {
    const formatter = this.yFormat();

    return d3Axis
      .axisRight()
      .scale(this.yScale)
      .tickValues(this.yTicks)
      .tickFormat(formatter);
  }

  @computed('yScale')
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

  @computed()
  get xAxisHeight() {
    // Avoid divide by zero errors by always having a height
    if (!this.element) return 1;

    const axis = this.element.querySelector('.x-axis');
    return axis && axis.getBBox().height;
  }

  @computed()
  get yAxisWidth() {
    // Avoid divide by zero errors by always having a width
    if (!this.element) return 1;

    const axis = this.element.querySelector('.y-axis');
    return axis && axis.getBBox().width;
  }

  @overridable('height', 'xAxisHeight', function() {
    return this.height - this.xAxisHeight;
  })
  xAxisOffset;

  @computed('width', 'yAxisWidth')
  get yAxisOffset() {
    return this.width - this.yAxisWidth;
  }

  @computed('data.[]', 'xScale', 'yScale', 'curveMethod')
  get line() {
    const { xScale, yScale, xProp, yProp, curveMethod } = this;

    const line = d3Shape
      .line()
      .curve(d3Shape[curveMethod])
      .defined(d => d[yProp] != null)
      .x(d => xScale(d[xProp]))
      .y(d => yScale(d[yProp]));

    return line(this.data);
  }

  @computed('data.[]', 'xScale', 'yScale', 'curveMethod')
  get area() {
    const { xScale, yScale, xProp, yProp, curveMethod } = this;

    const area = d3Shape
      .area()
      .curve(d3Shape[curveMethod])
      .defined(d => d[yProp] != null)
      .x(d => xScale(d[xProp]))
      .y0(yScale(0))
      .y1(d => yScale(d[yProp]));

    return area(this.data);
  }

  @computed('annotations.[]', 'xScale', 'xProp', 'timeseries')
  get processedAnnotations() {
    const { xScale, xProp, annotations, timeseries } = this;

    if (!annotations || !annotations.length) return null;

    let sortedAnnotations = annotations.sortBy(xProp);
    if (timeseries) {
      sortedAnnotations = sortedAnnotations.reverse();
    }

    let prevX = 0;
    let prevHigh = false;
    return sortedAnnotations.map(annotation => {
      const x = xScale(annotation[xProp]);
      if (prevX && !prevHigh && Math.abs(x - prevX) < 30) {
        prevHigh = true;
      } else if (prevHigh) {
        prevHigh = false;
      }
      const y = prevHigh ? -15 : 0;
      const formattedX = this.xFormat(timeseries)(annotation[xProp]);

      prevX = x;
      return {
        annotation,
        style: htmlSafe(`transform:translate(${x}px,${y}px)`),
        icon: iconFor[annotation.type],
        iconClass: iconClassFor[annotation.type],
        staggerClass: prevHigh ? 'is-staggered' : '',
        label: `${annotation.type} event at ${formattedX}`,
      };
    });
  }

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
  }

  didUpdateAttrs() {
    this.renderChart();
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

    this.set('activeDatum', datum);
    this.set('tooltipPosition', {
      left: xScale(datum[xProp]),
      top: yScale(datum[yProp]) - 10,
    });
  }

  @observes('data.[]')
  updateChart() {
    this.renderChart();
  }

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
    this.onAnnotationClick(annotation);
  }

  windowResizeHandler() {
    run.once(this, this.updateDimensions);
  }

  updateDimensions() {
    const $svg = this.element.querySelector('svg');
    const width = $svg.clientWidth;
    const height = $svg.clientHeight;

    this.setProperties({ width, height });
    this.renderChart();
  }
}
