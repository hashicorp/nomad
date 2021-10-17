import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { generateResources } from '../common';

const DRIVERS = ['docker', 'java', 'rkt', 'qemu', 'exec', 'raw_exec'];

export default Factory.extend({
  createRecommendations: false,

  // Hidden property used to compute the Summary hash
  groupNames: [],

  // Set in the TaskGroup factory
  volumeMounts: [],

  JobID: '',

  name: id => `task-${faker.hacker.noun().dasherize()}-${id}`,
  driver: () => faker.helpers.randomize(DRIVERS),

  originalResources: generateResources,
  resources: function() {
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

  Lifecycle: i => {
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
        recommendations.push(server.create('recommendation', { task, resource: 'CPU' }));
      }

      if (faker.random.number(10) >= 1) {
        recommendations.push(server.create('recommendation', { task, resource: 'MemoryMB' }));
      }

      task.save({ recommendationIds: recommendations.mapBy('id') });
    }
  },
});
