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
import { serialize, deserializedQueryParam as selection } from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';

@classic
export default class ClientController extends Controller.extend(Sortable, Searchable) {
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
      qpStatus: 'status',
    },
    {
      qpTaskGroup: 'taskGroup',
    },
  ];

  // Set in the route
  flagAsDraining = false;

  qpStatus = '';
  qpTaskGroup = '';
  currentPage = 1;
  pageSize = 8;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @computed()
  get searchProps() {
    return ['shortId', 'name'];
  }

  onlyPreemptions = false;

  @computed(
    'model.allocations.[]',
    'preemptions.[]',
    'onlyPreemptions',
    'selectionStatus',
    'selectionTaskGroup'
  )
  get visibleAllocations() {
    const allocations = this.onlyPreemptions ? this.preemptions : this.model.allocations;
    const { selectionStatus, selectionTaskGroup } = this;

    return allocations.filter(alloc => {
      if (selectionStatus.length && !selectionStatus.includes(alloc.clientStatus)) {
        return false;
      }
      if (selectionTaskGroup.length && !selectionTaskGroup.includes(alloc.taskGroupName)) {
        return false;
      }
      return true;
    });
  }

  @alias('visibleAllocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @selection('qpStatus') selectionStatus;
  @selection('qpTaskGroup') selectionTaskGroup;

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
    return this.get('model.events')
      .sortBy('time')
      .reverse();
  }

  @computed('model.drivers.@each.name')
  get sortedDrivers() {
    return this.get('model.drivers').sortBy('name');
  }

  @computed('model.hostVolumes.@each.name')
  get sortedHostVolumes() {
    return this.model.hostVolumes.sortBy('name');
  }

  @(task(function*(value) {
    try {
      yield value ? this.model.setEligible() : this.model.setIneligible();
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not set eligibility';
      this.set('eligibilityError', error);
    }
  }).drop())
  setEligibility;

  @(task(function*() {
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

  @(task(function*() {
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
    this.transitionToRoute('allocations.allocation', allocation);
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
      { key: 'queued', label: 'Queued' },
      { key: 'starting', label: 'Starting' },
      { key: 'running', label: 'Running' },
      { key: 'complete', label: 'Complete' },
      { key: 'failed', label: 'Failed' },
      { key: 'lost', label: 'Lost' },
    ];
  }

  @computed('model.allocations.[]', 'selectionTaskGroup')
  get optionsTaskGroups() {
    const taskGroups = Array.from(new Set(this.model.allocations.mapBy('taskGroupName'))).compact();

    // Update query param when the list of clients changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set('qpTaskGroup', serialize(intersection(taskGroups, this.selectionTaskGroup)));
    });

    return taskGroups.sort().map(dc => ({ key: dc, label: dc }));
  }

  @action
  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }
}
