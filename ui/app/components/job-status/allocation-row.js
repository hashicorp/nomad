import Component from '@glimmer/component';
import { action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';

export default class JobStatusAllocationRowComponent extends Component {

  @tracked element = null;

  @alias('element.clientWidth') width;

  get showSummaries() {
    return this.args.totalAllocs > 60; // TODO: arbitrary
  }

  calcPerc(count) {
    return count / this.args.totalAllocs * this.width;
  }

  @action
  captureElement(element) {
    this.element = element;
    console.log('elly', element.clientWidth);
  }

}
