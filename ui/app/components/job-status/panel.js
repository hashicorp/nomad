// @ts-check
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import config from 'nomad-ui/config/environment';
import { action } from '@ember/object';

export default class JobStatusPanelComponent extends Component {

  // Build note: allocTypes order matters! We will fill up to 100% of totalAllocs in this order.
  allocTypes = [
    "running",
    "failed",
    "unknown",
    "starting",
    "lost",
    "queued",
    "complete",
  ].map((type) => {
    return {
      label: type,
      property: `${type}Allocs`
    }
  })

  get allocBlocks() {
    let totalAllocs = this.totalAllocs;

    // Only fill up to 100% of totalAllocs. Once we've filled up, we can stop counting.
    return this.allocTypes.reduce((blocks, type) => {
      if (totalAllocs > 0) {
        blocks[type.label] = Math.min(totalAllocs, this.args.job[type.property]);
        totalAllocs -= blocks[type.label];
      } else {
        blocks[type.label] = 0;
      }
      // blocks[type.label] = this.args.job[type.property];
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
    // v----- Experimental method: Count all allocs. Good for testing but not a realistic representation of "Desired"
    // return this.allocTypes.reduce((sum, type) => sum + this.args.job[type.property], 0);

    // v----- Realistic method: Tally a job's task groups' "count" property
    return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);

  }

  @action
  modifyMockAllocs(propName, { target: { value } }) {
    this.args.job[propName] = +value;
  }
}
