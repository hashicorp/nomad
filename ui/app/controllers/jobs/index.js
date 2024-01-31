/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { restartableTask, timeout } from 'ember-concurrency';

export default class JobsIndexController extends Controller {
  @service router;
  @service system;
  @service store;
  @service watchList; // TODO: temp

  queryParams = [
    'cursorAt',
    'pageSize',
    // 'status',
    { qpNamespace: 'namespace' },
    // 'type',
    // 'searchTerm',
  ];

  isForbidden = false;

  get tableColumns() {
    return [
      'name',
      this.system.shouldShowNamespaces ? 'namespace' : null,
      this.system.shouldShowNodepools ? 'node pools' : null, // TODO: implement on system service
      'status',
      'type',
      'priority',
      'running allocations',
    ]
      .filter((c) => !!c)
      .map((c) => {
        return {
          label: c.charAt(0).toUpperCase() + c.slice(1),
          width: c === 'running allocations' ? '200px' : undefined,
        };
      });
  }

  @tracked jobs = [];
  @tracked jobIDs = [];
  @tracked pendingJobs = null;
  @tracked pendingJobIDs = null;

  @action
  gotoJob(job) {
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  }

  // #region pagination
  @tracked cursorAt;
  @tracked nextToken; // route sets this when new data is fetched
  @tracked previousTokens = [];

  /**
   *
   * @param {"prev"|"next"} page
   */
  @action handlePageChange(page, event, c) {
    console.log('hPC', page, event, c);
    // event.preventDefault();
    if (page === 'prev') {
      console.log('prev page');
      this.cursorAt = this.previousTokens.pop();
      this.previousTokens = [...this.previousTokens];
    } else if (page === 'next') {
      console.log('next page', this.nextToken);
      this.previousTokens = [...this.previousTokens, this.cursorAt];
      this.cursorAt = this.nextToken;
    }
  }

  get pendingJobIDDiff() {
    console.log('pending job IDs', this.pendingJobIDs, this.jobIDs);
    return (
      this.pendingJobIDs &&
      JSON.stringify(
        this.pendingJobIDs.map((j) => `${j.namespace}.${j.id}`)
      ) !== JSON.stringify(this.jobIDs.map((j) => `${j.namespace}.${j.id}`))
    );
  }

  @restartableTask *updateJobList() {
    console.log('updating jobs list');
    this.jobs = this.pendingJobs;
    this.pendingJobs = null;
    this.jobIDs = this.pendingJobIDs;
    this.pendingJobIDs = null;
    this.watchJobs.perform(this.jobIDs, 500);
  }

  @localStorageProperty('nomadLiveUpdateJobsIndex', false) liveUpdatesEnabled;

  // #endregion pagination

  //#region querying

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
      .catch((e) => {
        console.error('error fetching job ids', e);
      });
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
      .catch((e) => {
        console.error('error fetching job allocs', e);
      });
  }

  perPage = 3;
  defaultParams = {
    meta: true,
    per_page: this.perPage,
  };

  // TODO: this is a pretty hacky way of handling params-grabbing. Can probably iterate over this.queryParams instead.
  getCurrentParams() {
    let currentRouteName = this.router.currentRouteName;
    let currentRoute = this.router.currentRoute;
    let params = currentRoute.params[currentRouteName] || {};
    console.log('GCP', params, currentRoute, currentRouteName);
    return { ...this.defaultParams, ...params };
  }

  @restartableTask *watchJobIDs(params, throttle = 2000) {
    while (true) {
      let currentParams = this.getCurrentParams();
      console.log('xxx watchJobIDs', this.queryParams);
      const newJobs = yield this.jobQuery(currentParams, {
        queryType: 'update_ids',
      });
      if (newJobs.meta.nextToken) {
        this.nextToken = newJobs.meta.nextToken;
      }

      const jobIDs = newJobs.map((job) => ({
        id: job.plainId,
        namespace: job.belongsTo('namespace').id(),
      }));

      const okayToJostle = this.liveUpdatesEnabled;
      console.log('okay to jostle?', okayToJostle);
      if (okayToJostle) {
        this.jobIDs = jobIDs;
        this.watchList.jobsIndexDetailsController.abort();
        console.log(
          'new jobIDs have appeared, we should now watch them. We have cancelled the old hash req.',
          jobIDs
        );
        this.watchList.jobsIndexDetailsController = new AbortController();
        this.watchJobs.perform(jobIDs, 500);
      } else {
        // this.controller.set('pendingJobIDs', jobIDs);
        // this.controller.set('pendingJobs', newJobs);
        this.pendingJobIDs = jobIDs;
        this.pendingJobs = newJobs;
      }
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
        // this.controller.set('jobs', jobDetails);
        this.jobs = jobDetails;
        // }
      }
      // TODO: might need an else condition here for if there are no jobIDs,
      // which would indicate no jobs, but the updater above might not fire.
      yield timeout(throttle);
    }
  }

  //#endregion querying
}
