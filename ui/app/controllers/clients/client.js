import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

export default Controller.extend(Sortable, Searchable, {
  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
    onlyPreemptions: 'preemptions',
  },

  currentPage: 1,
  pageSize: 8,

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['shortId', 'name']),

  onlyPreemptions: false,

  visibleAllocations: computed(
    'model.allocations.[]',
    'preemptions.[]',
    'onlyPreemptions',
    function() {
      return this.onlyPreemptions ? this.preemptions : this.model.allocations;
    }
  ),

  listToSort: alias('visibleAllocations'),
  listToSearch: alias('listSorted'),
  sortedAllocations: alias('listSearched'),

  preemptions: computed('model.allocations.@each.wasPreempted', function() {
    return this.model.allocations.filterBy('wasPreempted');
  }),

  sortedEvents: computed('model.events.@each.time', function() {
    return this.get('model.events')
      .sortBy('time')
      .reverse();
  }),

  sortedDrivers: computed('model.drivers.@each.name', function() {
    return this.get('model.drivers').sortBy('name');
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },

    setPreemptionFilter(value) {
      this.set('onlyPreemptions', value);
    },
  },
});
