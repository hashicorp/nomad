/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { restartableTask, timeout } from 'ember-concurrency';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
// import { scheduleOnce } from '@ember/runloop';
import Ember from 'ember';

const DEFAULT_THROTTLE = 2000;

export default class JobsIndexController extends Controller {
  @service router;
  @service system;
  @service store;
  @service watchList; // TODO: temp

  // qpNamespace = '*';
  per_page = 3;
  reverse = false;

  queryParams = [
    'cursorAt',
    'per_page',
    // 'status',
    { qpNamespace: 'namespace' },
    // 'type',
    // 'searchTerm',
  ];

  isForbidden = false;

  // #region filtering and sorting

  @tracked jobQueryIndex = 0;
  @tracked jobAllocsQueryIndex = 0;

  @selection('qpNamespace') selectionNamespace;
  // @computed('qpNamespace', 'model.namespaces.[]')
  get optionsNamespace() {
    const availableNamespaces = this.model.namespaces.map((namespace) => ({
      key: namespace.name,
      label: namespace.name,
    }));

    availableNamespaces.unshift({
      key: '*',
      label: 'All (*)',
    });

    // // Unset the namespace selection if it was server-side deleted
    // if (!availableNamespaces.mapBy('key').includes(this.qpNamespace)) {
    //   scheduleOnce('actions', () => {
    //     this.set('qpNamespace', '*');
    //   });
    // }

    return availableNamespaces;
  }

  @action
  handleFilterChange(queryParamValue, option, queryParamLabel) {
    if (queryParamValue.includes(option)) {
      queryParamValue.removeObject(option);
    } else {
      queryParamValue.addObject(option);
    }
    this[queryParamLabel] = serialize(queryParamValue);
  }

  // #endregion filtering and sorting

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

  @action
  goToRun() {
    this.router.transitionTo('jobs.run');
  }

  // #region pagination
  @tracked cursorAt;
  @tracked nextToken; // route sets this when new data is fetched

  /**
   *
   * @param {"prev"|"next"} page
   */
  @action async handlePageChange(page) {
    // reset indexes
    this.jobQueryIndex = 0;
    this.jobAllocsQueryIndex = 0;

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

  /**
   * Manually, on click, update jobs from pendingJobs
   * when live updates are disabled (via nomadLiveUpdateJobsIndex)
   */
  @restartableTask *updateJobList() {
    this.jobs = this.pendingJobs;
    this.pendingJobs = null;
    this.jobIDs = this.pendingJobIDs;
    this.pendingJobIDs = null;
    yield this.watchJobs.perform(
      this.jobIDs,
      Ember.testing ? 0 : DEFAULT_THROTTLE
    );
  }

  @localStorageProperty('nomadLiveUpdateJobsIndex', true) liveUpdatesEnabled;

  // #endregion pagination

  //#region querying

  jobQuery(params) {
    this.watchList.jobsIndexIDsController.abort();
    this.watchList.jobsIndexIDsController = new AbortController();

    return this.store
      .query('job', params, {
        adapterOptions: {
          method: 'GET', // TODO: default
          abortController: this.watchList.jobsIndexIDsController,
        },
      })
      .catch((e) => {
        if (e.name !== 'AbortError') {
          console.log('error fetching job ids', e);
        }
        return;
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
          index: this.jobAllocsQueryIndex, // TODO: consider using a passed params object like jobQuery uses, rather than just passing jobIDs
        },
        {
          adapterOptions: {
            method: 'POST',
            abortController: this.watchList.jobsIndexDetailsController,
          },
        }
      )
      .catch((e) => {
        if (e.name !== 'AbortError') {
          console.log('error fetching job allocs', e);
        } else {
          console.log('|> jobAllocsQuery aborted');
        }
        return;
      });
  }

  async loadPreviousPageToken() {
    let prevPageToken = await this.store.query(
      'job',
      {
        prev_page_query: true, // TODO: debugging only!
        next_token: this.cursorAt,
        per_page: this.per_page + 1,
        reverse: true,
      },
      {
        adapterOptions: {
          method: 'GET',
        },
      }
    );
    return prevPageToken;
  }

  // TODO: set up isEnabled to check blockingQueries rather than just use while (true)
  @restartableTask *watchJobIDs(
    params,
    throttle = Ember.testing ? 0 : DEFAULT_THROTTLE
  ) {
    while (true) {
      // let watchlistIndex = this.watchList.getIndexFor(
      //   '/v1/jobs/statuses?per_page=3'
      // );
      // console.log('> watchJobIDs', params);
      let currentParams = params;
      // currentParams.index = watchlistIndex;
      currentParams.index = this.jobQueryIndex;
      const newJobs = yield this.jobQuery(currentParams, {});
      if (newJobs) {
        // console.log('|> watchJobIDs returned new job IDs', newJobs.length);
        if (newJobs.meta.index) {
          this.jobQueryIndex = newJobs.meta.index;
        }
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
          console.log(
            'new jobIDs have appeared, we should now watch them. We have cancelled the old hash req.',
            jobIDs
          );
          // Let's also reset the index for the job details query
          this.jobAllocsQueryIndex = 0;
          this.watchList.jobsIndexDetailsController = new AbortController();
          // make sure throttle has taken place!
          this.watchJobs.perform(jobIDs, throttle);
        } else {
          // this.controller.set('pendingJobIDs', jobIDs);
          // this.controller.set('pendingJobs', newJobs);
          this.pendingJobIDs = jobIDs;
          this.pendingJobs = newJobs;
        }
        yield timeout(throttle); // Moved to the end of the loop
      } else {
        // This returns undefined on page change / cursorAt change, resulting from the aborting of the old query.
        // console.log('|> watchJobIDs aborted');
        yield timeout(throttle);
        this.watchJobs.perform(this.jobIDs, throttle);
        continue;
      }
      if (Ember.testing) {
        break;
      }
    }
  }

  // Called in 3 ways:
  // 1. via the setupController of the jobs index route's model
  // (which can happen both on initial load, and should the queryParams change)
  // 2. via the watchJobIDs task seeing new jobIDs
  // 3. via the user manually clicking to updateJobList()
  @restartableTask *watchJobs(
    jobIDs,
    throttle = Ember.testing ? 0 : DEFAULT_THROTTLE
  ) {
    while (true) {
      console.log(
        '> watchJobs of IDs',
        jobIDs.map((j) => j.id)
      );
      // let jobIDs = this.controller.jobIDs;
      if (jobIDs && jobIDs.length > 0) {
        let jobDetails = yield this.jobAllocsQuery(jobIDs);
        if (jobDetails) {
          if (jobDetails.meta.index) {
            this.jobAllocsQueryIndex = jobDetails.meta.index;
          }
          console.log(
            '|> watchJobs returned with',
            jobDetails.map((j) => j.id)
          );
        }
        this.jobs = jobDetails;
      }
      yield timeout(throttle);
      if (Ember.testing) {
        break;
      }
    }
  }

  //#endregion querying
}
