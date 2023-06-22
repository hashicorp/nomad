/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import hbs from 'htmlbars-inline-precompile';
import DelayedTruth from '../utils/delayed-truth';
import { withKnobs, boolean } from '@storybook/addon-knobs';
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

export let FullViz = () => {};
