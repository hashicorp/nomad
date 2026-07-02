/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { assert } from '@ember/debug';
import { concat } from '@ember/helper';
import { default as d3Shape, area, line } from 'd3-shape';
import { guidFor } from '@ember/object/internals';

export default class ChartPrimitiveArea extends Component {
  get colorClass() {
    if (this.args.colorClass) return this.args.colorClass;
    if (this.args.colorScale && this.args.index != null)
      return `${this.args.colorScale} ${this.args.colorScale}-${
        this.args.index + 1
      }`;
    return 'is-primary';
  }

  get maskId() {
    return `area-mask-${guidFor(this)}`;
  }

  get fillId() {
    return `area-fill-${guidFor(this)}`;
  }

  get curveMethod() {
    const mappings = {
      linear: 'curveLinear',
      stepAfter: 'curveStepAfter',
    };
    assert(
      `Provided curve "${this.args.curve}" is not an allowed curve type`,
      mappings[this.args.curve],
    );
    return mappings[this.args.curve];
  }

  get line() {
    const { xScale, yScale, xProp, yProp } = this.args;

    const builder = line()
      .curve(d3Shape[this.curveMethod])
      .defined((d) => d[yProp] != null)
      .x((d) => xScale(d[xProp]))
      .y((d) => yScale(d[yProp]));

    return builder(this.args.data);
  }

  get area() {
    const { xScale, yScale, xProp, yProp } = this.args;

    const builder = area()
      .curve(d3Shape[this.curveMethod])
      .defined((d) => d[yProp] != null)
      .x((d) => xScale(d[xProp]))
      .y0(yScale(0))
      .y1((d) => yScale(d[yProp]));

    return builder(this.args.data);
  }

  <template>
    <defs>
      <linearGradient
        x1="0"
        x2="0"
        y1="0"
        y2="1"
        class={{this.colorClass}}
        id={{this.fillId}}
      >
        <stop class="start" offset="0%" />
        <stop class="end" offset="100%" />
      </linearGradient>
      <clipPath id={{this.maskId}}>
        <path class="fill" d={{this.area}} />
      </clipPath>
    </defs>
    <g
      data-test-chart-area
      class={{concat "area " this.colorClass}}
      ...attributes
    >
      <path class="line" d={{this.line}} />
      <rect
        class="fill"
        x="0"
        y="0"
        width={{@width}}
        height={{@height}}
        fill={{concat "url(#" this.fillId ")"}}
        clip-path={{concat "url(#" this.maskId ")"}}
      />
    </g>
  </template>
}
