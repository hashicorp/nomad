import config from 'nomad-ui/config/environment';

const withNamespaces = getConfigValue('mirageWithNamespaces', false);
const withTokens = getConfigValue('mirageWithTokens', true);
const withRegions = getConfigValue('mirageWithRegions', false);

const allScenarios = {
  smallCluster,
  mediumCluster,
  largeCluster,
  massiveCluster,
  allJobTypes,
  allNodeTypes,
  everyFeature,
  emptyCluster,
};

const scenario = getConfigValue('mirageScenario', 'emptyCluster');

// FIXME all changes to this file are for demonstration only and will be removed before merging.

export default function(server) {
  const activeScenario = ipV6Cluster;
  if (!activeScenario) {
    throw new Error(
      `Selected Mirage scenario does not exist.\n\n${scenario} not in list: \n\n\t${Object.keys(
        allScenarios
      ).join('\n\t')}`
    );
  }

  // if (withNamespaces) createNamespaces(server);
  // if (withTokens) createTokens(server);
  // if (withRegions) createRegions(server);
  activeScenario(server);
}

// Scenarios

function ipV6Cluster(server) {
  server.create('agent');

  const node = server.create('node');
  server.create('job', {
    id: 'a-job',
    groupsCount: 1,
    groupTasksCount: 2,
    createAllocations: false,
  });
  server.create('allocation', 'withTaskWithPorts', {
    id: '4dda8e19-a4a8-4843-bf30-bc824092c617',
    clientStatus: 'running',
  });

  // Make sure the node has an unhealthy driver
  node.update({
    driver: Object.assign(node.drivers, {
      docker: {
        detected: true,
        healthy: false,
      },
    }),
  });

  // Make sure a task for the allocation depends on the unhealthy driver
  server.schema.tasks.first().update({
    driver: 'docker',
  });
}

function smallCluster(server) {
  server.createList('agent', 3);
  server.createList('node', 5);
  server.createList('job', 5);
}

function mediumCluster(server) {
  server.createList('agent', 3);
  server.createList('node', 50);
  server.createList('job', 25);
}

// Due to Mirage performance, large cluster scenarios will be slow
function largeCluster(server) {
  server.createList('agent', 5);
  server.createList('node', 1000);
  server.createList('job', 100);
}

function massiveCluster(server) {
  server.createList('agent', 7);
  server.createList('node', 5000);
  server.createList('job', 2000);
}

function allJobTypes(server) {
  server.createList('agent', 3);
  server.createList('node', 5);

  server.create('job', { type: 'service' });
  server.create('job', { type: 'batch' });
  server.create('job', { type: 'system' });
  server.create('job', 'periodic');
  server.create('job', 'parameterized');
  server.create('job', { failedPlacements: true });
}

function allNodeTypes(server) {
  server.createList('agent', 3);

  server.create('node');
  server.create('node', 'forceIPv4');
  server.create('node', 'draining');
  server.create('node', 'forcedDraining');
  server.create('node', 'noDeadlineDraining');
  server.create('node', 'withMeta');

  server.createList('job', 3);
}

function everyFeature(server) {
  server.createList('agent', 3);

  server.create('node', 'forceIPv4');
  server.create('node', 'draining');
  server.create('node', 'forcedDraining');
  server.create('node', 'noDeadlineDraining');
  server.create('node', 'withMeta');

  const job1 = server.create('job', {
    type: 'service',
    activeDeployment: true,
    namespaceId: 'default',
    createAllocations: false,
  });
  server.create('job', { type: 'batch', failedPlacements: true, namespaceId: 'default' });
  server.create('job', { type: 'system', namespaceId: 'default' });
  server.create('job', 'periodic', { namespaceId: 'default' });
  server.create('job', 'parameterized', { namespaceId: 'default' });

  server.create('allocation', 'rescheduled', { jobId: job1.id });
  server.create('allocation', 'preempter', { jobId: job1.id });
  server.create('allocation', 'preempted', { jobId: job1.id });
}

function emptyCluster(server) {
  server.create('agent');
  server.create('node');
}

// Behaviors

function createTokens(server) {
  server.createList('token', 3);
  logTokens(server);
}

function createNamespaces(server) {
  server.createList('namespace', 3);
}

function createRegions(server) {
  ['americas', 'europe', 'asia', 'some-long-name-just-to-test'].forEach(id => {
    server.create('region', { id });
  });
}

/* eslint-disable */
function logTokens(server) {
  console.log('TOKENS:');
  server.db.tokens.forEach(token => {
    console.log(`
Name: ${token.name}
Secret: ${token.secretId}
Accessor: ${token.accessorId}

`);
  });
}

function getConfigValue(variableName, defaultValue) {
  const value = config.APP[variableName];
  if (value !== undefined) return value;

  console.warn(
    `No ENV.APP value set for "${variableName}". Defaulting to "${defaultValue}". To set a custom value, modify config/environment.js`
  );
  return defaultValue;
}
/* eslint-enable */
