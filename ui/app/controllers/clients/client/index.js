/* eslint-disable ember/no-observers */
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { observes } from '@ember-decorators/object';
import { task } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
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
  ];

  // Set in the route
  flagAsDraining = false;

  currentPage = 1;
  pageSize = 8;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @computed()
  get searchProps() {
    return ['shortId', 'name'];
  }

  onlyPreemptions = false;

  @computed('model.allocations.[]', 'preemptions.[]', 'onlyPreemptions')
  get visibleAllocations() {
    return this.onlyPreemptions ? this.preemptions : this.model.allocations;
  }

  @alias('visibleAllocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

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
}
