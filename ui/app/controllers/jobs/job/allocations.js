import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import classic from 'ember-classic-decorator';

@classic
export default class AllocationsController extends Controller.extend(
    Sortable,
    Searchable,
    WithNamespaceResetting
  ) {
  queryParams = [
    {
      currentPage: 'page',
    },
    {
      searchTerm: 'search',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  currentPage = 1;
  pageSize = 25;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @alias('model') job;

  @computed
  get searchProps() {
    return ['shortId', 'name', 'taskGroupName'];
  }

  @computed('model.allocations.[]')
  get allocations() {
    return this.get('model.allocations') || [];
  }

  @alias('allocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation);
  }
}
