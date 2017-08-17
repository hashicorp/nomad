import Ember from 'ember';
import d3 from 'npm:d3-selection';
import 'npm:d3-transition';
import styleStringProperty from '../utils/properties/style-string';

const { Component, computed, run, assign } = Ember;
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

    return data.map(({ label, value, className, layers }, index) => ({
      label,
      value,
      className,
      layers,
      percent: value / sum,
      offset: data.slice(0, index).mapBy('value').reduce(sumAggregate, 0) / sum,
    }));
  }),

  didInsertElement() {
    const chart = d3.select(this.$('svg')[0]);
    this.set('chart', chart);

    chart.on('mouseleave', () => {
      run(() => {
        this.set('isActive', false);
        chart.selectAll('g').classed('active', false).classed('inactive', false);
      });
    });

    this.renderChart();
  },

  didUpdateAttrs() {
    this.renderChart();
  },

  // prettier-ignore
  /* eslint-disable */
  renderChart() {
    const { chart, _data, isNarrow } = this.getProperties('chart', '_data', 'isNarrow');
    const width = this.$().width();
    const filteredData = _data.filter(d => d.value > 0);

    let slices = chart.select('.bars').selectAll('g').data(filteredData);
    let sliceCount = filteredData.length;

    slices.exit().remove();

    let slicesEnter = slices.enter()
      .append('g')
      .on('mouseenter', d => {
        run(() => {
          const slice = slices.filter(datum => datum === d);
          slices.classed('active', false).classed('inactive', true);
          slice.classed('active', true).classed('inactive', false);
          this.set('activeDatum', d);

          const box = slice.node().getBBox();
          const pos = box.x + box.width / 2;

          // Ensure that the position is set before the tooltip is visible
          run.schedule('afterRender', this, () => this.set('isActive', true));
          this.set('tooltipPosition', {
            left: pos,
          });
        });
      });

    slices = slices.merge(slicesEnter);
    slices.attr('class', d => d.className || `slice-${filteredData.indexOf(d)}`);

    const setWidth = d => `${width * d.percent - (d.index === sliceCount - 1 || d.index === 0 ? 1 : 2)}px`
    const setOffset = d => `${width * d.offset + (d.index === 0 ? 0 : 1)}px`

    let hoverTargets = slices.selectAll('.target').data(d => [d]);
    hoverTargets.enter()
        .append('rect')
        .attr('class', 'target')
        .attr('width', setWidth)
        .attr('height', '100%')
        .attr('x', setOffset)
      .merge(hoverTargets)
      .transition()
        .duration(200)
        .attr('width', setWidth)
        .attr('x', setOffset)


    let layers = slices.selectAll('.bar').data((d, i) => {
      return new Array(d.layers || 1).fill(assign({ index: i }, d));
    });
    layers.enter()
        .append('rect')
        .attr('width', setWidth)
        .attr('x', setOffset)
        .attr('y', '50%')
        .attr('clip-path', 'url(#corners)')
        .attr('height', () => isNarrow ? '6px' : '100%')
      .merge(layers)
        .attr('class', (d, i) => `bar layer-${i}`)
      .transition()
        .duration(200)
        .attr('width', setWidth)
        .attr('x', setOffset)

      if (isNarrow) {
        d3.select('.mask')
          .attr('height', '6px')
          .attr('y', '50%');
      }
  },
  /* eslint-enable */
});
