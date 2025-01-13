/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { restartableTask, timeout } from 'ember-concurrency';
import Ember from 'ember';

@classic
export default class IndexController extends Controller.extend(
  WithNamespaceResetting
) {
  @service system;
  @service watchList;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    'activeTask',
    'statusMode',
    'filter',
  ];

  currentPage = 1;

  @alias('model') job;

  sortProperty = 'name';
  sortDescending = false;

  @tracked activeTask = null;

  /**
   * @type {('current'|'historical')}
   */
  @tracked
  statusMode = 'current';

  @action
  setActiveTaskQueryParam(task) {
    if (task) {
      this.activeTask = `${task.allocation.id}-${task.name}`;
    } else {
      this.activeTask = null;
    }
  }

  @action
  updateFilter(filter) {
    console.log('=== controllers/jobs/job/index.js updateFilter', filter);
    // this.filter = filter;
    this.set('filter', filter);
    this.resetQueryIndex({
      id: this.job.get('plainId'),
      namespace: this.job.get('namespace.id'),
    });
    this.watchChildJobs.perform({
      id: this.job.get('plainId'),
      namespace: this.job.get('namespace.id'),
    });
  }

  //#region Filter

  // TODO: An awful lot of this is copied from the jobs/index.js file.
  // We should probably move this to a shared location.

  filter = '';
  searchText = '';
  rawSearchText = '';

  @action resetFilters() {
    this.searchText = '';
    this.rawSearchText = '';
    this.filterFacets.forEach((group) => {
      group.options.forEach((option) => {
        set(option, 'checked', false);
      });
    });
    this.namespaceFacet?.options.forEach((option) => {
      set(option, 'checked', false);
    });
    this.updateFilter();
  }

  /**
   * Updates the filter based on the input, distinguishing between simple job names and filter expressions.
   * A simple check for operators with surrounding spaces is used to identify filter expressions.
   *
   * @param {string} newFilter
   */
  @action
  updateSearchText(newFilter) {
    console.log('=== controllers/jobs/job/index.js updateSearchText', newFilter);
    if (!newFilter.trim()) {
      this.searchText = '';
      return;
    }

    newFilter = newFilter.trim();

    const operators = [
      '==',
      '!=',
      'contains',
      'not contains',
      'is empty',
      'is not empty',
      'matches',
      'not matches',
      'in',
      'not in',
    ];

    // Check for any operator surrounded by spaces
    let isFilterExpression = operators.some((op) =>
      newFilter.includes(` ${op}`)
    );

    if (isFilterExpression) {
      this.searchText = newFilter;
    } else {
      // If it's a string without a filter operator, assume the user is trying to look up a job name
      this.searchText = `Name contains "${newFilter}"`;
    }
  }

  //#endregion Filter

  /**
   * @param {('current'|'historical')} mode
   */
  @action
  setStatusMode(mode) {
    this.statusMode = mode;
  }

  @tracked
  childJobsController = new AbortController();

  childJobsQuery(params) {
    this.childJobsController.abort();
    this.childJobsController = new AbortController();

    return this.store
      .query('job', params, {
        adapterOptions: {
          abortController: this.childJobsController,
        },
      })
      .catch((e) => {
        if (e.name !== 'AbortError') {
          console.log('error fetching job ids', e);
        }
        return;
      });
  }

  @tracked childJobs = [];

  resetQueryIndex({ id, namespace }) {
    this.watchList.setIndexFor(`child-jobs-for-${id}-${namespace}`, 1);
  }

  @restartableTask *watchChildJobs(
    { id, namespace },
    throttle = Ember.testing ? 0 : 2000
  ) {
    this.childJobs = [];
    while (true) {
      console.log('=== controllers/jobs/job/index.js watchChildJobs', id, namespace);
      let filter = `ParentID == "${id}"`;
      if (this.filter) {
        // filter = `(${filter}) and (${this.filter})`;
        filter = `(${filter}) and (${this.filter})`;
      }
      let params = {
        filter,
        namespace,
        include_children: true,
      };
      params.index = this.watchList.getIndexFor(
        `child-jobs-for-${id}-${namespace}`
      );

      const childJobs = yield this.childJobsQuery(params);
      if (childJobs) {
        if (childJobs.meta.index) {
          this.watchList.setIndexFor(
            `child-jobs-for-${id}-${namespace}`,
            childJobs.meta.index
          );
        }
        this.childJobs = childJobs;
        yield timeout(throttle);
      }
      if (Ember.testing) {
        break;
      }
    }
  }
}
