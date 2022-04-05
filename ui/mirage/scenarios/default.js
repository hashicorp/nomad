import config from 'nomad-ui/config/environment';
import * as topoScenarios from './topo';
import * as sysbatchScenarios from './sysbatch';
import { pickOne } from '../utils';

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
  ...topoScenarios,
  ...sysbatchScenarios,
};

const scenario =
  getScenarioQueryParameter() ||
  getConfigValue('mirageScenario', 'emptyCluster');

export default function (server) {
  const activeScenario = allScenarios[scenario];
  if (!activeScenario) {
    throw new Error(
      `Selected Mirage scenario does not exist.\n\n${scenario} not in list: \n\n\t${Object.keys(
        allScenarios
      ).join('\n\t')}`
    );
  }

  if (withNamespaces) createNamespaces(server);
  if (withTokens) createTokens(server);
  if (withRegions) createRegions(server);
  activeScenario(server);
}

// Scenarios

function smallCluster(server) {
  server.create('feature', { name: 'Dynamic Application Sizing' });
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node', 5);
  server.createList('job', 5, { createRecommendations: true });
  server.createList('allocFile', 5);
  server.create('allocFile', 'dir', { depth: 2 });
  server.createList('csi-plugin', 2);

  // #region evaluations

  // Branching: a single eval that relates to 2 mutually-unrelated evals

  const rootBranchingEval = server.create('evaluation', {
    id: 'branching_1',
    status: 'failed',
  });
  const secondBranchingEval = server.create('evaluation', {
    id: 'branching_2',
    status: 'failed',
    previousEval: rootBranchingEval.id,
  });
  const thirdBranchingEval = server.create('evaluation', {
    id: 'branching_3',
    status: 'failed',
    previousEval: rootBranchingEval.id,
  });

  const rootBranchingEvalStub = server.create('evaluation-stub', {
    id: rootBranchingEval.id,
    status: 'failed',
  });

  const secondBranchingEvalStub = server.create('evaluation-stub', {
    id: secondBranchingEval.id,
    previousEval: rootBranchingEval.id,
    status: 'failed',
  });

  const thirdBranchingEvalStub = server.create('evaluation-stub', {
    id: thirdBranchingEval.id,
    previousEval: rootBranchingEval.id,
    status: 'failed',
  });

  rootBranchingEval.update({
    relatedEvals: [secondBranchingEvalStub, thirdBranchingEvalStub],
  });

  secondBranchingEval.update({
    relatedEvals: [rootBranchingEvalStub, thirdBranchingEvalStub],
  });

  thirdBranchingEval.update({
    relatedEvals: [rootBranchingEvalStub, secondBranchingEvalStub],
  });

  // Linear: a long line of 5 related evaluations

  const rootLinearEval = server.create('evaluation', {
    id: 'linear_1',
    status: 'failed',
  });
  const secondLinearEval = server.create('evaluation', {
    id: 'linear_2',
    status: 'failed',
    previousEval: rootLinearEval.id,
  });
  const thirdLinearEval = server.create('evaluation', {
    id: 'linear_3',
    status: 'failed',
    previousEval: secondLinearEval.id,
  });
  const fourthLinearEval = server.create('evaluation', {
    id: 'linear_4',
    status: 'failed',
    previousEval: thirdLinearEval.id,
  });
  const fifthLinearEval = server.create('evaluation', {
    id: 'linear_5',
    status: 'failed',
    previousEval: fourthLinearEval.id,
  });

  const rootLinearEvalStub = server.create('evaluation-stub', {
    id: rootLinearEval.id,
    previousEval: '',
    nextEval: secondLinearEval.id,
    status: 'failed',
  });

  const secondLinearEvalStub = server.create('evaluation-stub', {
    id: secondLinearEval.id,
    previousEval: rootLinearEval.id,
    nextEval: thirdLinearEval.id,
    status: 'failed',
  });

  const thirdLinearEvalStub = server.create('evaluation-stub', {
    id: thirdLinearEval.id,
    previousEval: secondLinearEval.id,
    nextEval: fourthLinearEval.id,
    status: 'failed',
  });

  const fourthLinearEvalStub = server.create('evaluation-stub', {
    id: fourthLinearEval.id,
    previousEval: thirdLinearEval.id,
    nextEval: fifthLinearEval.id,
    status: 'failed',
  });

  const fifthLinearEvalStub = server.create('evaluation-stub', {
    id: fifthLinearEval.id,
    previousEval: fourthLinearEval.id,
    status: 'failed',
  });

  rootLinearEval.update({
    relatedEvals: [
      secondLinearEvalStub,
      thirdLinearEvalStub,
      fourthLinearEvalStub,
      fifthLinearEvalStub,
    ],
  });
  secondLinearEval.update({
    relatedEvals: [
      rootLinearEvalStub,
      thirdLinearEvalStub,
      fourthLinearEvalStub,
      fifthLinearEvalStub,
    ],
  });
  thirdLinearEval.update({
    relatedEvals: [
      rootLinearEvalStub,
      secondLinearEvalStub,
      fourthLinearEvalStub,
      fifthLinearEvalStub,
    ],
  });
  fourthLinearEval.update({
    relatedEvals: [
      rootLinearEvalStub,
      secondLinearEvalStub,
      thirdLinearEvalStub,
      fifthLinearEvalStub,
    ],
  });
  fifthLinearEval.update({
    relatedEvals: [
      rootLinearEvalStub,
      secondLinearEvalStub,
      thirdLinearEvalStub,
      fourthLinearEvalStub,
    ],
  });

  // #endregion evaluations

  const csiAllocations = server.createList('allocation', 5);
  const volumes = server.schema.csiVolumes.all().models;
  csiAllocations.forEach((alloc) => {
    const volume = pickOne(volumes);
    volume.writeAllocs.add(alloc);
    volume.readAllocs.add(alloc);
    volume.save();
  });
}

function mediumCluster(server) {
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
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
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node', 5);

  server.create('job', { type: 'service' });
  server.create('job', { type: 'batch' });
  server.create('job', { type: 'system' });
  server.create('job', 'periodic');
  server.create('job', 'parameterized');
  server.create('job', 'periodicSysbatch');
  server.create('job', 'parameterizedSysbatch');
  server.create('job', { failedPlacements: true });
}

function allNodeTypes(server) {
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');

  server.create('node');
  server.create('node', 'forceIPv4');
  server.create('node', 'draining');
  server.create('node', 'forcedDraining');
  server.create('node', 'noDeadlineDraining');
  server.create('node', 'withMeta');

  server.createList('job', 3);
}

function everyFeature(server) {
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');

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
  server.create('job', {
    type: 'batch',
    failedPlacements: true,
    namespaceId: 'default',
  });
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
  ['americas', 'europe', 'asia', 'some-long-name-just-to-test'].forEach(
    (id) => {
      server.create('region', { id });
    }
  );
}

/* eslint-disable */
function logTokens(server) {
  console.log('TOKENS:');
  server.db.tokens.forEach((token) => {
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

function getScenarioQueryParameter() {
  const params = new URLSearchParams(window.location.search);
  return params.get('mirage-scenario');
}
/* eslint-enable */
