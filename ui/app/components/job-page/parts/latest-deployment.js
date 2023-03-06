import Component from '@glimmer/component';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { alias } from '@ember/object/computed';

// TODO: temp proof of concept
const groupBy = function (xs, key) {
  return xs.reduce(function (rv, x) {
    (rv[x[key]] = rv[x[key]] || []).push(x);
    return rv;
  }, {});
};

export default class LatestDeployment extends Component {
  // job = null;

  @alias('args.job') job;
  @alias('args.handleError') handleError = () => {};

  isShowingDeploymentDetails = false;

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

  // Build note: allocTypes order matters! We will fill up to 100% of totalAllocs in this order.
  // TODO: duplicate of what's in panel.js; move to a shared location
  allocTypes = [
    'running',
    'pending',
    'failed',
    'unknown',
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

  get totalAllocs() {
    // v----- Experimental method: Count all allocs. Good for testing but not a realistic representation of "Desired"
    // return this.allocTypes.reduce((sum, type) => sum + this.args.job[type.property], 0);

    // v----- Realistic method: Tally a job's task groups' "count" property
    return this.args.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
  }

  get oldVersionAllocBlocks() {
    // const totalOldAllocs = this.job.taskGroups.reduce((sum, tg) => sum + tg.count, 0);
    // let availableSlotsToFill = this.desiredTotal;
    // let allocationsOfDeploymentVersion = this.job.allocations.filter((a) => a.jobVersion === this.deployment.get('versionNumber'));
    // // Only fill up to 100% of desiredTotal. Once we've filled up, we can stop counting.
    // let allocationsOfShowableType = this.allocTypes.reduce((blocks, type) => {
    //   const jobAllocsOfType = allocationsOfDeploymentVersion.filterBy(
    //     'clientStatus',
    //     type.label
    //   );
    //   if (availableSlotsToFill > 0) {
    //     blocks[type.label] = Array(
    //       Math.min(availableSlotsToFill, jobAllocsOfType.length)
    //     )
    //       .fill()
    //       .map((_, i) => {
    //         return jobAllocsOfType[i];
    //       });
    //     availableSlotsToFill -= blocks[type.label].length;
    //   } else {
    //     blocks[type.label] = [];
    //   }
    //   return blocks;
    // }, {});
    // if (availableSlotsToFill > 0) {
    //   allocationsOfShowableType['unplaced'] = Array(availableSlotsToFill)
    //     .fill()
    //     .map(() => {
    //       return { clientStatus: 'unplaced' };
    //     });
    // }
    // return allocationsOfShowableType;
    return groupBy(
      this.job.allocations.filter(
        (a) =>
          a.clientStatus === 'running' &&
          a.jobVersion !== this.deployment.get('versionNumber')
      ),
      'clientStatus'
    );
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

  get jobAllocations() {}
}
