/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import faker from 'nomad-ui/mirage/faker';
import { generateNetworks, generatePorts } from '../common';

const genResources = (CPU, Memory) => ({
  Cpu: { CpuShares: CPU },
  Memory: { MemoryMB: Memory },
  Disk: { DiskMB: 10000 },
  Networks: generateNetworks(),
  Ports: generatePorts(),
});

export function topoSmall(server) {
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node-pool', 4);
  server.createList('node', 12, {
    datacenter: 'dc1',
    status: 'ready',
    nodeResources: genResources(3000, 5192),
  });

  const jobResources = [
    ['M: 2560, C: 150'],
    ['M: 128, C: 400'],
    ['M: 512, C: 100'],
    ['M: 256, C: 150'],
    ['M: 200, C: 50'],
    ['M: 64, C: 100'],
    ['M: 128, C: 150'],
    ['M: 1024, C: 500'],
    ['M: 100, C: 300', 'M: 200, C: 150'],
    ['M: 512, C: 250', 'M: 600, C: 200'],
  ];

  jobResources.forEach((spec) => {
    server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'service',
      createAllocations: false,
      resourceSpec: spec,
    });
  });

  server.createList('allocation', 25, {
    forceRunningClientStatus: true,
  });
}

export function topoMedium(server) {
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node', 10, {
    datacenter: 'us-west-1',
    status: 'ready',
    nodeResources: genResources(3000, 5192),
  });
  server.createList('node', 12, {
    datacenter: 'us-east-1',
    status: 'ready',
    nodeResources: genResources(3000, 5192),
  });
  server.createList('node', 11, {
    datacenter: 'eu-west-1',
    status: 'ready',
    nodeResources: genResources(3000, 5192),
  });

  server.createList('node', 8, {
    datacenter: 'us-west-1',
    status: 'ready',
    nodeResources: genResources(8000, 12192),
  });
  server.createList('node', 9, {
    datacenter: 'us-east-1',
    status: 'ready',
    nodeResources: genResources(8000, 12192),
  });

  const jobResources = [
    ['M: 2560, C: 150'],
    ['M: 128, C: 400'],
    ['M: 512, C: 100'],
    ['M: 256, C: 150'],
    ['M: 200, C: 50'],
    ['M: 64, C: 100'],
    ['M: 128, C: 150'],
    ['M: 1024, C: 500'],

    ['M: 1200, C: 50'],
    ['M: 1400, C: 200'],
    ['M: 50, C: 150'],
    ['M: 5000, C: 1800'],

    ['M: 100, C: 300', 'M: 200, C: 150'],
    ['M: 512, C: 250', 'M: 600, C: 200'],
  ];

  jobResources.forEach((spec) => {
    server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'service',
      createAllocations: false,
      createRecommendations: true,
      resourceSpec: spec,
    });
  });

  server.createList('allocation', 100, {
    forceRunningClientStatus: true,
  });
}
