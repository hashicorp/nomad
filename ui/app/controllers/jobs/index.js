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
    'perPage',
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

  /**
   *
   * @param {"prev"|"next"} page
   */
  @action async handlePageChange(page, event, c) {
    if (page === 'prev') {
      // Note (and TODO:) this isn't particularly efficient!
      // We're making an extra full request to get the nextToken we need,
      // but actually the results of that request are the reverse order, plus one job,
      // of what we actually want to show on the page!
      // I should investigate whether I can use the results of this query to
      // overwrite this controller's jobIDs, leverage its index, and
      // restart a blocking watchJobIDs here.
      let prevPageToken = await this.loadPreviousPageToken();
      if (prevPageToken.length > 1) {
        // if there's only one result, it'd be the job you passed into it as your nextToken (and the first shown on your current page)
        const [id, namespace] = JSON.parse(prevPageToken.lastObject.id);
        // If there's no nextToken, we're at the "start" of our list and can drop the cursorAt
        if (!prevPageToken.meta.nextToken) {
          this.cursorAt = null;
        } else {
          this.cursorAt = `${namespace}.${id}`;
        }
      }
    } else if (page === 'next') {
      // this.previousTokens = [...this.previousTokens, this.cursorAt];
      this.cursorAt = this.nextToken;
    }
  }

  get pendingJobIDDiff() {
    return (
      this.pendingJobIDs &&
      JSON.stringify(
        this.pendingJobIDs.map((j) => `${j.namespace}.${j.id}`)
      ) !== JSON.stringify(this.jobIDs.map((j) => `${j.namespace}.${j.id}`))
    );
  }

  @restartableTask *updateJobList() {
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
        // console.log('error fetching job ids', e);
        // TODO: gracefully handle this! Maybe don't return, but set a flag on the controller?
        // Or retry the request after a timeout?
        return;
      });
  }

  async loadPreviousPageToken() {
    let prevPageToken = await this.store.query(
      'job',
      {
        next_token: this.cursorAt,
        per_page: this.perPage + 1,
        reverse: true,
      },
      {
        adapterOptions: {
          method: 'GET',
          queryType: 'initialize',
        },
      }
    );
    return prevPageToken;
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
        // console.log('error fetching job allocs', e);
        return;
      });
  }

  perPage = 3;
  defaultParams = {
    meta: true,
    per_page: this.perPage,
  };

  // TODO: set up isEnabled to check blockingQueries rather than just use while (true)
  @restartableTask *watchJobIDs(params, throttle = 2000) {
    while (true) {
      let currentParams = params;
      const newJobs = yield this.jobQuery(currentParams, {
        queryType: 'update_ids',
      });
      if (newJobs.meta.nextToken) {
        this.nextToken = newJobs.meta.nextToken;
      } else {
        this.nextToken = null;
      }

      const jobIDs = newJobs.map((job) => ({
        id: job.plainId,
        namespace: job.belongsTo('namespace').id(),
      }));

      const okayToJostle = this.liveUpdatesEnabled;
      if (okayToJostle) {
        this.jobIDs = jobIDs;
        this.watchList.jobsIndexDetailsController.abort();
        // console.log(
        //   'new jobIDs have appeared, we should now watch them. We have cancelled the old hash req.',
        //   jobIDs
        // );
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
        this.jobs = jobDetails;
      }
      yield timeout(throttle);
    }
  }

  //#endregion querying
}
