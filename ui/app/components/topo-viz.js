import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action, set } from '@ember/object';
import { run } from '@ember/runloop';
import { task } from 'ember-concurrency';
import { scaleLinear } from 'd3-scale';
import { extent, deviation, mean } from 'd3-array';
import RSVP from 'rsvp';

export default class TopoViz extends Component {
  @tracked heightScale = null;
  @tracked isLoaded = false;
  @tracked element = null;
  @tracked topology = { datacenters: [] };

  @tracked activeNode = null;
  @tracked activeAllocation = null;
  @tracked activeEdges = [];

  get isSingleColumn() {
    if (this.topology.datacenters.length <= 1) return true;

    // Compute the coefficient of variance to determine if it would be
    // better to stack datacenters or place them in columns
    const nodeCounts = this.topology.datacenters.map(datacenter => datacenter.nodes.length);
    const variationCoefficient = deviation(nodeCounts) / mean(nodeCounts);

    // The point at which the varation is too extreme for a two column layout
    const threshold = 0.5;
    if (variationCoefficient > threshold) return true;
    return false;
  }

  get datacenterIsSingleColumn() {
    // If there are enough nodes, use two columns of nodes within
    // a single column layout of datacenteres to increase density.
    return !this.isSingleColumn || (this.isSingleColumn && this.args.nodes.length <= 20);
  }

  // Once a cluster is large enough, the exact details of a node are
  // typically irrelevant and a waste of space.
  get isDense() {
    return this.args.nodes.length > 50;
  }

  dataForNode(node) {
    return {
      node,
      datacenter: node.datacenter,
      memory: node.resources.memory,
      cpu: node.resources.cpu,
      allocations: [],
    };
  }

  dataForAllocation(allocation, node) {
    const jobId = allocation.belongsTo('job').id();
    return {
      allocation,
      node,
      jobId,
      groupKey: JSON.stringify([jobId, allocation.taskGroupName]),
      memory: allocation.resources.memory,
      cpu: allocation.resources.cpu,
      memoryPercent: allocation.resources.memory / node.memory,
      cpuPercent: allocation.resources.cpu / node.cpu,
      isSelected: false,
    };
  }

  @task(function*() {
    const nodes = this.args.nodes;
    const allocations = this.args.allocations;

    // Nodes are probably partials and we'll need the resources on them
    // TODO: this is an API update waiting to happen.
    yield RSVP.all(nodes.map(node => (node.isPartial ? node.reload() : RSVP.resolve(node))));

    // Wrap nodes in a topo viz specific data structure and build an index to speed up allocation assignment
    const nodeContainers = [];
    const nodeIndex = {};
    nodes.forEach(node => {
      const container = this.dataForNode(node);
      nodeContainers.push(container);
      nodeIndex[node.id] = container;
    });

    // Wrap allocations in a topo viz specific data structure, assign allocations to nodes, and build an allocation
    // index keyed off of job and task group
    const allocationIndex = {};
    allocations.forEach(allocation => {
      const nodeId = allocation.belongsTo('node').id();
      const nodeContainer = nodeIndex[nodeId];
      if (!nodeContainer)
        throw new Error(`Node ${nodeId} for alloc ${allocation.id} not in index???`);

      const allocationContainer = this.dataForAllocation(allocation, nodeContainer);
      nodeContainer.allocations.push(allocationContainer);

      const key = allocationContainer.groupKey;
      if (!allocationIndex[key]) allocationIndex[key] = [];
      allocationIndex[key].push(allocationContainer);
    });

    // Group nodes into datacenters
    const datacentersMap = nodeContainers.reduce((datacenters, nodeContainer) => {
      if (!datacenters[nodeContainer.datacenter]) datacenters[nodeContainer.datacenter] = [];
      datacenters[nodeContainer.datacenter].push(nodeContainer);
      return datacenters;
    }, {});

    // Turn hash of datacenters into a sorted array
    const datacenters = Object.keys(datacentersMap)
      .map(key => ({ name: key, nodes: datacentersMap[key] }))
      .sortBy('name');

    const topology = {
      datacenters,
      allocationIndex,
      selectedKey: null,
      heightScale: scaleLinear()
        .range([15, 40])
        .domain(extent(nodeContainers.mapBy('memory'))),
    };
    this.topology = topology;
  })
  buildTopology;

  @action
  async loadNodes() {
    await RSVP.all(this.args.nodes.map(node => node.reload()));

    // TODO: Make the range dynamic based on the extent of the domain
    this.heightScale = scaleLinear()
      .range([15, 40])
      .domain(extent(this.args.nodes.map(node => node.resources.memory)));
    this.isLoaded = true;
  }

  @action
  captureElement(element) {
    this.element = element;
  }

  @action
  showNodeDetails(node) {
    if (this.activeNode) {
      set(this.activeNode, 'isSelected', false);
    }

    this.activeNode = this.activeNode === node ? null : node;

    if (this.activeNode) {
      set(this.activeNode, 'isSelected', true);
    }

    if (this.args.onNodeSelect) this.args.onNodeSelect(this.activeNode);
  }

  @action
  associateAllocations(allocation) {
    if (this.activeAllocation === allocation) {
      this.activeAllocation = null;
      this.activeEdges = [];

      if (this.topology.selectedKey) {
        const selectedAllocations = this.topology.allocationIndex[this.topology.selectedKey];
        if (selectedAllocations) {
          selectedAllocations.forEach(allocation => {
            set(allocation, 'isSelected', false);
          });
        }
        set(this.topology, 'selectedKey', null);
      }
    } else {
      if (this.activeNode) {
        set(this.activeNode, 'isSelected', false);
      }
      this.activeNode = null;
      this.activeAllocation = allocation;
      const selectedAllocations = this.topology.allocationIndex[this.topology.selectedKey];
      if (selectedAllocations) {
        selectedAllocations.forEach(allocation => {
          set(allocation, 'isSelected', false);
        });
      }

      set(this.topology, 'selectedKey', allocation.groupKey);
      const newAllocations = this.topology.allocationIndex[this.topology.selectedKey];
      if (newAllocations) {
        newAllocations.forEach(allocation => {
          set(allocation, 'isSelected', true);
        });
      }

      this.computedActiveEdges();
    }
    if (this.args.onAllocationSelect)
      this.args.onAllocationSelect(this.activeAllocation && this.activeAllocation.allocation);
    if (this.args.onNodeSelect) this.args.onNodeSelect(this.activeNode);
  }

  @action
  computedActiveEdges() {
    // Wait a render cycle
    run.next(() => {
      const activeEl = this.element.querySelector(
        `[data-allocation-id="${this.activeAllocation.allocation.id}"]`
      );
      const selectedAllocations = this.element.querySelectorAll('.memory .bar.is-selected');
      const activeBBox = activeEl.getBoundingClientRect();

      const vLeft = window.visualViewport.pageLeft;
      const vTop = window.visualViewport.pageTop;

      // Lines to the memory rect of each selected allocation
      const edges = [];
      for (let allocation of selectedAllocations) {
        if (allocation !== activeEl) {
          const bbox = allocation.getBoundingClientRect();
          edges.push({
            x1: activeBBox.x + activeBBox.width / 2 + vLeft,
            y1: activeBBox.y + activeBBox.height / 2 + vTop,
            x2: bbox.x + bbox.width / 2 + vLeft,
            y2: bbox.y + bbox.height / 2 + vTop,
          });
        }
      }

      // Lines from the memory rect to the cpu rect
      for (let allocation of selectedAllocations) {
        const id = allocation.closest('[data-allocation-id]').dataset.allocationId;
        const cpu = allocation
          .closest('.topo-viz-node')
          .querySelector(`.cpu .bar[data-allocation-id="${id}"]`);
        const bboxMem = allocation.getBoundingClientRect();
        const bboxCpu = cpu.getBoundingClientRect();
        edges.push({
          x1: bboxMem.x + bboxMem.width / 2 + vLeft,
          y1: bboxMem.y + bboxMem.height / 2 + vTop,
          x2: bboxCpu.x + bboxCpu.width / 2 + vLeft,
          y2: bboxCpu.y + bboxCpu.height / 2 + vTop,
        });
      }

      this.activeEdges = edges;
    });
  }
}
