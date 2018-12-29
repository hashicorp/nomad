import { Factory, faker } from 'ember-cli-mirage';

const REF_TIME = new Date();

export default Factory.extend({
  name: '',

  autoRevert: () => Math.random() > 0.5,
  promoted: () => Math.random() > 0.5,

  requiresPromotion: false,

  requireProgressBy: () => faker.date.past(0.5 / 365, REF_TIME),

  desiredTotal: faker.random.number({ min: 1, max: 10 }),

  desiredCanaries() {
    return faker.random.number(Math.floor(this.desiredTotal / 2));
  },

  // PlacedCanaries is an array of allocation IDs. Since the IDs aren't currently
  // used for associating allocations, any random value will do for now.
  placedCanaries() {
    return Array(faker.random.number(this.desiredCanaries))
      .fill(null)
      .map(() => faker.random.uuid());
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
