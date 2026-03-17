/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'miragejs';

import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  // Hidden property used to compute the Summary hash
  groupNames: [],

  namespace: null,

  withSummary: trait({
    Summary: function () {
      return this.groupNames.reduce((summary, group) => {
        summary[group] = {
          Queued: faker.random.number(10),
          Complete: faker.random.number(10),
          Failed: faker.random.number(10),
          Running: faker.random.number(10),
          Starting: faker.random.number(10),
          Lost: faker.random.number(10),
          Unknown: faker.random.number(10),
        };
        return summary;
      }, {});
    },
    afterCreate(jobSummary, server) {
      // Update the summary alloc types to match server allocations with same job ID
      const jobAllocs = server.db.allocations.where({
        jobId: jobSummary.jobId,
      });
      const countByStatus = (group, clientStatus) => {
        return jobAllocs.filter(
          (alloc) =>
            alloc.taskGroup === group && alloc.clientStatus === clientStatus,
        ).length;
      };
      let summary = jobSummary.groupNames.reduce((summary, group) => {
        summary[group] = {
          Queued: countByStatus(group, 'pending'),
          Complete: countByStatus(group, 'complete'),
          Failed: countByStatus(group, 'failed'),
          Running: countByStatus(group, 'running'),
          Starting: countByStatus(group, 'starting'),
          Lost: countByStatus(group, 'lost'),
          Unknown: countByStatus(group, 'unknown'),
        };
        return summary;
      }, {});

      jobSummary.update({ summary });
    },
  }),

  withChildren: trait({
    Children: () => ({
      Pending: faker.random.number(10),
      Running: faker.random.number(10),
      Dead: faker.random.number(10),
    }),
  }),
});
