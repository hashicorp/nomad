import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  name: '',

  autoRevert: () => Math.random() > 0.5,
  promoted: () => Math.random() > 0.5,

  desiredTotal: faker.random.number({ min: 1, max: 10 }),

  desiredCanaries() {
    return faker.random.number(Math.floor(this.desiredTotal / 2));
  },

  placedCanaries() {
    return faker.random.number(this.desiredCanaries);
  },

  placedAllocs() {
    return faker.random.number(this.desiredTotal);
  },

  healthyAllocs() {
    return faker.random.number(this.placedAllocs);
  },

  unhealthyAllocs() {
    return this.placedAllocs - this.healthyAllocs;
  },
});
