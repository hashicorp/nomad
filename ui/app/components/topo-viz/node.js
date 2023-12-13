/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { guidFor } from '@ember/object/internals';

export default class TopoVizNode extends Component {
  @tracked data = { cpu: [], memory: [] };
  @tracked dimensionsWidth = 0;
  @tracked padding = 5;
  @tracked activeAllocation = null;

  get height() {
    return this.args.heightScale
      ? this.args.heightScale(this.args.node.memory)
      : 15;
  }

  get labelHeight() {
    return this.height / 2;
  }

  get paddingLeft() {
    const labelWidth = 20;
    return this.padding + labelWidth;
  }

  // Since strokes are placed centered on the perimeter of fills, The width of the stroke needs to be removed from
  // the height of the fill to match unstroked height and avoid clipping.
  get selectedHeight() {
    return this.height - 1;
  }

  // Since strokes are placed centered on the perimeter of fills, half the width of the stroke needs to be added to
  // the yOffset to match heights with unstroked shapes.
  get selectedYOffset() {
    return this.height + 2.5;
  }

  get yOffset() {
    return this.height + 2;
  }

  get maskHeight() {
    return this.height + this.yOffset;
  }

  get totalHeight() {
    return this.maskHeight + this.padding * 2;
  }

  get maskId() {
    return `topo-viz-node-mask-${guidFor(this)}`;
  }

  get count() {
    return this.allocations.length;
  }

  get allocations() {
    // Sort by the delta between memory and cpu percent. This creates the least amount of
    // drift between the positional alignment of an alloc's cpu and memory representations.
    return this.args.node.allocations
      .filterBy('allocation.isScheduled')
      .sort((a, b) => {
        const deltaA = Math.abs(a.memoryPercent - a.cpuPercent);
        const deltaB = Math.abs(b.memoryPercent - b.cpuPercent);
        return deltaA - deltaB;
      });
  }

  @action
  async reloadNode() {
    if (this.args.node.isPartial) {
      await this.args.node.reload();
      this.data = this.computeData(this.dimensionsWidth);
    }
  }

  @action
  render(svg) {
    this.dimensionsWidth = svg.clientWidth - this.padding - this.paddingLeft;
    this.data = this.computeData(this.dimensionsWidth);
  }

  @action
  updateRender(svg) {
    // Only update all data when the width changes
    const newWidth = svg.clientWidth - this.padding - this.paddingLeft;
    if (newWidth !== this.dimensionsWidth) {
      this.dimensionsWidth = newWidth;
      this.data = this.computeData(this.dimensionsWidth);
    }
  }

  @action
  highlightAllocation(allocation, { target }) {
    this.activeAllocation = allocation;
    this.args.onAllocationFocus &&
      this.args.onAllocationFocus(allocation, target);
  }

  @action
  allocationBlur() {
    this.args.onAllocationBlur && this.args.onAllocationBlur();
  }

  @action
  clearHighlight() {
    this.activeAllocation = null;
  }

  @action
  selectNode() {
    if (this.args.isDense && this.args.onNodeSelect) {
      this.args.onNodeSelect(this.args.node.isSelected ? null : this.args.node);
    }
  }

  @action
  selectAllocation(allocation) {
    if (this.args.onAllocationSelect) this.args.onAllocationSelect(allocation);
  }

  containsActiveTaskGroup() {
    return this.args.node.allocations.some(
      (allocation) =>
        allocation.taskGroupName === this.args.activeTaskGroup &&
        allocation.belongsTo('job').id() === this.args.activeJobId
    );
  }

  computeData(width) {
    const allocations = this.allocations;
    let cpuOffset = 0;
    let memoryOffset = 0;

    const cpu = [];
    const memory = [];
    for (const allocation of allocations) {
      const { cpuPercent, memoryPercent, isSelected } = allocation;
      const isFirst = allocation === allocations[0];

      let cpuWidth = cpuPercent * width - 1;
      let memoryWidth = memoryPercent * width - 1;
      if (isFirst) {
        cpuWidth += 0.5;
        memoryWidth += 0.5;
      }
      if (isSelected) {
        cpuWidth--;
        memoryWidth--;
      }

      cpu.push({
        allocation,
        offset: cpuOffset * 100,
        percent: cpuPercent * 100,
        width: Math.max(cpuWidth, 0),
        x: cpuOffset * width + (isFirst ? 0 : 0.5) + (isSelected ? 0.5 : 0),
        className: allocation.allocation.clientStatus,
      });
      memory.push({
        allocation,
        offset: memoryOffset * 100,
        percent: memoryPercent * 100,
        width: Math.max(memoryWidth, 0),
        x: memoryOffset * width + (isFirst ? 0 : 0.5) + (isSelected ? 0.5 : 0),
        className: allocation.allocation.clientStatus,
      });

      cpuOffset += cpuPercent;
      memoryOffset += memoryPercent;
    }

    const cpuRemainder = {
      x: cpuOffset * width + 0.5,
      width: Math.max(width - cpuOffset * width, 0),
    };
    const memoryRemainder = {
      x: memoryOffset * width + 0.5,
      width: Math.max(width - memoryOffset * width, 0),
    };

    return {
      cpu,
      memory,
      cpuRemainder,
      memoryRemainder,
      cpuLabel: { x: -this.paddingLeft / 2, y: this.height / 2 + this.yOffset },
      memoryLabel: { x: -this.paddingLeft / 2, y: this.height / 2 },
    };
  }
}
