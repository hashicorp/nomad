import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { pickOne } from '../utils';
import { provide } from '../utils';

export default Factory.extend({
  ServiceName: () => `${faker.hacker.adjective()}-${faker.hacker.noun()}`,
  Tags: () => {
    if (!faker.random.boolean()) {
      return provide(
        faker.random.number({ min: 0, max: 2 }),
        faker.hacker.noun.bind(faker.hacker.noun)
      );
    } else {
      return null;
    }
  },
});
