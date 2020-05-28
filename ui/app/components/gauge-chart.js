import Component from '@ember/component';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import { guidFor } from '@ember/object/internals';
import { run } from '@ember/runloop';
import d3Shape from 'd3-shape';
import WindowResizable from 'nomad-ui/mixins/window-resizable';

export default Component.extend(WindowResizable, {
  classNames: ['chart', 'gauge-chart'],

  value: null,
  complement: null,
  total: null,
  chartClass: 'is-info',

  width: 0,
  height: 0,

  percent: computed('value', 'complement', 'total', function() {
    assert(
      'Provide complement OR total to GaugeChart, not both.',
      this.complement != null || this.total != null
    );

    if (this.complement != null) {
      return this.value / (this.value + this.complement);
    }

    return this.value / this.total;
  }),

  fillId: computed(function() {
    return `gauge-chart-fill-${guidFor(this)}`;
  }),

  maskId: computed(function() {
    return `gauge-chart-mask-${guidFor(this)}`;
  }),

  radius: computed('width', function() {
    return this.width / 2;
  }),

  weight: 4,

  backgroundArc: computed('radius', 'weight', function() {
    const { radius, weight } = this;
    const arc = d3Shape
      .arc()
      .outerRadius(radius)
      .innerRadius(radius - weight)
      .cornerRadius(weight)
      .startAngle(-Math.PI / 2)
      .endAngle(Math.PI / 2);
    return arc();
  }),

  valueArc: computed('radius', 'weight', 'percent', function() {
    const { radius, weight, percent } = this;

    const arc = d3Shape
      .arc()
      .outerRadius(radius)
      .innerRadius(radius - weight)
      .cornerRadius(weight)
      .startAngle(-Math.PI / 2)
      .endAngle(-Math.PI / 2 + Math.PI * percent);
    return arc();
  }),

  didInsertElement() {
    this.updateDimensions();
  },

  updateDimensions() {
    const width = this.element.querySelector('svg').clientWidth;
    this.setProperties({ width, height: width / 2 });
  },

  windowResizeHandler() {
    run.once(this, this.updateDimensions);
  },
});
