import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import Sortable from 'nomad-ui/mixins/sortable';

export default class ServersController extends Controller.extend(Sortable) {
  @alias('model.nodes') nodes;
  @alias('model.agents') agents;

  queryParams = {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  };

  currentPage = 1;
  pageSize = 8;

  sortProperty = 'isLeader';
  sortDescending = true;

  isForbidden = false;

  @alias('agents') listToSort;
  @alias('listSorted') sortedAgents;
}
