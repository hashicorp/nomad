// @ts-check
import Component from '@glimmer/component';
import { task } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';
import { alias } from '@ember/object/computed';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class JobStatusPanelDeployingComponent extends Component {
  @alias('args.job') job;
  @alias('args.handleError') handleError = () => {};

  // Build note: allocTypes order matters! We will fill up to 100% of totalAllocs in this order.
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
      property: `${type}Allocs`,
    };
  });

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

  get oldVersionAllocBlocks() {
    return this.job.allocations
      .filter((allocation) => this.oldVersionAllocBlockIDs.includes(allocation))
      .reduce((alloGroups, currentAlloc) => {
        (alloGroups[currentAlloc.clientStatus] =
          alloGroups[currentAlloc.clientStatus] || []).push(currentAlloc);
        return alloGroups;
      }, {});
  }

  get newVersionAllocBlocks() {
    let availableSlotsToFill = this.desiredTotal;
    let allocationsOfDeploymentVersion = this.job.allocations.filter(
      (a) => a.jobVersion === this.deployment.get('versionNumber')
    );
    // Only fill up to 100% of desiredTotal. Once we've filled up, we can stop counting.
    let allocationsOfShowableType = this.allocTypes.reduce((blocks, type) => {
      const jobAllocsOfType = allocationsOfDeploymentVersion.filterBy(
        'clientStatus',
        type.label
      );
      if (availableSlotsToFill > 0) {
        blocks[type.label] = Array(
          Math.min(availableSlotsToFill, jobAllocsOfType.length)
        )
          .fill()
          .map((_, i) => {
            return jobAllocsOfType[i];
          });
        availableSlotsToFill -= blocks[type.label].length;
      } else {
        blocks[type.label] = [];
      }
      return blocks;
    }, {});
    if (availableSlotsToFill > 0) {
      allocationsOfShowableType['unplaced'] = Array(availableSlotsToFill)
        .fill()
        .map(() => {
          return { clientStatus: 'unplaced' };
        });
    }
    return allocationsOfShowableType;
  }
}
