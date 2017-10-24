import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

const { Controller, computed, inject } = Ember;

export default Controller.extend(Sortable, Searchable, {
  system: inject.service(),
  jobsController: inject.controller('jobs'),

  isForbidden: computed.alias('jobsController.isForbidden'),

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

  filteredJobs: computed(
    'model.[]',
    'system.activeNamespace',
    'system.namespaces.length',
    function() {
      if (this.get('system.namespaces.length')) {
        return this.get('model').filterBy('namespace.id', this.get('system.activeNamespace.id'));
      } else {
        return this.get('model');
      }
    }
  ),

  listToSort: computed.alias('filteredJobs'),
  listToSearch: computed.alias('listSorted'),
  sortedJobs: computed.alias('listSearched'),

  isShowingDeploymentDetails: false,

  actions: {
    gotoJob(job) {
      this.transitionToRoute('jobs.job', job);
    },

    refresh() {
      this.send('refreshRoute');
    },
  },
});
