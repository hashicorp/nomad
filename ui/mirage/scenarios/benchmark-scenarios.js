import faker from 'nomad-ui/mirage/faker';

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
    clientStatus: 'complete',
    rescheduleStatus: 'starting',
    jobVersion: 1,
  });
}

export function uxrTask2a(server) {
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

export function uxrTask2b(server) {
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
    clientStatus: 'starting',
    jobVersion: 1,
  });
}

export function uxrTask2c(server) {
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

  server.create('allocation', {
    jobId: 'web-server',
    clientStatus: 'running',
    jobVersion: 1,
  });
}
export function uxrTask3a(server) {
  // !! Needs logs to be mocked on the stderr tab
  // No data path specified for data migration. Set the DATA_PATH env var.
  //
  // exit 1
  server.createList('agent', 3);
  server.createList('node', 5, 'forceAllDrivers');

  server.createList('job', 3, { status: 'running', ratio: '10: R 1' });
  server.create('job', { status: 'running', ratio: '10: R 1 C 2' });
  server.create('job', { status: 'running', ratio: '10: S 1 R 2 C 2' });
  server.create('job', {
    id: 'data-migrate',
    status: 'dead',
    type: 'batch',
    ratio: '1: F 1',
    noFailedPlacements: true,
    noDeployments: true,
    createAllocations: false,
  });

  server.create('allocation', {
    jobId: 'data-migrate',
    clientStatus: 'failed',
    jobVersion: 1,
  });
}

export function uxrTask4a(server) {
  const [summary] = task4Macro(server, '5: Q 2 R 1 C 2');
  summary.update({
    desiredCanaries: 1,
    desiredTotal: 3,
    placedCanaries: [faker.random.uuid()],
    placedAllocs: 1,
    healthyAllocs: 0,
  });
}

export function uxrTask4b(server) {
  const [summary] = task4Macro(server, '5: Q 2 R 1 C 2');
  summary.update({
    desiredCanaries: 1,
    desiredTotal: 3,
    placedCanaries: [faker.random.uuid()],
    placedAllocs: 1,
    healthyAllocs: 1,
  });
}

export function uxrTask4c(server) {
  const [summary, deployment] = task4Macro(server, '5: R 3 C 2');
  summary.update({
    desiredCanaries: 1,
    desiredTotal: 3,
    placedCanaries: [faker.random.uuid()],
    placedAllocs: 3,
    healthyAllocs: 3,
  });

  deployment.update({ status: 'successful' });
}

function task4Macro(server, ratio) {
  server.createList('agent', 3);
  server.createList('node', 5, 'forceAllDrivers');

  const job = server.create('job', {
    ratio,
    status: 'running',
    type: 'service',
    noFailedPlacements: true,
    activeDeployment: true,
  });

  // Return the active deployment task group summary for manipulation
  const deployment = server.schema.deployments.where({ jobId: job.id }).models[0];
  return [
    server.schema.deploymentTaskGroupSummaries.where({
      deploymentId: deployment.id,
    }).models[0],
    deployment,
  ];
}

export function uxrTask6a(server) {
  server.createList('agent', 3);
  server.createList('node', 5, 'forceAllDrivers');

  server.create('job', { status: 'running', ratio: '2: R 1 C 1' });
  server.create('job', { status: 'running', ratio: '8: S 1 R 6 C 1 F 1' });
  server.createList('job', 3, { status: 'running', ratio: '8: R 1' });

  const job = server.create('job', {
    id: 'jupyter-worker',
    ratio: '6: R 1 C 1',
    status: 'running',
    type: 'service',
    noFailedPlacements: true,
    noDeployments: true,
    createAllocations: false,
  });

  server.createList('allocation', 3, {
    jobId: 'jupyter-worker',
    clientStatus: 'running',
    version: 2,
  });

  server.createList('allocation', 3, {
    jobId: 'jupyter-worker',
    clientStatus: 'complete',
    version: 1,
  });

  server.create('job-version', {
    job,
    version: 1,
    noActiveDeployment: true,
    diff: null,
  });

  server.create('job-version', {
    job,
    version: 2,
    noActiveDeployment: true,
    diff: {
      Fields: null,
      ID: 'jupyter-worker',
      Objects: null,
      TaskGroups: [
        {
          Fields: null,
          Name: 'jupyter-worker',
          Objects: null,
          Tasks: [
            {
              Annotations: null,
              Fields: null,
              Name: 'jupyter-worker',
              Objects: [
                {
                  Fields: [
                    {
                      Annotations: null,
                      Name: 'image',
                      New: 'corpindustriesltd/jupyter-worker:1.1.0',
                      Old: 'corpindustriesltd/jupyter-worker:1.0.5',
                      Type: 'Edited',
                    },
                  ],
                  Name: 'Config',
                  Objects: null,
                  Type: 'Edited',
                },
                {
                  Fields: [
                    {
                      Annotations: null,
                      Name: 'CPU',
                      New: '500',
                      Old: '500',
                      Type: 'None',
                    },
                    {
                      Annotations: null,
                      Name: 'DiskMB',
                      New: '0',
                      Old: '0',
                      Type: 'None',
                    },
                    {
                      Annotations: null,
                      Name: 'IOPS',
                      New: '0',
                      Old: '0',
                      Type: 'None',
                    },
                    {
                      Annotations: null,
                      Name: 'MemoryMB',
                      New: '512',
                      Old: '256',
                      Type: 'Edited',
                    },
                  ],
                  Name: 'Resources',
                  Objects: null,
                  Type: 'Edited',
                },
              ],
              Type: 'Edited',
            },
          ],
          Type: 'Edited',
          Updates: null,
        },
      ],
      Type: 'Edited',
    },
  });
}

export function uxrTask7a(server) {
  server.createList('agent', 3);

  const common = {
    schedulingEligibility: 'eligible',
    drain: false,
    status: 'ready',
    datacenter: 'us-west',
  };

  server.createList('node', 5, 'forceIPv4', 'forceAllDrivers', {
    nodeClass: 't3.small',
    ...common,
  });
  server.createList('node', 5, 'forceIPv4', 'forceAllDrivers', {
    nodeClass: 't3.medium',
    ...common,
  });
  server.createList('node', 3, 'forceIPv4', 'forceAllDrivers', {
    nodeClass: 't3.xlarge',
    ...common,
  });
  server.createList('node', 2, 'forceIPv4', 'forceAllDrivers', {
    nodeClass: 'm5.12xlarge',
    ...common,
  });

  server.create('job');

  server.createList('allocation', 15, { shallow: true });
}
