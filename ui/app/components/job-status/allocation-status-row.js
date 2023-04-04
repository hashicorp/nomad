// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobStatusAllocationStatusRowComponent extends Component {
  @tracked width = 0;

  get allocBlockSlots() {
    return Object.values(this.args.allocBlocks)
      .flatMap((statusObj) => Object.values(statusObj))
      .flatMap((healthObj) => Object.values(healthObj))
      .reduce(
        (totalSlots, allocsByCanary) =>
          totalSlots + (allocsByCanary ? allocsByCanary.length : 0),
        0
      );
  }

  get showSummaries() {
    return this.allocBlockSlots * 42 - 10 > this.width;
  }

  calcPerc(count) {
    return (count / this.allocBlockSlots) * this.width;
  }

  @action reflow(element) {
    this.width = element.clientWidth;
  }

  @action
  captureElement(element) {
    this.width = element.clientWidth;
  }
}
