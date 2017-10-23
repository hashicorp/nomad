import { Factory, faker } from 'ember-cli-mirage';
import { provide, provider, pickOne } from '../utils';
import { DATACENTERS } from '../common';

const JOB_PREFIXES = provide(5, faker.hacker.abbreviation);
const JOB_TYPES = ['service', 'batch', 'system'];
const JOB_STATUSES = ['pending', 'running', 'dead'];

export default Factory.extend({
  id: i => `job-${i}`,
  name: i => `${faker.list.random(...JOB_PREFIXES)()}-${faker.hacker.noun()}-${i}`,

  groupsCount: () => faker.random.number({ min: 1, max: 5 }),

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

  // Directive used to control sub-resources

  // When false, no allocations are made
  createAllocations: true,

  // When true, deployments for the job will never have a 'running' status
  noActiveDeployment: false,

  // When true, deployments for the job will always have a 'running' status
  activeDeployment: false,

  afterCreate(job, server) {
    const groups = server.createList('task-group', job.groupsCount, {
      job,
      createAllocations: job.createAllocations,
    });

    job.update({
      taskGroupIds: groups.mapBy('id'),
      task_group_ids: groups.mapBy('id'),
    });

    if (!job.namespaceId) {
      const namespace = server.db.namespaces.length ? pickOne(server.db.namespaces).id : null;
      job.update({
        namespace,
        namespaceId: namespace,
      });
    } else {
      job.update({
        namespace: job.namespaceId,
      });
    }

    const jobSummary = server.create('job-summary', {
      groupNames: groups.mapBy('name'),
      job,
    });

    job.update({
      jobSummaryId: jobSummary.id,
      job_summary_id: jobSummary.id,
    });

    Array(faker.random.number({ min: 1, max: 10 }))
      .fill(null)
      .map((_, index) => {
        return server.create('job-version', {
          job,
          version: index,
          noActiveDeployment: job.noActiveDeployment,
          activeDeployment: job.activeDeployment,
        });
      });
  },
});
