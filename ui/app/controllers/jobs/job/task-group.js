import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';

const { Controller, computed, inject } = Ember;

export default Controller.extend(Sortable, {
  jobController: inject.controller('jobs.job'),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: 10,

  sortProperty: 'name',
  sortDescending: false,

  allocations: computed('model.allocations.[]', function() {
    return this.get('model.allocations') || [];
  }),

  listToSort: computed.alias('allocations'),
  sortedAllocations: computed.alias('listSorted'),

  breadcrumbs: computed('jobController.breadcrumbs.[]', 'model.{name}', function() {
    return this.get('jobController.breadcrumbs').concat([
      { label: this.get('model.name'), args: ['jobs.job.task-group', this.get('model.name')] },
    ]);
  }),
});
