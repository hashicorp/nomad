/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { assert } from '@ember/debug';
import { default as d3Shape, area, line } from 'd3-shape';
import uniquely from 'nomad-ui/utils/properties/uniquely';

export default class ChartPrimitiveArea extends Component {
  get colorClass() {
    if (this.args.colorClass) return this.args.colorClass;
    if (this.args.colorScale && this.args.index != null)
      return `${this.args.colorScale} ${this.args.colorScale}-${
        this.args.index + 1
      }`;
    return 'is-primary';
  }

  @uniquely('area-mask') maskId;
  @uniquely('area-fill') fillId;

  get curveMethod() {
    const mappings = {
      linear: 'curveLinear',
      stepAfter: 'curveStepAfter',
    };
    assert(
      `Provided curve "${this.curve}" is not an allowed curve type`,
      mappings[this.args.curve]
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
}
