import Component from '@ember/component';
import { computed, observer, set } from '@ember/object';
import { run } from '@ember/runloop';
import { assign } from '@ember/polyfills';
import { guidFor } from '@ember/object/internals';
import { copy } from 'ember-copy';
import d3 from 'd3-selection';
import 'd3-transition';
import WindowResizable from '../mixins/window-resizable';
import styleStringProperty from '../utils/properties/style-string';

const sumAggregate = (total, val) => total + val;

export default Component.extend(WindowResizable, {
  classNames: ['chart', 'distribution-bar'],
  classNameBindings: ['isNarrow:is-narrow'],

  chart: null,
  data: null,
  activeDatum: null,
  isNarrow: false,

  tooltipStyle: styleStringProperty('tooltipPosition'),
  maskId: null,

  _data: computed('data', function() {
    const data = copy(this.data, true);
    const sum = data.mapBy('value').reduce(sumAggregate, 0);

    return data.map(({ label, value, className, layers }, index) => ({
      label,
      value,
      className,
      layers,
      index,
      percent: value / sum,
      offset:
        data
          .slice(0, index)
          .mapBy('value')
          .reduce(sumAggregate, 0) / sum,
    }));
  }),

  didInsertElement() {
    const chart = d3.select(this.$('svg')[0]);
    const maskId = `dist-mask-${guidFor(this)}`;
    this.setProperties({ chart, maskId });

    this.$('svg clipPath').attr('id', maskId);

    chart.on('mouseleave', () => {
      run(() => {
        this.set('isActive', false);
        this.set('activeDatum', null);
        chart
          .selectAll('g')
          .classed('active', false)
          .classed('inactive', false);
      });
    });

    this.renderChart();
  },

  didUpdateAttrs() {
    this.renderChart();
  },

  updateChart: observer('_data.@each.{value,label,className}', function() {
    this.renderChart();
  }),

  // prettier-ignore
  /* eslint-disable */
  renderChart() {
    const { chart, _data, isNarrow } = this;
    const width = this.$('svg').width();
    const filteredData = _data.filter(d => d.value > 0);
    filteredData.forEach((d, index) => {
      set(d, 'index', index);
    });

    let slices = chart.select('.bars').selectAll('g').data(filteredData, d => d.label);
    let sliceCount = filteredData.length;

    slices.exit().remove();

    let slicesEnter = slices.enter()
      .append('g')
      .on('mouseenter', d => {
        run(() => {
          const slices = this.slices;
          const slice = slices.filter(datum => datum.label === d.label);
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
    slices.attr('class', d => {
      const className = d.className || `slice-${_data.indexOf(d)}`
      const activeDatum = this.activeDatum;
      const isActive = activeDatum && activeDatum.label === d.label;
      const isInactive = activeDatum && activeDatum.label !== d.label;
      return [ className, isActive && 'active', isInactive && 'inactive' ].compact().join(' ');
    });

    this.set('slices', slices);

    const setWidth = d => {
      // Remove a pixel from either side of the slice
      let modifier = 2;
      if (d.index === 0) modifier--; // But not the left side
      if (d.index === sliceCount - 1) modifier--; // But not the right side

      return `${width * d.percent - modifier}px`;
    };
    const setOffset = d => `${width * d.offset + (d.index === 0 ? 0 : 1)}px`;

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
        .attr('y', () => isNarrow ? '50%' : 0)
        .attr('clip-path', `url(#${this.maskId})`)
        .attr('height', () => isNarrow ? '6px' : '100%')
        .attr('transform', () => isNarrow ? 'translate(0, -3)' : '')
      .merge(layers)
        .attr('class', (d, i) => `bar layer-${i}`)
      .transition()
        .duration(200)
        .attr('width', setWidth)
        .attr('x', setOffset)

      if (isNarrow) {
        d3.select(this.element).select('.mask')
          .attr('height', '6px')
          .attr('y', '50%');
      }
  },
  /* eslint-enable */

  windowResizeHandler() {
    run.once(this, this.renderChart);
  },
});
