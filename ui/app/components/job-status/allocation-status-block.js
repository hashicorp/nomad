import Component from '@glimmer/component';

export default class JobStatusAllocationStatusBlockComponent extends Component {
  // Only show as much as can reasonably fit in the panel, given @cxount and @percentage
  get countToShow() {
    console.log('CTS', this.args.status, this.args.count/this.args.percentage);
    return 10;
  }

  get remaining() {
    return this.args.count - this.countToShow;
  }
}
