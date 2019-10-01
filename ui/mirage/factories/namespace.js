import { Factory } from 'ember-cli-mirage';

import faker from 'faker';

export default Factory.extend({
  id: i => (i === 0 ? 'default' : `namespace-${i}`),

  name() {
    return this.id;
  },

  hash: () => faker.random.uuid(),
  description: '',
});
