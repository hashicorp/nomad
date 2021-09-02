import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import { alias } from '@ember/object/computed';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';
import { serialize, deserializedQueryParam as selection } from 'nomad-ui/utils/qp-serialize';

export default class ClientsController extends Controller.extend(
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
      qpStatus: 'status',
    },
    {
      qpDatacenter: 'dc',
    },
    {
      qpNodeClass: 'nodeClass',
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

  @computed
  get searchProps() {
    return ['id', 'name', 'taskGroupName'];
  }

  qpStatus = '';
  qpDatacenter = '';
  qpNodeClass = '';

  @selection('qpStatus') selectionStatus;
  @selection('qpDatacenter') selectionDatacenter;
  @selection('qpNodeClass') selectionNodeClass;

  @computed
  get optionsStatus() {
    return [
      { key: 'queued', label: 'Queued' },
      { key: 'notScheduled', label: 'Not Scheduled' },
      { key: 'starting', label: 'Starting' },
      { key: 'running', label: 'Running' },
      { key: 'complete', label: 'Complete' },
      { key: 'degraded', label: 'Degraded' },
      { key: 'failed', label: 'Failed' },
      { key: 'lost', label: 'Lost' },
    ];
  }

  @alias('model') job;
  @jobClientStatus('nodes', 'job') jobClientStatus;

  @alias('filteredNodes') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedClients;

  get nodes() {
    return this.store.peekAll('node');
  }

  @computed('nodes', 'selectionStatus')
  get filteredNodes() {
    const { selectionStatus: statuses } = this;

    return this.nodes.filter(node => {
      if (statuses.length && !statuses.includes(this.jobClientStatus.byNode[node.id])) {
        return false;
      }
      return true;
    });
  }

  @computed('selectionDatacenter', 'job.datacenters')
  get optionsDatacenter() {
    // eslint-disable-next-line ember/no-incorrect-calls-with-inline-anonymous-functions
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        this.qpDatacenter,
        serialize(intersection(this.job.datacenters, this.selectionDatacenter))
      );
    });

    return this.job.datacenters.sort().map(dc => ({ key: dc, label: dc }));
  }

  @computed('selectionNodeClass', 'nodes')
  get optionsNodeClass() {
    const nodeClasses = Array.from(new Set(this.nodes.mapBy('nodeClass')));
    // eslint-disable-next-line ember/no-incorrect-calls-with-inline-anonymous-functions
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(this.qpNodeClass, serialize(intersection(nodeClasses, this.selectionNodeClassß)));
    });

    return nodeClasses.sort().map(nodeClass => ({ key: nodeClass, label: nodeClass }));
  }

  @action
  gotoClient(client) {
    this.transitionToRoute('clients.client', client);
  }

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }
}
