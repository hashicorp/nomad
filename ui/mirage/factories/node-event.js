import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';

const STATES = provide(10, faker.system.fileExt.bind(faker.system));

export default Factory.extend({
  subsystem: () => faker.helpers.randomize(STATES),
  message: () => faker.lorem.sentence(),
  time: () => faker.date.past(2 / 365, new Date()),
  details: null,
});
