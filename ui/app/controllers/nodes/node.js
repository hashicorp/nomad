import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
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

  sortedAllocations: computed('model.allocations.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = this.get('model.allocations').sortBy(this.get('sortProperty'));
    if (this.get('sortDescending')) {
      return sorted.reverse();
    }
    return sorted;
  }),
});
