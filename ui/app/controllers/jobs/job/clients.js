/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import { alias } from '@ember/object/computed';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';
import { serialize, deserializedQueryParam as selection } from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';

@classic
export default class ClientsController extends Controller.extend(
    SortableFactory(['id', 'name', 'jobStatus']),
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
      qpNodeClass: 'nodeclass',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  qpStatus = '';
  qpDatacenter = '';
  qpNodeClass = '';

  currentPage = 1;
  pageSize = 25;

  sortProperty = 'jobStatus';
  sortDescending = false;

  @selection('qpStatus') selectionStatus;
  @selection('qpDatacenter') selectionDatacenter;
  @selection('qpNodeClass') selectionNodeClass;

  @alias('model') job;
  @jobClientStatus('allNodes', 'job') jobClientStatus;

  @alias('filteredNodes') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedClients;

  @computed('store')
  get allNodes() {
    return this.store.peekAll('node').length
      ? this.store.peekAll('node')
      : this.store.findAll('node');
  }

  @computed('allNodes', 'jobClientStatus.byNode')
  get nodes() {
    return this.allNodes.filter(node => this.jobClientStatus.byNode[node.id]);
  }

  @computed
  get searchProps() {
    return ['node.id', 'node.name'];
  }

  @computed(
    'nodes',
    'job.allocations',
    'jobClientStatus.byNode',
    'selectionStatus',
    'selectionDatacenter',
    'selectionNodeClass'
  )
  get filteredNodes() {
    const {
      selectionStatus: statuses,
      selectionDatacenter: datacenters,
      selectionNodeClass: nodeclasses,
    } = this;

    return this.nodes
      .filter(node => {
        if (statuses.length && !statuses.includes(this.jobClientStatus.byNode[node.id])) {
          return false;
        }
        if (datacenters.length && !datacenters.includes(node.datacenter)) {
          return false;
        }
        if (nodeclasses.length && !nodeclasses.includes(node.nodeClass)) {
          return false;
        }

        return true;
      })
      .map(node => {
        const allocations = this.job.allocations.filter(alloc => alloc.get('node.id') == node.id);

        return {
          node,
          jobStatus: this.jobClientStatus.byNode[node.id],
          allocations,
          createTime: eldestCreateTime(allocations),
          modifyTime: mostRecentModifyTime(allocations),
        };
      });
  }

  @computed
  get optionsJobStatus() {
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

  @computed('selectionDatacenter', 'nodes')
  get optionsDatacenter() {
    const datacenters = Array.from(new Set(this.nodes.mapBy('datacenter'))).compact();

    // Update query param when the list of datacenters changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set('qpDatacenter', serialize(intersection(datacenters, this.selectionDatacenter)));
    });

    return datacenters.sort().map(dc => ({ key: dc, label: dc }));
  }

  @computed('selectionNodeClass', 'nodes')
  get optionsNodeClass() {
    const nodeClasses = Array.from(new Set(this.nodes.mapBy('nodeClass'))).compact();

    // Update query param when the list of datacenters changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set('qpNodeClass', serialize(intersection(nodeClasses, this.selectionNodeClass)));
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

function eldestCreateTime(allocations) {
  let eldest = null;
  for (const alloc of allocations) {
    if (!eldest || alloc.createTime < eldest) {
      eldest = alloc.createTime;
    }
  }
  return eldest;
}

function mostRecentModifyTime(allocations) {
  let mostRecent = null;
  for (const alloc of allocations) {
    if (!mostRecent || alloc.modifyTime > mostRecent) {
      mostRecent = alloc.modifyTime;
    }
  }
  return mostRecent;
}
