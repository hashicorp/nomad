import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';

const { Controller, computed } = Ember;

export default Controller.extend(Sortable, {
  pendingJobs: computed.filterBy('model', 'status', 'pending'),
  runningJobs: computed.filterBy('model', 'status', 'running'),
  deadJobs: computed.filterBy('model', 'status', 'dead'),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: 10,

  sortProperty: 'modifyIndex',
  sortDescending: true,

  listToSort: computed.alias('model'),
  sortedJobs: computed.alias('listSorted'),
});
