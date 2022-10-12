import Component from '@glimmer/component';
import { action, computed } from '@ember/object';

export default class EventNodeComponent extends Component {
  get width() {
    return this.args.xScale.bandwidth(this.args.data.Index);
  }

  get height() {
    return this.args.yScale(0) - this.args.yScale(this.args.data.value);
  }

  get x() {
    return this.args.xScale(this.args.data.Index);
  }

  get y() {
    // return this.args.yScale(this.args.data.value);
    return this.args.height / 2 - this.width / 2;
  }
}
