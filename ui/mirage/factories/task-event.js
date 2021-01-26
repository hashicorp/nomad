import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';

const STATES = provide(10, faker.system.fileExt.bind(faker.system));

export default Factory.extend({
  type: () => faker.helpers.randomize(STATES),

  signal: () => '',
  exitCode: () => null,
  time: () => faker.date.past(2 / 365, new Date()) * 1000000,

  displayMessage: () => faker.lorem.sentence(),
});
