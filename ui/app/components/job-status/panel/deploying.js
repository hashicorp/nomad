// @ts-check
import Component from '@glimmer/component';
import { task } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';
import { alias } from '@ember/object/computed';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class JobStatusPanelDeployingComponent extends Component {
  @alias('args.job') job;
  @alias('args.handleError') handleError = () => {};

  allocTypes = [
    'running',
    'pending',
    'failed',
    // 'unknown',
    // 'lost',
    // 'queued',
    // 'complete',
    'unplaced',
  ].map((type) => {
    return {
      label: type,
      // property: `${type}Allocs`,
    };
  });

  // clientStatuses = [
  //   'running',
  //   'pending',
  //   'failed',
  //   'unknown',
  // ];

  // propertyStatuses = [
  //   'isCanary',
  //   'isHealthy',
  //   'isNotHealthy'
  // ];

  // // Make a matrix of clientStatuses and propertyStatuses
  // allocTypes = this.clientStatuses.reduce((acc, clientStatus) => {
  //   this.propertyStatuses.forEach(propertyStatus => {
  //     acc.push({
  //       label: `${clientStatus} ${propertyStatus}`,
  //       clientStatus,
  //       propertyStatus,
  //     });
  //   });
  //   return acc;
  // }, []);

  // allocTypes = [
  //   { clientStatus: 'running',  isCanary: true},
  //   { clientStatus: 'running',  isCanary: false},
  //   { clientStatus: 'pending',  isCanary: true},
  //   { clientStatus: 'pending',  isCanary: false},
  //   { clientStatus: 'failed',   isCanary: true},
  //   { clientStatus: 'failed',   isCanary: false},
  //   // 'unknown',
  //   // 'lost',
  //   // 'queued',
  //   // 'complete',
  //   'unplaced',
  // ].map((type) => {
  //   return {
  //     label: type,
  //     // property: `${type}Allocs`,
  //   };
  // });

  @tracked oldVersionAllocBlockIDs = [];

  // Called via did-insert; sets a static array of "outgoing"
  // allocations we can track throughout a deployment
  establishOldAllocBlockIDs() {
    this.oldVersionAllocBlockIDs = this.job.allocations.filter(
      (a) =>
        a.clientStatus === 'running' &&
        a.jobVersion !== this.deployment.get('versionNumber')
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
  @alias('deployment.desiredTotal') desiredTotal;

  // get oldVersionAllocBlocks() {
  //   return this.job.allocations
  //     .filter((allocation) => this.oldVersionAllocBlockIDs.includes(allocation))
  //     .reduce((alloGroups, currentAlloc) => {
  //       (alloGroups[currentAlloc.clientStatus] =
  //         alloGroups[currentAlloc.clientStatus] || []).push(currentAlloc);
  //       return alloGroups;
  //     }, {});
  // }
  // get oldVersionAllocBlocks() {
  //   return this.job.allocations
  //     .filter((allocation) => this.oldVersionAllocBlockIDs.includes(allocation))
  //     .reduce((alloGroups, currentAlloc) => {
  //       const key = `${currentAlloc.clientStatus} old`;
  //       (alloGroups[key] = alloGroups[key] || []).push(currentAlloc);
  //       return alloGroups;
  //     }, {});
  // }

  get oldVersionAllocBlocks() {
    return this.job.allocations
      .filter((allocation) => this.oldVersionAllocBlockIDs.includes(allocation))
      .reduce((alloGroups, currentAlloc) => {
        const status = currentAlloc.clientStatus;

        if (!alloGroups[status]) {
          alloGroups[status] = {
            healthy: { nonCanary: [] },
            unhealthy: { nonCanary: [] },
          };
        }
        alloGroups[status].healthy.nonCanary.push(currentAlloc);

        return alloGroups;
      }, {});
  }

  get newVersionAllocBlocks() {
    let availableSlotsToFill = this.desiredTotal;
    let allocationsOfDeploymentVersion = this.job.allocations.filter(
      (a) => a.jobVersion === this.deployment.get('versionNumber')
    );

    let allocationCategories = this.allocTypes.reduce((categories, type) => {
      categories[type.label] = {
        healthy: { canary: [], nonCanary: [] },
        unhealthy: { canary: [], nonCanary: [] },
      };
      return categories;
    }, {});

    for (let alloc of allocationsOfDeploymentVersion) {
      if (availableSlotsToFill <= 0) {
        break;
      }
      let status = alloc.clientStatus;
      let health = alloc.isHealthy ? 'healthy' : 'unhealthy';
      let canary = alloc.isCanary ? 'canary' : 'nonCanary';

      if (allocationCategories[status]) {
        allocationCategories[status][health][canary].push(alloc);
        availableSlotsToFill--;
      }
    }

    // Fill unplaced slots if availableSlotsToFill > 0
    if (availableSlotsToFill > 0) {
      allocationCategories['unplaced'] = {
        healthy: { canary: [], nonCanary: [] },
        unhealthy: { canary: [], nonCanary: [] },
      };
      allocationCategories['unplaced']['healthy']['nonCanary'] = Array(
        availableSlotsToFill
      )
        .fill()
        .map(() => {
          return { clientStatus: 'unplaced' };
        });
    }
    console.log('allocationCategories', allocationCategories);

    return allocationCategories;
  }

  get newRunningHealthyAllocBlocks() {
    return [
      ...this.newVersionAllocBlocks['running']['healthy']['canary'],
      ...this.newVersionAllocBlocks['running']['healthy']['nonCanary'],
    ];
  }
  // TODO: eventually we will want this from a new property on a job.
  // TODO: consolidate w/ the one in steady.js
  get totalAllocs() {
    // v----- Experimental method: Count all allocs. Good for testing but not a realistic representation of "Desired"
    // return this.allocTypes.reduce((sum, type) => sum + this.args.job[type.property], 0);

    // v----- Realistic method: Tally a job's task groups' "count" property
    return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
  }
}
