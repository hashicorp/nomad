import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

const { Controller, computed, inject } = Ember;

export default Controller.extend(Sortable, Searchable, WithNamespaceResetting, {
  jobController: inject.controller('jobs.job'),

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

  searchProps: computed(() => ['shortId', 'name']),

  allocations: computed('model.allocations.[]', function() {
    return this.get('model.allocations') || [];
  }),

  listToSort: computed.alias('allocations'),
  listToSearch: computed.alias('listSorted'),
  sortedAllocations: computed.alias('listSearched'),

  breadcrumbs: computed('jobController.breadcrumbs.[]', 'model.{name}', function() {
    return this.get('jobController.breadcrumbs').concat([
      { label: this.get('model.name'), args: ['jobs.job.task-group', this.get('model.name')] },
    ]);
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
