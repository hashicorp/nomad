import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';

export default Controller.extend(Sortable, Searchable, {
  clientsController: controller('clients'),

  nodes: alias('model.nodes'),
  agents: alias('model.agents'),

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

  listToSort: alias('nodes'),
  listToSearch: alias('listSorted'),
  sortedNodes: alias('listSearched'),

  isForbidden: alias('clientsController.isForbidden'),

  actions: {
    gotoNode(node) {
      this.transitionToRoute('clients.client', node);
    },
  },
});
