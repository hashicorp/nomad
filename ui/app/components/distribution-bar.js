import Ember from 'ember';
import d3 from 'npm:d3-selection';
import 'npm:d3-transition';
import styleStringProperty from '../utils/properties/style-string';

const { Component, computed, run } = Ember;
const sumAggregate = (total, val) => total + val;

export default Component.extend({
  classNames: ['chart', 'distribution-bar'],

  chart: null,
  data: null,
  activeDatum: null,

  tooltipStyle: styleStringProperty('tooltipPosition'),

  _data: computed('data', function() {
    const data = this.get('data');
    const sum = data.mapBy('value').reduce(sumAggregate, 0);

    return data.map(({ label, value, className }, index) => ({
      label,
      value,
      className,
      percent: value / sum * 100,
      offset:
        data.slice(0, index).mapBy('value').reduce(sumAggregate, 0) / sum * 100,
    }));
  }),

  didInsertElement() {
    const chart = d3.select(this.$('svg')[0]);
    this.set('chart', chart);

    chart.on('mouseenter', () => {
      run(() => {
        this.set('isActive', true);
      });
    });

    chart.on('mouseleave', () => {
      run(() => {
        this.set('isActive', false);
        chart
          .selectAll('rect')
          .classed('active', false)
          .classed('inactive', false);
      });
    });

    this.renderChart();
  },

  didUpdateAttrs() {
    this.renderChart();
  },

  renderChart() {
    const { chart, _data } = this.getProperties('chart', '_data');

    let slices = chart.select('.bars').selectAll('rect').data(_data);
    let slicesEnter = slices
      .enter()
      .append('rect')
      .attr('width', d => `${d.percent + 1}%`)
      .attr('x', d => `${d.offset}%`)
      .on('mouseover', d => {
        run(() => {
          const slice = slices.filter(datum => datum === d);
          slices.classed('active', false).classed('inactive', true);
          slice.classed('active', true).classed('inactive', false);
          this.set('activeDatum', d);

          const box = slice.node().getBBox();
          const pos = box.x + box.width / 2;
          this.set('tooltipPosition', {
            left: pos,
          });
        });
      });

    slices = slicesEnter.merge(slices);
    slices
      .attr('class', d => d.className)
      .transition()
      .duration(200)
      .attr('width', d => `${d.percent}%`)
      .attr('x', d => `${d.offset}%`);
  },
});
