import Component from '@glimmer/component';
import { default as d3Shape, area, line } from 'd3-shape';
import uniquely from 'nomad-ui/utils/properties/uniquely';

export default class ChartPrimitiveArea extends Component {
  get colorClass() {
    return this.args.colorClass || `${this.args.colorScale}-${this.args.index}`;
  }

  @uniquely('area-mask') maskId;
  @uniquely('area-fill') fillId;

  get line() {
    const { xScale, yScale, xProp, yProp, curveMethod } = this.args;

    const builder = line()
      .curve(d3Shape[curveMethod])
      .defined(d => d[yProp] != null)
      .x(d => xScale(d[xProp]))
      .y(d => yScale(d[yProp]));

    return builder(this.args.data);
  }

  get area() {
    const { xScale, yScale, xProp, yProp, curveMethod } = this.args;

    const builder = area()
      .curve(d3Shape[curveMethod])
      .defined(d => d[yProp] != null)
      .x(d => xScale(d[xProp]))
      .y0(yScale(0))
      .y1(d => yScale(d[yProp]));

    return builder(this.args.data);
  }
}
