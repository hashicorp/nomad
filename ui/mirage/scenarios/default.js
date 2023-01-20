import { assign } from '@ember/polyfills';
import config from 'nomad-ui/config/environment';
import * as topoScenarios from './topo';
import * as sysbatchScenarios from './sysbatch';
import { pickOne } from '../utils';
import faker from 'nomad-ui/mirage/faker';

const withNamespaces = getConfigValue('mirageWithNamespaces', false);
const withTokens = getConfigValue('mirageWithTokens', true);
const withRegions = getConfigValue('mirageWithRegions', false);

export const allScenarios = {
  smallCluster,
  mediumCluster,
  largeCluster,
  massiveCluster,
  allJobTypes,
  allNodeTypes,
  everyFeature,
  emptyCluster,
  variableTestCluster,
  servicesTestCluster,
  policiesTestCluster,
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
  faker.seed(1);
  server.create('feature', { name: 'Dynamic Application Sizing' });
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node', 5);
  server.createList('job', 1, { createRecommendations: true });
  server.create('job', {
    withGroupServices: true,
    withTaskServices: true,
    name: 'Service-haver',
    id: 'service-haver',
    namespaceId: 'default',
  });
  server.createList('allocFile', 5);
  server.create('allocFile', 'dir', { depth: 2 });
  server.createList('csi-plugin', 2);
  server.createList('variable', 3);

  const variableLinkedJob = server.db.jobs[0];
  const variableLinkedGroup = server.db.taskGroups.findBy({
    jobId: variableLinkedJob.id,
  });
  const variableLinkedTask = server.db.tasks.findBy({
    taskGroupId: variableLinkedGroup.id,
  });
  [
    'a/b/c/foo0',
    'a/b/c/bar1',
    'a/b/c/d/e/foo2',
    'a/b/c/d/e/bar3',
    'a/b/c/d/e/f/foo4',
    'a/b/c/d/e/f/g/foo5',
    'a/b/c/x/y/z/foo6',
    'a/b/c/x/y/z/bar7',
    'a/b/c/x/y/z/baz8',
    'w/x/y/foo9',
    'w/x/y/z/foo10',
    'w/x/y/z/bar11',
    'just some arbitrary file',
    'another arbitrary file',
    'another arbitrary file again',
  ].forEach((path) => server.create('variable', { id: path }));

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}/${variableLinkedTask.name}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}`,
    namespace: variableLinkedJob.namespace,
  });

  const newJobName = 'new-job';
  const newJobTaskGroupName = 'redis';
  const jsonJob = (overrides) => {
    return JSON.stringify(
      assign(
        {},
        {
          Name: newJobName,
          Namespace: 'default',
          Datacenters: ['dc1'],
          Priority: 50,
          TaskGroups: [
            {
              Name: newJobTaskGroupName,
              Tasks: [
                {
                  Name: 'redis',
                  Driver: 'docker',
                },
              ],
            },
          ],
        },
        overrides
      ),
      null,
      2
    );
  };

  server.create('variable', {
    id: `nomad/job-templates/foo-bar`,
    namespace: 'namespace-2',
    Items: {
      description: 'a description',
      template: jsonJob(),
    },
  });

  server.create('variable', {
    id: `nomad/job-templates/baz-qud`,
    namespace: 'default',
    Items: {
      description: 'another different description',
      template: jsonJob(),
    },
  });

  server.create('variable', {
    id: 'Auto-conflicting Variable',
    namespace: 'default',
  });

  // #region evaluations

  // Branching: a single eval that relates to N-1 mutually-unrelated evals
  const NUM_BRANCHING_EVALUATIONS = 3;
  Array(NUM_BRANCHING_EVALUATIONS)
    .fill()
    .map((_, i) => {
      return {
        evaluation: server.create('evaluation', {
          id: `branching_${i}`,
          previousEval: i > 0 ? `branching_0` : '',
          jobID: pickOne(server.db.jobs).id,
        }),

        evaluationStub: server.create('evaluation-stub', {
          id: `branching_${i}`,
          previousEval: i > 0 ? `branching_0` : '',
          status: 'failed',
        }),
      };
    })
    .map((x, i, all) => {
      x.evaluation.update({
        relatedEvals:
          i === 0
            ? all.filter((_, j) => j !== 0).map((e) => e.evaluation)
            : all.filter((_, j) => j !== i).map((e) => e.evaluation),
      });
      return x;
    });

  // Linear: a long line of N related evaluations
  const NUM_LINEAR_EVALUATIONS = 20;
  Array(NUM_LINEAR_EVALUATIONS)
    .fill()
    .map((_, i) => {
      return {
        evaluation: server.create('evaluation', {
          id: `linear_${i}`,
          previousEval: i > 0 ? `linear_${i - 1}` : '',
          jobID: pickOne(server.db.jobs).id,
        }),

        evaluationStub: server.create('evaluation-stub', {
          id: `linear_${i}`,
          previousEval: i > 0 ? `linear_${i - 1}` : '',
          nextEval: `linear_${i + 1}`,
          status: 'failed',
        }),
      };
    })
    .map((x, i, all) => {
      x.evaluation.update({
        relatedEvals: all.filter((_, j) => i !== j).map((e) => e.evaluation),
      });
      return x;
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

  server.create('auth-method', {name: 'vault'});
  server.create('auth-method', {name: 'auth0'});
  server.create('auth-method', {name: 'cognito'});

}

function mediumCluster(server) {
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node', 50);
  server.createList('job', 25);
}

function variableTestCluster(server) {
  faker.seed(1);
  createTokens(server);
  createNamespaces(server);
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node', 5);
  server.createList('job', 3);
  server.createList('variable', 3);
  // server.createList('allocFile', 5);
  // server.create('allocFile', 'dir', { depth: 2 });
  // server.createList('csi-plugin', 2);

  const variableLinkedJob = server.db.jobs[0];
  const variableLinkedGroup = server.db.taskGroups.findBy({
    jobId: variableLinkedJob.id,
  });
  const variableLinkedTask = server.db.tasks.findBy({
    taskGroupId: variableLinkedGroup.id,
  });
  [
    'a/b/c/foo0',
    'a/b/c/bar1',
    'a/b/c/d/e/foo2',
    'a/b/c/d/e/bar3',
    'a/b/c/d/e/f/foo4',
    'a/b/c/d/e/f/g/foo5',
    'a/b/c/x/y/z/foo6',
    'a/b/c/x/y/z/bar7',
    'a/b/c/x/y/z/baz8',
    'w/x/y/foo9',
    'w/x/y/z/foo10',
    'w/x/y/z/bar11',
  ].forEach((path) => server.create('variable', { id: path }));

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}/${variableLinkedTask.name}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: 'just some arbitrary file',
    namespace: 'namespace-2',
  });

  server.create('variable', {
    id: 'another arbitrary file',
    namespace: 'namespace-2',
  });

  server.create('variable', {
    id: 'another arbitrary file again',
    namespace: 'namespace-2',
  });

  server.create('variable', {
    id: 'Auto-conflicting Variable',
    namespace: 'default',
  });
}

function policiesTestCluster(server) {
  faker.seed(1);
  createTokens(server);
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
}


function servicesTestCluster(server) {
  faker.seed(1);
  server.create('feature', { name: 'Dynamic Application Sizing' });
  server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
  server.createList('node', 5);
  server.createList('job', 1, { createRecommendations: true });
  server.create('job', {
    withGroupServices: true,
    withTaskServices: true,
    name: 'Service-haver',
    id: 'service-haver',
    namespaceId: 'default',
  });
  server.createList('allocFile', 5);
  server.create('allocFile', 'dir', { depth: 2 });
  server.createList('csi-plugin', 2);
  server.createList('variable', 3);

  const variableLinkedJob = server.db.jobs[0];
  const variableLinkedGroup = server.db.taskGroups.findBy({
    jobId: variableLinkedJob.id,
  });
  const variableLinkedTask = server.db.tasks.findBy({
    taskGroupId: variableLinkedGroup.id,
  });
  [
    'a/b/c/foo0',
    'a/b/c/bar1',
    'a/b/c/d/e/foo2',
    'a/b/c/d/e/bar3',
    'a/b/c/d/e/f/foo4',
    'a/b/c/d/e/f/g/foo5',
    'a/b/c/x/y/z/foo6',
    'a/b/c/x/y/z/bar7',
    'a/b/c/x/y/z/baz8',
    'w/x/y/foo9',
    'w/x/y/z/foo10',
    'w/x/y/z/bar11',
    'just some arbitrary file',
    'another arbitrary file',
    'another arbitrary file again',
  ].forEach((path) => server.create('variable', { id: path }));

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}/${variableLinkedTask.name}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}/${variableLinkedGroup.name}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: `nomad/jobs/${variableLinkedJob.id}`,
    namespace: variableLinkedJob.namespace,
  });

  server.create('variable', {
    id: 'Auto-conflicting Variable',
    namespace: 'default',
  });

  // #region evaluations

  // Branching: a single eval that relates to N-1 mutually-unrelated evals
  const NUM_BRANCHING_EVALUATIONS = 3;
  Array(NUM_BRANCHING_EVALUATIONS)
    .fill()
    .map((_, i) => {
      return {
        evaluation: server.create('evaluation', {
          id: `branching_${i}`,
          previousEval: i > 0 ? `branching_0` : '',
          jobID: pickOne(server.db.jobs).id,
        }),

        evaluationStub: server.create('evaluation-stub', {
          id: `branching_${i}`,
          previousEval: i > 0 ? `branching_0` : '',
          status: 'failed',
        }),
      };
    })
    .map((x, i, all) => {
      x.evaluation.update({
        relatedEvals:
          i === 0
            ? all.filter((_, j) => j !== 0).map((e) => e.evaluation)
            : all.filter((_, j) => j !== i).map((e) => e.evaluation),
      });
      return x;
    });

  // Linear: a long line of N related evaluations
  const NUM_LINEAR_EVALUATIONS = 20;
  Array(NUM_LINEAR_EVALUATIONS)
    .fill()
    .map((_, i) => {
      return {
        evaluation: server.create('evaluation', {
          id: `linear_${i}`,
          previousEval: i > 0 ? `linear_${i - 1}` : '',
          jobID: pickOne(server.db.jobs).id,
        }),

        evaluationStub: server.create('evaluation-stub', {
          id: `linear_${i}`,
          previousEval: i > 0 ? `linear_${i - 1}` : '',
          nextEval: `linear_${i + 1}`,
          status: 'failed',
        }),
      };
    })
    .map((x, i, all) => {
      x.evaluation.update({
        relatedEvals: all.filter((_, j) => i !== j).map((e) => e.evaluation),
      });
      return x;
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
  server.create('token', {
    name: 'Secure McVariables',
    id: '53cur3-v4r14bl35',
  });
  server.create('token', {
    name: "Safe O'Constants",
    id: 'f3w3r-53cur3-v4r14bl35',
  });
  server.create('token', {
    name: 'Lazarus MacMarbh',
    id: '3XP1R35-1N-3L3V3N-M1NU735',
  });
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
