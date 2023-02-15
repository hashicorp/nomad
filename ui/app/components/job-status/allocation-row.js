import Component from '@glimmer/component';

export default class JobStatusAllocationRowComponent extends Component {
  get showSummaries() {
    return this.args.totalAllocs > 60; // TODO: arbitrary
  }

  calcPerc(count) {
    return count / this.args.totalAllocs * 100;
  }
}
