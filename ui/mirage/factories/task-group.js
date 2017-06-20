import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  name: id => `group-${id}`,
  count: () => faker.random.number({ min: 1, max: 20 }),
});
