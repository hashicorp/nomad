import Controller from '@ember/controller';
import { computed, action } from '@ember/object';
import classic from 'ember-classic-decorator';
import { reduceToLargestUnit } from 'nomad-ui/helpers/format-bytes';

@classic
export default class TopologyControllers extends Controller {
  get datacenters() {
    return Array.from(new Set(this.model.nodes.mapBy('datacenter'))).compact();
  }

  @computed('model.nodes.@each.resources')
  get totalMemory() {
    const mibs = this.model.nodes
      .mapBy('resources.memory')
      .reduce((sum, memory) => sum + (memory || 0), 0);
    return mibs * 1024 * 1024;
  }

  @computed('model.nodes.@each.resources')
  get totalCPU() {
    return this.model.nodes.mapBy('resources.cpu').reduce((sum, cpu) => sum + (cpu || 0), 0);
  }

  @computed('totalMemory')
  get totalMemoryFormatted() {
    return reduceToLargestUnit(this.totalMemory)[0].toFixed(2);
  }

  @computed('totalCPU')
  get totalMemoryUnits() {
    return reduceToLargestUnit(this.totalMemory)[1];
  }

  @computed('model.allocations.@each.resources')
  get totalReservedMemory() {
    const mibs = this.model.allocations
      .mapBy('resources.memory')
      .reduce((sum, memory) => sum + (memory || 0), 0);
    return mibs * 1024 * 1024;
  }

  @computed('model.allocations.@each.resources')
  get totalReservedCPU() {
    return this.model.allocations.mapBy('resources.cpu').reduce((sum, cpu) => sum + (cpu || 0), 0);
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

  @computed('activeAllocation', 'model.allocations.@each.{taskGroupName,job}')
  get siblingAllocations() {
    if (!this.activeAllocation) return [];
    const taskGroup = this.activeAllocation.taskGroupName;
    const jobId = this.activeAllocation.belongsTo('job').id();

    return this.model.allocations.filter(allocation => {
      return allocation.taskGroupName === taskGroup && allocation.belongsTo('job').id() === jobId;
    });
  }

  @computed('activeNode')
  get nodeUtilization() {
    const node = this.activeNode;
    const [formattedMemory, memoryUnits] = reduceToLargestUnit(node.memory * 1024 * 1024);
    const totalReservedMemory = node.allocations
      .mapBy('memory')
      .reduce((sum, memory) => sum + (memory || 0), 0);
    const totalReservedCPU = node.allocations
      .mapBy('cpu')
      .reduce((sum, cpu) => sum + (cpu || 0), 0);

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
    return this.siblingAllocations.mapBy('node').uniq();
  }

  @action
  setAllocation(allocation) {
    this.set('activeAllocation', allocation);
    if (allocation) {
      allocation.reload();
    }
  }

  @action
  setNode(node) {
    this.set('activeNode', node);
  }
}
