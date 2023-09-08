/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { task } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';
import { alias } from '@ember/object/computed';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { jobAllocStatuses } from '../../../utils/allocation-client-statuses';

export default class JobStatusPanelDeployingComponent extends Component {
  @alias('args.job') job;
  @alias('args.handleError') handleError = () => {};

  get allocTypes() {
    return jobAllocStatuses[this.args.job.type].map((type) => {
      return {
        label: type,
      };
    });
  }

  @tracked oldVersionAllocBlockIDs = [];

  // Called via did-insert; sets a static array of "outgoing"
  // allocations we can track throughout a deployment
  establishOldAllocBlockIDs() {
    this.oldVersionAllocBlockIDs = this.job.allocations.filter(
      (a) => a.clientStatus === 'running' && a.isOld
    );
  }

  /**
   * Promotion of a deployment will error if the canary allocations are not of status "Healthy";
   * this function will check for that and disable the promote button if necessary.
   * @returns {boolean}
   */
  get canariesHealthy() {
    const relevantAllocs = this.job.allocations.filter(
      (a) => !a.isOld && a.isCanary && !a.hasBeenRescheduled
    );
    return relevantAllocs.every(
      (a) => a.clientStatus === 'running' && a.isHealthy
    );
  }

  get someCanariesHaveFailed() {
    const relevantAllocs = this.job.allocations.filter(
      (a) => !a.isOld && a.isCanary && !a.hasBeenRescheduled
    );
    return relevantAllocs.some(
      (a) =>
        a.clientStatus === 'failed' ||
        a.clientStatus === 'lost' ||
        a.isUnhealthy
    );
  }

  @task(function* () {
    try {
      yield this.job.latestDeployment.content.promote();
    } catch (err) {
      this.handleError({
        title: 'Could Not Promote Deployment',
        description: messageFromAdapterError(err, 'promote deployments'),
      });
    }
  })
  promote;

  @task(function* () {
    try {
      yield this.job.latestDeployment.content.fail();
    } catch (err) {
      this.handleError({
        title: 'Could Not Fail Deployment',
        description: messageFromAdapterError(err, 'fail deployments'),
      });
    }
  })
  fail;

  @alias('job.latestDeployment') deployment;
  @alias('totalAllocs') desiredTotal;

  get oldVersionAllocBlocks() {
    return this.job.allocations
      .filter((allocation) => this.oldVersionAllocBlockIDs.includes(allocation))
      .reduce((alloGroups, currentAlloc) => {
        const status = currentAlloc.clientStatus;

        if (!alloGroups[status]) {
          alloGroups[status] = {
            healthy: { nonCanary: [] },
            unhealthy: { nonCanary: [] },
            health_unknown: { nonCanary: [] },
          };
        }
        alloGroups[status].healthy.nonCanary.push(currentAlloc);

        return alloGroups;
      }, {});
  }

  get newVersionAllocBlocks() {
    let availableSlotsToFill = this.desiredTotal;
    let allocationsOfDeploymentVersion = this.job.allocations.filter(
      (a) => !a.isOld
    );

    let allocationCategories = this.allocTypes.reduce((categories, type) => {
      categories[type.label] = {
        healthy: { canary: [], nonCanary: [] },
        unhealthy: { canary: [], nonCanary: [] },
        health_unknown: { canary: [], nonCanary: [] },
      };
      return categories;
    }, {});

    for (let alloc of allocationsOfDeploymentVersion) {
      if (availableSlotsToFill <= 0) {
        break;
      }
      let status = alloc.clientStatus;
      let canary = alloc.isCanary ? 'canary' : 'nonCanary';

      // Health status only matters in the context of a "running" allocation.
      // However, healthy/unhealthy is never purged when an allocation moves to a different clientStatus
      // Thus, we should only show something as "healthy" in the event that it is running.
      // Otherwise, we'd have arbitrary groupings based on previous health status.
      let health =
        status === 'running'
          ? alloc.isHealthy
            ? 'healthy'
            : alloc.isUnhealthy
            ? 'unhealthy'
            : 'health_unknown'
          : 'health_unknown';

      if (allocationCategories[status]) {
        // If status is failed or lost, we only want to show it IF it's used up its restarts/rescheds.
        // Otherwise, we'd be showing an alloc that had been replaced.
        if (alloc.willNotRestart) {
          if (!alloc.willNotReschedule) {
            // Dont count it
            continue;
          }
        }
        allocationCategories[status][health][canary].push(alloc);
        availableSlotsToFill--;
      }
    }

    // Fill unplaced slots if availableSlotsToFill > 0
    if (availableSlotsToFill > 0) {
      allocationCategories['unplaced'] = {
        healthy: { canary: [], nonCanary: [] },
        unhealthy: { canary: [], nonCanary: [] },
        health_unknown: { canary: [], nonCanary: [] },
      };
      allocationCategories['unplaced']['healthy']['nonCanary'] = Array(
        availableSlotsToFill
      )
        .fill()
        .map(() => {
          return { clientStatus: 'unplaced' };
        });
    }

    return allocationCategories;
  }

  get newRunningHealthyAllocBlocks() {
    return [
      ...this.newVersionAllocBlocks['running']['healthy']['canary'],
      ...this.newVersionAllocBlocks['running']['healthy']['nonCanary'],
    ];
  }

  get newRunningUnhealthyAllocBlocks() {
    return [
      ...this.newVersionAllocBlocks['running']['unhealthy']['canary'],
      ...this.newVersionAllocBlocks['running']['unhealthy']['nonCanary'],
    ];
  }

  get newRunningHealthUnknownAllocBlocks() {
    return [
      ...this.newVersionAllocBlocks['running']['health_unknown']['canary'],
      ...this.newVersionAllocBlocks['running']['health_unknown']['nonCanary'],
    ];
  }

  get rescheduledAllocs() {
    return this.job.allocations.filter((a) => !a.isOld && a.hasBeenRescheduled);
  }

  get restartedAllocs() {
    return this.job.allocations.filter((a) => !a.isOld && a.hasBeenRestarted);
  }

  // #region legend
  get newAllocsByStatus() {
    return Object.entries(this.newVersionAllocBlocks).reduce(
      (counts, [status, healthStatusObj]) => {
        counts[status] = Object.values(healthStatusObj)
          .flatMap((canaryStatusObj) => Object.values(canaryStatusObj))
          .flatMap((canaryStatusArray) => canaryStatusArray).length;
        return counts;
      },
      {}
    );
  }

  get newAllocsByCanary() {
    return Object.values(this.newVersionAllocBlocks)
      .flatMap((healthStatusObj) => Object.values(healthStatusObj))
      .flatMap((canaryStatusObj) => Object.entries(canaryStatusObj))
      .reduce((counts, [canaryStatus, items]) => {
        counts[canaryStatus] = (counts[canaryStatus] || 0) + items.length;
        return counts;
      }, {});
  }

  get newAllocsByHealth() {
    return {
      healthy: this.newRunningHealthyAllocBlocks.length,
      unhealthy: this.newRunningUnhealthyAllocBlocks.length,
      health_unknown: this.newRunningHealthUnknownAllocBlocks.length,
    };
  }
  // #endregion legend

  get oldRunningHealthyAllocBlocks() {
    return this.oldVersionAllocBlocks.running?.healthy?.nonCanary || [];
  }
  get oldCompleteHealthyAllocBlocks() {
    return this.oldVersionAllocBlocks.complete?.healthy?.nonCanary || [];
  }

  // TODO: eventually we will want this from a new property on a job.
  // TODO: consolidate w/ the one in steady.js
  get totalAllocs() {
    // v----- Experimental method: Count all allocs. Good for testing but not a realistic representation of "Desired"
    // return this.allocTypes.reduce((sum, type) => sum + this.args.job[type.property], 0);

    // v----- Realistic method: Tally a job's task groups' "count" property
    return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
  }

  get deploymentIsAutoPromoted() {
    return this.job.latestDeployment?.get('isAutoPromoted');
  }

  get oldVersions() {
    const oldVersions = Object.values(this.oldRunningHealthyAllocBlocks)
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

    return oldVersions;
  }

  get newVersions() {
    // Note: it's probably safe to assume all new allocs have the latest job version, but
    // let's map just in case there's ever a situation with multiple job versions going out
    // in a deployment for some reason
    const newVersions = Object.values(this.newVersionAllocBlocks)
      .flatMap((allocType) => Object.values(allocType))
      .flatMap((allocHealth) => Object.values(allocHealth))
      .flatMap((allocCanary) => Object.values(allocCanary))
      .filter((a) => a.jobVersion && a.jobVersion !== 'unknown')
      .map((a) => a.jobVersion)
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
    return newVersions;
  }

  get versions() {
    return [...this.oldVersions, ...this.newVersions];
  }
}
