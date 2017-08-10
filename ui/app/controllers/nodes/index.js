import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';

const { Controller, computed } = Ember;

export default Controller.extend(Sortable, {
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

  listToSort: computed.alias('nodes'),
  sortedNodes: computed.alias('listSorted'),
});
