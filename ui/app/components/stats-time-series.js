/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import moment from 'moment';
import d3TimeFormat from 'd3-time-format';
import d3Format from 'd3-format';
import { scaleTime, scaleLinear } from 'd3-scale';
import d3Array from 'd3-array';
import formatDuration from 'nomad-ui/utils/format-duration';

export default class StatsTimeSeries extends Component {
  get xFormat() {
    return d3TimeFormat.timeFormat('%H:%M:%S');
  }

  get yFormat() {
    return d3Format.format('.1~%');
  }

  get useDefaults() {
    return !this.args.dataProp;
  }

  // Specific a11y descriptors
  get description() {
    const data = this.args.data;
    const yRange = d3Array.extent(data, (d) => d.percent);
    const xRange = d3Array.extent(data, (d) => d.timestamp);
    const yFormatter = this.yFormat;

    const duration = formatDuration(xRange[1] - xRange[0], 'ms', true);

    return `Time series data for the last ${duration}, with values ranging from ${yFormatter(
      yRange[0]
    )} to ${yFormatter(yRange[1])}`;
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
    const yValues = (data || []).mapBy(
      this.args.dataProp ? 'percentStack' : 'percent'
    );

    let [low, high] = [0, 1];
    if (yValues.compact().length) {
      [low, high] = d3Array.extent(yValues);
    }

    return scaleLinear()
      .rangeRound([xAxisOffset, 10])
      .domain([Math.min(0, low), Math.max(1, high)]);
  }
}
