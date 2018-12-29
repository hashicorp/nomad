import { Factory, faker } from 'ember-cli-mirage';
import { generateResources } from '../common';

export default Factory.extend({
  // Hidden property used to compute the Summary hash
  groupNames: [],

  JobID: '',

  name: id => `task-${faker.hacker.noun()}-${id}`,

  Resources: generateResources,
});
