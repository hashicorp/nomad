import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';

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
  @jobClientStatus('nodes', 'job.status', 'job.allocations') jobClientStatus;

  @alias('uniqueNodes') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedClients;

  @computed('job')
  get uniqueNodes() {
    // add datacenter filter
    const allocs = this.job.allocations;
    const nodes = allocs.mapBy('node');
    const uniqueNodes = nodes.uniqBy('id').toArray();
    const result = uniqueNodes.map(nodeId => {
      return {
        [nodeId.get('id')]: allocs
          .toArray()
          .filter(alloc => nodeId.get('id') === alloc.get('node.id'))
          .map(alloc => ({
            nodeId,
            ...alloc.getProperties('clientStatus', 'name', 'createTime', 'modifyTime'),
          })),
      };
    });
    return result;
  }

  @action
  gotoClient(client) {
    console.log('goToClient', client);
    this.transitionToRoute('clients.client', client);
  }
}
