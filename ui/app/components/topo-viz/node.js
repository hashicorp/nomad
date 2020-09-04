import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { guidFor } from '@ember/object/internals';

export default class TopoVizNode extends Component {
  @tracked data = { cpu: [], memory: [] };
  // @tracked height = 15;
  @tracked dimensionsWidth = 0;
  @tracked padding = 5;
  @tracked activeAllocation = null;

  get height() {
    return this.args.heightScale ? this.args.heightScale(this.args.node.resources.memory) : 15;
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
    return this.args.node.allocations.filterBy('isScheduled').sortBy('resources.memory');
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
    this.dimensionsWidth = svg.clientWidth - this.padding * 2;
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

      let cpuWidth = cpuPercent * width - 1;
      let memoryWidth = memoryPercent * width - 1;
      if (isFirst) {
        cpuWidth += 0.5;
        memoryWidth += 0.5;
      }

      cpu.push({
        allocation,
        offset: cpuOffset * 100,
        percent: cpuPercent * 100,
        width: cpuWidth,
        x: cpuOffset * width + (isFirst ? 0 : 0.5),
        className: allocation.clientStatus,
      });
      memory.push({
        allocation,
        offset: memoryOffset * 100,
        percent: memoryPercent * 100,
        width: memoryWidth,
        x: memoryOffset * width + (isFirst ? 0 : 0.5),
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

    return { cpu, memory, cpuRemainder, memoryRemainder };
  }
}

// capture width on did insert element
// update width on window resize
// recompute data when width changes
