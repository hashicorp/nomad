// @ts-check
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class JobStatusPanelComponent extends Component {
  /**
   * @type {('current'|'historical')}
   */
  @tracked mode = 'current'; // can be either "current" or "historical"

  // TODO: TEMP
  get totalAllocs() {
    return +(+this.args.job.runningAllocs + +this.args.job.failedAllocs + +this.args.job.unknownAllocs);
  }
}
