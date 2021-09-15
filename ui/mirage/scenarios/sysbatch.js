export function sysbatchSmall(server) {
  server.createList('agent', 3);
  const clients = server.createList('node', 12, {
    datacenter: 'dc1',
    status: 'ready',
  });

  // Create some clients not targeted by the sysbatch job.
  server.createList('node', 3, {
    datacenter: 'dc3',
    status: 'ready',
  });

  // Job with 1 task group.
  const job1 = server.create('job', {
    status: 'running',
    datacenters: ['dc1', 'dc2'],
    type: 'sysbatch',
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
    type: 'sysbatch',
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
    type: 'sysbatch',
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
    type: 'sysbatch',
    resourceSpec: ['M: 256, C: 500'],
    createAllocations: false,
  });
  clients.forEach((c, i) => {
    if (i > clients.length - 3) return;
    server.create('allocation', { jobId: jobNotScheduled.id, nodeId: c.id });
  });
}
