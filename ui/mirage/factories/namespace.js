import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  id: i => `namespace-${i}`,
  name() {
    return this.id;
  },
  hash: () => faker.random.uuid(),
  description: '',
});
