/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchAll, watchQuery } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import { task, restartableTask, timeout } from 'ember-concurrency';

export default class IndexRoute extends Route.extend(
  // WithWatchers,
  WithForbiddenState
) {
  @service store;
  @service watchList;

  perPage = 10;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
    nextToken: {
      refreshModel: true,
    },
  };

  defaultParams = {
    meta: true,
    per_page: this.perPage,
  };

  getCurrentParams() {
    let queryParams = this.paramsFor(this.routeName); // Get current query params
    return { ...this.defaultParams, ...queryParams };
  }

  jobQuery(params, options = {}) {
    return this.store
      .query('job', params, {
        adapterOptions: {
          method: 'GET', // TODO: default
          queryType: options.queryType,
        },
      })
      .catch(notifyForbidden(this));
  }

  jobAllocsQuery(jobIDs) {
    return this.store
      .query(
        'job',
        {
          jobs: jobIDs,
        },
        {
          adapterOptions: {
            method: 'POST',
            queryType: 'update',
          },
        }
      )
      .catch(notifyForbidden(this));
  }

  @restartableTask *watchJobIDs(params, throttle = 2000) {
    while (true) {
      let currentParams = this.getCurrentParams();
      const newJobs = yield this.jobQuery(currentParams, {
        queryType: 'update_ids',
      });

      const jobIDs = newJobs.map((job) => ({
        id: job.plainId,
        namespace: job.belongsTo('namespace').id(),
      }));
      this.controller.set('jobIDs', jobIDs);
      // BIG TODO: MAKE ANY jobIDs UPDATES TRIGGER A NEW WATCHJOBS TASK
      this.watchJobs.perform({}, 500);

      yield timeout(throttle); // Moved to the end of the loop
    }
  }

  // @restartableTask *watchJobIDs(params, throttle = 2000) {
  //   while (true) {
  //     // let currentParams = this.getCurrentParams();
  //     // const jobs = yield this.jobQuery(currentParams);
  //     // yield timeout(throttle);
  //     let currentParams = this.getCurrentParams();
  //     currentParams.queryType = 'update_ids';
  //     const newJobs = yield this.jobQuery(currentParams);

  //     const jobIDs = newJobs.map((job) => {
  //       return {
  //         id: job.plainId,
  //         namespace: job.belongsTo('namespace').id(),
  //       };
  //     });
  //     this.controller.set('jobIDs', jobIDs);

  //     yield timeout(throttle);

  //     // this.watchJobs.perform(params, 2000); // TODO mismatched throttle
  //   }
  // }

  @restartableTask *watchJobs(params, throttle = 2000) {
    // TODO: THURSDAY MORNING:
    // Most of the ordering stuff feels disjointed!
    // use the index from the watchList of the initial query here, too

    while (true) {
      let jobIDs = this.controller.jobIDs;
      console.log('watchJobs called', jobIDs);
      // console.log('jobids in watchjobs', jobIDs);
      // console.log('watchList list', this.watchList.list);

      // Either get index from watchlist entry for this particular hash, or
      // get index from watchlist entry for the initial query

      if (jobIDs && jobIDs.length > 0) {
        let jobDetails = yield this.jobAllocsQuery(jobIDs);
        this.controller.set('jobs', jobDetails);
      }
      // TODO: might need an else condition here for if there are no jobIDs,
      // which would indicate no jobs, but the updater above might not fire.
      yield timeout(throttle);
    }
  }

  async model(params) {
    // console.log('model firing');
    // console.log('sending off params', params);
    let currentParams = this.getCurrentParams();
    // currentParams.queryType = 'initialize';

    return RSVP.hash({
      jobs: await this.jobQuery(currentParams, { queryType: 'initialize' }),
      namespaces: this.store.findAll('namespace'),
      nodePools: this.store.findAll('node-pool'),
    });
  }

  setupController(controller, model) {
    super.setupController(controller, model);
    controller.set('jobs', model.jobs);
    controller.set(
      'jobIDs',
      model.jobs.map((job) => {
        return {
          id: job.plainId,
          namespace: job.belongsTo('namespace').id(),
        };
      })
    );

    this.watchJobIDs.perform({}, 2000);
    this.watchJobs.perform({}, 500); // Start watchJobs independently with its own throttle
  }

  // afterModel(model, transition) {
  //   console.log('afterModel firing', model, transition);
  //   // let jobs = this.watchJobsTask.perform(params);
  //   let params = this.getCurrentParams();
  //   let jobs = this.watchJobsTask.perform(params, 200);

  //   console.log('jobs', jobs);
  //   // model.jobs = jobs;
  // }

  // startWatchers(controller, model) {
  //   controller.set('namespacesWatch', this.watchNamespaces.perform());
  //   controller.set(
  //     'modelWatch',
  //     this.watchJobs.perform(
  //       {
  //         namespace: controller.qpNamespace,
  //         per_page: this.perPage,
  //         meta: true,
  //         queryType: 'initialize',
  //       },
  //       1000,
  //       { model }
  //     ) // TODO: VERY HACKY WAY TO PASS MODEL
  //   );
  //   controller.set(
  //     'jobsWatch',
  //     this.watchJobsAllocs.perform({
  //       namespace: controller.qpNamespace,
  //       meta: true,
  //       queryType: 'update',
  //       jobs: model.jobs.map((job) => {
  //         // TODO: maybe this should be set on controller for user-controlled updates?
  //         return {
  //           id: job.plainId,
  //           namespace: job.belongsTo('namespace').id(),
  //         };
  //       }),
  //     })
  //   );
  // }

  // @watchQuery('job') watchJobs;
  // @watchQuery('job', { queryType: 'update' }) watchJobsAllocs;
  // // @watchQuery('job', { queryType: 'update' }) watchJobsUpdate;
  // @watchAll('namespace') watchNamespaces;
  // @collect('watchJobs', 'watchJobsAllocs', 'watchNamespaces') watchers;
}
