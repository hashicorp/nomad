import { Factory, faker } from 'ember-cli-mirage';
import { generateResources } from '../common';

const DRIVERS = ['docker', 'java', 'rkt', 'qemu', 'exec', 'raw_exec'];

export default Factory.extend({
  // Hidden property used to compute the Summary hash
  groupNames: [],

  JobID: '',

  name: id => `task-${faker.hacker.noun().dasherize()}-${id}`,
  driver: faker.list.random(...DRIVERS),

  Resources: generateResources,
});
