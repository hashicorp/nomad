import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { task, timeout } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import { stats } from 'nomad-ui/utils/classes/node-stats-tracker';

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

  stats: stats('model', function statsFetch() {
    return url => this.get('token').authorizedRequest(url);
  }),

  pollStats: task(function*() {
    while (true) {
      yield this.get('stats').poll();
      yield timeout(1000);
    }
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
