/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { alias, readOnly } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import Controller, { inject as controller } from '@ember/controller';
import { action, computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';

@classic
export default class IndexController extends Controller.extend(
  SortableFactory(['id', 'name', 'compositeStatus', 'datacenter', 'version']),
  Searchable
) {
  @service userSettings;
  @controller('clients') clientsController;

  @alias('model.nodes') nodes;
  @alias('model.agents') agents;

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
      qpState: 'state',
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

  currentPage = 1;
  @readOnly('userSettings.pageSize') pageSize;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @computed
  get searchProps() {
    return ['id', 'name', 'datacenter'];
  }

  qpClass = '';
  qpState = '';
  qpDatacenter = '';
  qpVersion = '';
  qpVolume = '';
  qpNodePool = '';

  @selection('qpClass') selectionClass;
  @selection('qpState') selectionState;
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
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpClass',
        serialize(intersection(classes, this.selectionClass))
      );
    });

    return classes.sort().map((dc) => ({ key: dc, label: dc }));
  }

  @computed
  get optionsState() {
    return [
      { key: 'initializing', label: 'Initializing' },
      { key: 'ready', label: 'Ready' },
      { key: 'down', label: 'Down' },
      { key: 'ineligible', label: 'Ineligible' },
      { key: 'draining', label: 'Draining' },
      { key: 'disconnected', label: 'Disconnected' },
    ];
  }

  @computed('nodes.[]', 'selectionDatacenter')
  get optionsDatacenter() {
    const datacenters = Array.from(
      new Set(this.nodes.mapBy('datacenter'))
    ).compact();

    // Remove any invalid datacenters from the query param/selection
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpDatacenter',
        serialize(intersection(datacenters, this.selectionDatacenter))
      );
    });

    return datacenters.sort().map((dc) => ({ key: dc, label: dc }));
  }

  @computed('nodes.[]', 'selectionVersion')
  get optionsVersion() {
    const versions = Array.from(new Set(this.nodes.mapBy('version'))).compact();

    // Remove any invalid versions from the query param/selection
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpVersion',
        serialize(intersection(versions, this.selectionVersion))
      );
    });

    return versions.sort().map((v) => ({ key: v, label: v }));
  }

  @computed('nodes.[]', 'selectionVolume')
  get optionsVolume() {
    const flatten = (acc, val) => acc.concat(val.toArray());

    const allVolumes = this.nodes.mapBy('hostVolumes').reduce(flatten, []);
    const volumes = Array.from(new Set(allVolumes.mapBy('name')));

    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpVolume',
        serialize(intersection(volumes, this.selectionVolume))
      );
    });

    return volumes.sort().map((volume) => ({ key: volume, label: volume }));
  }

  @computed('selectionNodePool', 'model.nodePools.[]')
  get optionsNodePool() {
    const availableNodePools = this.model.nodePools.filter(
      (p) => p.name !== 'all'
    );

    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpNodePool',
        serialize(
          intersection(
            availableNodePools.map(({ name }) => name),
            this.selectionNodePool
          )
        )
      );
    });

    return availableNodePools.map((nodePool) => ({
      key: nodePool.name,
      label: nodePool.name,
    }));
  }

  @computed(
    'nodes.[]',
    'selectionClass',
    'selectionState',
    'selectionDatacenter',
    'selectionNodePool',
    'selectionVersion',
    'selectionVolume'
  )
  get filteredNodes() {
    const {
      selectionClass: classes,
      selectionState: states,
      selectionDatacenter: datacenters,
      selectionNodePool: nodePools,
      selectionVersion: versions,
      selectionVolume: volumes,
    } = this;

    const onlyIneligible = states.includes('ineligible');
    const onlyDraining = states.includes('draining');

    // states is a composite of node status and other node states
    const statuses = states.without('ineligible').without('draining');

    return this.nodes.filter((node) => {
      if (classes.length && !classes.includes(node.get('nodeClass')))
        return false;
      if (statuses.length && !statuses.includes(node.get('status')))
        return false;
      if (datacenters.length && !datacenters.includes(node.get('datacenter')))
        return false;
      if (versions.length && !versions.includes(node.get('version')))
        return false;
      if (
        volumes.length &&
        !node.hostVolumes.find((volume) => volumes.includes(volume.name))
      )
        return false;
      if (nodePools.length && !nodePools.includes(node.get('nodePool'))) {
        return false;
      }

      if (onlyIneligible && node.get('isEligible')) return false;
      if (onlyDraining && !node.get('isDraining')) return false;

      return true;
    });
  }

  @alias('filteredNodes') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedNodes;

  @alias('clientsController.isForbidden') isForbidden;

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }

  @action
  gotoNode(node) {
    this.transitionToRoute('clients.client', node);
  }
}
