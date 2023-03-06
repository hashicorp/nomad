import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

const UNGROUPED_ALLOCS_THRESHOLD = 50;

export default class JobStatusAllocationStatusRowComponent extends Component {
  @tracked width = 0;

  get allocBlockSlots() {
    return Object.values(this.args.allocBlocks).reduce(
      (m, n) => m + n.length,
      0
    );
  }

  get showSummaries() {
    return this.allocBlockSlots > UNGROUPED_ALLOCS_THRESHOLD;
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
