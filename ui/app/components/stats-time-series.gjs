/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import moment from 'moment';
import d3TimeFormat from 'd3-time-format';
import d3Format from 'd3-format';
import { scaleTime, scaleLinear } from 'd3-scale';
import d3Array from 'd3-array';
import bind from 'nomad-ui/helpers/bind';
import LineChart from 'nomad-ui/components/line-chart';
import formatDuration from 'nomad-ui/utils/format-duration';

export default class StatsTimeSeries extends Component {
  xFormat = d3TimeFormat.timeFormat('%H:%M:%S');

  yFormat = d3Format.format('.1~%');

  get useDefaults() {
    return !this.args.dataProp;
  }

  get description() {
    const data = this.args.data;
    const yRange = d3Array.extent(data, (d) => d.percent);
    const xRange = d3Array.extent(data, (d) => d.timestamp);

    const duration = formatDuration(xRange[1] - xRange[0], 'ms', true);

    return `Time series data for the last ${duration}, with values ranging from ${this.yFormat(
      yRange[0],
    )} to ${this.yFormat(yRange[1])}`;
  }

  xScale(data, yAxisOffset) {
    const scale = scaleTime();

    const [low, high] = d3Array.extent(data, (d) => d.timestamp);
    const minLow = moment(high).subtract(5, 'minutes').toDate();

    const extent = data.length
      ? [Math.min(low, minLow), high]
      : [minLow, new Date()];
    scale.rangeRound([10, yAxisOffset]).domain(extent);

    return scale;
  }

  yScale(data, xAxisOffset) {
    const yValueKey = this.args.dataProp ? 'percentStack' : 'percent';
    const yValues = (data || []).map((datum) => datum?.[yValueKey]);

    let [low, high] = [0, 1];
    if (yValues.filter((value) => value != null).length) {
      [low, high] = d3Array.extent(yValues);
    }

    return scaleLinear()
      .rangeRound([xAxisOffset, 10])
      .domain([Math.min(0, low), Math.max(1, high)]);
  }

  <template>
    <LineChart
      @data={{@data}}
      @dataProp={{@dataProp}}
      @xProp="timestamp"
      @yProp={{if @dataProp "percentStack" "percent"}}
      @chartClass={{@chartClass}}
      @timeseries={{true}}
      @title="Stats Time Series Chart"
      @description={{this.description}}
      @xScale={{bind this.xScale this}}
      @yScale={{bind this.yScale this}}
      @xFormat={{this.xFormat}}
      @yFormat={{this.yFormat}}
    >
      <:svg as |c|>
        {{#if this.useDefaults}}
          <c.Area @data={{@data}} @colorClass={{@chartClass}} />
        {{/if}}
        {{yield c to="svg"}}
      </:svg>
      <:after as |c|>
        {{#if this.useDefaults}}
          <c.Tooltip class="is-snappy" as |series datum|>
            <li>
              <span class="label"><span
                  class="color-swatch {{@chartClass}}"
                />{{datum.formattedX}}</span>
              <span class="value">{{datum.formattedY}}</span>
            </li>
          </c.Tooltip>
        {{/if}}
        {{yield c to="after"}}
      </:after>
    </LineChart>
  </template>
}
