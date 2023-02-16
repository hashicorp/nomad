// @ts-check
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import config from 'nomad-ui/config/environment';
import { action } from '@ember/object';

export default class JobStatusPanelComponent extends Component {

  allocTypes = [
    "runningAllocs",
    "failedAllocs",
    "unknownAllocs",
    "queuedAllocs",
    "completeAllocs",
    "startingAllocs",
    "lostAllocs"
  ]
  /**
   * @type {('current'|'historical')}
   */
  @tracked mode = 'current'; // can be either "current" or "historical"

  // Convenience UI for manipulating number of allocations. Temporary and mirage only.
  get showDataFaker() {
    return config['ember-cli-mirage'];
  }

  // TODO: eventually we will want this from a new property on a job.
  get totalAllocs() {
    return this.allocTypes.reduce((sum, type) => sum + this.args.job[type], 0);
  }

  @action
  modifyMockAllocs(type, { target: { value } }) {
    this.args.job[type] = +value;
  }
}
