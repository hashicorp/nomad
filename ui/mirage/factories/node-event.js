import { Factory, faker } from 'ember-cli-mirage';
import { provide } from '../utils';

const REF_TIME = new Date();
const STATES = provide(10, faker.system.fileExt.bind(faker.system));

export default Factory.extend({
  subsystem: faker.list.random(...STATES),
  message: () => faker.lorem.sentence(),
  time: () => faker.date.past(2 / 365, REF_TIME),
  details: null,
});
