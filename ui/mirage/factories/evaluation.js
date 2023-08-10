/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
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
const REF_TIME = new Date();

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
const generateClassFiltered = generateCountMap(
  3,
  provide(10, faker.hacker.abbreviation)
);
const generateClassExhausted = generateClassFiltered;
const generateDimensionExhausted = generateCountMap(1, [
  'cpu',
  'mem',
  'disk',
  'iops',
]);
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

  type: () => faker.helpers.randomize(EVAL_TYPES),
  triggeredBy: () => faker.helpers.randomize(EVAL_TRIGGERED_BY),
  status: () => faker.helpers.randomize(EVAL_STATUSES),
  statusDescription: () => faker.lorem.sentence(),

  failedTGAllocs: null,

  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),
  modifyTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  createIndex: () => faker.random.number({ min: 10, max: 2000 }),
  createTime() {
    return (
      faker.date.past(2 / 365, new Date(this.modifyTime / 1000000)) * 1000000
    );
  },

  waitUntil: null,

  withPlacementFailures: trait({
    status: 'blocked',
    afterCreate(evaluation, server) {
      assignJob(evaluation, server);
      const taskGroups = server.db.taskGroups.where({
        jobId: evaluation.jobId,
      });

      const taskGroupNames = taskGroups.mapBy('name');
      const failedTaskGroupsCount = faker.random.number({
        min: 1,
        max: taskGroupNames.length,
      });
      const failedTaskGroupNames = [];
      for (let i = 0; i < failedTaskGroupsCount; i++) {
        failedTaskGroupNames.push(
          ...taskGroupNames.splice(
            faker.random.number(taskGroupNames.length - 1),
            1
          )
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
    if (!evaluation.nodeId) {
      assignJob(evaluation, server);
    }
  },
});

function assignJob(evaluation, server) {
  Ember.assert(
    '[Mirage] No jobs! make sure jobs are created before evaluations',
    server.db.jobs.length
  );

  const job = evaluation.jobId
    ? server.db.jobs.find(evaluation.jobId)
    : pickOne(server.db.jobs);
  evaluation.update({
    jobId: job.id,
  });
}

export function generateTaskGroupFailures() {
  return {
    CoalescedFailures: faker.random.number({ min: 1, max: 20 }),
    NodesEvaluated: faker.random.number({ min: 1, max: 100 }),
    NodesExhausted: faker.random.number({ min: 1, max: 100 }),

    NodesAvailable:
      faker.random.number(10) >= 7 ? generateNodesAvailable() : null,
    ClassFiltered:
      faker.random.number(10) >= 7 ? generateClassFiltered() : null,
    ConstraintFiltered:
      faker.random.number(10) >= 7 ? generateConstraintFiltered() : null,
    ClassExhausted:
      faker.random.number(10) >= 7 ? generateClassExhausted() : null,
    DimensionExhausted:
      faker.random.number(10) >= 7 ? generateDimensionExhausted() : null,
    QuotaExhausted:
      faker.random.number(10) >= 7 ? generateQuotaExhausted() : null,
    Scores: faker.random.number(10) >= 7 ? generateScores() : null,
  };
}
