import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { run } from '@ember/runloop';
import { scaleLinear } from 'd3-scale';
import { extent } from 'd3-array';
import RSVP from 'rsvp';

export default class TopoViz extends Component {
  @tracked heightScale = null;
  @tracked isLoaded = false;
  @tracked element = null;

  @tracked activeAllocation = null;
  @tracked activeEdges = [];

  get activeTaskGroup() {
    return this.activeAllocation && this.activeAllocation.taskGroupName;
  }

  get activeJobId() {
    return this.activeAllocation && this.activeAllocation.belongsTo('job').id();
  }

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
      .range([15, 40])
      .domain(extent(this.args.nodes.map(node => node.resources.memory)));
    this.isLoaded = true;

    // schedule masonry
    run.schedule('afterRender', () => {
      this.masonry();
    });
  }

  @action
  masonry() {
    run.next(() => {
      const datacenterSections = this.element.querySelectorAll('.topo-viz-datacenter');
      const elementStyles = window.getComputedStyle(this.element);
      if (!elementStyles) return;

      const rowHeight = parseInt(elementStyles.getPropertyValue('grid-auto-rows')) || 0;
      const rowGap = parseInt(elementStyles.getPropertyValue('grid-row-gap')) || 0;

      if (!rowHeight) return;

      for (let dc of datacenterSections) {
        const contents = dc.querySelector('.masonry-container');
        const height = contents.getBoundingClientRect().height;
        const rowSpan = Math.ceil((height + rowGap) / (rowHeight + rowGap));
        dc.style.gridRowEnd = `span ${rowSpan}`;
      }
    });
  }

  @action
  captureElement(element) {
    this.element = element;
  }

  @action
  associateAllocations(allocation) {
    if (this.activeAllocation === allocation) {
      this.activeAllocation = null;
      this.activeEdges = [];
    } else {
      this.activeAllocation = allocation;
      this.computedActiveEdges();
    }
    if (this.args.onAllocationSelect) this.args.onAllocationSelect(this.activeAllocation);
  }

  @action
  computedActiveEdges() {
    // Wait a render cycle
    run.next(() => {
      const activeEl = this.element.querySelector(
        `[data-allocation-id="${this.activeAllocation.id}"]`
      );
      const selectedAllocations = this.element.querySelectorAll('.memory .bar.is-selected');
      const activeBBox = activeEl.getBoundingClientRect();

      const vLeft = window.visualViewport.pageLeft;
      const vTop = window.visualViewport.pageTop;

      const edges = [];
      for (let allocation of selectedAllocations) {
        if (allocation !== activeEl) {
          const bbox = allocation.getBoundingClientRect();
          edges.push({
            x1: activeBBox.x + activeBBox.width / 2 + vLeft,
            y1: activeBBox.y + activeBBox.height / 2 + vTop,
            x2: bbox.x + bbox.width / 2 + vLeft,
            y2: bbox.y + bbox.height / 2 + vTop,
          });
        }
      }

      this.activeEdges = edges;
    });
    // get element for active alloc
    // get element for all selected allocs
    // draw lines between centroid of each
  }
}
