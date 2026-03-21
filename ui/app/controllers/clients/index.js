/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { service } from '@ember/service';
import Controller, { inject as controller } from '@ember/controller';
import { action, computed, set } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';

export default class IndexController extends Controller.extend(
  SortableFactory(['id', 'name', 'compositeStatus', 'datacenter', 'version']),
  Searchable,
) {
  @service userSettings;
  @service router;
  @controller('clients') clientsController;

  get nodes() {
    return this.model.nodes;
  }

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
      qpClass: 'class',
    },
    {
      qpDatacenter: 'dc',
    },
    {
      qpVersion: 'version',
    },
    {
      qpVolume: 'volume',
    },
    {
      qpNodePool: 'nodePool',
    },
  ];

  filterFunc = (node) => {
    return node.isEligible;
  };

  clientFilterToggles = {
    state: [
      {
        label: 'initializing',
        qp: 'state_initializing',
        default: true,
        filter: (node) => node.status === 'initializing',
      },
      {
        label: 'ready',
        qp: 'state_ready',
        default: true,
        filter: (node) => node.status === 'ready',
      },
      {
        label: 'down',
        qp: 'state_down',
        default: true,
        filter: (node) => node.status === 'down',
      },
      {
        label: 'disconnected',
        qp: 'state_disconnected',
        default: true,
        filter: (node) => node.status === 'disconnected',
      },
    ],
    eligibility: [
      {
        label: 'eligible',
        qp: 'eligibility_eligible',
        default: true,
        filter: (node) => node.isEligible,
      },
      {
        label: 'ineligible',
        qp: 'eligibility_ineligible',
        default: true,
        filter: (node) => !node.isEligible,
      },
    ],
    drainStatus: [
      {
        label: 'draining',
        qp: 'drain_status_draining',
        default: true,
        filter: (node) => node.isDraining,
      },
      {
        label: 'not draining',
        qp: 'drain_status_not_draining',
        default: true,
        filter: (node) => !node.isDraining,
      },
    ],
  };

  @computed(
    'state_initializing',
    'state_ready',
    'state_down',
    'state_disconnected',
    'eligibility_eligible',
    'eligibility_ineligible',
    'drain_status_draining',
    'drain_status_not_draining',
    'allToggles.[]',
  )
  get activeToggles() {
    return this.allToggles.filter((t) => this[t.qp]);
  }

  get allToggles() {
    return Object.values(this.clientFilterToggles).reduce(
      (acc, filters) => acc.concat(filters),
      [],
    );
  }

  constructor() {
    super(...arguments);
    this.addDynamicQueryParams();
  }

  addDynamicQueryParams() {
    this.clientFilterToggles.state.forEach((filter) => {
      this.queryParams.push({ [filter.qp]: filter.qp });
      this[filter.qp] = filter.default;
    });
    this.clientFilterToggles.eligibility.forEach((filter) => {
      this.queryParams.push({ [filter.qp]: filter.qp });
      this[filter.qp] = filter.default;
    });
    this.clientFilterToggles.drainStatus.forEach((filter) => {
      this.queryParams.push({ [filter.qp]: filter.qp });
      this[filter.qp] = filter.default;
    });
  }

  currentPage = 1;

  get pageSize() {
    return this.userSettings.pageSize;
  }

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @computed
  get searchProps() {
    return ['id', 'name', 'datacenter'];
  }

  qpClass = '';
  qpDatacenter = '';
  qpVersion = '';
  qpVolume = '';
  qpNodePool = '';

  @selection('qpClass') selectionClass;
  @selection('qpDatacenter') selectionDatacenter;
  @selection('qpVersion') selectionVersion;
  @selection('qpVolume') selectionVolume;
  @selection('qpNodePool') selectionNodePool;

  @computed('nodes.[]', 'selectionClass')
  get optionsClass() {
    const classes = Array.from(new Set(this.nodes.mapBy('nodeClass')))
      .compact()
      .without('');

    // Remove any invalid node classes from the query param/selection
    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      set(
        this,
        'qpClass',
        serialize(intersection(classes, this.selectionClass)),
      );
    });

    return classes.sort().map((dc) => ({ key: dc, label: dc }));
  }

  @computed('nodes.[]', 'selectionDatacenter')
  get optionsDatacenter() {
    const datacenters = Array.from(
      new Set(this.nodes.mapBy('datacenter')),
    ).compact();

    // Remove any invalid datacenters from the query param/selection
    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      set(
        this,
        'qpDatacenter',
        serialize(intersection(datacenters, this.selectionDatacenter)),
      );
    });

    return datacenters.sort().map((dc) => ({ key: dc, label: dc }));
  }

  @computed('nodes.[]', 'selectionVersion')
  get optionsVersion() {
    const versions = Array.from(new Set(this.nodes.mapBy('version'))).compact();

    // Remove any invalid versions from the query param/selection
    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      set(
        this,
        'qpVersion',
        serialize(intersection(versions, this.selectionVersion)),
      );
    });

    return versions.sort().map((v) => ({ key: v, label: v }));
  }

  @computed('nodes.[]', 'selectionVolume')
  get optionsVolume() {
    const flatten = (acc, val) => acc.concat(val.toArray());

    const allVolumes = this.nodes.mapBy('hostVolumes').reduce(flatten, []);
    const volumes = Array.from(new Set(allVolumes.mapBy('name')));

    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      set(
        this,
        'qpVolume',
        serialize(intersection(volumes, this.selectionVolume)),
      );
    });

    return volumes.sort().map((volume) => ({ key: volume, label: volume }));
  }

  @computed('selectionNodePool', 'model.nodePools.[]')
  get optionsNodePool() {
    const availableNodePools = this.model.nodePools.filter(
      (p) => p.name !== 'all',
    );

    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      set(
        this,
        'qpNodePool',
        serialize(
          intersection(
            availableNodePools.map(({ name }) => name),
            this.selectionNodePool,
          ),
        ),
      );
    });

    return availableNodePools.map((nodePool) => ({
      key: nodePool.name,
      label: nodePool.name,
    }));
  }

  @computed(
    'clientFilterToggles',
    'drain_status_draining',
    'drain_status_not_draining',
    'eligibility_eligible',
    'eligibility_ineligible',
    'nodes.[]',
    'selectionClass',
    'selectionDatacenter',
    'selectionNodePool',
    'selectionVersion',
    'selectionVolume',
    'state_disconnected',
    'state_down',
    'state_initializing',
    'state_ready',
  )
  get filteredNodes() {
    const {
      selectionClass: classes,
      selectionDatacenter: datacenters,
      selectionNodePool: nodePools,
      selectionVersion: versions,
      selectionVolume: volumes,
    } = this;

    let nodes = this.nodes;

    // new QP style filtering
    for (let category in this.clientFilterToggles) {
      nodes = nodes.filter((node) => {
        let includeNode = false;
        for (let filter of this.clientFilterToggles[category]) {
          if (this[filter.qp] && filter.filter(node)) {
            includeNode = true;
            break;
          }
        }
        return includeNode;
      });
    }

    return nodes.filter((node) => {
      if (classes.length && !classes.includes(node.nodeClass)) return false;
      if (datacenters.length && !datacenters.includes(node.datacenter))
        return false;
      if (versions.length && !versions.includes(node.version)) return false;
      if (
        volumes.length &&
        !node.hostVolumes.find((volume) => volumes.includes(volume.name))
      )
        return false;
      if (nodePools.length && !nodePools.includes(node.nodePool)) {
        return false;
      }

      return true;
    });
  }

  get listToSort() {
    return this.filteredNodes;
  }

  get listToSearch() {
    return this.listSorted;
  }

  get sortedNodes() {
    return this.listSearched;
  }

  get isForbidden() {
    return this.clientsController.isForbidden;
  }

  @action
  setFacetQueryParam(queryParam, selection) {
    set(this, queryParam, serialize(selection));
  }

  @action
  handleFilterChange(queryParamValue, option, queryParamLabel) {
    if (queryParamValue.includes(option)) {
      queryParamValue.removeObject(option);
    } else {
      queryParamValue.addObject(option);
    }
    set(this, queryParamLabel, serialize(queryParamValue));
  }

  @action
  toggleClientFilter(queryParam) {
    set(this, queryParam, !this[queryParam]);
  }

  @action
  updateSearchTerm(searchTerm) {
    this.searchTerm = searchTerm;
    this.resetPagination();
  }

  @action
  gotoNode(node) {
    const nodeId = node?.get?.('id') ?? node?.id;

    if (nodeId) {
      this.router.transitionTo('clients.client', nodeId);
    }
  }
}
