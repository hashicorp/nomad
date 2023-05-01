// @ts-check
import Component from '@glimmer/component';
import { alias } from '@ember/object/computed';

export default class JobStatusPanelSteadyComponent extends Component {
  @alias('args.job') job;

  // Build note: allocTypes order matters! We will fill up to 100% of totalAllocs in this order.
  allocTypes = [
    'running',
    'pending',
    'failed',
    // 'unknown',
    'lost',
    // 'queued',
    // 'complete',
    'unplaced',
  ].map((type) => {
    return {
      label: type,
    };
  });

  get allocBlocks() {
    let availableSlotsToFill = this.totalAllocs;
    // Only fill up to 100% of totalAllocs. Once we've filled up, we can stop counting.
    let allocationsOfShowableType = this.allocTypes.reduce((blocks, type) => {
      const jobAllocsOfType = this.args.job.allocations
        .sortBy('jobVersion') // Try counting from latest deployment's allocs and work backwards if needed
        .reverse()
        .filterBy('clientStatus', type.label);
      if (availableSlotsToFill > 0) {
        blocks[type.label] = {
          healthy: {
            nonCanary: Array(
              Math.min(availableSlotsToFill, jobAllocsOfType.length)
            )
              .fill()
              .map((_, i) => {
                return jobAllocsOfType[i];
              }),
          },
        };
        availableSlotsToFill -= blocks[type.label].healthy.nonCanary.length;
      } else {
        blocks[type.label] = { healthy: { nonCanary: [] } };
      }
      return blocks;
    }, {});
    if (availableSlotsToFill > 0) {
      allocationsOfShowableType['unplaced'] = {
        healthy: {
          nonCanary: Array(availableSlotsToFill)
            .fill()
            .map(() => {
              return { clientStatus: 'unplaced' };
            }),
        },
      };
    }
    return allocationsOfShowableType;
  }

  // TODO: eventually we will want this from a new property on a job.
  get totalAllocs() {
    // v----- Experimental method: Count all allocs. Good for testing but not a realistic representation of "Desired"
    // return this.allocTypes.reduce((sum, type) => sum + this.args.job[type.property], 0);

    // v----- Realistic method: Tally a job's task groups' "count" property
    return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
  }

  get versions() {
    return Object.values(this.allocBlocks)
      .flatMap((allocType) => Object.values(allocType))
      .flatMap((allocHealth) => Object.values(allocHealth))
      .flatMap((allocCanary) => Object.values(allocCanary))
      .map((a) => (!isNaN(a?.jobVersion) ? a.jobVersion : 'pending')) // "starting" allocs, and possibly others, do not yet have a jobVersion
      .reduce(
        (result, item) => ({
          ...result,
          [item]: [...(result[item] || []), item],
        }),
        []
      );
  }

  get rescheduledAllocs() {
    return this.job.allocations.filter(
      (a) =>
        a.jobVersion === this.job.latestDeployment.get('versionNumber') &&
        a.hasBeenRescheduled
    );
  }

  get restartedAllocs() {
    return this.job.allocations.filter(
      (a) =>
        a.jobVersion === this.job.latestDeployment.get('versionNumber') &&
        a.hasBeenRestarted
    );
  }
}
