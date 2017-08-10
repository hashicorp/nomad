import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';

const { Controller, computed } = Ember;

export default Controller.extend(Sortable, {
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

  listToSort: computed.alias('model.allocations'),
  sortedAllocations: computed.alias('listSorted'),
});
