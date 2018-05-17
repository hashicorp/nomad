import { Factory, faker, trait } from 'ember-cli-mirage';
import { provide } from '../utils';

const REF_TIME = new Date();
const STATES = provide(10, faker.system.fileExt.bind(faker.system));

export default Factory.extend({
  type: faker.list.random(...STATES),

  signal: () => '',
  exitCode: () => null,
  time: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  displayMessage: () => faker.lorem.sentence(),
});
