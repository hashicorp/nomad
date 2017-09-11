import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

const { Controller, computed } = Ember;

export default Controller.extend(Sortable, Searchable, {
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

  searchProps: computed(() => ['id', 'name']),

  listToSort: computed.alias('model'),
  listToSearch: computed.alias('listSorted'),
  sortedJobs: computed.alias('listSearched'),

  isShowingDeploymentDetails: false,

  actions: {
    gotoJob(job) {
      this.transitionToRoute('jobs.job', job);
    },
  },
});
