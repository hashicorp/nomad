/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assign } from '@ember/polyfills';
import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide, pickOne } from '../utils';
import { DATACENTERS } from '../common';
import { dasherize } from '@ember/string';

const REF_TIME = new Date();
const JOB_PREFIXES = provide(5, faker.hacker.abbreviation);
const JOB_TYPES = ['service', 'batch', 'system', 'sysbatch'];
const JOB_STATUSES = ['pending', 'running', 'dead'];

export default Factory.extend({
  id(i) {
    if (this.parameterized && this.parentId) {
      const shortUUID = faker.random.uuid().split('-')[0];
      const dispatchId = `dispatch-${this.submitTime / 1000}-${shortUUID}`;
      return `${this.parentId}/${dispatchId}`;
    }

    return `${faker.helpers.randomize(JOB_PREFIXES)}-${dasherize(
      faker.hacker.noun()
    )}-${i}`.toLowerCase();
  },

  name() {
    return this.id;
  },

  version: 1,
  submitTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  // When provided, the resourceSpec will inform how many task groups to create
  // and how much of each resource that task group reserves.
  //
  // One task group, 256 MiB memory and 500 MHz cpu
  // resourceSpec: ['M: 256, C: 500']
  //
  // Two task groups
  // resourceSpec: ['M: 256, C: 500', 'M: 1024, C: 1200']
  resourceSpec: null,

  groupsCount() {
    return this.resourceSpec
      ? this.resourceSpec.length
      : faker.random.number({ min: 1, max: 2 });
  },

  region: () => 'global',
  type: () => faker.helpers.randomize(JOB_TYPES),
  priority: () => faker.random.number(100),
  allAtOnce: faker.random.boolean,
  status: () => faker.helpers.randomize(JOB_STATUSES),
  datacenters: () =>
    faker.helpers
      .shuffle(DATACENTERS)
      .slice(0, faker.random.number({ min: 1, max: 4 })),

  childrenCount: () => faker.random.number({ min: 1, max: 2 }),

  meta: null,

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

  periodicSysbatch: trait({
    type: 'sysbatch',
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
    // parameterized job object
    // serializer update for bool vs details object
    parameterizedJob: () => ({
      MetaOptional: generateMetaFields(faker.random.number(10), 'optional'),
      MetaRequired: generateMetaFields(faker.random.number(10), 'required'),
      Payload: faker.random.boolean() ? 'required' : null,
    }),
  }),

  parameterizedSysbatch: trait({
    type: 'sysbatch',
    parameterized: true,
    // parameterized job object
    // serializer update for bool vs details object
    parameterizedJob: () => ({
      MetaOptional: generateMetaFields(faker.random.number(10), 'optional'),
      MetaRequired: generateMetaFields(faker.random.number(10), 'required'),
      Payload: faker.random.boolean() ? 'required' : null,
    }),
  }),

  periodicChild: trait({
    // Periodic children need a parent job,
    // It is the Periodic job's responsibility to create
    // periodicChild jobs and provide a parent job.
    type: 'batch',
  }),

  periodicSysbatchChild: trait({
    // Periodic children need a parent job,
    // It is the Periodic job's responsibility to create
    // periodicChild jobs and provide a parent job.
    type: 'sysbatch',
  }),

  parameterizedChild: trait({
    // Parameterized children need a parent job,
    // It is the Parameterized job's responsibility to create
    // parameterizedChild jobs and provide a parent job.
    type: 'batch',
    parameterized: true,
    dispatched: true,
    payload: window.btoa(faker.lorem.sentence()),
  }),

  parameterizedSysbatchChild: trait({
    // Parameterized children need a parent job,
    // It is the Parameterized job's responsibility to create
    // parameterizedChild jobs and provide a parent job.
    type: 'sysbatch',
    parameterized: true,
    dispatched: true,
    payload: window.btoa(faker.lorem.sentence()),
  }),

  pack: trait({
    meta: () => ({
      'pack.name': faker.hacker.noun(),
      'pack.version': faker.system.semver(),
    }),
  }),

  createIndex: (i) => i,
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

  // When true, all task groups get the noHostVolumes trait
  noHostVolumes: false,

  // When true, allocations for this job will fail and reschedule, randomly succeeding or not
  withRescheduling: false,

  // When true, task groups will have services
  withGroupServices: false,

  // When true, tasks will have services
  withTaskServices: false,

  // When true, dynamic application sizing recommendations will be made
  createRecommendations: false,

  // When true, only task groups and allocations are made
  shallow: false,

  afterCreate(job, server) {
    Ember.assert(
      '[Mirage] No node pools! make sure node pools are created before jobs',
      server.db.nodePools.length
    );

    if (!job.namespaceId) {
      const namespace = server.db.namespaces.length
        ? pickOne(server.db.namespaces).id
        : null;
      job.update({
        namespace,
        namespaceId: namespace,
      });
    } else {
      job.update({
        namespace: job.namespaceId,
      });
    }

    if (!job.nodePool) {
      job.update({
        nodePool: pickOne(server.db.nodePools).name,
      });
    }

    const groupProps = {
      job,
      createAllocations: job.createAllocations,
      withRescheduling: job.withRescheduling,
      withServices: job.withGroupServices,
      withTaskServices: job.withTaskServices,
      createRecommendations: job.createRecommendations,
      shallow: job.shallow,
      allocStatusDistribution: job.allocStatusDistribution,
    };

    if (job.groupTaskCount) {
      groupProps.count = job.groupTaskCount;
    }

    let groups;
    if (job.noHostVolumes) {
      groups = provide(job.groupsCount, (_, idx) =>
        server.create('task-group', 'noHostVolumes', {
          ...groupProps,
          resourceSpec:
            job.resourceSpec &&
            job.resourceSpec.length &&
            job.resourceSpec[idx],
        })
      );
    } else {
      groups = provide(job.groupsCount, (_, idx) =>
        server.create('task-group', {
          ...groupProps,
          resourceSpec:
            job.resourceSpec &&
            job.resourceSpec.length &&
            job.resourceSpec[idx],
        })
      );
    }

    job.update({
      taskGroupIds: groups.mapBy('id'),
    });

    const hasChildren = job.periodic || (job.parameterized && !job.parentId);
    const jobSummary = server.create(
      'job-summary',
      hasChildren ? 'withChildren' : 'withSummary',
      {
        jobId: job.id,
        groupNames: groups.mapBy('name'),
        namespace: job.namespace,
      }
    );

    job.update({
      jobSummaryId: jobSummary.id,
    });

    const jobScale = server.create('job-scale', {
      groupNames: groups.mapBy('name'),
      jobId: job.id,
      namespace: job.namespace,
      shallow: job.shallow,
    });

    job.update({
      jobScaleId: jobScale.id,
    });

    if (!job.noDeployments) {
      Array(faker.random.number({ min: 1, max: 3 }))
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

    if (!job.shallow) {
      const knownEvaluationProperties = {
        jobId: job.id,
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
    }

    if (job.periodic) {
      let childType;
      switch (job.type) {
        case 'batch':
          childType = 'periodicChild';
          break;
        case 'sysbatch':
          childType = 'periodicSysbatchChild';
          break;
      }

      // Create child jobs
      server.createList('job', job.childrenCount, childType, {
        parentId: job.id,
        namespaceId: job.namespaceId,
        namespace: job.namespace,
        datacenters: job.datacenters,
        createAllocations: job.createAllocations,
        shallow: job.shallow,
        noActiveDeployment: job.noActiveDeployment,
      });
    }

    if (job.parameterized && !job.parentId) {
      let childType;
      switch (job.type) {
        case 'batch':
          childType = 'parameterizedChild';
          break;
        case 'sysbatch':
          childType = 'parameterizedSysbatchChild';
          break;
      }

      // Create child jobs
      server.createList('job', job.childrenCount, childType, {
        parentId: job.id,
        namespaceId: job.namespaceId,
        namespace: job.namespace,
        datacenters: job.datacenters,
        createAllocations: job.createAllocations,
        shallow: job.shallow,
        noActiveDeployment: job.noActiveDeployment,
      });
    }
  },
});

function generateMetaFields(num, prefix = '') {
  // Use an object to avoid duplicate meta fields.
  // The prefix param helps to avoid duplicate entries across function calls.
  let meta = {};
  for (let i = 0; i < num; i++) {
    const field = `${prefix}-${faker.hacker.noun()}`;
    meta[field] = true;
  }
  return Object.keys(meta);
}
