/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import { guidFor } from '@ember/object/internals';
import { once } from '@ember/runloop';
import d3Shape from 'd3-shape';
import WindowResizable from 'nomad-ui/mixins/window-resizable';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('chart', 'gauge-chart')
export default class GaugeChart extends Component.extend(WindowResizable) {
  value = null;
  complement = null;
  total = null;
  chartClass = 'is-info';

  width = 0;
  height = 0;

  @computed('value', 'complement', 'total')
  get percent() {
    assert(
      'Provide complement OR total to GaugeChart, not both.',
      this.complement != null || this.total != null
    );

    if (this.complement != null) {
      return this.value / (this.value + this.complement);
    }

    return this.value / this.total;
  }

  @computed
  get fillId() {
    return `gauge-chart-fill-${guidFor(this)}`;
  }

  @computed
  get maskId() {
    return `gauge-chart-mask-${guidFor(this)}`;
  }

  @computed('width')
  get radius() {
    return this.width / 2;
  }

  weight = 4;

  @computed('radius', 'weight')
  get backgroundArc() {
    const { radius, weight } = this;
    const arc = d3Shape
      .arc()
      .outerRadius(radius)
      .innerRadius(radius - weight)
      .cornerRadius(weight)
      .startAngle(-Math.PI / 2)
      .endAngle(Math.PI / 2);
    return arc();
  }

  @computed('radius', 'weight', 'percent')
  get valueArc() {
    const { radius, weight, percent } = this;

    const arc = d3Shape
      .arc()
      .outerRadius(radius)
      .innerRadius(radius - weight)
      .cornerRadius(weight)
      .startAngle(-Math.PI / 2)
      .endAngle(-Math.PI / 2 + Math.PI * percent);
    return arc();
  }

  didInsertElement() {
    super.didInsertElement(...arguments);
    this.updateDimensions();
  }

  updateDimensions() {
    const width = this.element.querySelector('svg').clientWidth;
    this.setProperties({ width, height: width / 2 });
  }

  windowResizeHandler() {
    once(this, this.updateDimensions);
  }
}
