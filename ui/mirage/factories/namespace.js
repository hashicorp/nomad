import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  id: i => (i === 0 ? 'default' : `namespace-${i}`),

  name() {
    return this.id;
  },

  hash: () => faker.random.uuid(),
  description: '',
});
