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
  },

  currentPage: 1,
  pageSize: 8,

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['shortId', 'name']),

  listToSort: alias('model.allocations'),
  listToSearch: alias('listSorted'),
  sortedAllocations: alias('listSearched'),

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
  },
});
