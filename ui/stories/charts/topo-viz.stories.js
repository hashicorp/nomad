/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import DelayedTruth from '../utils/delayed-truth';
import { withKnobs, boolean } from '@storybook/addon-knobs';
import { getOwner } from '@ember/application';
import { tracked } from '@glimmer/tracking';
import { scaleLinear } from 'd3-scale';
import faker from 'faker';

export default {
  title: 'Charts/Topo Viz',
  decorators: [withKnobs],
};

const nodeGen = (name, datacenter, memory, cpu, allocations = []) => ({
  datacenter,
  memory,
  cpu,
  node: { name, isEligible: true, isDraining: false },
  allocations: allocations.map((alloc) => ({
    memory: alloc.memory,
    cpu: alloc.cpu,
    memoryPercent: alloc.memory / memory,
    cpuPercent: alloc.cpu / cpu,
    allocation: {
      id: faker.random.uuid(),
      isScheduled: true,
      clientStatus: alloc.clientStatus,
    },
  })),
});

const nodeModelGen = (datacenter, id, name, resources = '2000/1000') => {
  const [cpu, memory] = resources.split('/');
  return {
    datacenter,
    id,
    name,
    isEligible: true,
    isDraining: false,
    resources: { cpu, memory },
  };
};

const allocModelGen = (
  id,
  taskGroupName,
  clientStatus,
  nodeId,
  jobId,
  resources = '100/100'
) => {
  const [cpu, memory] = resources.split('/');
  return {
    id,
    taskGroupName,
    clientStatus,
    isScheduled: true,
    allocatedResources: { cpu, memory },
    belongsTo(t) {
      return {
        id() {
          return t === 'node' ? nodeId : jobId;
        },
      };
    },
  };
};

export let Node = () => ({
  template: hbs`
    <SvgPatterns />
    {{#if delayedTruth.complete}}
      <TopoViz::Node
        @node={{node}}
        @isDense={{isDense}}
        @heightScale={{heightScale}}
      />
    {{/if}}
  `,
  context: {
    delayedTruth: DelayedTruth.create(),
    isDense: boolean('isDense', false),
    heightScale: scaleLinear().range([15, 40]).domain([100, 1000]),
    node: nodeGen('Node One', 'dc1', 1000, 1000, [
      { memory: 100, cpu: 100, clientStatus: 'pending' },
      { memory: 250, cpu: 300, clientStatus: 'running' },
      { memory: 300, cpu: 200, clientStatus: 'running' },
    ]),
  },
});

export let Datacenter = () => ({
  template: hbs`
    <SvgPatterns />
    {{#if delayedTruth.complete}}
      <TopoViz::Datacenter
        @datacenter={{dc}}
        @isSingleColumn={{isSingleColumn}}
        @isDense={{isDense}}
        @heightScale={{heightScale}}
      />
    {{/if}}
  `,
  context: {
    delayedTruth: DelayedTruth.create(),
    isSingleColumn: boolean('isSingleColumn', false),
    isDense: boolean('isDense', false),
    heightScale: scaleLinear().range([15, 40]).domain([100, 1000]),
    dc: {
      name: 'dc1',
      nodes: [
        nodeGen('Node One', 'dc1', 1000, 1000, [
          { memory: 100, cpu: 100, clientStatus: 'pending' },
          { memory: 250, cpu: 300, clientStatus: 'running' },
          { memory: 300, cpu: 200, clientStatus: 'running' },
        ]),
        nodeGen('And Two', 'dc1', 500, 1000, [
          { memory: 100, cpu: 100, clientStatus: 'pending' },
          { memory: 250, cpu: 300, clientStatus: 'running' },
          { memory: 100, cpu: 100, clientStatus: 'running' },
          { memory: 100, cpu: 100, clientStatus: 'running' },
          { memory: 100, cpu: 100, clientStatus: 'running' },
        ]),
        nodeGen('Three', 'dc1', 500, 500, [
          { memory: 100, cpu: 300, clientStatus: 'running' },
          { memory: 300, cpu: 200, clientStatus: 'pending' },
        ]),
      ],
    },
  },
});

export let FullViz = () => ({
  template: hbs`
    {{#if delayedTruth.complete}}
      <TopoViz
        @nodes={{nodes}}
        @allocations={{allocations}}
      />
    {{/if}}
  `,
  context: {
    delayedTruth: DelayedTruth.create(),
    nodes: [
      nodeModelGen('dc1', '1', 'pdx-1', '2000/1000'),
      nodeModelGen('dc1', '2', 'pdx-2', '2000/1000'),
      nodeModelGen('dc1', '3', 'pdx-3', '2000/3000'),
      nodeModelGen('dc2', '4', 'yyz-1', '2000/1000'),
      nodeModelGen('dc2', '5', 'yyz-2', '2000/2000'),
    ],
    allocations: [
      allocModelGen('1', 'name', 'running', '1', 'job-1', '200/500'),
      allocModelGen('1', 'name', 'running', '5', 'job-1', '200/500'),
    ],
  },
});

export let EmberData = () => ({
  template: hbs`
    <div class="notification is-info">
      <h3 class='title is-4'>This visualization uses data from mirage.</h3>
      <p>Change the mirage scenario to see different cluster states visualized.</p>
    </div>
    {{#if (and delayedTruth.complete nodes allocations)}}
      <TopoViz
        @nodes={{nodes}}
        @allocations={{allocations}}
      />
    {{/if}}
  `,
  context: {
    delayedTruth: DelayedTruth.create(),
    nodes: tracked([]),
    allocations: tracked([]),

    async init() {
      this._super(...arguments);

      const owner = getOwner(this);
      const store = owner.lookup('service:store');

      this.nodes = await store.query('node', { resources: true });
      this.allocations = await store.query('allocation', {
        resources: true,
        task_states: false,
        namespace: '*',
      });
    },
  },
});
