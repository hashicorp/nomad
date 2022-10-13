// @ts-check

import Component from '@glimmer/component';
import d3 from 'd3';
import { action, set, computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { next } from '@ember/runloop';

export default class EventsGraphComponent extends Component {
  @tracked
  height = 400;

  @tracked
  width = 400;
  margin = { top: 0, right: 0, bottom: 30, left: 0 };

  @tracked
  graphElement = null;

  @tracked
  xAxisElement = null;

  @tracked
  graph = null;

  // // TODO: TEMP
  // @tracked
  // data = [
  //   { name: 'E', value: 0.12702 },
  //   { name: 'T', value: 0.09056 },
  //   { name: 'A', value: 0.08167 },
  //   { name: 'O', value: 0.07507 },
  //   { name: 'I', value: 0.06966 },
  //   { name: 'N', value: 0.06749 },
  //   { name: 'S', value: 0.06327 },
  //   { name: 'H', value: 0.06094 },
  //   { name: 'R', value: 0.05987 },
  //   { name: 'D', value: 0.04253 },
  //   { name: 'L', value: 0.04025 },
  //   { name: 'C', value: 0.02782 },
  //   { name: 'U', value: 0.02758 },
  //   { name: 'M', value: 0.02406 },
  //   { name: 'W', value: 0.0236 },
  //   { name: 'F', value: 0.02288 },
  //   { name: 'G', value: 0.02015 },
  //   { name: 'Y', value: 0.01974 },
  //   { name: 'P', value: 0.01929 },
  //   { name: 'B', value: 0.01492 },
  //   { name: 'V', value: 0.00978 },
  //   { name: 'K', value: 0.00772 },
  //   { name: 'J', value: 0.00153 },
  //   { name: 'X', value: 0.0015 },
  //   { name: 'Q', value: 0.00095 },
  //   { name: 'Z', value: 0.00074 },
  // ];

  get data() {
    console.count('data');
    return this.args.data.map((d) => {
      d.x = 5;
      return d;
    });
  }

  get xBand() {
    let scale = d3
      .scaleBand()
      .domain(this.args.data.map((d) => d.Index))
      .range([this.margin.left, this.width - this.margin.right])
      .padding(0.75);

    if (this.zoomTransform) {
      scale.range(
        [this.margin.left, this.width - this.margin.right].map((d) =>
          this.zoomTransform.applyX(d)
        )
      );
    }
    return scale;
  }

  get yBand() {
    return d3
      .scaleLinear()
      .domain([0, d3.max(this.args.data, (d) => d.value)])
      .nice()
      .range([this.height - this.margin.bottom, this.margin.top]);
  }

  @action
  initializeGraph(el) {
    this.graphElement = el;
    this.height = el.clientHeight;
    this.width = el.clientWidth;

    window.d3 = d3; // TODO: temp

    this.graph = d3.select(el).call(this.zoom);

    this.transformXAxis();
    // this.forceDirectedGraph();
  }

  @action runSimulation() {
    console.log('running simulation');
    this.forceDirectedGraph();
  }

  @action
  onResize() {
    this.width = this.graphElement.clientWidth;
    this.height = this.graphElement.clientHeight;
    this.transformXAxis();
  }

  @action
  initializeXAxis(el) {
    this.xAxisElement = el;
  }

  @action
  transformXAxis() {
    const axis = d3.select(this.xAxisElement);
    axis
      .attr('transform', `translate(0,${this.height - this.margin.bottom})`)
      .call(d3.axisBottom(this.xBand).tickSizeOuter(0));
  }

  @action
  zoom(svg) {
    const { margin, width, height } = this;
    const extent = [
      [margin.left, margin.top],
      [width - margin.right, height - margin.top],
    ];

    svg.call(
      d3
        .zoom()
        .scaleExtent([1, 8])
        .translateExtent(extent)
        .extent(extent)
        .on('zoom', this.refitDataToZoom)
    );
  }

  @tracked zoomTransform;

  @action
  refitDataToZoom(event) {
    this.zoomTransform = event.transform;
    this.graph.selectAll('.x-axis').call(this.transformXAxis);
  }

  //#region Force Layout
  nodeBuffer = 3;
  // get simulation() {
  //   const sim = d3
  //     .forceSimulation()
  //     .force('charge', d3.forceManyBody().strength(-1))
  //     .force(
  //       'xPos',
  //       d3
  //         .forceX((d) => d.x)
  //         .strength((d) => {
  //           return 1;
  //           // return d.saturation === 1 || d.comparisonSaturation === 1
  //           //   ? 1
  //           //   : 0.01; // try our best to centre the sat:1 / sat:0 items
  //         })
  //     )
  //     .force('yPos', d3.forceY((d) => d.y).strength(1.5))
  //     .force(
  //       'collide',
  //       d3
  //         .forceCollide((d) => {
  //           console.log('forceCollide', d);
  //           return 3;
  //           return d.radius * 1.0 + this.nodeBuffer
  //         })
  //         .strength(1.5)
  //         .iterations(20)
  //     )
  //     // .force('box_force', box_force)
  //     // .velocityDecay(0.9);
  //     // .alphaDecay(0.0003);
  //     .alphaDecay(0.15)
  //     .alphaMin(0.000001);

  //   return sim;
  // }

  // get simulation() {
  //   return d3
  //     .forceSimulation(this.data)
  //     .alphaDecay(0.15)
  //     .force('charge', d3.forceManyBody().strength(-1))
  //     .force('xPos', d3.forceX((d) => d.x).strength(1))
  //     .force('yPos', d3.forceY((d) => d.y).strength(1));
  //   // .force('collide', d3.forceCollide((d) => d.r * 1.2).strength(1));
  // }

  // simulation.nodes(traits).on('tick', ticked);

  @action
  ticked(simulation) {
    console.log('ticked', simulation.alpha());
    // let nodes = simulation.nodes();
    // this.nodes = nodes.map((node) => {
    //   set(node, 'offset', 150);
    //   return node;
    // });

    simulation.nodes().forEach((node) => {
      set(node, 'offset', 150 * simulation.alpha());
      // TODO: demo
      // set(node, 'y', this.height / 2 - this.xBand.bandwidth(node.Index) / 2 * (simulation.alpha() * 20));
      // set(node, 'y', this.height / 2 - this.xBand.bandwidth(node.Index) / 2);
      // set(node, 'x', this.xBand(this.args.data.Index));
      // set(node, 'r', this.xBand.bandwidth(node.Index));
    });
    // .attr('cx', (d) => d.x).attr('cy', (d) => d.y);
  }

  // get nodes() {
  //   console.log('nodes recompute', this.simulation.nodes());
  //   return this.simulation.nodes();
  // }

  @tracked nodes;

  @action
  forceDirectedGraph() {
    console.log('fDG', this.data, this.graph);

    // this.simulation.nodes()[5].y = 100;

    const simulation = d3
      .forceSimulation(this.data)
      .alphaDecay(0.15)
      .force('charge', d3.forceManyBody().strength(-1))
      .force('xPos', d3.forceX((d) => d.x).strength(1))
      .force('yPos', d3.forceY((d) => d.y).strength(1));

    // this.nodes = simulation.nodes();
    // this.nodes.forEach((node) => node.offset = -150);
    simulation.nodes(this.data).on('tick', () => {
      this.ticked(simulation);
      set(this, 'nodes', simulation.nodes());
    });

    // this.graph
    //   .selectAll('circle')
    //   .transition()
    //   .duration(1200)
    //   .delay((_d, i) => 2000 + i * 10)
    //   .attrTween('r', (d) => {
    //     console.log('tweenr', d);
    //     let i = d3.interpolate(0, d.radius);
    //     return function (t) {
    //       return (d.r = i(t));
    //     };
    //   });
  }

  // ticked() {
  //   // nodes.attr('transform', function(d) {
  //   //   let x = Math.max(d.radius, Math.min(width - d.radius, d.x));
  //   //   let y = Math.max(d.radius, Math.min(height - d.radius, d.y));
  //   //   return 'translate(' + x + ',' + y + ')';
  //   // });
  //   this.nodes.attr('transform', (d: any) => {
  //     if (d.x && d.y) {
  //       return `translate(${d.x},${d.y})`;
  //     }
  //     return '';
  //   });
  // }

  //#endregion Force Layout
}
