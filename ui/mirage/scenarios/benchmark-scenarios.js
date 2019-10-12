export function uxrTask0(server) {
  // !! Needs a corresponding job file
  server.createList('agent', 3);
  server.createList('node', 5);
  server.createList('job', 5, { status: 'running', ratio: '10: R 1' });
  server.create('job', { status: 'running', ratio: '5: R 1 C 4' });
  server.create('job', { status: 'running', ratio: '10: R 5 F 1 L 1' });
  server.create('job', { status: 'dead', ratio: '10: C 1' });
  server.create('job', {
    id: 'web-server',
    status: 'running',
    ratio: '5: R 1',
  });
}

export function uxrTask1a(server) {
  server.createList('agent', 3);
  server.createList('node', 5, 'forceAllDrivers');

  server.create('job', {
    id: 'web-server',
    status: 'running',
    groupsCount: 1,
    ratio: '50: S 1 R 4 C 1',
    createAllocations: false,
    noFailedPlacements: true,
  });

  server.createList('allocation', 4, {
    jobId: 'web-server',
    clientStatus: 'running',
    jobVersion: 1,
  });

  server.create('allocation', 'rescheduled', {
    jobId: 'web-server',
    clientStatus: 'completed',
    rescheduleStatus: 'starting',
    jobVersion: 1,
  });
}

export function uxrTask2(server) {
  server.createList('agent', 3);
  server.createList('node', 5, 'forceAllDrivers');

  server.create('job', {
    id: 'web-server',
    status: 'running',
    groupsCount: 1,
    ratio: '3: R 1',
    createAllocations: false,
    noFailedPlacements: true,
    noDeployments: true,
  });

  server.createList('allocation', 2, {
    jobId: 'web-server',
    clientStatus: 'running',
    jobVersion: 1,
  });

  server.create('allocation', 'highUsage', {
    jobId: 'web-server',
    clientStatus: 'running',
    jobVersion: 1,
  });
}
