// @ts-check
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import config from 'nomad-ui/config/environment';
import { action } from '@ember/object';

export default class JobStatusPanelComponent extends Component {

  allocTypes = [
    "running",
    "failed",
    "unknown",
    // "queued",
    "complete",
    // "starting",
    "lost"
  ].map((type) => {
    return {
      label: type,
      property: `${type}Allocs`
    }
  })

  get allocBlocks() {
    return this.allocTypes.reduce((blocks, type) => {
      blocks[type.label] = this.args.job[type.property];
      return blocks;
    }, {});
  }

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
    return this.allocTypes.reduce((sum, type) => sum + this.args.job[type.property], 0);
  }

  @action
  modifyMockAllocs(propName, { target: { value } }) {
    this.args.job[propName] = +value;
  }
}
