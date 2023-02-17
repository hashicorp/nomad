import Component from '@glimmer/component';
import { action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';

const UNGROUPED_ALLOCS_THRESHOLD = 50;

export default class JobStatusAllocationRowComponent extends Component {
  @tracked element = null;

  @alias('element.clientWidth') width;

  get allocBlockSlots() {
    return Object.values(this.args.allocBlocks).reduce((m, n) => m + n, 0);
  }

  get showSummaries() {
    return this.allocBlockSlots > UNGROUPED_ALLOCS_THRESHOLD;
  }

  calcPerc(count) {
    return (count / this.allocBlockSlots) * this.width;
  }

  @action
  captureElement(element) {
    this.element = element;
  }
}
