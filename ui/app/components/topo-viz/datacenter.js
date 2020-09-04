import RSVP from 'rsvp';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class TopoVizNode extends Component {
  @tracked scheduledAllocations = [];
  @tracked aggregatedNodeResources = { cpu: 0, memory: 0 };
  @tracked isLoaded = false;

  get aggregateNodeResources() {
    return this.args.nodes.mapBy('resources');
  }

  get aggregatedAllocationResources() {
    return this.scheduledAllocations.mapBy('resources').reduce(
      (totals, allocation) => {
        totals.cpu += allocation.cpu;
        totals.memory += allocation.memory;
        return totals;
      },
      { cpu: 0, memory: 0 }
    );
  }

  @action
  async loadAllocations() {
    await RSVP.all(this.args.nodes.mapBy('allocations'));

    this.scheduledAllocations = this.args.nodes.reduce(
      (all, node) => all.concat(node.allocations.filterBy('isScheduled')),
      []
    );

    this.aggregatedNodeResources = this.args.nodes.mapBy('resources').reduce(
      (totals, node) => {
        totals.cpu += node.cpu;
        totals.memory += node.memory;
        return totals;
      },
      { cpu: 0, memory: 0 }
    );

    this.isLoaded = true;
  }
}
