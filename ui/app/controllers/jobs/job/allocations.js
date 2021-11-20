import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { serialize, deserializedQueryParam as selection } from 'nomad-ui/utils/qp-serialize';
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
    {
      qpStatus: 'status',
    },
  ];

  qpStatus = '';
  currentPage = 1;
  pageSize = 25;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @alias('model') job;

  @computed
  get searchProps() {
    return ['shortId', 'name', 'taskGroupName'];
  }

  @computed('model.allocations.[]', 'selectionStatus')
  get allocations() {
    const allocations = this.get('model.allocations') || [];
    const { selectionStatus } = this;

    if (!allocations.length) return allocations;

    return allocations.filter(alloc => {
      if (selectionStatus.length && !selectionStatus.includes(alloc.status)) {
        return false;
      }

      return true;
    });
  }

  @selection('qpStatus') selectionStatus;

  @alias('allocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation);
  }

  get optionsAllocationStatus() {
    return [
      { key: 'queued', label: 'Queued' },
      { key: 'starting', label: 'Starting' },
      { key: 'running', label: 'Running' },
      { key: 'complete', label: 'Complete' },
      { key: 'failed', label: 'Failed' },
      { key: 'lost', label: 'Lost' },
    ];
  }

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }
}
