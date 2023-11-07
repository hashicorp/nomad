/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-observers */
/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { observes } from '@ember-decorators/object';
import { scheduleOnce } from '@ember/runloop';
import { task } from 'ember-concurrency';
import intersection from 'lodash.intersection';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

@classic
export default class ClientController extends Controller.extend(
  Sortable,
  Searchable
) {
  @service notifications;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      searchTerm: 'search',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    {
      onlyPreemptions: 'preemptions',
    },
    {
      qpNamespace: 'namespace',
    },
    {
      qpJob: 'job',
    },
    {
      qpStatus: 'status',
    },
    'activeTask',
  ];

  // Set in the route
  flagAsDraining = false;

  qpNamespace = '';
  qpJob = '';
  qpStatus = '';
  currentPage = 1;
  pageSize = 8;
  activeTask = null;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @localStorageProperty('nomadShowSubTasks', false) showSubTasks;

  @action
  toggleShowSubTasks(e) {
    e.preventDefault();
    this.set('showSubTasks', !this.get('showSubTasks'));
  }

  @computed()
  get searchProps() {
    return ['shortId', 'name'];
  }

  onlyPreemptions = false;

  @computed('model.allocations.[]', 'preemptions.[]', 'onlyPreemptions')
  get visibleAllocations() {
    return this.onlyPreemptions ? this.preemptions : this.model.allocations;
  }

  @computed(
    'visibleAllocations.[]',
    'selectionNamespace',
    'selectionJob',
    'selectionStatus'
  )
  get filteredAllocations() {
    const { selectionNamespace, selectionJob, selectionStatus } = this;

    return this.visibleAllocations.filter((alloc) => {
      if (
        selectionNamespace.length &&
        !selectionNamespace.includes(alloc.get('namespace'))
      ) {
        return false;
      }
      if (
        selectionJob.length &&
        !selectionJob.includes(alloc.get('plainJobId'))
      ) {
        return false;
      }
      if (
        selectionStatus.length &&
        !selectionStatus.includes(alloc.clientStatus)
      ) {
        return false;
      }
      return true;
    });
  }

  @alias('filteredAllocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @selection('qpNamespace') selectionNamespace;
  @selection('qpJob') selectionJob;
  @selection('qpStatus') selectionStatus;

  eligibilityError = null;
  stopDrainError = null;
  drainError = null;
  showDrainNotification = false;
  showDrainUpdateNotification = false;
  showDrainStoppedNotification = false;

  @computed('model.allocations.@each.wasPreempted')
  get preemptions() {
    return this.model.allocations.filterBy('wasPreempted');
  }

  @computed('model.events.@each.time')
  get sortedEvents() {
    return this.get('model.events').sortBy('time').reverse();
  }

  @computed('model.drivers.@each.name')
  get sortedDrivers() {
    return this.get('model.drivers').sortBy('name');
  }

  @computed('model.hostVolumes.@each.name')
  get sortedHostVolumes() {
    return this.model.hostVolumes.sortBy('name');
  }

  @(task(function* (value) {
    try {
      yield value ? this.model.setEligible() : this.model.setIneligible();
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not set eligibility';
      this.set('eligibilityError', error);
    }
  }).drop())
  setEligibility;

  @(task(function* () {
    try {
      this.set('flagAsDraining', false);
      yield this.model.cancelDrain();
      this.set('showDrainStoppedNotification', true);
    } catch (err) {
      this.set('flagAsDraining', true);
      const error = messageFromAdapterError(err) || 'Could not stop drain';
      this.set('stopDrainError', error);
    }
  }).drop())
  stopDrain;

  @(task(function* () {
    try {
      yield this.model.forceDrain({
        IgnoreSystemJobs: this.model.drainStrategy.ignoreSystemJobs,
      });
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not force drain';
      this.set('drainError', error);
    }
  }).drop())
  forceDrain;

  @observes('model.isDraining')
  triggerDrainNotification() {
    if (!this.model.isDraining && this.flagAsDraining) {
      this.set('showDrainNotification', true);
    }

    this.set('flagAsDraining', this.model.isDraining);
  }

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation.id);
  }

  @action
  setPreemptionFilter(value) {
    this.set('onlyPreemptions', value);
  }

  @action
  drainNotify(isUpdating) {
    this.set('showDrainUpdateNotification', isUpdating);
  }

  @action
  setDrainError(err) {
    const error = messageFromAdapterError(err) || 'Could not run drain';
    this.set('drainError', error);
  }

  get optionsAllocationStatus() {
    return [
      { key: 'pending', label: 'Pending' },
      { key: 'running', label: 'Running' },
      { key: 'complete', label: 'Complete' },
      { key: 'failed', label: 'Failed' },
      { key: 'lost', label: 'Lost' },
      { key: 'unknown', label: 'Unknown' },
    ];
  }

  @computed('model.allocations.[]', 'selectionJob', 'selectionNamespace')
  get optionsJob() {
    // Only show options for jobs in the selected namespaces, if any.
    const ns = this.selectionNamespace;
    const jobs = Array.from(
      new Set(
        this.model.allocations
          .filter((a) => ns.length === 0 || ns.includes(a.namespace))
          .mapBy('plainJobId')
      )
    ).compact();

    // Update query param when the list of jobs changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set('qpJob', serialize(intersection(jobs, this.selectionJob)));
    });

    return jobs.sort().map((job) => ({ key: job, label: job }));
  }

  @computed('model.allocations.[]', 'selectionNamespace')
  get optionsNamespace() {
    const ns = Array.from(
      new Set(this.model.allocations.mapBy('namespace'))
    ).compact();

    // Update query param when the list of namespaces changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpNamespace',
        serialize(intersection(ns, this.selectionNamespace))
      );
    });

    return ns.sort().map((n) => ({ key: n, label: n }));
  }

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }

  @action
  setActiveTaskQueryParam(task) {
    if (task) {
      this.set('activeTask', `${task.allocation.id}-${task.name}`);
    } else {
      this.set('activeTask', null);
    }
  }

  // #region metadata

  @tracked editingMetadata = false;

  get hasMeta() {
    return (
      this.model.meta?.structured && Object.keys(this.model.meta?.structured)
    );
  }

  @tracked newMetaData = {
    key: '',
    value: '',
  };

  @action resetNewMetaData() {
    this.newMetaData = {
      key: '',
      value: '',
    };
  }

  @action validateMetadata(event) {
    if (event.key === 'Escape') {
      this.resetNewMetaData();
      this.editingMetadata = false;
    }
  }

  @action async addDynamicMetaData({ key, value }, e) {
    try {
      e.preventDefault();
      await this.model.addMeta({ [key]: value });

      this.notifications.add({
        title: 'Metadata added',
        message: `${key} successfully saved`,
        color: 'success',
      });
    } catch (err) {
      const error =
        messageFromAdapterError(err) || 'Could not save new dynamic metadata';
      this.notifications.add({
        title: `Error saving Metadata`,
        message: error,
        color: 'critical',
        sticky: true,
      });
    }
  }
  // #endregion metadata
}
