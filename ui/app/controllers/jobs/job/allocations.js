/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
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
    {
      qpClient: 'client',
    },
    {
      qpTaskGroup: 'taskGroup',
    },
  ];

  qpStatus = '';
  qpClient = '';
  qpTaskGroup = '';
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

  @computed('allocations.[]', 'selectionStatus', 'selectionClient', 'selectionTaskGroup')
  get filteredAllocations() {
    const { selectionStatus, selectionClient, selectionTaskGroup } = this;

    return this.allocations.filter(alloc => {
      if (selectionStatus.length && !selectionStatus.includes(alloc.clientStatus)) {
        return false;
      }
      if (selectionClient.length && !selectionClient.includes(alloc.get('node.shortId'))) {
        return false;
      }
      if (selectionTaskGroup.length && !selectionTaskGroup.includes(alloc.taskGroupName)) {
        return false;
      }
      return true;
    });
  }

  @alias('filteredAllocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @selection('qpStatus') selectionStatus;
  @selection('qpClient') selectionClient;
  @selection('qpTaskGroup') selectionTaskGroup;

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation);
  }

  get optionsAllocationStatus() {
    return [
      { key: 'pending', label: 'Pending' },
      { key: 'running', label: 'Running' },
      { key: 'complete', label: 'Complete' },
      { key: 'failed', label: 'Failed' },
      { key: 'lost', label: 'Lost' },
    ];
  }

  @computed('model.allocations.[]', 'selectionClient')
  get optionsClients() {
    const clients = Array.from(new Set(this.model.allocations.mapBy('node.shortId'))).compact();

    // Update query param when the list of clients changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set('qpClient', serialize(intersection(clients, this.selectionClient)));
    });

    return clients.sort().map(c => ({ key: c, label: c }));
  }

  @computed('model.allocations.[]', 'selectionTaskGroup')
  get optionsTaskGroups() {
    const taskGroups = Array.from(new Set(this.model.allocations.mapBy('taskGroupName'))).compact();

    // Update query param when the list of task groups changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set('qpTaskGroup', serialize(intersection(taskGroups, this.selectionTaskGroup)));
    });

    return taskGroups.sort().map(tg => ({ key: tg, label: tg }));
  }

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }
}
