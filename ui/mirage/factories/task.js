import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { generateResources } from '../common';

const DRIVERS = ['docker', 'java', 'rkt', 'qemu', 'exec', 'raw_exec'];

export default Factory.extend({
  // Hidden property used to compute the Summary hash
  groupNames: [],

  // Set in the TaskGroup factory
  volumeMounts: [],

  JobID: '',

  name: id => `task-${faker.hacker.noun().dasherize()}-${id}`,
  driver: () => faker.helpers.randomize(DRIVERS),

  Resources: generateResources,

  Lifecycle: i => {
    const cycle = i % 4;

    if (cycle === 0) {
      return null;
    } else if (cycle === 1) {
      return { Hook: 'prestart', Sidecar: false };
    } else if (cycle === 2) {
      return { Hook: 'prestart', Sidecar: true };
    } else {
      return { Hook: 'poststop' };
    }
  },
});
