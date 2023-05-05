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

  /**
   * @typedef {Object} HealthStatus
   * @property {Array} nonCanary
   * @property {Array} canary
   */

  /**
   * @typedef {Object} AllocationStatus
   * @property {HealthStatus} healthy
   * @property {HealthStatus} unhealthy
   */

  /**
   * @typedef {Object} AllocationBlock
   * @property {AllocationStatus} [RUNNING]
   * @property {AllocationStatus} [PENDING]
   * @property {AllocationStatus} [FAILED]
   * @property {AllocationStatus} [LOST]
   * @property {AllocationStatus} [UNPLACED]
   */

  /**
   * Looks through running/pending allocations with the aim of filling up your desired number of allocations.
   * If any desired remain, it will walk backwards through job versions and other allocation types to build
   * a picture of the job's overall status.
   *
   * @returns {AllocationBlock} An object containing healthy non-canary allocations
   *                            for each clientStatus.
   */
  get allocBlocks() {
    let availableSlotsToFill = this.totalAllocs;

    // Initialize allocationsOfShowableType with empty arrays for each clientStatus
    /**
     * @type {AllocationBlock}
     */
    let allocationsOfShowableType = this.allocTypes.reduce(
      (accumulator, type) => {
        accumulator[type.label] = { healthy: { nonCanary: [] } };
        return accumulator;
      },
      {}
    );

    // First accumulate the Running/Pending allocations
    for (const alloc of this.job.allocations.filter(
      (a) => a.clientStatus === 'running' || a.clientStatus === 'pending'
    )) {
      if (availableSlotsToFill === 0) {
        break;
      }

      const status = alloc.clientStatus;
      allocationsOfShowableType[status].healthy.nonCanary.push(alloc);
      availableSlotsToFill--;
    }

    // Sort all allocs by jobVersion in descending order
    const sortedAllocs = this.args.job.allocations
      .filter(
        (a) => a.clientStatus !== 'running' && a.clientStatus !== 'pending'
      )
      .sortBy('jobVersion')
      .reverse();

    // Iterate over the sorted allocs
    for (const alloc of sortedAllocs) {
      if (availableSlotsToFill === 0) {
        break;
      }

      const status = alloc.clientStatus;
      // If the alloc has another clientStatus, add it to the corresponding list
      // as long as we haven't reached the totalAllocs limit for that clientStatus
      if (
        this.allocTypes.map(({ label }) => label).includes(status) &&
        allocationsOfShowableType[status].healthy.nonCanary.length <
          this.totalAllocs
      ) {
        allocationsOfShowableType[status].healthy.nonCanary.push(alloc);
        availableSlotsToFill--;
      }
    }

    // Handle unplaced allocs
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

  get nodes() {
    return this.args.nodes;
  }

  get totalAllocs() {
    if (this.args.job.type === 'service') {
      return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
    } else if (this.atMostOneAllocPerNode) {
      return this.args.job.allocations.uniqBy('nodeID').length;
    } else {
      return this.args.job.count; // TODO: this is probably not the correct totalAllocs count for any type.
    }
  }

  get atMostOneAllocPerNode() {
    return this.args.job.type === 'system';
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
    return this.job.allocations.filter((a) => !a.isOld && a.hasBeenRescheduled);
  }

  get restartedAllocs() {
    return this.job.allocations.filter((a) => !a.isOld && a.hasBeenRestarted);
  }

  get supportsRescheduling() {
    return this.job.type !== 'system';
  }
}
