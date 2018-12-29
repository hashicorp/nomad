import Ember from 'ember';
import { Factory, faker, trait } from 'ember-cli-mirage';
import { provide, pickOne } from '../utils';
import { DATACENTERS } from '../common';

const EVAL_TYPES = ['system', 'service', 'batch'];
const EVAL_STATUSES = ['pending', 'complete', 'failed', 'canceled'];
const EVAL_TRIGGERED_BY = [
  'job-register',
  'job-deregister',
  'periodic-job',
  'node-update',
  'scheduled',
  'rolling-update',
  'deployment-watcher',
  'failed-follow-up',
  'max-plan-attempts',
];

const generateCountMap = (keysCount, list) => () => {
  const sample = Array(keysCount)
    .fill(null)
    .map(() => pickOne(list))
    .uniq();
  return sample.reduce((hash, key) => {
    hash[key] = faker.random.number({ min: 1, max: 5 });
    return hash;
  }, {});
};

const generateNodesAvailable = generateCountMap(5, DATACENTERS);
const generateClassFiltered = generateCountMap(3, provide(10, faker.hacker.abbreviation));
const generateClassExhausted = generateClassFiltered;
const generateDimensionExhausted = generateCountMap(1, ['cpu', 'mem', 'disk', 'iops']);
const generateQuotaExhausted = generateDimensionExhausted;
const generateScores = generateCountMap(1, ['binpack', 'job-anti-affinity']);
const generateConstraintFiltered = generateCountMap(2, [
  'prop = val',
  'driver = docker',
  'arch = x64',
]);

export default Factory.extend({
  id: () => faker.random.uuid(),

  priority: () => faker.random.number(100),

  type: faker.list.random(...EVAL_TYPES),
  triggeredBy: faker.list.random(...EVAL_TRIGGERED_BY),
  status: faker.list.random(...EVAL_STATUSES),
  statusDescription: () => faker.lorem.sentence(),

  failedTGAllocs: null,

  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),

  waitUntil: null,

  withPlacementFailures: trait({
    status: 'blocked',
    afterCreate(evaluation, server) {
      assignJob(evaluation, server);
      const taskGroups = server.db.taskGroups.where({ jobId: evaluation.jobId });

      const taskGroupNames = taskGroups.mapBy('name');
      const failedTaskGroupsCount = faker.random.number({ min: 1, max: taskGroupNames.length });
      const failedTaskGroupNames = [];
      for (let i = 0; i < failedTaskGroupsCount; i++) {
        failedTaskGroupNames.push(
          ...taskGroupNames.splice(faker.random.number(taskGroupNames.length - 1), 1)
        );
      }

      const placementFailures = failedTaskGroupNames.reduce((hash, name) => {
        hash[name] = generateTaskGroupFailures();
        return hash;
      }, {});

      evaluation.update({
        failedTGAllocs: placementFailures,
      });
    },
  }),

  afterCreate(evaluation, server) {
    assignJob(evaluation, server);
  },
});

function assignJob(evaluation, server) {
  Ember.assert(
    '[Mirage] No jobs! make sure jobs are created before evaluations',
    server.db.jobs.length
  );

  const job = evaluation.jobId ? server.db.jobs.find(evaluation.jobId) : pickOne(server.db.jobs);
  evaluation.update({
    jobId: job.id,
    job_id: job.id,
  });
}

export function generateTaskGroupFailures() {
  return {
    CoalescedFailures: faker.random.number({ min: 1, max: 20 }),
    NodesEvaluated: faker.random.number({ min: 1, max: 100 }),
    NodesExhausted: faker.random.number({ min: 1, max: 100 }),

    NodesAvailable: Math.random() > 0.7 ? generateNodesAvailable() : null,
    ClassFiltered: Math.random() > 0.7 ? generateClassFiltered() : null,
    ConstraintFiltered: Math.random() > 0.7 ? generateConstraintFiltered() : null,
    ClassExhausted: Math.random() > 0.7 ? generateClassExhausted() : null,
    DimensionExhausted: Math.random() > 0.7 ? generateDimensionExhausted() : null,
    QuotaExhausted: Math.random() > 0.7 ? generateQuotaExhausted() : null,
    Scores: Math.random() > 0.7 ? generateScores() : null,
  };
}
