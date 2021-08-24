export function sysbatchSmall(server) {
  server.createList('agent', 3);
  server.createList('node', 12, {
    datacenter: 'dc1',
    status: 'ready',
  });

  const jobConstraints = [[], [], [], [], [], []];

  jobConstraints.forEach(spec => {
    server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'sysbatch',
      createAllocations: true,
    });
  });
}
