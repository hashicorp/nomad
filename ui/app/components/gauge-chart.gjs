/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { assert } from '@ember/debug';
import { guidFor } from '@ember/object/internals';
import { once } from '@ember/runloop';
import { concat } from '@ember/helper';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import d3Shape from 'd3-shape';
import formatPercentage from 'nomad-ui/helpers/format-percentage';
import windowResize from 'nomad-ui/modifiers/window-resize';

export default class GaugeChart extends Component {
  @tracked width = 0;
  @tracked height = 0;

  svgElement = null;
  weight = 4;

  get value() {
    return this.args.value;
  }

  get complement() {
    return this.args.complement;
  }

  get total() {
    return this.args.total;
  }

  get label() {
    return this.args.label;
  }

  get chartClass() {
    return this.args.chartClass || 'is-info';
  }

  get percent() {
    assert(
      'Provide complement OR total to GaugeChart, not both.',
      this.complement != null || this.total != null,
    );

    if (this.complement != null) {
      return this.value / (this.value + this.complement);
    }

    return this.value / this.total;
  }

  get fillId() {
    return `gauge-chart-fill-${guidFor(this)}`;
  }

  get maskId() {
    return `gauge-chart-mask-${guidFor(this)}`;
  }

  get radius() {
    return this.width / 2;
  }

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

  setSvgElement = (element) => {
    this.svgElement = element;
    this.updateDimensions();
  };

  updateDimensions = () => {
    const width = this.svgElement?.clientWidth || 0;
    this.width = width;
    this.height = width / 2;
  };

  handleResize = () => {
    once(this, this.updateDimensions);
  };

  <template>
    <div
      class="chart gauge-chart"
      {{windowResize this.handleResize}}
      ...attributes
    >
      <svg
        data-test-gauge-svg
        role="img"
        height={{this.height}}
        title="gauge chart"
        {{didInsert this.setSvgElement}}
      >
        <defs>
          <linearGradient
            x1="0"
            x2="1"
            y1="0"
            y2="0"
            class={{this.chartClass}}
            id={{this.fillId}}
          >
            <stop class="start" offset="0%" />
            <stop class="end" offset="100%" />
          </linearGradient>
          <clipPath id={{this.maskId}}>
            <path class="fill" d={{this.valueArc}} />
          </clipPath>
        </defs>
        <g class="canvas {{this.chartClass}}">
          <path class="background" d={{this.backgroundArc}} />
          <rect
            class="area"
            x="0"
            y="0"
            width="100%"
            height="100%"
            fill={{concat "url(#" this.fillId ")"}}
            clip-path={{concat "url(#" this.maskId ")"}}
          />
        </g>
      </svg>
      <div class="metric">
        <h3 data-test-label class="label">{{this.label}}</h3>
        <p data-test-percentage class="value">{{formatPercentage
            this.value
            total=this.total
            complement=this.complement
          }}</p>
      </div>
    </div>
  </template>
}
