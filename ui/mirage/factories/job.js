import { Factory, faker } from 'ember-cli-mirage';
import { provide, provider } from '../utils';
import { DATACENTERS } from '../common';

const JOB_PREFIXES = provide(5, faker.hacker.abbreviation);
const JOB_TYPES = ['service', 'batch', 'system'];
const JOB_STATUSES = ['pending', 'running', 'dead'];

export default Factory.extend({
  id: i => `job-${i}`,
  name: i => `${faker.list.random(...JOB_PREFIXES)()}-${faker.hacker.noun()}-${i}`,

  groupsCount: () => faker.random.number({ min: 1, max: 10 }),

  region: () => 'global',
  type: faker.list.random(...JOB_TYPES),
  priority: () => faker.random.number(200),
  all_at_once: faker.random.boolean,
  status: faker.list.random(...JOB_STATUSES),
  datacenters: provider(
    () => faker.random.number({ min: 1, max: 4 }),
    faker.list.random(...DATACENTERS)
  ),

  periodic: () => Math.random() > 0.5,
  parameterized() {
    return !this.periodic;
  },

  createIndex: i => i,
  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),

  afterCreate(job, server) {
    const groups = server.createList('task-group', job.groupsCount, {
      job,
    });

    job.update({
      taskGroupIds: groups.mapBy('id'),
      task_group_ids: groups.mapBy('id'),
    });

    const jobSummary = server.create('job-summary', {
      groupNames: groups.mapBy('name'),
      job,
    });

    job.update({
      jobSummaryId: jobSummary.id,
      job_summary_id: jobSummary.id,
    });
  },
});
