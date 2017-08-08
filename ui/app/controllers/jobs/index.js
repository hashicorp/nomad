import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
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

  sortedJobs: computed('model.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = this.get('model').sortBy(this.get('sortProperty'));
    if (this.get('sortDescending')) {
      return sorted.reverse();
    }
    return sorted;
  }),
});
