import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default Controller.extend(SortableFactory(['updateTime', 'healthy']), {
  userSettings: service(),

  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: readOnly('userSettings.pageSize'),

  sortProperty: 'updateTime',
  sortDescending: false,

  combinedAllocations: computed('model.controllers.[]', 'model.nodes.[]', function() {
    return this.model.controllers.toArray().concat(this.model.nodes.toArray());
  }),

  listToSort: alias('combinedAllocations'),
  sortedAllocations: alias('listSorted'),
  // TODO: Add facets for filtering
  filteredAllocations: alias('sortedAllocations'),

  resetPagination() {
    if (this.currentPage != null) {
      this.set('currentPage', 1);
    }
  },

  actions: {
    gotoAllocation(allocation, event) {
      lazyClick([() => this.transitionToRoute('allocations.allocation', allocation), event]);
    },
  },
});
