/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'ember-cli-mirage';

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
      let summary = jobSummary.groupNames.reduce((summary, group) => {
        summary[group] = {
          Queued: jobAllocs
            .filterBy('taskGroup', group)
            .filterBy('clientStatus', 'pending').length,
          Complete: jobAllocs
            .filterBy('taskGroup', group)
            .filterBy('clientStatus', 'complete').length,
          Failed: jobAllocs
            .filterBy('taskGroup', group)
            .filterBy('clientStatus', 'failed').length,
          Running: jobAllocs
            .filterBy('taskGroup', group)
            .filterBy('clientStatus', 'running').length,
          Starting: jobAllocs
            .filterBy('taskGroup', group)
            .filterBy('clientStatus', 'starting').length,
          Lost: jobAllocs
            .filterBy('taskGroup', group)
            .filterBy('clientStatus', 'lost').length,
          Unknown: jobAllocs
            .filterBy('taskGroup', group)
            .filterBy('clientStatus', 'unknown').length,
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
