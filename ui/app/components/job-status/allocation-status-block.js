import Component from '@glimmer/component';
import { action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';

export default class JobStatusAllocationStatusBlockComponent extends Component {
  // Only show as much as can reasonably fit in the panel, given @cxount and @percentage

  @tracked element = null;
  @alias('element.clientWidth') width;

  get countToShow() {
    // TODO: 60 is a magic number representing the rest element + 10px gap. Make less magic.
    console.log('consider percentage', this.args.percentage, this.args.count, this.args.percentage * this.args.count)
    // Show only as many as can fit within width, assuming each is 30px wide
    // console.log('CTS', this.args.status, Math.floor((this.width - 60) / 30));
    // Show all if there's room
    // console.log('about to compare for', this.args.status, this.args.count, this.width / 30);
    let cts = Math.floor((this.width-60) / 20);
    return cts > 0 ? cts : 0;
  }

  get remaining() {
    return this.args.count - this.countToShow;
  }

  @action
  captureElement(element) {
    this.element = element;
    console.log('elly', element.clientWidth);
  }

}
