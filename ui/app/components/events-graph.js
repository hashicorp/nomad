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

  get data() {
    // console.count('data');
    return this.args.data.map((d, i) => {
      // d.x = 1;
      d.startY = 0;
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
        .scaleExtent([1, 20])
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
    this.restartSimulation();
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
    // console.log('ticked', simulation.alpha());
    // let nodes = simulation.nodes();
    // this.nodes = nodes.map((node) => {
    //   set(node, 'offset', 150);
    //   return node;
    // });

    this.graph
      .selectAll('circle')
      .data(this.data)
      .each((d, i, g) => {
        if (i === 10) {
          // console.log("eye",d,d.y, g[i].getAttribute('cy'));
        }
        set(simulation.nodes()[i], 'yMod', d.y);
        // set(simulation.nodes()[i], 'xMod', d.x);
      });
  }

  @tracked nodes;

  @action restartSimulation() {
    if (!this.simulation) {
      this.forceDirectedGraph();
    } else {
      this.simulation.nodes(this.data);
      this.simulation.alpha(0.01);
      console.log('about to restart the', this.simulation);
      this.simulation.restart();
    }
  }

  @action
  custom_collider(alpha) {
    // console.log('collider',alpha, this.yBand);
    let radius = this.graph.select('circle').node()
      ? this.graph.select('circle').node().getAttribute('r')
      : 0;
    // console.log('radius', radius);
    this.simulation.nodes().forEach((node, i, g) => {
      // console.log('gee', node.Index, g[5]);
      const peers = g.filter((d) => d.Index === node.Index);
      // console.log("peers for", node.Index, peers);
      if (peers.length > 1 && peers.indexOf(node) > 0) {
        node.y = peers.indexOf(node) * (i % 2 ? -radius : +radius);
        set(node, 'y', node.y);
      } else {
        set(node, 'y', 0);
      }
      // node.y = radius * i;
    });
    // let windowBorders = 20;
    // let leftWindowBorders = 20;
    // if (width > 500) {
    //   windowBorders = 50;
    //   leftWindowBorders = 100;
    // }
    // for (var i = 0, n = this.simulation.nodes().length; i < n; ++i) {
    //   let curr_node = traits[i];
    //   curr_node.x = Math.max(
    //     curr_node.radius + leftWindowBorders,
    //     Math.min(width - curr_node.radius - windowBorders, curr_node.x)
    //   );
    //   curr_node.y = Math.max(
    //     curr_node.radius + windowBorders,
    //     Math.min(height - curr_node.radius - windowBorders, curr_node.y)
    //   );
    // }
  }

  @action
  forceDirectedGraph() {
    console.log('fDG', this.data, this.graph);

    const simulation = d3
      .forceSimulation(this.data)
      .alpha(0.01)
      // .alphaDecay(0.05)
      // .force('charge', d3.forceManyBody().strength(1))
      // .force("center", d3.forceCenter().strength(1))
      // .force('xPos', d3.forceX((d) => d.x).strength(1))
      .force('yPos', d3.forceY((d) => d.startY).strength(1))
      .force('custom_collider', this.custom_collider);
    // .force('collide', d3.forceCollide((d) => 5).strength(1));
    // .force('collide', d3.forceCollide((d) => this.zoomTransform.k * 5).strength(10))

    this.simulation = simulation;

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

  @action highlightEvent(event) {
    console.log('highlightEvent', event);
    set(event, 'highlight', true);
    this.nodes
      .reject((d) => d.Key === event.Key)
      .forEach((d) => {
        set(d, 'blur', true);
      });
    this.nodes
      .filter((d) => d.Key === event.Key)
      .forEach((d) => {
        set(d, 'highlight', true);
      });
  }
  @action blurEvent(event) {
    console.log('blurEvent', event);
    set(event, 'highlight', false);
    this.nodes.forEach((d) => {
      set(d, 'highlight', false);
      set(d, 'blur', false);
    });
  }
}
