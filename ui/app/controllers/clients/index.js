import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

const { Controller, computed, inject } = Ember;

export default Controller.extend(Sortable, Searchable, {
  clientsController: inject.controller('clients'),

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

  searchProps: computed(() => ['id', 'name', 'datacenter']),

  listToSort: computed.alias('nodes'),
  listToSearch: computed.alias('listSorted'),
  sortedNodes: computed.alias('listSearched'),

  isForbidden: computed.alias('clientsController.isForbidden'),

  actions: {
    gotoNode(node) {
      this.transitionToRoute('clients.client', node);
    },
  },
});
