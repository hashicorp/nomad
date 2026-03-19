/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import formatBytes from 'nomad-ui/helpers/format-bytes';
import formatHertz from 'nomad-ui/helpers/format-hertz';
import FlexMasonry from 'nomad-ui/components/flex-masonry';
import TopoVizNode from 'nomad-ui/components/topo-viz/node';

export default class TopoVizDatacenter extends Component {
  get scheduledAllocations() {
    return this.args.datacenter.nodes.reduce(
      (all, node) =>
        all.concat(node.allocations.filterBy('allocation.isScheduled')),
      [],
    );
  }

  get aggregatedAllocationResources() {
    return this.scheduledAllocations.reduce(
      (totals, allocation) => {
        totals.cpu += allocation.cpu;
        totals.memory += allocation.memory;
        return totals;
      },
      { cpu: 0, memory: 0 },
    );
  }

  get aggregatedNodeResources() {
    return this.args.datacenter.nodes.reduce(
      (totals, node) => {
        totals.cpu += node.cpu;
        totals.memory += node.memory;
        return totals;
      },
      { cpu: 0, memory: 0 },
    );
  }

  <template>
    <div
      data-test-topo-viz-datacenter
      class="boxed-section topo-viz-datacenter"
    >
      <div
        data-test-topo-viz-datacenter-label
        class="boxed-section-head is-hollow"
      >
        <span class="tooltip" aria-label="Datacenter"><strong
          >{{@datacenter.name}}</strong></span>
        <span
          class="bumper-left tooltip"
          aria-label="Number of Allocations"
        >{{this.scheduledAllocations.length}} Allocs</span>
        <span
          class="bumper-left tooltip"
          aria-label="Number of Nodes"
        >{{@datacenter.nodes.length}} Nodes</span>
        <span class="bumper-left is-faded">
          <span class="tooltip" aria-label="Memory Allocated">{{formatBytes
              this.aggregatedAllocationResources.memory
              start="MiB"
            }}</span>
          /
          <span class="tooltip" aria-label="Total Memory">{{formatBytes
              this.aggregatedNodeResources.memory
              start="MiB"
            }},</span>
          <span class="tooltip" aria-label="CPU Allocated">{{formatHertz
              this.aggregatedAllocationResources.cpu
            }}</span>
          /
          <span class="tooltip" aria-label="Total CPU">{{formatHertz
              this.aggregatedNodeResources.cpu
            }}</span>
        </span>
      </div>
      <div class="boxed-section-body">
        {{#let (if @isSingleColumn 1 2) as |layoutColumns|}}
          <FlexMasonry
            @columns={{layoutColumns}}
            @items={{@datacenter.nodes}}
            as |node|
          >
            <TopoVizNode
              @node={{node}}
              @layoutColumns={{layoutColumns}}
              @isDense={{@isDense}}
              @heightScale={{@heightScale}}
              @onAllocationSelect={{@onAllocationSelect}}
              @onAllocationFocus={{@onAllocationFocus}}
              @onAllocationBlur={{@onAllocationBlur}}
              @onNodeSelect={{@onNodeSelect}}
            />
          </FlexMasonry>
        {{/let}}
      </div>
    </div>
  </template>
}
