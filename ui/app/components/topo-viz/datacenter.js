/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class TopoVizDatacenter extends Component {
  get scheduledAllocations() {
    return this.args.datacenter.nodes.reduce(
      (all, node) =>
        all.concat(node.allocations.filterBy('allocation.isScheduled')),
      []
    );
  }

  get aggregatedAllocationResources() {
    return this.scheduledAllocations.reduce(
      (totals, allocation) => {
        totals.cpu += allocation.cpu;
        totals.memory += allocation.memory;
        return totals;
      },
      { cpu: 0, memory: 0 }
    );
  }

  get aggregatedNodeResources() {
    return this.args.datacenter.nodes.reduce(
      (totals, node) => {
        totals.cpu += node.cpu;
        totals.memory += node.memory;
        return totals;
      },
      { cpu: 0, memory: 0 }
    );
  }
}
