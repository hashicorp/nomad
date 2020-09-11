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
    return this.args.heightScale ? this.args.heightScale(this.args.node.resources.memory) : 15;
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
    return this.args.node.get('allocations.length');
  }

  get allocations() {
    // return this.args.node.allocations.filterBy('isScheduled').sortBy('resources.memory');
    const totalCPU = this.args.node.resources.cpu;
    const totalMemory = this.args.node.resources.memory;

    // Sort by the delta between memory and cpu percent. This creates the least amount of
    // drift between the positional alignment of an alloc's cpu and memory representations.
    return this.args.node.allocations.filterBy('isScheduled').sort((a, b) => {
      const deltaA = Math.abs(a.resources.memory / totalMemory - a.resources.cpu / totalCPU);
      const deltaB = Math.abs(b.resources.memory / totalMemory - b.resources.cpu / totalCPU);
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
  highlightAllocation(allocation) {
    this.activeAllocation = allocation;
  }

  @action
  clearHighlight() {
    this.activeAllocation = null;
  }

  @action
  selectAllocation(allocation) {
    if (this.args.onAllocationSelect) this.args.onAllocationSelect(allocation);
  }

  computeData(width) {
    // TODO: differentiate reserved and resources
    if (!this.args.node.resources) return;

    const totalCPU = this.args.node.resources.cpu;
    const totalMemory = this.args.node.resources.memory;
    let cpuOffset = 0;
    let memoryOffset = 0;

    const cpu = [];
    const memory = [];
    for (const allocation of this.allocations) {
      const cpuPercent = allocation.resources.cpu / totalCPU;
      const memoryPercent = allocation.resources.memory / totalMemory;
      const isFirst = allocation === this.allocations[0];
      const isSelected =
        allocation.taskGroupName === this.args.activeTaskGroup &&
        allocation.belongsTo('job').id() === this.args.activeJobId;

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
        isSelected,
        offset: cpuOffset * 100,
        percent: cpuPercent * 100,
        width: cpuWidth,
        x: cpuOffset * width + (isFirst ? 0 : 0.5) + (isSelected ? 0.5 : 0),
        className: allocation.clientStatus,
      });
      memory.push({
        allocation,
        isSelected,
        offset: memoryOffset * 100,
        percent: memoryPercent * 100,
        width: memoryWidth,
        x: memoryOffset * width + (isFirst ? 0 : 0.5) + (isSelected ? 0.5 : 0),
        className: allocation.clientStatus,
      });

      cpuOffset += cpuPercent;
      memoryOffset += memoryPercent;
    }

    const cpuRemainder = {
      x: cpuOffset * width + 0.5,
      width: width - cpuOffset * width,
    };
    const memoryRemainder = {
      x: memoryOffset * width + 0.5,
      width: width - memoryOffset * width,
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

// capture width on did insert element
// update width on window resize
// recompute data when width changes
