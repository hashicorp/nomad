import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import { serialize, deserializedQueryParam as selection } from 'nomad-ui/utils/qp-serialize';

export default Controller.extend(SortableFactory(['updateTime', 'healthy']), {
  userSettings: service(),

  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
    qpHealth: 'healthy',
    qpType: 'type',
  },

  currentPage: 1,
  pageSize: readOnly('userSettings.pageSize'),

  sortProperty: 'updateTime',
  sortDescending: false,

  qpType: '',
  qpHealth: '',

  selectionType: selection('qpType'),
  selectionHealth: selection('qpHealth'),

  optionsType: computed(() => [
    { key: 'controller', label: 'Controller' },
    { key: 'node', label: 'Node' },
  ]),

  optionsHealth: computed(() => [
    { key: true, label: 'Healthy' },
    { key: false, label: 'Unhealthy' },
  ]),

  combinedAllocations: computed('model.controllers.[]', 'model.nodes.[]', function() {
    return this.model.controllers.toArray().concat(this.model.nodes.toArray());
  }),

  filteredAllocations: computed(
    'combinedAllocations.[]',
    'model.controllers.[]',
    'model.nodes.[]',
    'selectionType',
    'selectionHealth',
    function() {
      const { selectionType: types, selectionHealth: healths } = this;

      // Instead of filtering the combined list, revert back to one of the two
      // pre-existing lists.
      let listToFilter = this.combinedAllocations;
      if (types.length === 1 && types[0] === 'controller') {
        listToFilter = this.model.controllers;
      } else if (types.length === 1 && types[0] === 'node') {
        listToFilter = this.model.nodes;
      }

      if (healths.length === 1 && healths[0]) return listToFilter.filterBy('healthy');
      if (healths.length === 1 && !healths[0]) return listToFilter.filterBy('healthy', false);
      return listToFilter;
    }
  ),

  listToSort: alias('filteredAllocations'),
  sortedAllocations: alias('listSorted'),

  resetPagination() {
    if (this.currentPage != null) {
      this.set('currentPage', 1);
    }
  },

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  },

  actions: {
    gotoAllocation(allocation, event) {
      lazyClick([() => this.transitionToRoute('allocations.allocation', allocation), event]);
    },
  },
});
