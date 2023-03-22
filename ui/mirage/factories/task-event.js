import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';

const REF_TIME = new Date();
const STATES = provide(10, faker.system.fileExt.bind(faker.system));

export default Factory.extend({
  type: () => faker.helpers.randomize(STATES),

  signal: () => '',
  exitCode: () => null,
  time: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  // TODO: I think this is probably an archaic property name. See if tests collapse with this removal and decide to remove then.
  // displayMessage: () => faker.lorem.sentence(),
  message: () => faker.lorem.sentence(),
});
