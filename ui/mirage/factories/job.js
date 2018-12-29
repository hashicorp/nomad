import { assign } from '@ember/polyfills';
import { Factory, faker, trait } from 'ember-cli-mirage';
import { provide, provider, pickOne } from '../utils';
import { DATACENTERS } from '../common';

const JOB_PREFIXES = provide(5, faker.hacker.abbreviation);
const JOB_TYPES = ['service', 'batch', 'system'];
const JOB_STATUSES = ['pending', 'running', 'dead'];

export default Factory.extend({
  id: i =>
    `${faker.list.random(...JOB_PREFIXES)()}-${faker.hacker.noun().dasherize()}-${i}`.toLowerCase(),

  name() {
    return this.id;
  },

  groupsCount: () => faker.random.number({ min: 1, max: 5 }),

  region: () => 'global',
  type: faker.list.random(...JOB_TYPES),
  priority: () => faker.random.number(100),
  all_at_once: faker.random.boolean,
  status: faker.list.random(...JOB_STATUSES),
  datacenters: provider(
    () => faker.random.number({ min: 1, max: 4 }),
    faker.list.random(...DATACENTERS)
  ),

  childrenCount: () => faker.random.number({ min: 1, max: 5 }),

  periodic: trait({
    type: 'batch',
    periodic: true,
    // periodic details object
    // serializer update for bool vs details object
    periodicDetails: () => ({
      Enabled: true,
      ProhibitOverlap: true,
      Spec: '*/5 * * * * *',
      SpecType: 'cron',
      TimeZone: 'UTC',
    }),
  }),

  parameterized: trait({
    type: 'batch',
    parameterized: true,
    // parameterized details object
    // serializer update for bool vs details object
    parameterizedDetails: () => ({
      MetaOptional: null,
      MetaRequired: null,
      Payload: Math.random() > 0.5 ? 'required' : null,
    }),
  }),

  periodicChild: trait({
    // Periodic children need a parent job,
    // It is the Periodic job's responsibility to create
    // periodicChild jobs and provide a parent job.
    type: 'batch',
  }),

  parameterizedChild: trait({
    // Parameterized children need a parent job,
    // It is the Parameterized job's responsibility to create
    // parameterizedChild jobs and provide a parent job.
    type: 'batch',
    payload: window.btoa(faker.lorem.sentence()),
  }),

  createIndex: i => i,
  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),

  // Directive used to control sub-resources

  // When false, no allocations are made
  createAllocations: true,

  // When true, deployments for the job will never have a 'running' status
  noActiveDeployment: false,

  // When true, deployments for the job will always have a 'running' status
  activeDeployment: false,

  // When true, the job will have no versions or deployments (and in turn no latest deployment)
  noDeployments: false,

  // When true, an evaluation with a high modify index and placement failures is created
  failedPlacements: false,

  // When true, no evaluations have failed placements
  noFailedPlacements: false,

  // When true, allocations for this job will fail and reschedule, randomly succeeding or not
  withRescheduling: false,

  afterCreate(job, server) {
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

    const groups = server.createList('task-group', job.groupsCount, {
      job,
      createAllocations: job.createAllocations,
      withRescheduling: job.withRescheduling,
    });

    job.update({
      taskGroupIds: groups.mapBy('id'),
      task_group_ids: groups.mapBy('id'),
    });

    const hasChildren = job.periodic || job.parameterized;
    const jobSummary = server.create('job-summary', hasChildren ? 'withChildren' : 'withSummary', {
      groupNames: groups.mapBy('name'),
      job,
      job_id: job.id,
      JobID: job.id,
      namespace: job.namespace,
    });

    job.update({
      jobSummaryId: jobSummary.id,
      job_summary_id: jobSummary.id,
    });

    if (!job.noDeployments) {
      Array(faker.random.number({ min: 1, max: 10 }))
        .fill(null)
        .map((_, index) => {
          return server.create('job-version', {
            job,
            namespace: job.namespace,
            version: index,
            noActiveDeployment: job.noActiveDeployment,
            activeDeployment: job.activeDeployment,
          });
        });
    }

    const knownEvaluationProperties = {
      job,
      namespace: job.namespace,
    };
    server.createList(
      'evaluation',
      faker.random.number({ min: 1, max: 5 }),
      knownEvaluationProperties
    );
    if (!job.noFailedPlacements) {
      server.createList(
        'evaluation',
        faker.random.number(3),
        'withPlacementFailures',
        knownEvaluationProperties
      );
    }

    if (job.failedPlacements) {
      server.create(
        'evaluation',
        'withPlacementFailures',
        assign(knownEvaluationProperties, {
          modifyIndex: 4000,
        })
      );
    }

    if (job.periodic) {
      // Create periodicChild jobs
      server.createList('job', job.childrenCount, 'periodicChild', {
        parentId: job.id,
        namespaceId: job.namespaceId,
        namespace: job.namespace,
        createAllocations: job.createAllocations,
      });
    }

    if (job.parameterized) {
      // Create parameterizedChild jobs
      server.createList('job', job.childrenCount, 'parameterizedChild', {
        parentId: job.id,
        namespaceId: job.namespaceId,
        namespace: job.namespace,
        createAllocations: job.createAllocations,
      });
    }
  },
});
