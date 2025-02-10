/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed, set } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { restartableTask, timeout } from 'ember-concurrency';
import Ember from 'ember';
// eslint-disable-next-line no-unused-vars
import JobModel from '../../models/job';

const JOB_LIST_THROTTLE = 5000;
const JOB_DETAILS_THROTTLE = 1000;

export default class JobsIndexController extends Controller {
  @service router;
  @service system;
  @service store;
  @service userSettings;
  @service watchList;
  @service notifications;

  @tracked pageSize;

  constructor() {
    super(...arguments);
    this.pageSize = this.userSettings.pageSize;
    this.rawSearchText = this.searchText || '';
  }

  queryParams = ['cursorAt', 'pageSize', 'filter'];

  isForbidden = false;

  @tracked jobQueryIndex = 0;
  @tracked jobAllocsQueryIndex = 0;

  get tableColumns() {
    return [
      'name',
      this.system.shouldShowNamespaces ? 'namespace' : null,
      'status',
      'type',
      this.system.shouldShowNodepools ? 'node pool' : null, // TODO: implement on system service
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

  /**
   * Trigger can either be the pointer event itself, or if the keyboard shorcut was used, the html element corresponding to the job.
   * @param {JobModel} job
   * @param {PointerEvent|HTMLElement} trigger
   */
  @action
  gotoJob(job, trigger) {
    // Don't navigate if the user clicked on a link; this will happen with modifier keys like cmd/ctrl on the link itself
    if (
      trigger instanceof PointerEvent &&
      /** @type {HTMLElement} */ (trigger.target).tagName === 'A'
    ) {
      return;
    }
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
      if (!this.cursorAt) {
        return;
      }
      // Note (and TODO:) this isn't particularly efficient!
      // We're making an extra full request to get the nextToken we need,
      // but actually the results of that request are the reverse order, plus one job,
      // of what we actually want to show on the page!
      // I should investigate whether I can use the results of this query to
      // overwrite this controller's jobIDs, leverage its index, and
      // restart a blocking watchJobIDs here.
      let prevPageToken = await this.loadPreviousPageToken();
      // If there's no nextToken, we're at the "start" of our list and can drop the cursorAt
      if (!prevPageToken.meta.nextToken) {
        this.cursorAt = undefined;
      } else {
        // cursorAt should be the highest modifyIndex from the previous query.
        // This will immediately fire the route model hook with the new cursorAt
        this.cursorAt = prevPageToken
          .sortBy('modifyIndex')
          .get('lastObject').modifyIndex;
      }
    } else if (page === 'next') {
      if (!this.nextToken) {
        return;
      }
      this.cursorAt = this.nextToken;
    } else if (page === 'first') {
      this.cursorAt = undefined;
    } else if (page === 'last') {
      let prevPageToken = await this.loadPreviousPageToken({ last: true });
      this.cursorAt = prevPageToken
        .sortBy('modifyIndex')
        .get('lastObject').modifyIndex;
    }
  }

  @action handlePageSizeChange(size) {
    this.pageSize = size;
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
      Ember.testing ? 0 : JOB_DETAILS_THROTTLE
    );
  }

  /**
   * In case the user wants to specifically stop polling for new jobs
   */
  @action pauseJobFetching() {
    let notification = this.notifications.queue.find(
      (n) => n.title === 'Error fetching jobs'
    );
    if (notification) {
      notification.destroyMessage();
    }
    this.watchList.jobsIndexIDsController.abort();
    this.watchList.jobsIndexDetailsController.abort();
    this.watchJobIDs.cancelAll();
    this.watchJobs.cancelAll();
  }

  @action restartJobList() {
    this.showingCachedJobs = false;
    let notification = this.notifications.queue.find(
      (n) => n.title === 'Error fetching jobs'
    );
    if (notification) {
      notification.destroyMessage();
    }
    this.watchList.jobsIndexIDsController.abort();
    this.watchList.jobsIndexDetailsController.abort();
    this.watchJobIDs.cancelAll();
    this.watchJobs.cancelAll();
    this.watchJobIDs.perform({}, JOB_LIST_THROTTLE);
    this.watchJobs.perform(this.jobIDs, JOB_DETAILS_THROTTLE);
  }

  @localStorageProperty('nomadLiveUpdateJobsIndex', true) liveUpdatesEnabled;

  // #endregion pagination

  //#region querying

  /**
   *
   * Let the user know that there was difficulty fetching jobs, but don't overload their screen with notifications.
   * Set showingCachedJobs to tell the template to prompt them to extend timeouts
   * @param {Error} e
   */
  notifyFetchError(e) {
    const firstError = e.errors?.objectAt(0);
    this.notifications.add({
      title: 'Error fetching jobs',
      message: `The backend returned an error with status ${firstError.status} while fetching jobs`,
      color: 'critical',
      sticky: true,
      preventDuplicates: true,
    });
    // Specific check for a proxy timeout error
    if (
      !this.showingCachedJobs &&
      (firstError.status === '502' || firstError.status === '504')
    ) {
      this.showingCachedJobs = true;
    }
  }

  @tracked showingCachedJobs = false;

  jobQuery(params) {
    this.watchList.jobsIndexIDsController.abort();
    this.watchList.jobsIndexIDsController = new AbortController();

    return this.store
      .query('job', params, {
        adapterOptions: {
          abortController: this.watchList.jobsIndexIDsController,
        },
      })
      .then((jobs) => {
        this.showingCachedJobs = false;
        return jobs;
      })
      .catch((e) => {
        if (e.name !== 'AbortError') {
          console.log('error fetching job ids', e);
          this.notifyFetchError(e);
        }
        if (this.jobs?.length) {
          return this.jobs;
        }
        return;
      });
  }

  jobAllocsQuery(params) {
    this.watchList.jobsIndexDetailsController.abort();
    this.watchList.jobsIndexDetailsController = new AbortController();
    params.namespace = '*';
    return this.store
      .query('job', params, {
        adapterOptions: {
          method: 'POST',
          abortController: this.watchList.jobsIndexDetailsController,
        },
      })
      .catch((e) => {
        if (e.name !== 'AbortError') {
          console.log('error fetching job allocs', e);
          this.notifyFetchError(e);
        }
        if (this.jobs?.length) {
          return this.jobs;
        }
        return;
      });
  }

  // Ask for the previous #page_size jobs, starting at the first job that's currently shown
  // on our page, and the last one in our list should be the one we use for our
  // subsequent nextToken.
  async loadPreviousPageToken({ last = false } = {}) {
    let next_token = +this.cursorAt + 1;
    if (last) {
      next_token = undefined;
    }
    let prevPageToken = await this.store.query(
      'job',
      {
        next_token,
        per_page: this.pageSize,
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

  @restartableTask *watchJobIDs(
    params,
    throttle = Ember.testing ? 0 : JOB_LIST_THROTTLE
  ) {
    while (true) {
      let currentParams = params;
      currentParams.index = this.jobQueryIndex;
      const newJobs = yield this.jobQuery(currentParams, {});
      if (newJobs) {
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
          this.jobAllocsQueryIndex = 0;
          this.watchList.jobsIndexDetailsController = new AbortController();
          this.watchJobs.perform(jobIDs, throttle);
        } else {
          this.pendingJobIDs = jobIDs;
          this.pendingJobs = newJobs;
        }
        if (Ember.testing) {
          break;
        }
        yield timeout(throttle);
      } else {
        if (Ember.testing) {
          break;
        }
        // This returns undefined on page change / cursorAt change, resulting from the aborting of the old query.
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
    throttle = Ember.testing ? 0 : JOB_DETAILS_THROTTLE
  ) {
    while (true) {
      if (jobIDs && jobIDs.length > 0) {
        let jobDetails = yield this.jobAllocsQuery({
          jobs: jobIDs,
          index: this.jobAllocsQueryIndex,
        });
        if (jobDetails) {
          if (jobDetails.meta.index) {
            this.jobAllocsQueryIndex = jobDetails.meta.index;
          }
        }
        this.jobs = jobDetails;
      } else {
        // No jobs have returned, so clear the list
        this.jobs = [];
      }
      yield timeout(throttle);
      if (Ember.testing) {
        break;
      }
    }
  }
  //#endregion querying

  //#region filtering and searching

  @tracked statusFacet = {
    label: 'Status',
    options: [
      {
        key: 'pending',
        string: 'Status == pending',
        checked: false,
      },
      {
        key: 'running',
        string: 'Status == running',
        checked: false,
      },
      {
        key: 'dead',
        string: 'Status == dead',
        checked: false,
      },
    ],
  };

  @tracked typeFacet = {
    label: 'Type',
    options: [
      {
        key: 'batch',
        string: 'Type == batch',
        checked: false,
      },
      {
        key: 'service',
        string: 'Type == service',
        checked: false,
      },
      {
        key: 'system',
        string: 'Type == system',
        checked: false,
      },
      {
        key: 'sysbatch',
        string: 'Type == sysbatch',
        checked: false,
      },
    ],
  };

  @tracked nodePoolFacet = {
    label: 'NodePool',
    options: (this.model.nodePools || []).map((nodePool) => ({
      key: nodePool.name,
      string: `NodePool == "${nodePool.name}"`,
      checked: false,
    })),
    filterable: true,
    filter: '',
  };

  @tracked namespaceFacet = {
    label: 'Namespace',
    options: [
      ...(this.model.namespaces || []).map((ns) => ({
        key: ns.name,
        string: `Namespace == "${ns.name}"`,
        checked: false,
      })),
    ],
    filterable: true,
    filter: '',
  };

  @computed('namespaceFacet.{filter,options}')
  get filteredNamespaceOptions() {
    return this.namespaceFacet.options.filter((ns) =>
      ns.key.toLowerCase().includes(this.namespaceFacet.filter.toLowerCase())
    );
  }

  @computed('nodePoolFacet.{filter,options}')
  get filteredNodePoolOptions() {
    return this.nodePoolFacet.options.filter((np) =>
      np.key.toLowerCase().includes(this.nodePoolFacet.filter.toLowerCase())
    );
  }

  @tracked namespaceFilter = '';

  get shownNamespaces() {
    return this.namespaceFacet.options.filter((option) =>
      option.label.toLowerCase().includes(this.namespaceFilter)
    );
  }

  /**
   * Pares down the list of namespaces
   * @param {InputEvent & { target: HTMLInputElement }} event - The input event
   */
  @action filterNamespaces(event) {
    this.namespaceFilter = event.target.value.toLowerCase();
  }

  get filterFacets() {
    let facets = [this.statusFacet, this.typeFacet];
    if (this.system.shouldShowNodepools) {
      facets.push(this.nodePoolFacet);
    }
    // Note: there is a timing problem with using system.shouldShowNamespaces here, and that's
    // due to parseFilter() below depending on this and being called a single time from the route's
    // setupController.
    // The system service's shouldShowNamespaces is a getter, and therefore cannot be made to be async,
    // and since we only want to parseFilter a single time, we can use a simpler check to establish whether
    // we should show the namespace facet, rendering the whole "check checkboxes based on queryParams" logic quicker.
    if ((this.model.namespaces || []).length > 1) {
      facets.push(this.namespaceFacet);
    }
    return facets;
  }

  /**
   * On page load, takes the ?filter queryParam, and extracts it into those
   * properties used by the dropdown filter toggles, and the search text.
   */
  parseFilter() {
    let filterString = this.filter;
    if (!filterString) {
      return;
    }

    const filterParts = filterString.split(' and ');

    let unmatchedFilters = [];

    // For each of those splits, if it starts and ends with (), and if all entries within it have thes ame Propname and operator of ==, populate them into the appropriate dropdown
    // If it doesnt start with and end with (), or if it does but not all entries are the same propname, or not all entries have == operators, populate them into the searchbox

    filterParts.forEach((part) => {
      let matched = false;
      if (part.startsWith('(') && part.endsWith(')')) {
        part = part.slice(1, -1); // trim the parens
        // Check to see if the property name (first word) is one of the ones for which we have a dropdown
        let propName = part.split(' ')[0];
        if (this.filterFacets.find((facet) => facet.label === propName)) {
          // Split along "or" and check that all parts have the same propName
          let facetParts = part.split(' or ');
          let allMatch = facetParts.every((facetPart) =>
            facetPart.startsWith(propName)
          );
          let allEqualityOperators = facetParts.every((facetPart) =>
            facetPart.includes('==')
          );
          if (allMatch && allEqualityOperators) {
            // Set all the options in the dropdown to checked
            this.filterFacets.forEach((group) => {
              if (group.label === propName) {
                group.options.forEach((option) => {
                  set(option, 'checked', facetParts.includes(option.string));
                });
              }
            });
            matched = true;
          }
        }
      }
      if (!matched) {
        unmatchedFilters.push(part);
      }
    });

    // Combine all unmatched filter parts into the searchText
    this.searchText = unmatchedFilters.join(' and ');
    this.rawSearchText = this.searchText;
  }

  @computed(
    'filterFacets',
    'nodePoolFacet.options.@each.checked',
    'searchText',
    'statusFacet.options.@each.checked',
    'typeFacet.options.@each.checked',
    'namespaceFacet.options.@each.checked'
  )
  get computedFilter() {
    let parts = this.searchText ? [this.searchText] : [];
    this.filterFacets.forEach((group) => {
      let groupParts = [];
      group.options.forEach((option) => {
        if (option.checked) {
          groupParts.push(option.string);
        }
      });
      if (groupParts.length) {
        parts.push(`(${groupParts.join(' or ')})`);
      }
    });
    return parts.join(' and ');
  }

  @action
  toggleOption(option) {
    set(option, 'checked', !option.checked);
    this.updateFilter();
  }

  @action
  updateFilter() {
    this.cursorAt = null;
    this.filter = this.computedFilter;
  }

  @tracked filter = '';
  @tracked searchText = '';
  @tracked rawSearchText = '';

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

  get humanizedFilterError() {
    let baseString = `No jobs match your current filter selection: ${this.filter}.`;
    if (this.model.error?.humanized) {
      return `${baseString} ${this.model.error.humanized}`;
    }
    return baseString;
  }

  @action correctFilterKey({ incorrectKey, correctKey }) {
    this.searchText = this.searchText.replace(incorrectKey, correctKey);
    this.rawSearchText = this.searchText;
    this.updateFilter();
  }

  @action suggestFilter({ example }) {
    this.searchText = example;
    this.rawSearchText = this.searchText;
    this.updateFilter();
  }

  // A list of combinatorial filters to show off filter expressions
  // Make use of our various operators, and our various known keys
  @computed('filter')
  get exampleFilter() {
    let examples = [
      '(Status == dead) and (Type != batch)',
      '(Version != 0) and (Namespace == default)',
      '(StatusDescription not contains "progress deadline")',
      '(Region != global) and (NodePool is not empty)',
      '(Namespace != myNamespace) and (Status != running)',
      'NodePool is not empty',
      '(dc1 in Datacenters) or (dc2 in Datacenters)',
    ];
    return examples[Math.floor(Math.random() * examples.length)];
  }

  //#endregion filtering and searching
}
