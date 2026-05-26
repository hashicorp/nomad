/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { hash } from '@ember/helper';
import { guidFor } from '@ember/object/internals';
import { schedule, next } from '@ember/runloop';
import { didInsert, didUpdate } from '@ember/render-modifiers';
import d3 from 'd3-selection';
import d3Scale from 'd3-scale';
import d3Axis from 'd3-axis';
import d3Array from 'd3-array';
import d3Format from 'd3-format';
import d3TimeFormat from 'd3-time-format';
import Area from 'nomad-ui/components/chart-primitives/area';
import HAnnotations from 'nomad-ui/components/chart-primitives/h-annotations';
import Tooltip from 'nomad-ui/components/chart-primitives/tooltip';
import VAnnotations from 'nomad-ui/components/chart-primitives/v-annotations';
import windowResize from 'nomad-ui/modifiers/window-resize';
import styleString from 'nomad-ui/utils/properties/glimmer-style-string';

const lerp = ([low, high], numPoints) => {
  const step = (high - low) / (numPoints - 1);
  const values = [];
  for (let index = 0; index < numPoints; index++) {
    values.push(low + step * index);
  }
  return values;
};

const nice = (value) =>
  value instanceof Array ? value.map(nice) : Math.round(value);

const defaultXScale = (data, yAxisOffset, xProp, timeseries) => {
  const scale = timeseries ? d3Scale.scaleTime() : d3Scale.scaleLinear();
  const domain = data.length
    ? d3Array.extent(data, (datum) => datum[xProp])
    : [0, 1];

  scale.rangeRound([10, yAxisOffset]).domain(domain);

  return scale;
};

const defaultYScale = (data, xAxisOffset, yProp) => {
  let max = d3Array.max(data, (datum) => datum[yProp]) || 1;
  if (max > 1) {
    max = nice(max);
  }

  return d3Scale.scaleLinear().rangeRound([xAxisOffset, 10]).domain([0, max]);
};

export default class LineChart extends Component {
  @tracked width = 0;
  @tracked height = 0;
  @tracked isActive = false;
  @tracked activeDatum = null;
  @tracked activeData = [];
  @tracked tooltipPosition = null;
  @tracked element = null;
  @tracked ready = false;

  latestMouseX = 0;

  get titleId() {
    return `title-${guidFor(this)}`;
  }

  get descriptionId() {
    return `desc-${guidFor(this)}`;
  }

  get xProp() {
    return this.args.xProp || 'time';
  }

  get yProp() {
    return this.args.yProp || 'value';
  }

  get data() {
    if (!this.args.data) return [];
    if (this.args.dataProp) {
      return this.args.data.map((item) => item?.[this.args.dataProp]).flat();
    }
    return this.args.data;
  }

  get curve() {
    return this.args.curve || 'linear';
  }

  xFormat = (timeseries) => {
    if (this.args.xFormat) return this.args.xFormat;
    return timeseries
      ? d3TimeFormat.timeFormat('%b %d, %H:%M')
      : d3Format.format(',');
  };

  yFormat = () => {
    if (this.args.yFormat) return this.args.yFormat;
    return d3Format.format(',.2~r');
  };

  get title() {
    return this.args.title || 'Line Chart';
  }

  get description() {
    return this.args.description;
  }

  get activeDatumLabel() {
    const datum = this.activeDatum;

    if (!datum) return undefined;

    return this.xFormat(this.args.timeseries)(datum[this.xProp]);
  }

  get activeDatumValue() {
    const datum = this.activeDatum;

    if (!datum) return undefined;

    return this.yFormat()(datum[this.yProp]);
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
    const formatter = this.xFormat(this.args.timeseries);
    return d3Array
      .extent(this.data, (datum) => datum[this.xProp])
      .map(formatter);
  }

  get yRange() {
    const formatter = this.yFormat();
    return d3Array
      .extent(this.data, (datum) => datum[this.yProp])
      .map(formatter);
  }

  get xRangeStart() {
    return this.xRange[0];
  }

  get xRangeEnd() {
    return this.xRange[this.xRange.length - 1];
  }

  get yRangeStart() {
    return this.yRange[0];
  }

  get yRangeEnd() {
    return this.yRange[this.yRange.length - 1];
  }

  get yScale() {
    const fn = this.args.yScale || defaultYScale;
    return fn(this.data, this.xAxisOffset, this.yProp);
  }

  get xAxis() {
    return d3Axis
      .axisBottom()
      .scale(this.xScale)
      .ticks(5)
      .tickFormat(this.xFormat(this.args.timeseries));
  }

  get yTicks() {
    const tickCount = Math.ceil(this.xAxisOffset / 120) * 2 + 1;
    const domain = this.yScale.domain();
    const ticks = lerp(domain, tickCount);
    return domain[1] - domain[0] > 1 ? nice(ticks) : ticks;
  }

  get yAxis() {
    return d3Axis
      .axisRight()
      .scale(this.yScale)
      .tickValues(this.yTicks)
      .tickFormat(this.yFormat());
  }

  get yGridlines() {
    const [, ...ticks] = this.yTicks;

    return d3Axis
      .axisRight()
      .scale(this.yScale)
      .tickValues(ticks)
      .tickSize(-this.canvasDimensions.width)
      .tickFormat('');
  }

  get xAxisHeight() {
    if (!this.element) return 1;

    const axis = this.element.querySelector('.x-axis');
    return axis && axis.getBBox().height;
  }

  get yAxisWidth() {
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

  get canvasDimensions() {
    const [left, right] = this.xScale.range();
    const [top, bottom] = this.yScale.range();
    return { left, width: right - left, top, height: bottom - top };
  }

  onInsert = (element) => {
    this.element = element;
    this.updateDimensions();

    const canvas = d3.select(this.element.querySelector('.hover-target'));
    const updateActiveDatum = this.updateActiveDatum.bind(this);

    canvas.on('mouseenter', (event) => {
      const mouseX = d3.pointer(event, canvas.node())[0];
      this.latestMouseX = mouseX;
      updateActiveDatum(mouseX);
      schedule('afterRender', this, () => (this.isActive = true));
    });

    canvas.on('mousemove', (event) => {
      const mouseX = d3.pointer(event, canvas.node())[0];
      this.latestMouseX = mouseX;
      updateActiveDatum(mouseX);
    });

    canvas.on('mouseleave', () => {
      schedule('afterRender', this, () => (this.isActive = false));
      this.activeDatum = null;
      this.activeData = [];
    });
  };

  updateActiveDatum(mouseX) {
    if (!this.data?.length) return;

    const { xScale, xProp, yScale, yProp } = this;
    let { dataProp, data } = this.args;

    if (!dataProp) {
      dataProp = 'data';
      data = [{ data: this.data }];
    }

    const bisector = d3Array.bisector((datum) => datum[xProp]).left;
    const x = xScale.invert(mouseX);

    const activeData = data
      .map((series, seriesIndex) => {
        const dataset = series[dataProp];
        if (!dataset.length) return null;

        const index = bisector(dataset, x, 1);
        const dLeft = dataset[index - 1];
        const dRight = dataset[index];

        let datum;
        if (dLeft && !dRight) {
          datum = dLeft;
        } else {
          datum = x - dLeft[xProp] > dRight[xProp] - x ? dRight : dLeft;
        }

        return {
          series,
          datum: {
            formattedX: this.xFormat(this.args.timeseries)(datum[xProp]),
            formattedY: this.yFormat()(datum[yProp]),
            datum,
          },
          index: data.length - seriesIndex - 1,
        };
      })
      .filter(Boolean);

    const closestDatum = activeData
      .slice()
      .sort(
        (a, b) =>
          Math.abs(a.datum.datum[xProp] - x) -
          Math.abs(b.datum.datum[xProp] - x),
      )[0];

    const dist = Math.abs(xScale(closestDatum.datum.datum[xProp]) - mouseX);
    const filteredData = activeData.filter(
      (entry) =>
        Math.abs(xScale(entry.datum.datum[xProp]) - mouseX) < dist + 10,
    );

    this.activeData = filteredData;
    this.activeDatum = closestDatum.datum.datum;
    this.tooltipPosition = {
      left: xScale(this.activeDatum[xProp]),
      top: yScale(this.activeDatum[yProp]) - 10,
    };
  }

  renderChart = () => {
    if (!this.element) return;

    this.mountD3Elements();

    next(() => {
      this.mountD3Elements();
      this.ready = true;
      if (this.isActive) {
        this.updateActiveDatum(this.latestMouseX);
      }
    });
  };

  recomputeXAxis = (element) => {
    if (!this.isDestroyed && !this.isDestroying) {
      d3.select(element.querySelector('.x-axis')).call(this.xAxis);
    }
  };

  recomputeYAxis = (element) => {
    if (!this.isDestroyed && !this.isDestroying) {
      d3.select(element.querySelector('.y-axis')).call(this.yAxis);
    }
  };

  mountD3Elements() {
    if (!this.isDestroyed && !this.isDestroying) {
      d3.select(this.element.querySelector('.x-axis')).call(this.xAxis);
      d3.select(this.element.querySelector('.y-axis')).call(this.yAxis);
      d3.select(this.element.querySelector('.y-gridlines')).call(
        this.yGridlines,
      );
    }
  }

  annotationClick = (annotation) => {
    this.args.onAnnotationClick?.(annotation);
  };

  updateDimensions = () => {
    const svg = this.element.querySelector('svg');

    this.height = svg.clientHeight;
    this.width = svg.clientWidth;
    this.renderChart();
  };

  <template>
    <div
      class="chart line-chart"
      ...attributes
      {{didInsert this.onInsert}}
      {{didUpdate this.renderChart}}
      {{didUpdate this.recomputeXAxis this.xScale}}
      {{didUpdate this.recomputeYAxis this.yScale}}
      {{windowResize this.updateDimensions}}
    >
      <svg
        data-test-line-chart
        aria-labelledby={{this.titleId}}
        aria-describedby={{this.descriptionId}}
      >
        <title id={{this.titleId}}>{{this.title}}</title>
        <desc id={{this.descriptionId}}>
          {{#if this.description}}
            {{this.description}}
          {{else}}
            X-axis values range from
            {{this.xRangeStart}}
            to
            {{this.xRangeEnd}}, and Y-axis values range from
            {{this.yRangeStart}}
            to
            {{this.yRangeEnd}}.
          {{/if}}
        </desc>
        <g
          class="y-gridlines gridlines"
          transform="translate({{this.yAxisOffset}}, 0)"
        ></g>
        {{#if this.ready}}
          {{yield
            (hash
              Area=(component
                Area
                curve="linear"
                xScale=this.xScale
                yScale=this.yScale
                xProp=this.xProp
                yProp=this.yProp
                width=this.yAxisOffset
                height=this.xAxisOffset
              )
            )
            to="svg"
          }}
        {{/if}}
        <g
          aria-hidden="true"
          class="x-axis axis"
          transform="translate(0, {{this.xAxisOffset}})"
        ></g>
        <g
          aria-hidden="true"
          class="y-axis axis"
          transform="translate({{this.yAxisOffset}}, 0)"
        ></g>
        <rect
          data-test-hover-target
          class="hover-target"
          x="0"
          y="0"
          width="{{this.yAxisOffset}}"
          height="{{this.xAxisOffset}}"
        />
      </svg>
      {{#if this.ready}}
        {{yield
          (hash
            VAnnotations=(component
              VAnnotations
              timeseries=@timeseries
              format=this.xFormat
              scale=this.xScale
              prop=this.xProp
              height=this.xAxisOffset
            )
            HAnnotations=(component
              HAnnotations
              format=this.yFormat
              scale=this.yScale
              prop=this.yProp
              left=this.canvasDimensions.left
              width=this.canvasDimensions.width
            )
            Tooltip=(component
              Tooltip
              active=this.activeData.length
              style=this.tooltipStyle
              data=this.activeData
            )
          )
          to="after"
        }}
      {{/if}}
    </div>
  </template>
}
