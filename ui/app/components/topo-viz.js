/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action, set } from '@ember/object';
import { inject as service } from '@ember/service';
import { next } from '@ember/runloop';
import { scaleLinear } from 'd3-scale';
import { extent, deviation, mean } from 'd3-array';
import { line, curveBasis } from 'd3-shape';
import styleStringProperty from '../utils/properties/style-string';

export default class TopoViz extends Component {
  @service system;

  @tracked element = null;
  @tracked topology = { datacenters: [] };

  @tracked activeNode = null;
  @tracked activeAllocation = null;
  @tracked activeEdges = [];
  @tracked edgeOffset = { x: 0, y: 0 };
  @tracked viewportColumns = 2;

  @tracked highlightAllocation = null;
  @tracked tooltipProps = {};

  @styleStringProperty('tooltipProps') tooltipStyle;

  get isSingleColumn() {
    if (this.topology.datacenters.length <= 1 || this.viewportColumns === 1)
      return true;

    // Compute the coefficient of variance to determine if it would be
    // better to stack datacenters or place them in columns
    const nodeCounts = this.topology.datacenters.map(
      (datacenter) => datacenter.nodes.length
    );
    const variationCoefficient = deviation(nodeCounts) / mean(nodeCounts);

    // The point at which the varation is too extreme for a two column layout
    const threshold = 0.5;
    if (variationCoefficient > threshold) return true;
    return false;
  }

  get datacenterIsSingleColumn() {
    // If there are enough nodes, use two columns of nodes within
    // a single column layout of datacenters to increase density.
    if (this.viewportColumns === 1) return true;
    return (
      !this.isSingleColumn ||
      (this.isSingleColumn && this.args.nodes.length <= 20)
    );
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
      isSelected: false,
    };
  }

  dataForAllocation(allocation, node) {
    const jobId = allocation.belongsTo('job').id();
    return {
      allocation,
      node,
      jobId,
      groupKey: JSON.stringify([jobId, allocation.taskGroupName]),
      memory: allocation.allocatedResources.memory,
      cpu: allocation.allocatedResources.cpu,
      memoryPercent: allocation.allocatedResources.memory / node.memory,
      cpuPercent: allocation.allocatedResources.cpu / node.cpu,
      isSelected: false,
    };
  }

  @action
  buildTopology() {
    const nodes = this.args.nodes;
    const allocations = this.args.allocations;

    // Nodes may not have a resources property due to having an old Nomad agent version.
    const badNodes = [];

    // Wrap nodes in a topo viz specific data structure and build an index to speed up allocation assignment
    const nodeContainers = [];
    const nodeIndex = {};
    nodes.forEach((node) => {
      if (!node.resources) {
        badNodes.push(node);
        return;
      }

      const container = this.dataForNode(node);
      nodeContainers.push(container);
      nodeIndex[node.id] = container;
    });

    // Wrap allocations in a topo viz specific data structure, assign allocations to nodes, and build an allocation
    // index keyed off of job and task group
    const allocationIndex = {};
    allocations.forEach((allocation) => {
      const nodeId = allocation.belongsTo('node').id();
      const nodeContainer = nodeIndex[nodeId];

      // Ignore orphaned allocations and allocations on nodes with an old Nomad agent version.
      if (!nodeContainer) return;

      const allocationContainer = this.dataForAllocation(
        allocation,
        nodeContainer
      );
      nodeContainer.allocations.push(allocationContainer);

      const key = allocationContainer.groupKey;
      if (!allocationIndex[key]) allocationIndex[key] = [];
      allocationIndex[key].push(allocationContainer);
    });

    // Group nodes into datacenters
    const datacentersMap = nodeContainers.reduce(
      (datacenters, nodeContainer) => {
        if (!datacenters[nodeContainer.datacenter])
          datacenters[nodeContainer.datacenter] = [];
        datacenters[nodeContainer.datacenter].push(nodeContainer);
        return datacenters;
      },
      {}
    );

    // Turn hash of datacenters into a sorted array
    const datacenters = Object.keys(datacentersMap)
      .map((key) => ({ name: key, nodes: datacentersMap[key] }))
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

    if (badNodes.length && this.args.onDataError) {
      this.args.onDataError([
        {
          type: 'filtered-nodes',
          context: badNodes,
        },
      ]);
    }
  }

  @action
  captureElement(element) {
    this.element = element;
    this.determineViewportColumns();
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

  @action showTooltip(allocation, element) {
    const bbox = element.getBoundingClientRect();
    this.highlightAllocation = allocation;
    this.tooltipProps = {
      left: window.scrollX + bbox.left + bbox.width / 2,
      top: window.scrollY + bbox.top,
    };
  }

  @action hideTooltip() {
    this.highlightAllocation = null;
  }

  @action
  associateAllocations(allocation) {
    if (this.activeAllocation === allocation) {
      this.activeAllocation = null;
      this.activeEdges = [];

      if (this.topology.selectedKey) {
        const selectedAllocations =
          this.topology.allocationIndex[this.topology.selectedKey];
        if (selectedAllocations) {
          selectedAllocations.forEach((allocation) => {
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
      const selectedAllocations =
        this.topology.allocationIndex[this.topology.selectedKey];
      if (selectedAllocations) {
        selectedAllocations.forEach((allocation) => {
          set(allocation, 'isSelected', false);
        });
      }

      set(this.topology, 'selectedKey', allocation.groupKey);
      const newAllocations =
        this.topology.allocationIndex[this.topology.selectedKey];
      if (newAllocations) {
        newAllocations.forEach((allocation) => {
          set(allocation, 'isSelected', true);
        });
      }

      // Only show the lines if the selected allocations are sparse (low count relative to the client count or low count generally).
      if (
        newAllocations.length < 10 ||
        newAllocations.length < this.args.nodes.length * 0.75
      ) {
        this.computedActiveEdges();
      } else {
        this.activeEdges = [];
      }
    }
    if (this.args.onAllocationSelect)
      this.args.onAllocationSelect(
        this.activeAllocation && this.activeAllocation.allocation
      );
    if (this.args.onNodeSelect) this.args.onNodeSelect(this.activeNode);
  }

  @action
  determineViewportColumns() {
    this.viewportColumns = this.element.clientWidth < 900 ? 1 : 2;
  }

  @action
  resizeEdges() {
    if (this.activeEdges.length > 0) {
      this.computedActiveEdges();
    }
  }

  @action
  computedActiveEdges() {
    // Wait a render cycle
    next(() => {
      const path = line().curve(curveBasis);
      // 1. Get the active element
      const allocation = this.activeAllocation.allocation;
      const activeEl = this.element.querySelector(
        `[data-allocation-id="${allocation.id}"]`
      );
      const activePoint = centerOfBBox(activeEl.getBoundingClientRect());

      // 2. Collect the mem and cpu pairs for all selected allocs
      const selectedMem = Array.from(
        this.element.querySelectorAll('.memory .bar.is-selected')
      );
      const selectedPairs = selectedMem.map((mem) => {
        const id = mem.closest('[data-allocation-id]').dataset.allocationId;
        const cpu = mem
          .closest('.topo-viz-node')
          .querySelector(`.cpu .bar[data-allocation-id="${id}"]`);
        return [mem, cpu];
      });
      const selectedPoints = selectedPairs.map((pair) => {
        return pair.map((el) => centerOfBBox(el.getBoundingClientRect()));
      });

      // 3. For each pair, compute the midpoint of the truncated triangle of points [Mem, Cpu, Active]
      selectedPoints.forEach((points) => {
        const d1 = pointBetween(points[0], activePoint, 100, 0.5);
        const d2 = pointBetween(points[1], activePoint, 100, 0.5);
        points.push(midpoint(d1, d2));
      });

      // 4. Generate curves for each active->mem and active->cpu pair going through the bisector
      const curves = [];
      // Steps are used to restrict the range of curves. The closer control points are placed, the less
      // curvature the curve generator will generate.
      const stepsMain = [0, 0.8, 1.0];
      // The second prong the fork does not need to retrace the entire path from the activePoint
      const stepsSecondary = [0.8, 1.0];
      selectedPoints.forEach((points) => {
        curves.push(
          curveFromPoints(
            ...pointsAlongPath(activePoint, points[2], stepsMain),
            points[0]
          ),
          curveFromPoints(
            ...pointsAlongPath(activePoint, points[2], stepsSecondary),
            points[1]
          )
        );
      });

      this.activeEdges = curves.map((curve) => path(curve));
      this.edgeOffset = { x: window.scrollX, y: window.scrollY };
    });
  }
}

function centerOfBBox(bbox) {
  return {
    x: bbox.x + bbox.width / 2,
    y: bbox.y + bbox.height / 2,
  };
}

function dist(p1, p2) {
  return Math.sqrt(Math.pow(p2.x - p1.x, 2) + Math.pow(p2.y - p1.y, 2));
}

// Return the point between p1 and p2 at len (or pct if len > dist(p1, p2))
function pointBetween(p1, p2, len, pct) {
  const d = dist(p1, p2);
  const ratio = d < len ? pct : len / d;
  return pointBetweenPct(p1, p2, ratio);
}

function pointBetweenPct(p1, p2, pct) {
  const dx = p2.x - p1.x;
  const dy = p2.y - p1.y;
  return { x: p1.x + dx * pct, y: p1.y + dy * pct };
}

function pointsAlongPath(p1, p2, pcts) {
  return pcts.map((pct) => pointBetweenPct(p1, p2, pct));
}

function midpoint(p1, p2) {
  return pointBetweenPct(p1, p2, 0.5);
}

function curveFromPoints(...points) {
  return points.map((p) => [p.x, p.y]);
}
