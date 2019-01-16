import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default Controller.extend(Sortable, Searchable, WithNamespaceResetting, {
  jobController: controller('jobs.job'),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: 10,

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['shortId', 'name']),

  allocations: computed('model.allocations.[]', function() {
    return this.get('model.allocations') || [];
  }),

  listToSort: alias('allocations'),
  listToSearch: alias('listSorted'),
  sortedAllocations: alias('listSearched'),

  breadcrumbs: computed('jobController.breadcrumbs.[]', 'model.{name}', function() {
    return this.get('jobController.breadcrumbs').concat([
      {
        label: this.get('model.name'),
        args: [
          'jobs.job.task-group',
          this.get('model.name'),
          qpBuilder({ jobNamespace: this.get('model.job.namespace.name') || 'default' }),
        ],
      },
    ]);
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
