import Component from '@glimmer/component';
import { action, computed } from '@ember/object';

export default class EventNodeComponent extends Component {
  get r() {
    return this.args.xScale.bandwidth(this.args.data.Index);
  }

  get x() {
    return this.args.xScale(this.args.data.Index);
  }

  @computed('args.data.yMod')
  get y() {
    // console.log('yupdate', this.args.data.yMod);
    // return this.args.vy;
    // return this.args.yScale(this.args.data.value);
    // console.log('why', this.args.height, this.r, this.args.offset);
    return this.args.height / 2 - this.r / 2 + (this.args.data.yMod || 0);
  }

  @action onMouseEnter() {
    this.args.highlightEvent(this.args.data);
  }
  @action onMouseLeave() {
    this.args.blurEvent(this.args.data);
  }
}
