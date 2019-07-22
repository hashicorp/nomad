import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import { serialize, deserializedQueryParam as selection } from 'nomad-ui/utils/qp-serialize';

export default Controller.extend(Sortable, Searchable, {
  clientsController: controller('clients'),

  nodes: alias('model.nodes'),
  agents: alias('model.agents'),

  queryParams: {
    currentPage: 'page',
    searchTerm: 'search',
    sortProperty: 'sort',
    sortDescending: 'desc',
    qpClass: 'class',
    qpState: 'state',
    qpDatacenter: 'dc',
  },

  currentPage: 1,
  pageSize: 8,

  sortProperty: 'modifyIndex',
  sortDescending: true,

  searchProps: computed(() => ['id', 'name', 'datacenter']),

  qpClass: '',
  qpState: '',
  qpDatacenter: '',

  selectionClass: selection('qpClass'),
  selectionState: selection('qpState'),
  selectionDatacenter: selection('qpDatacenter'),

  optionsClass: computed('nodes.[]', function() {
    const classes = Array.from(new Set(this.nodes.mapBy('nodeClass'))).compact();

    // Remove any invalid node classes from the query param/selection
    scheduleOnce('actions', () => {
      this.set('qpClass', serialize(intersection(classes, this.selectionClass)));
    });

    return classes.sort().map(dc => ({ key: dc, label: dc }));
  }),

  optionsState: computed(() => [
    { key: 'initializing', label: 'Initializing' },
    { key: 'ready', label: 'Ready' },
    { key: 'down', label: 'Down' },
    { key: 'ineligible', label: 'Ineligible' },
    { key: 'draining', label: 'Draining' },
  ]),

  optionsDatacenter: computed('nodes.[]', function() {
    const datacenters = Array.from(new Set(this.nodes.mapBy('datacenter'))).compact();

    // Remove any invalid datacenters from the query param/selection
    scheduleOnce('actions', () => {
      this.set('qpDatacenter', serialize(intersection(datacenters, this.selectionDatacenter)));
    });

    return datacenters.sort().map(dc => ({ key: dc, label: dc }));
  }),

  filteredNodes: computed(
    'nodes.[]',
    'selectionClass',
    'selectionState',
    'selectionDatacenter',
    function() {
      const {
        selectionClass: classes,
        selectionState: states,
        selectionDatacenter: datacenters,
      } = this;

      const onlyIneligible = states.includes('ineligible');
      const onlyDraining = states.includes('draining');

      // states is a composite of node status and other node states
      const statuses = states.without('ineligible').without('draining');

      return this.nodes.filter(node => {
        if (classes.length && !classes.includes(node.get('nodeClass'))) return false;
        if (statuses.length && !statuses.includes(node.get('status'))) return false;
        if (datacenters.length && !datacenters.includes(node.get('datacenter'))) return false;

        if (onlyIneligible && node.get('isEligible')) return false;
        if (onlyDraining && !node.get('isDraining')) return false;

        return true;
      });
    }
  ),

  listToSort: alias('filteredNodes'),
  listToSearch: alias('listSorted'),
  sortedNodes: alias('listSearched'),

  isForbidden: alias('clientsController.isForbidden'),

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  },

  actions: {
    gotoNode(node) {
      this.transitionToRoute('clients.client', node);
    },
  },
});
