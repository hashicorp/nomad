import Component from '@glimmer/component';
import { action, computed } from '@ember/object';

export default class EventNodeComponent extends Component {
  get r() {
    return this.args.xScale.bandwidth(this.args.data.Index);
  }

  get x() {
    return this.args.xScale(this.args.data.Index);
  }

  get y() {
    // return this.args.yScale(this.args.data.value);
    console.log('why', this.args.height, this.r, this.args.offset);
    return this.args.height / 2 - this.r / 2 + this.args.offset;
  }
}
