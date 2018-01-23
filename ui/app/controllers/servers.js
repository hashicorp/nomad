import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import Sortable from 'nomad-ui/mixins/sortable';

export default Controller.extend(Sortable, {
  nodes: alias('model.nodes'),
  agents: alias('model.agents'),

  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: 8,

  sortProperty: 'isLeader',
  sortDescending: true,

  isForbidden: false,

  listToSort: alias('agents'),
  sortedAgents: alias('listSorted'),
});
