/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { generateResources } from '../common';
import { dasherize } from '@ember/string';
import { pickOne } from '../utils';

const DRIVERS = ['docker', 'java', 'rkt', 'qemu', 'exec', 'raw_exec'];

export default Factory.extend({
  createRecommendations: false,

  withServices: false,

  // Hidden property used to compute the Summary hash
  groupNames: [],

  // Set in the TaskGroup factory
  volumeMounts: [],

  JobID: '',

  name: (id) => `task-${dasherize(faker.hacker.noun())}-${id}`,
  driver: () => faker.helpers.randomize(DRIVERS),

  originalResources: generateResources,
  resources: function () {
    // Generate resources the usual way, but transform to the old
    // shape because that's what the job spec uses.
    const resources = this.originalResources;
    return {
      CPU: resources.Cpu.CpuShares,
      MemoryMB: resources.Memory.MemoryMB,
      MemoryMaxMB: resources.Memory.MemoryMaxMB,
      DiskMB: resources.Disk.DiskMB,
    };
  },

  Lifecycle: (i) => {
    const cycle = i % 6;

    if (cycle === 0) {
      return null;
    } else if (cycle === 1) {
      return { Hook: 'prestart', Sidecar: false };
    } else if (cycle === 2) {
      return { Hook: 'prestart', Sidecar: true };
    } else if (cycle === 3) {
      return { Hook: 'poststart', Sidecar: false };
    } else if (cycle === 4) {
      return { Hook: 'poststart', Sidecar: true };
    } else if (cycle === 5) {
      return { Hook: 'poststop' };
    }
  },

  afterCreate(task, server) {
    if (task.createRecommendations) {
      const recommendations = [];

      if (faker.random.number(10) >= 1) {
        recommendations.push(
          server.create('recommendation', { task, resource: 'CPU' })
        );
      }

      if (faker.random.number(10) >= 1) {
        recommendations.push(
          server.create('recommendation', { task, resource: 'MemoryMB' })
        );
      }

      task.save({ recommendationIds: recommendations.mapBy('id') });
    }

    if (task.withServices) {
      const services = server.createList('service-fragment', 1, {
        provider: 'nomad',
        taskName: task.name,
      });

      services.push(
        server.create('service-fragment', {
          provider: 'consul',
          taskName: task.name,
        })
      );
      services.forEach((fragment) => {
        server.createList('service', 5, {
          serviceName: fragment.name,
        });
      });
      task.update({ services });
    }
  },
});
