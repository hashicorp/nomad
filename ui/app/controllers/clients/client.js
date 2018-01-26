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

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
