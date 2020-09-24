import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class TopoVizDatacenter extends Component {
  @tracked scheduledAllocations = [];
  @tracked aggregatedNodeResources = { cpu: 0, memory: 0 };

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

  @action
  loadAllocations() {
    this.scheduledAllocations = this.args.datacenter.nodes.reduce(
      (all, node) => all.concat(node.allocations.filterBy('allocation.isScheduled')),
      []
    );

    this.aggregatedNodeResources = this.args.datacenter.nodes.reduce(
      (totals, node) => {
        totals.cpu += node.cpu;
        totals.memory += node.memory;
        return totals;
      },
      { cpu: 0, memory: 0 }
    );

    this.args.onLoad && this.args.onLoad();
  }
}
