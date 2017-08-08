import Ember from 'ember';

const { Controller, computed, inject } = Ember;

export default Controller.extend({
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

  jobController: inject.controller('jobs.job'),

  allocations: computed('model.allocations.[]', function() {
    return this.get('model.allocations') || [];
  }),

  breadcrumbs: computed('jobController.breadcrumbs.[]', 'model.{name}', function() {
    return this.get('jobController.breadcrumbs').concat([
      { label: this.get('model.name'), args: ['jobs.job.task-group', this.get('model.name')] },
    ]);
  }),

  sortedAllocations: computed('allocations.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = this.get('allocations').sortBy(this.get('sortProperty'));
    if (this.get('sortDescending')) {
      return sorted.reverse();
    }
    return sorted;
  }),
});
