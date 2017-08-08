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
  breadcrumbs: computed.alias('jobController.breadcrumbs'),

  taskGroups: computed('model.taskGroups.[]', function() {
    return this.get('model.taskGroups') || [];
  }),

  sortedTaskGroups: computed('taskGroups.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = this.get('taskGroups').sortBy(this.get('sortProperty'));
    if (this.get('sortDescending')) {
      return sorted.reverse();
    }
    return sorted;
  }),
});
