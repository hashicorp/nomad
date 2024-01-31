/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { restartableTask } from 'ember-concurrency';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

const ALL_NAMESPACE_WILDCARD = '*';

export default class JobsIndexController extends Controller {
  @service router;
  @service system;
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

  // /**
  //  * If job_ids are different from jobs, it means our GET summaries has returned
  //  * some new jobs. Instead of jostling the list for the user, give them the option
  //  * to refresh the list.
  //  */
  // @computed('jobs.[]', 'jobIDs.[]')
  // get jobListChangePending() {
  //   const stringifiedJobsEntries = JSON.stringify(this.jobs.map((j) => j.id));
  //   const stringifiedJobIDsEntries = JSON.stringify(
  //     this.jobIDs.map((j) => JSON.stringify(Object.values(j)))
  //   );
  //   console.log(
  //     'checking jobs list pending',
  //     this.jobs,
  //     this.jobIDs,
  //     stringifiedJobsEntries,
  //     stringifiedJobIDsEntries
  //   );
  //   return stringifiedJobsEntries !== stringifiedJobIDsEntries;
  //   // return this.jobs.map((j) => j.id).join() !== this.jobIDs.join();
  //   // return true;
  // }

  @tracked pendingJobs = null;
  @tracked pendingJobIDs = null;

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
    // TODO: need to re-kick-off the watchJobs task with updated jobIDs
  }

  @localStorageProperty('nomadLiveUpdateJobsIndex', false) liveUpdatesEnabled;

  // @action updateJobList() {
  //   console.log('updating jobs list');
  //   this.jobs = this.pendingJobs;
  // }
  // #endregion pagination
}
