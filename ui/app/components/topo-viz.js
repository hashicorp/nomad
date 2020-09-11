import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { scaleLinear } from 'd3-scale';
import { max } from 'd3-array';
import RSVP from 'rsvp';

export default class TopoViz extends Component {
  @tracked heightScale = null;
  @tracked isLoaded = false;

  @tracked activeTaskGroup = null;
  @tracked activeJobId = null;

  get datacenters() {
    const datacentersMap = this.args.nodes.reduce((datacenters, node) => {
      if (!datacenters[node.datacenter]) datacenters[node.datacenter] = [];
      datacenters[node.datacenter].push(node);
      return datacenters;
    }, {});

    return Object.keys(datacentersMap)
      .map(key => ({ name: key, nodes: datacentersMap[key] }))
      .sortBy('name');
  }

  @action
  async loadNodes() {
    await RSVP.all(this.args.nodes.map(node => node.reload()));

    // TODO: Make the range dynamic based on the extent of the domain
    this.heightScale = scaleLinear()
      .range([15, 30])
      .domain([0, max(this.args.nodes.map(node => node.resources.memory))]);
    this.isLoaded = true;
  }

  @action
  associateAllocations(allocation) {
    const taskGroup = allocation.taskGroupName;
    const jobId = allocation.belongsTo('job').id();
    if (this.activeTaskGroup === taskGroup && this.activeJobId === jobId) {
      this.activeTaskGroup = null;
      this.activeJobId = null;
      if (this.args.onAllocationSelect) this.args.onAllocationSelect(null);
    } else {
      this.activeTaskGroup = taskGroup;
      this.activeJobId = jobId;
      if (this.args.onAllocationSelect) this.args.onAllocationSelect(allocation);
    }
  }
}
