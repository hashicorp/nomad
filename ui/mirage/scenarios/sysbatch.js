/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export function sysbatchSmall(server) {
  return sysbatchScenario(server, 15);
}

export function sysbatchLarge(server) {
  return sysbatchScenario(server, 55);
}

function sysbatchScenario(server, clientCount) {
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');

  const clients = server.createList('node', clientCount, {
    datacenter: 'dc1',
    status: 'ready',
  });

  // Create some clients not targeted by the sysbatch job.
  server.createList('node', 3, {
    datacenter: 'dc3',
    status: 'ready',
  });

  // Generate non-system/sysbatch job as counter-example.
  server.create('job', {
    status: 'running',
    type: 'service',
    resourceSpec: ['M: 256, C: 500'],
    createAllocations: true,
  });

  server.create('job', 'pack');

  ['system', 'sysbatch'].forEach(type => {
    // Job with 1 task group.
    const job1 = server.create('job', {
      status: 'running',
      datacenters: ['dc1', 'dc2'],
      type,
      resourceSpec: ['M: 256, C: 500'],
      createAllocations: false,
    });
    clients.forEach(c => {
      server.create('allocation', { jobId: job1.id, nodeId: c.id });
    });

    // Job with 2 task groups.
    const job2 = server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type,
      resourceSpec: ['M: 256, C: 500', 'M: 256, C: 500'],
      createAllocations: false,
    });
    clients.forEach(c => {
      server.create('allocation', { jobId: job2.id, nodeId: c.id });
      server.create('allocation', { jobId: job2.id, nodeId: c.id });
    });

    // Job with 3 task groups.
    const job3 = server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type,
      resourceSpec: ['M: 256, C: 500', 'M: 256, C: 500', 'M: 256, C: 500'],
      createAllocations: false,
    });
    clients.forEach(c => {
      server.create('allocation', { jobId: job3.id, nodeId: c.id });
      server.create('allocation', { jobId: job3.id, nodeId: c.id });
      server.create('allocation', { jobId: job3.id, nodeId: c.id });
    });

    // Job with client not scheduled.
    const jobNotScheduled = server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type,
      resourceSpec: ['M: 256, C: 500'],
      createAllocations: false,
    });
    clients.forEach((c, i) => {
      if (i > clients.length - 3) return;
      server.create('allocation', { jobId: jobNotScheduled.id, nodeId: c.id });
    });
  });
}
