import Controller from '@ember/controller';
import { computed, action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import classic from 'ember-classic-decorator';
import { reduceBytes, reduceHertz } from 'nomad-ui/utils/units';

const sumAggregator = (sum, value) => sum + (value || 0);
const formatter = new Intl.NumberFormat(window.navigator.locale || 'en', {
  maximumFractionDigits: 2,
});

@classic
export default class TopologyControllers extends Controller {
  @service userSettings;

  @alias('userSettings.showTopoVizPollingNotice') showPollingNotice;

  @tracked filteredNodes = null;

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
    const filteredNodesError = errors.findBy('type', 'filtered-nodes');
    if (filteredNodesError) {
      this.filteredNodes = filteredNodesError.context;
    }
  }
}
