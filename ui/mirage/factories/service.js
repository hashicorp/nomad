import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';

export default Factory.extend({
  name: id => `${faker.hacker.noun().dasherize()}-${id}-service`,
  portLabel: () => faker.hacker.noun().dasherize(),
  tags: () => {
    if (!faker.random.boolean()) {
      return provide(
        faker.random.number({ min: 0, max: 2 }),
        faker.hacker.noun.bind(faker.hacker.noun)
      );
    } else {
      return null;
    }
  },
  Connect: {
    SidecarService: {
      Proxy: {
        Upstreams: [
          {
            DestinationName: faker.hacker.noun().dasherize(),
            LocalBindPort: faker.random.number({ min: 5000, max: 60000 }),
          },
        ],
      },
    },
  },
});
