import { alias, readOnly } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

export default Controller.extend(Sortable, Searchable, WithNamespaceResetting, {
  userSettings: service(),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: readOnly('userSettings.pageSize'),

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['shortId', 'name']),

  allocations: computed('model.allocations.[]', function() {
    return this.get('model.allocations') || [];
  }),

  listToSort: alias('allocations'),
  listToSearch: alias('listSorted'),
  sortedAllocations: alias('listSearched'),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
