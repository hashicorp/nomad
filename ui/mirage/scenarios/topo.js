import faker from 'nomad-ui/mirage/faker';
import { generateNetworks, generatePorts } from '../common';

export function topoSmall(server) {
  server.createList('agent', 3);
  server.createList('node', 12, {
    datacenter: 'dc1',
    status: 'ready',
    resources: {
      CPU: 3000,
      MemoryMB: 5192,
      DiskMB: 10000,
      IOPS: 100000,
      Networks: generateNetworks(),
      Ports: generatePorts(),
    },
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

  jobResources.forEach(spec => {
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
  //server.createList('allocation', 15, { clientStatus: 'pending' });
}

export function topoSmallProblems(server) {}

export function topoMedium(server) {}

export function topoMediumBatch(server) {}

export function topoMediumVariadic(server) {}
