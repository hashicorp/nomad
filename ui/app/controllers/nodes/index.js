import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
  nodes: computed.alias('model.nodes'),
  agents: computed.alias('model.agents'),

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

  sortedNodes: computed('nodes.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = this.get('nodes').sortBy(this.get('sortProperty'));
    if (this.get('sortDescending')) {
      return sorted.reverse();
    }
    return sorted;
  }),
});
