/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import Controller from '@ember/controller';
import { computed, action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import classic from 'ember-classic-decorator';
import { reduceBytes, reduceHertz } from 'nomad-ui/utils/units';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import Searchable from 'nomad-ui/mixins/searchable';

const sumAggregator = (sum, value) => sum + (value || 0);
const formatter = new Intl.NumberFormat(window.navigator.locale || 'en', {
  maximumFractionDigits: 2,
});

@classic
export default class TopologyControllers extends Controller.extend(Searchable) {
  @service userSettings;

  queryParams = [
    {
      searchTerm: 'search',
    },
    {
      qpState: 'status',
    },
    {
      qpVersion: 'version',
    },
    {
      qpClass: 'class',
    },
    {
      qpDatacenter: 'dc',
    },
    {
      qpNodePool: 'nodePool',
    },
  ];

  @tracked searchTerm = '';
  qpState = '';
  qpVersion = '';
  qpClass = '';
  qpDatacenter = '';
  qpNodePool = '';

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }

  @selection('qpState') selectionState;
  @selection('qpClass') selectionClass;
  @selection('qpDatacenter') selectionDatacenter;
  @selection('qpNodePool') selectionNodePool;
  @selection('qpVersion') selectionVersion;

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

  @computed('model.nodes', 'nodes.[]', 'selectionClass')
  get optionsClass() {
    const classes = Array.from(new Set(this.model.nodes.mapBy('nodeClass')))
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

  @computed('model.nodes', 'nodes.[]', 'selectionDatacenter')
  get optionsDatacenter() {
    const datacenters = Array.from(
      new Set(this.model.nodes.mapBy('datacenter'))
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

  @computed('model.nodePools.[]', 'selectionNodePool')
  get optionsNodePool() {
    const availableNodePools = this.model.nodePools;

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

    return availableNodePools.sort().map((nodePool) => ({
      key: nodePool.name,
      label: nodePool.name,
    }));
  }

  @computed('model.nodes', 'nodes.[]', 'selectionVersion')
  get optionsVersion() {
    const versions = Array.from(
      new Set(this.model.nodes.mapBy('version'))
    ).compact();

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

  @alias('userSettings.showTopoVizPollingNotice') showPollingNotice;

  @tracked pre09Nodes = null;

  get filteredNodes() {
    const { nodes } = this.model;
    return nodes.filter((node) => {
      const {
        searchTerm,
        selectionState,
        selectionVersion,
        selectionDatacenter,
        selectionClass,
        selectionNodePool,
      } = this;
      const matchState =
        selectionState.includes(node.status) ||
        (selectionState.includes('ineligible') && !node.isEligible) ||
        (selectionState.includes('draining') && node.isDraining);

      return (
        (selectionState.length ? matchState : true) &&
        (selectionVersion.length
          ? selectionVersion.includes(node.version)
          : true) &&
        (selectionDatacenter.length
          ? selectionDatacenter.includes(node.datacenter)
          : true) &&
        (selectionClass.length
          ? selectionClass.includes(node.nodeClass)
          : true) &&
        (selectionNodePool.length
          ? selectionNodePool.includes(node.nodePool)
          : true) &&
        (node.name.includes(searchTerm) ||
          node.datacenter.includes(searchTerm) ||
          node.nodeClass.includes(searchTerm))
      );
    });
  }

  @computed('model.nodes.@each.datacenter')
  get datacenters() {
    return Array.from(new Set(this.model.nodes.mapBy('datacenter'))).compact();
  }

  @computed('model.allocations.@each.isScheduled')
  get scheduledAllocations() {
    return this.model.allocations.filterBy('isScheduled');
  }

  @computed('model.nodes.@each.resources')
  get totalMemory() {
    const mibs = this.model.nodes
      .mapBy('resources.memory')
      .reduce(sumAggregator, 0);
    return mibs * 1024 * 1024;
  }

  @computed('model.nodes.@each.resources')
  get totalCPU() {
    return this.model.nodes
      .mapBy('resources.cpu')
      .reduce((sum, cpu) => sum + (cpu || 0), 0);
  }

  @computed('totalMemory')
  get totalMemoryFormatted() {
    return formatter.format(reduceBytes(this.totalMemory)[0]);
  }

  @computed('totalMemory')
  get totalMemoryUnits() {
    return reduceBytes(this.totalMemory)[1];
  }

  @computed('totalCPU')
  get totalCPUFormatted() {
    return formatter.format(reduceHertz(this.totalCPU, null, 'MHz')[0]);
  }

  @computed('totalCPU')
  get totalCPUUnits() {
    return reduceHertz(this.totalCPU, null, 'MHz')[1];
  }

  @computed('scheduledAllocations.@each.allocatedResources')
  get totalReservedMemory() {
    const mibs = this.scheduledAllocations
      .mapBy('allocatedResources.memory')
      .reduce(sumAggregator, 0);
    return mibs * 1024 * 1024;
  }

  @computed('scheduledAllocations.@each.allocatedResources')
  get totalReservedCPU() {
    return this.scheduledAllocations
      .mapBy('allocatedResources.cpu')
      .reduce(sumAggregator, 0);
  }

  @computed('totalMemory', 'totalReservedMemory')
  get reservedMemoryPercent() {
    if (!this.totalReservedMemory || !this.totalMemory) return 0;
    return this.totalReservedMemory / this.totalMemory;
  }

  @computed('totalCPU', 'totalReservedCPU')
  get reservedCPUPercent() {
    if (!this.totalReservedCPU || !this.totalCPU) return 0;
    return this.totalReservedCPU / this.totalCPU;
  }

  @computed(
    'activeAllocation.taskGroupName',
    'scheduledAllocations.@each.{job,taskGroupName}'
  )
  get siblingAllocations() {
    if (!this.activeAllocation) return [];
    const taskGroup = this.activeAllocation.taskGroupName;
    const jobId = this.activeAllocation.belongsTo('job').id();

    return this.scheduledAllocations.filter((allocation) => {
      return (
        allocation.taskGroupName === taskGroup &&
        allocation.belongsTo('job').id() === jobId
      );
    });
  }

  @computed('activeNode')
  get nodeUtilization() {
    const node = this.activeNode;
    const [formattedMemory, memoryUnits] = reduceBytes(
      node.memory * 1024 * 1024
    );
    const totalReservedMemory = node.allocations
      .mapBy('memory')
      .reduce(sumAggregator, 0);
    const totalReservedCPU = node.allocations
      .mapBy('cpu')
      .reduce(sumAggregator, 0);

    return {
      totalMemoryFormatted: formattedMemory.toFixed(2),
      totalMemoryUnits: memoryUnits,

      totalMemory: node.memory * 1024 * 1024,
      totalReservedMemory: totalReservedMemory * 1024 * 1024,
      reservedMemoryPercent: totalReservedMemory / node.memory,

      totalCPU: node.cpu,
      totalReservedCPU,
      reservedCPUPercent: totalReservedCPU / node.cpu,
    };
  }

  @computed('siblingAllocations.@each.node')
  get uniqueActiveAllocationNodes() {
    return this.siblingAllocations.mapBy('node.id').uniq();
  }

  @action
  async setAllocation(allocation) {
    if (allocation) {
      await allocation.reload();
      await allocation.job.reload();
    }
    this.set('activeAllocation', allocation);
  }

  @action
  setNode(node) {
    this.set('activeNode', node);
  }

  @action
  handleTopoVizDataError(errors) {
    const pre09NodesError = errors.findBy('type', 'filtered-nodes');
    if (pre09NodesError) {
      this.pre09Nodes = pre09NodesError.context;
    }
  }
}
