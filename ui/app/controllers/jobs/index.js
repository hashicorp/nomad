import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

const { Controller, computed, observer, inject } = Ember;

export default Controller.extend(Sortable, Searchable, {
  system: inject.service(),

  pendingJobs: computed.filterBy('model', 'status', 'pending'),
  runningJobs: computed.filterBy('model', 'status', 'running'),
  deadJobs: computed.filterBy('model', 'status', 'dead'),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
    jobNamespace: 'namespace',
  },

  currentPage: 1,
  pageSize: 10,
  jobNamespace: 'default',

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['id', 'name']),

  filteredJobs: computed('model.[]', 'jobNamespace', function() {
    if (this.get('system.namespaces.length')) {
      return this.get('model').filterBy('namespace.name', this.get('jobNamespace'));
    } else {
      return this.get('model');
    }
  }),

  listToSort: computed.alias('filteredJobs'),
  listToSearch: computed.alias('listSorted'),
  sortedJobs: computed.alias('listSearched'),

  isShowingDeploymentDetails: false,

  // The namespace query param should act as an alias to the system active namespace.
  // But query param defaults can't be CPs: https://github.com/emberjs/ember.js/issues/9819
  syncNamespaceService: observer('jobNamespace', function() {
    const newNamespace = this.get('jobNamespace');
    const currentNamespace = this.get('system.activeNamespace.id');
    const bothAreDefault =
      currentNamespace == undefined ||
      (currentNamespace === 'default' && newNamespace == undefined) ||
      newNamespace === 'default';

    if (currentNamespace !== newNamespace && !bothAreDefault) {
      this.set('system.activeNamespace', newNamespace);
      this.send('refreshRoute');
    }
  }),

  actions: {
    gotoJob(job) {
      this.transitionToRoute('jobs.job', job);
    },

    refresh() {
      this.send('refreshRoute');
    },
  },
});
