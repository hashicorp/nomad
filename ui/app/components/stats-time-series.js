import { computed } from '@ember/object';
import moment from 'moment';
import d3TimeFormat from 'd3-time-format';
import d3Format from 'd3-format';
import d3Scale from 'd3-scale';
import d3Array from 'd3-array';
import LineChart from 'nomad-ui/components/line-chart';
import formatDuration from 'nomad-ui/utils/format-duration';

export default LineChart.extend({
  xProp: 'timestamp',
  yProp: 'percent',
  timeseries: true,

  xFormat() {
    return d3TimeFormat.timeFormat('%H:%M:%S');
  },

  yFormat() {
    return d3Format.format('.1~%');
  },

  // Specific a11y descriptors
  title: 'Stats Time Series Chart',

  description: computed('data.[]', 'xProp', 'yProp', function() {
    const { xProp, yProp, data } = this;
    const yRange = d3Array.extent(data, d => d[yProp]);
    const xRange = d3Array.extent(data, d => d[xProp]);
    const yFormatter = this.yFormat();

    const duration = formatDuration(xRange[1] - xRange[0], 'ms', true);

    return `Time series data for the last ${duration}, with values ranging from ${yFormatter(yRange[0])} to ${yFormatter(yRange[1])}`;
  }),

  xScale: computed('data.[]', 'xProp', 'timeseries', 'yAxisOffset', function() {
    const xProp = this.xProp;
    const scale = this.timeseries ? d3Scale.scaleTime() : d3Scale.scaleLinear();
    const data = this.data;

    const [low, high] = d3Array.extent(data, d => d[xProp]);
    const minLow = moment(high)
      .subtract(5, 'minutes')
      .toDate();

    const extent = data.length ? [Math.min(low, minLow), high] : [minLow, new Date()];
    scale.rangeRound([10, this.yAxisOffset]).domain(extent);

    return scale;
  }),

  yScale: computed('data.[]', 'yProp', 'xAxisOffset', function() {
    const yProp = this.yProp;
    const yValues = (this.data || []).mapBy(yProp);

    let [low, high] = [0, 1];
    if (yValues.compact().length) {
      [low, high] = d3Array.extent(yValues);
    }

    return d3Scale
      .scaleLinear()
      .rangeRound([this.xAxisOffset, 10])
      .domain([Math.min(0, low), Math.max(1, high)]);
  }),
});
