import { computed } from '@ember/object';
import moment from 'moment';
import d3TimeFormat from 'd3-time-format';
import d3Format from 'd3-format';
import d3Scale from 'd3-scale';
import d3Array from 'd3-array';
import LineChart from 'nomad-ui/components/line-chart';

export default LineChart.extend({
  xProp: 'timestamp',
  yProp: 'value',
  timeseries: true,

  xFormat() {
    return d3TimeFormat.timeFormat('%H:%M:%S');
  },

  yFormat() {
    return d3Format.format('.1~%');
  },

  xScale: computed('data.[]', 'xProp', 'timeseries', 'yAxisOffset', function() {
    const xProp = this.get('xProp');
    const scale = this.get('timeseries') ? d3Scale.scaleTime() : d3Scale.scaleLinear();

    const [low, high] = d3Array.extent(this.get('data'), d => d[xProp]);
    const minLow = moment(high)
      .subtract(5, 'minutes')
      .toDate();
    scale.rangeRound([10, this.get('yAxisOffset')]).domain([Math.min(low, minLow), high]);

    return scale;
  }),

  yScale: computed('data.[]', 'yProp', 'xAxisOffset', function() {
    return d3Scale
      .scaleLinear()
      .rangeRound([this.get('xAxisOffset'), 10])
      .domain([0, 1]);
  }),
});
