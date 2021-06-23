import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';
import { DATACENTERS } from '../common';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const AGENT_STATUSES = ['alive', 'leaving', 'left', 'failed'];

export default Factory.extend({
  id: i => (i / 100 >= 1 ? `${UUIDS[i]}-${i}` : UUIDS[i]),
  name: () => `nomad@${faker.random.boolean() ? faker.internet.ip() : faker.internet.ipv6()}`,

  status: () => faker.helpers.randomize(AGENT_STATUSES),
  serfPort: () => faker.random.number({ min: 4000, max: 4999 }),

  address() {
    return this.name.split('@')[1];
  },

  tags() {
    const rpcPortCandidate = faker.random.number({ min: 4000, max: 4999 });
    return {
      port: rpcPortCandidate === this.serfPort ? rpcPortCandidate + 1 : rpcPortCandidate,
      dc: faker.helpers.randomize(DATACENTERS),
    };
  },
});
