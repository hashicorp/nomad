/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import { task, restartableTask, timeout } from 'ember-concurrency';
import { action } from '@ember/object';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default class IndexRoute extends Route.extend(
  WithWatchers,
  WithForbiddenState
) {
  @service store;
  @service watchList;

  perPage = 3;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
    cursorAt: {
      refreshModel: true,
    },
  };

  defaultParams = {
    meta: true,
    per_page: this.perPage,
  };

  getCurrentParams() {
    let queryParams = this.paramsFor(this.routeName); // Get current query params
    queryParams.next_token = queryParams.cursorAt;
    delete queryParams.cursorAt; // TODO: hacky, should be done in the serializer/adapter?
    return { ...this.defaultParams, ...queryParams };
  }

  jobQuery(params, options = {}) {
    this.watchList.jobsIndexIDsController.abort();
    this.watchList.jobsIndexIDsController = new AbortController();

    return this.store
      .query('job', params, {
        adapterOptions: {
          method: 'GET', // TODO: default
          queryType: options.queryType,
          abortController: this.watchList.jobsIndexIDsController,
        },
      })
      .catch(notifyForbidden(this));
  }

  jobAllocsQuery(jobIDs) {
    this.watchList.jobsIndexDetailsController.abort();
    this.watchList.jobsIndexDetailsController = new AbortController();
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
            abortController: this.watchList.jobsIndexDetailsController,
          },
        }
      )
      .catch(notifyForbidden(this));
  }

  @localStorageProperty('nomadLiveUpdateJobsIndex', false) liveUpdatesEnabled;

  @restartableTask *watchJobIDs(params, throttle = 2000) {
    while (true) {
      let currentParams = this.getCurrentParams();
      const newJobs = yield this.jobQuery(currentParams, {
        queryType: 'update_ids',
      });
      if (newJobs.meta.nextToken) {
        this.controller.set('nextToken', newJobs.meta.nextToken);
      }

      const jobIDs = newJobs.map((job) => ({
        id: job.plainId,
        namespace: job.belongsTo('namespace').id(),
      }));

      const okayToJostle = this.controller.get('liveUpdatesEnabled');
      console.log('okay to jostle?', okayToJostle);
      if (okayToJostle) {
        // this.controller.set('jobs', newJobs);
        this.controller.set('jobIDs', jobIDs);
        this.watchList.jobsIndexDetailsController.abort();
        console.log(
          'new jobIDs have appeared, we should now watch them. We have cancelled the old hash req.',
          jobIDs
        );
        this.watchList.jobsIndexDetailsController = new AbortController();
        this.watchJobs.perform(jobIDs, 500);
      } else {
        this.controller.set('pendingJobIDs', jobIDs);
        this.controller.set('pendingJobs', newJobs);
      }

      // const stringifiedJobsEntries = JSON.stringify(jobDetails.map(j => j.id));
      // const stringifiedJobIDsEntries = JSON.stringify(jobIDs.map(j => JSON.stringify(Object.values(j))));
      // console.log('checking jobs list pending', this.jobs, this.jobIDs, stringifiedJobsEntries, stringifiedJobIDsEntries);
      // if (stringifiedJobsEntries !== stringifiedJobIDsEntries) {
      //   this.controller.set('jobListChangePending', true);
      //   this.controller.set('pendingJobs', jobDetails);
      // } else {
      //   this.controller.set('jobListChangePending', false);
      //   this.controller.set('jobs', jobDetails);
      // }

      // this.watchList.jobsIndexDetailsController.abort();
      // console.log(
      //   'new jobIDs have appeared, we should now watch them. We have cancelled the old hash req.',
      //   jobIDs
      // );
      // // ^--- TODO: bad assumption!
      // this.watchList.jobsIndexDetailsController = new AbortController();
      // this.watchJobs.perform(jobIDs, 500);

      yield timeout(throttle); // Moved to the end of the loop
    }
  }

  @restartableTask *watchJobs(jobIDs, throttle = 2000) {
    while (true) {
      // let jobIDs = this.controller.jobIDs;
      if (jobIDs && jobIDs.length > 0) {
        let jobDetails = yield this.jobAllocsQuery(jobIDs);

        // // Just a sec: what if the user doesnt want their list jostled?
        // console.log('xxx jobIds and jobDetails', jobIDs, jobDetails);

        // const stringifiedJobsEntries = JSON.stringify(jobDetails.map(j => j.id));
        // const stringifiedJobIDsEntries = JSON.stringify(jobIDs.map(j => JSON.stringify(Object.values(j))));
        // console.log('checking jobs list pending', this.jobs, this.jobIDs, stringifiedJobsEntries, stringifiedJobIDsEntries);
        // if (stringifiedJobsEntries !== stringifiedJobIDsEntries) {
        //   this.controller.set('jobListChangePending', true);
        //   this.controller.set('pendingJobs', jobDetails);
        // } else {
        //   this.controller.set('jobListChangePending', false);
        this.controller.set('jobs', jobDetails);
        // }
      }
      // TODO: might need an else condition here for if there are no jobIDs,
      // which would indicate no jobs, but the updater above might not fire.
      yield timeout(throttle);
    }
  }

  async model(params) {
    let currentParams = this.getCurrentParams();

    return RSVP.hash({
      jobs: await this.jobQuery(currentParams, { queryType: 'initialize' }),
      namespaces: this.store.findAll('namespace'),
      nodePools: this.store.findAll('node-pool'),
    });
  }

  setupController(controller, model) {
    super.setupController(controller, model);
    controller.set('jobs', model.jobs);
    controller.set('nextToken', model.jobs.meta.nextToken);
    controller.set(
      'jobIDs',
      model.jobs.map((job) => {
        return {
          id: job.plainId,
          namespace: job.belongsTo('namespace').id(),
        };
      })
    );

    // Now that we've set the jobIDs, immediately start watching them
    this.watchJobs.perform(controller.jobIDs, 500);

    // And also watch for any changes to the jobIDs list
    this.watchJobIDs.perform({}, 2000);
  }

  startWatchers(controller, model) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
  }

  @action
  willTransition(transition) {
    if (transition.intent.name.startsWith(this.routeName)) {
      this.watchList.jobsIndexDetailsController.abort();
      this.watchList.jobsIndexIDsController.abort();
    }
  }

  @watchAll('namespace') watchNamespaces;
  @collect('watchNamespaces') watchers;
}
