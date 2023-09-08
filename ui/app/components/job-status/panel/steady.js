/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { alias } from '@ember/object/computed';
import { jobAllocStatuses } from '../../../utils/allocation-client-statuses';

export default class JobStatusPanelSteadyComponent extends Component {
  @alias('args.job') job;

  get allocTypes() {
    return jobAllocStatuses[this.args.job.type].map((type) => {
      return {
        label: type,
      };
    });
  }

  /**
   * @typedef {Object} HealthStatus
   * @property {Array} nonCanary
   * @property {Array} canary
   */

  /**
   * @typedef {Object} AllocationStatus
   * @property {HealthStatus} healthy
   * @property {HealthStatus} unhealthy
   * @property {HealthStatus} health unknown
   */

  /**
   * @typedef {Object} AllocationBlock
   * @property {AllocationStatus} [running]
   * @property {AllocationStatus} [pending]
   * @property {AllocationStatus} [failed]
   * @property {AllocationStatus} [lost]
   * @property {AllocationStatus} [unplaced]
   * @property {AllocationStatus} [complete]
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
      .sort((a, b) => {
        // First sort by jobVersion
        if (a.jobVersion > b.jobVersion) return 1;
        if (a.jobVersion < b.jobVersion) return -1;

        // If jobVersion is the same, sort by status order
        if (a.jobVersion === b.jobVersion) {
          return (
            jobAllocStatuses[this.args.job.type].indexOf(b.clientStatus) -
            jobAllocStatuses[this.args.job.type].indexOf(a.clientStatus)
          );
        } else {
          return 0;
        }
      })
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
    if (this.args.job.type === 'service' || this.args.job.type === 'batch') {
      return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
    } else if (this.atMostOneAllocPerNode) {
      return this.args.job.allocations.uniqBy('nodeID').length;
    } else {
      return this.args.job.count; // TODO: this is probably not the correct totalAllocs count for any type.
    }
  }

  get totalNonCompletedAllocs() {
    return this.totalAllocs - this.completedAllocs.length;
  }

  get allAllocsComplete() {
    return this.completedAllocs.length && this.totalNonCompletedAllocs === 0;
  }

  get atMostOneAllocPerNode() {
    return this.args.job.type === 'system' || this.args.job.type === 'sysbatch';
  }

  get versions() {
    const versions = Object.values(this.allocBlocks)
      .flatMap((allocType) => Object.values(allocType))
      .flatMap((allocHealth) => Object.values(allocHealth))
      .flatMap((allocCanary) => Object.values(allocCanary))
      .map((a) => (!isNaN(a?.jobVersion) ? a.jobVersion : 'unknown')) // "starting" allocs, GC'd allocs, etc. do not have a jobVersion
      .sort((a, b) => a - b)
      .reduce((result, item) => {
        const existingVersion = result.find((v) => v.version === item);
        if (existingVersion) {
          existingVersion.allocations.push(item);
        } else {
          result.push({ version: item, allocations: [item] });
        }
        return result;
      }, []);
    return versions;
  }

  get rescheduledAllocs() {
    return this.job.allocations.filter((a) => !a.isOld && a.hasBeenRescheduled);
  }

  get restartedAllocs() {
    return this.job.allocations.filter((a) => !a.isOld && a.hasBeenRestarted);
  }

  get completedAllocs() {
    return this.job.allocations.filter(
      (a) => !a.isOld && a.clientStatus === 'complete'
    );
  }

  get supportsRescheduling() {
    return this.job.type !== 'system';
  }

  get latestVersionAllocations() {
    return this.job.allocations.filter((a) => !a.isOld);
  }

  /**
   * @typedef {Object} CurrentStatus
   * @property {"Healthy"|"Failed"|"Degraded"|"Recovering"|"Complete"|"Running"} label - The current status of the job
   * @property {"highlight"|"success"|"warning"|"critical"} state -
   */

  /**
   * A general assessment for how a job is going, in a non-deployment state
   * @returns {CurrentStatus}
   */
  get currentStatus() {
    // If all allocs are running, the job is Healthy
    const totalAllocs = this.totalAllocs;

    if (this.job.type === 'batch' || this.job.type === 'sysbatch') {
      // If all the allocs are complete, the job is Complete
      const completeAllocs = this.allocBlocks.complete?.healthy?.nonCanary;
      if (completeAllocs?.length === totalAllocs) {
        return { label: 'Complete', state: 'success' };
      }

      // If any allocations are running the job is "Running"
      const healthyAllocs = this.allocBlocks.running?.healthy?.nonCanary;
      if (healthyAllocs?.length + completeAllocs?.length === totalAllocs) {
        return { label: 'Running', state: 'success' };
      }
    }

    const healthyAllocs = this.allocBlocks.running?.healthy?.nonCanary;
    if (healthyAllocs?.length === totalAllocs) {
      return { label: 'Healthy', state: 'success' };
    }

    // If any allocations are pending the job is "Recovering"
    const pendingAllocs = this.allocBlocks.pending?.healthy?.nonCanary;
    if (pendingAllocs?.length > 0) {
      return { label: 'Recovering', state: 'highlight' };
    }

    // If any allocations are failed, lost, or unplaced in a steady state, the job is "Degraded"
    const failedOrLostAllocs = [
      ...this.allocBlocks.failed?.healthy?.nonCanary,
      ...this.allocBlocks.lost?.healthy?.nonCanary,
      ...this.allocBlocks.unplaced?.healthy?.nonCanary,
    ];

    if (failedOrLostAllocs.length === totalAllocs) {
      return { label: 'Failed', state: 'critical' };
    } else {
      return { label: 'Degraded', state: 'warning' };
    }
  }
}
