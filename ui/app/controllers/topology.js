import Controller from '@ember/controller';
import { computed } from '@ember/object';
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
    return this.totalReservedMemory / this.totalMemory;
  }

  @computed('totalCPU', 'totalReservedCPU')
  get reservedCPUPercent() {
    return this.totalReservedCPU / this.totalCPU;
  }
}
