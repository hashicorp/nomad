// @ts-check

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import ArrayProxy from '@ember/array/proxy';
import { A } from '@ember/array';
// import from d3-force
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceRadial,
  forceCollide,
  forceX,
  forceY,
} from 'd3-force';
import forceBoundary from 'd3-force-boundary';
import { zoom as d3Zoom, select as d3Select } from 'd3';

import { tracked } from '@glimmer/tracking';

export default class ClusterVizComponent extends Component {
  @service cluster;

  // #region default values
  @tracked chargeStrength = -500; // TODO: deprecated
  @tracked boundaryBuffer = 20;
  // @tracked collisionBuffer = 10;
  get collisionBuffer() {
    return this.width / 30 || 16;
  }
  @tracked radialStrength = 2;
  @tracked centerX;
  @tracked centerY;
  @tracked width;
  @tracked height;

  get nodeRadius() {
    return this.collisionBuffer;
  }

  get nodeIconOffset() {
    return -this.nodeRadius / 2;
  }

  get allocationRadius() {
    return this.nodeRadius / 4; // TODO: make even smaller tbh
  }

  primaryRadiusMultiplier = 0.05;
  secondaryRadiusMultiplier = 0.2;
  tertiaryRadiusMultiplier = 0.4;

  get primaryRadius() {
    return Math.min(this.width, this.height) * this.primaryRadiusMultiplier;
  }
  get secondaryRadius() {
    return Math.min(this.width, this.height) * this.secondaryRadiusMultiplier;
  }
  get tertiaryRadius() {
    return Math.min(this.width, this.height) * this.tertiaryRadiusMultiplier;
  }
  // #endregion default values

  /**
   * on didInsert or didUpdate, we want to resize the svg element to fit our container bounds.
   */
  @action
  sizeCanvas(element) {
    console.log('sizeCanvas', element);
    // TODO: SUPER HACK
    this.centerX = element.clientWidth / 2;
    this.centerY = element.clientHeight / 2;
    this.width = element.clientWidth;
    this.height = element.clientHeight;
    setTimeout(() => {
      // set midpoints based on element size
      this.centerX = element.clientWidth / 2;
      this.centerY = element.clientHeight / 2;
      this.width = element.clientWidth;
      this.height = element.clientHeight;
      console.log(
        'setting to',
        this.centerX,
        this.centerY,
        this.width,
        this.height
      );
      if (this.simulation) {
        this.restartSimulation();
      }
    }, 1000);
    // Calculate and adjust sizes or positions based on the element's size
    // This is where you would dynamically adjust node positions if necessary
    // console.log('SVG Size:', element.clientWidth, element.clientHeight);
  }

  @tracked nodes = [];
  @tracked edges = [];
  @tracked simulation = null;

  @action async updateNodes(element, b, c) {
    console.log('update nodes', element, b, c);
    if (!this.cluster.data) {
      return;
    }

    // const jobs = await this.cluster.data;
    // const clients = await this.cluster.clients;
    // const servers = await this.cluster.servers;
    let [jobs, clients, servers] = await Promise.all([
      this.cluster.data, // Assuming this is a Promise
      this.cluster.clients, // Assuming this is a Promise
      this.cluster.servers, // Assuming this is a Promise
    ]);

    // const total = this.cluster.data.length;
    // const total = jobs.length + clients.length + servers.length;
    // console.log('so total', total);
    // const radius = this.tertiaryRadius; // TODO: buggy here

    // let allObjects = [];
    // allObjects.pushObjects(jobs);
    // allObjects.pushObjects(clients);
    // allObjects.pushObjects(servers);

    // const allObjects = ArrayProxy.create();
    // allObjects.pushObjects(jobs);
    // allObjects.pushObjects(clients);
    // allObjects.pushObjects(servers);

    this.nodes = this.cluster.data.map((item, index) => {
      // this.nodes = [...jobs.toArray(), ...clients.toArray(), ...servers.toArray()].map((item, index) => {
      // this.nodes = allObjects.map((item, index) => {
      // this.nodes = jobs.map(j => j.toJSON()).concat(clients.map(c => c.toJSON())).concat(servers.map(s => s.toJSON())).map((item, index) => {
      // return {
      //   ...item,
      //   x: this.nodes[index] ? this.nodes[index].x : index * 100,
      //   y: this.nodes[index] ? this.nodes[index].y : index * 100,
      // };

      const angle = (index / jobs.length) * 2 * Math.PI; // Angle for each node in radians
      // item.x = this.centerX + this.tertiaryRadius * Math.cos(angle);
      // item.y = this.centerY + this.tertiaryRadius * Math.sin(angle);
      // return item;

      let jobX = this.centerX + this.tertiaryRadius * Math.cos(angle);
      let jobY = this.centerY + this.tertiaryRadius * Math.sin(angle);
      // const jobAllocationCount = item.allocations.length;
      // const angleIncrement = (2 * Math.PI) / jobAllocationCount;
      // let allocations = item.allocations.map((alloc, index) => {
      //   let allocNode = {};
      //   allocNode.theta = index * angleIncrement;
      //   allocNode.r = this.nodeRadius / jobAllocationCount;
      //   allocNode.buffer = this.nodeRadius + 2;
      //   allocNode.x = jobX + Math.cos(allocNode.theta) * allocNode.r;
      //   allocNode.y = jobY + Math.sin(allocNode.theta) * allocNode.r;
      //   return allocNode;
      // });
      return {
        // ...item,
        // modelName: item.constructor.modelName,
        x: jobX,
        y: jobY,
        model: item,
        // x: this.nodes[index] ? this.nodes[index].x : this.centerX,
        // y: this.nodes[index] ? this.nodes[index].y : this.centerY,
      };
    });

    // Add allocations of jobs
    this.nodes
      .filter((node) => node.model.constructor.modelName === 'job')
      .forEach((job) => {
        console.log('job and alloc blocks', job.model.allocBlocks);
        let jobAllocBlockArray = [];
        Object.keys(job.model.allocBlocks).forEach((status) => {
          let statusGroup = job.model.allocBlocks[status];

          // Iterate through each health status (e.g., "healthy")
          Object.keys(statusGroup).forEach((healthStatus) => {
            let healthGroup = statusGroup[healthStatus];

            // Iterate through each group type (e.g., "nonCanary")
            Object.keys(healthGroup).forEach((canaryType) => {
              let items = healthGroup[canaryType];

              // Iterate through each item, adding 'status' and 'healthStatus'
              items.forEach((item) => {
                jobAllocBlockArray.push({
                  ...item, // Spread the original item properties
                  status,
                  healthStatus,
                  canaryType,
                  constructor: {
                    // TOODO: big ol hack
                    modelName: 'allocation',
                  },
                });
              });
            });
          });
        });
        console.log('so finally', jobAllocBlockArray);
        // job.model.allocations.forEach((allocation, index) => {
        jobAllocBlockArray.forEach((allocation, index) => {
          console.log('allocs', allocation);
          let allocNode = {};
          allocNode.parentJob = job;
          allocNode.index = index;
          allocNode.model = allocation;
          // color: a switch statement method
          allocNode.color = (() => {
            switch (allocation.status) {
              case 'running':
                return '#2eb039';
              case 'pending':
                return '#bbc4d1';
              case 'failed':
                return '#c84034';
              default:
                return '#000000';
            }
          })();
          this.nodes.push(allocNode);
        });
      });

    // add clients
    this.nodes.pushObjects(
      clients.map((item, index) => {
        const angle = (index / clients.length) * 2 * Math.PI; // Angle for each node in radians
        return {
          x: this.centerX + this.secondaryRadius * Math.cos(angle),
          y: this.centerY + this.secondaryRadius * Math.sin(angle),
          model: item,
        };
      })
    );
    // add servers
    this.nodes.pushObjects(
      servers.map((item, index) => {
        const angle = (index / servers.length) * 2 * Math.PI; // Angle for each node in radians
        return {
          x: this.centerX + this.primaryRadius * Math.cos(angle),
          y: this.centerY + this.primaryRadius * Math.sin(angle),
          color: item.isLeader ? '#2eb039' : '#bbc4d1',
          model: item,
        };
      })
    );

    if (this.simulation) {
      // Restart the simulation with new nodes
      this.simulation.nodes(this.nodes).alpha(1).restart();
    }
  }

  @action
  setupLinks() {
    // Assuming this.nodes is already populated with both jobs and clients
    let edges = [];
    console.log('setting up links and here are my nodes', this.nodes);

    this.nodes.forEach((node, index) => {
      if (node.model.constructor.modelName === 'job') {
        console.log('setting up links for', node, node.model.allocations);
        // Assuming 'clients' is an array of client IDs within each job model
        node.model.allocations.forEach((alloc) => {
          let clientId = alloc.nodeID;
          // Find the corresponding client node
          let targetIndex = this.nodes.findIndex(
            (n) =>
              n.model.constructor.modelName === 'node' &&
              n.model.id === clientId
          );
          // Source is job, target is client
          if (targetIndex !== -1) {
            edges.push({ source: index, target: targetIndex });
          }
        });
      }
      // Bind alloc to job
      if (node.model.constructor.modelName === 'allocation') {
        let parentJob = node.parentJob;
        if (parentJob) {
          // Source is alloc, target is job
          edges.push({ source: index, target: this.nodes.indexOf(parentJob) });
        }
      }
    });

    // Store the links in your component for rendering or simulation use
    console.log('so then edges', edges);
    this.edges = edges;
  }

  @action
  initializeSimulation(element) {
    console.log('initializing sim on', this.nodes);
    this.simulation = forceSimulation(this.nodes)
      .force(
        'link',
        forceLink(this.edges)
          .distance((d) => {
            console.log('d link force', d);
            if (d.source.model.constructor.modelName === 'allocation') {
              return 0;
            }
            return 30;
          })
          .strength((d) => {
            console.log('d link force', d);
            if (d.source.model.constructor.modelName === 'allocation') {
              return 1;
            }
            return 0.02;
          })
        // .strength(0.2)
      )
      .force(
        'charge',
        forceManyBody().strength((d) => {
          switch (d.model.constructor.modelName) {
            case 'job':
              return -100;
            case 'allocation':
              return -30;
            default:
              return -30;
          }
          // if (d.model.constructor.modelName === 'job') {
          //   return -100;
          // } else {
          //   return -30;
          // }
        })
      )
      // .force(
      //   "center",
      //   forceCenter(this.centerX, this.centerY)
      // )
      .force(
        'collide',
        forceCollide((d) => {
          console.log('force collid on d', d.model.constructor.modelName);
          if (d.model.constructor.modelName === 'allocation') {
            return this.allocationRadius + 2;
          } else {
            return this.collisionBuffer + 2;
          }
        })
          // .strength((d) => {
          //   console.log('d collide force', d, b, c);
          //   return 1;
          //   // switch (d.model.constructor.modelName) {
          //   //   case 'allocation':
          //   //     return 0;
          //   //   default:
          //   //     return 0.5;
          //   // }
          // })
          .iterations(4)
      )
      .force(
        'radial',
        forceRadial(
          (d, i) => {
            switch (d.model.constructor.modelName) {
              case 'agent':
                return this.primaryRadius;
              case 'node':
                return this.secondaryRadius;
              case 'job':
                return this.tertiaryRadius;
              default:
                return this.secondaryRadius;
            }
          },
          this.width / 2,
          this.height / 2
        ).strength((d) => {
          switch (d.model.constructor.modelName) {
            case 'allocation':
              return 0;
            default:
              return this.radialStrength;
          }
        })
      )
      .force(
        'forceBoundary',
        forceBoundary(
          this.boundaryBuffer,
          this.boundaryBuffer,
          this.width - this.boundaryBuffer,
          this.height - this.boundaryBuffer
        ).strength((d) => {
          switch (d.model.constructor.modelName) {
            case 'allocation':
              return 0;
            default:
              return 0.2;
          }
        })
      )
      .force(
        'x',
        forceX((d) => {
          return d.x;
        }).strength((d) => {
          switch (d.model.constructor.modelName) {
            case 'job':
              return 0.1;
            case 'allocation':
              return 0;
            default:
              return 1;
          }
        })
      )
      .force(
        'y',
        forceY((d) => {
          return d.y;
        }).strength((d) => {
          switch (d.model.constructor.modelName) {
            case 'job':
              return 0.1;
            case 'allocation':
              return 0;
            default:
              return 1;
          }
        })
      )
      .on('tick', this.onTick);
    console.log('so sim', this.simulation);

    //   if (this.simulation) {
    //     this.simulation
    //         .force('link', forceLink(this.links).id(d => d.index)) // Adjust 'id' accessor based on your nodes' identifiers
    //         .alpha(1) // Reheat the simulation if needed
    //         .restart();
    // }
  }

  @action
  onZoom(event) {
    d3Select(this.zoomContainer).select('g').attr('transform', event.transform);
  }

  onTick = () => {
    this.nodes = this.nodes.map((node, i) => {
      let simNode = this.simulation.nodes()[i]; // Get the corresponding node from the simulation
      // node.x = simNode.x;
      // node.y = simNode.y;
      // return node;

      return {
        ...node,
        x: simNode.x,
        y: simNode.y,
        // ...simNode,
        // model: node
      };
    });

    this.edges = this.edges.map((edge) => ({ ...edge })); // Force update edges
  };

  @action
  updateBoundaryBuffer(event) {
    const buffer = parseInt(event.target.value, 10);
    this.simulation.force(
      'forceBoundary',
      forceBoundary(buffer, buffer, this.width - buffer, this.height - buffer)
    );
    this.restartSimulation();
  }

  @action
  updateCollisionBuffer(event) {
    const buffer = parseInt(event.target.value, 10);
    this.simulation.force(
      'forceCollide',
      forceCollide(buffer + 1).iterations(4)
    );
    this.restartSimulation();
  }

  @action
  updateChargeStrength(event) {
    const chargeStrength = parseInt(event.target.value, 10);
    this.simulation.force('charge', forceManyBody().strength(chargeStrength));
    this.restartSimulation();
  }

  @action
  updateRadialRadius(event) {
    const radialRadius = parseFloat(event.target.value);
    this.simulation.force(
      'radial',
      forceRadial(
        Math.min(this.width, this.height) * radialRadius,
        this.width / 2,
        this.height / 2
      ).strength(this.radialStrength)
    );
    this.restartSimulation();
  }

  @action
  updateRadialStrength(event) {
    const radialStrength = parseFloat(event.target.value);
    this.simulation.force(
      'radial',
      forceRadial(
        this.tertiaryRadius,
        this.width / 2,
        this.height / 2
      ).strength(radialStrength)
    );
    this.restartSimulation();
  }

  // @action
  // updateCenterX(event) {
  //     const centerX = parseInt(event.target.value, 10);
  //     this.simulation.force('center', forceCenter(centerX, this.centerY));
  //     this.restartSimulation();
  // }

  // @action
  // updateCenterY(event) {
  //     const centerY = parseInt(event.target.value, 10);
  //     this.simulation.force('center', forceCenter(this.centerX, centerY));
  //     this.restartSimulation();
  // }

  @action
  restartSimulation() {
    if (this.simulation) {
      this.simulation
        // .force('charge', forceManyBody().strength(this.chargeStrength))
        // .force('center', forceCenter(this.centerX, this.centerY))
        // .alpha(1) // Reset the cooling parameter
        // .alphaTarget(0.3)
        .alpha(0.3)
        .restart();
    }
  }

  @action setupZoom(svgElement) {
    let zoom = d3Zoom()
      .scaleExtent([0.5, 3])
      .on('zoom', (event) => {
        d3Select(svgElement)
          .select('.zoom-container')
          .attr('transform', event.transform);
      });

    d3Select(svgElement).call(zoom);
  }

  // #region actions
  onNodeMouseOver(node) {
    console.log(node.model.id, node.model.status);
  }
  // #endregion actions
}
