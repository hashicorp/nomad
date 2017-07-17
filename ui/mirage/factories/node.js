import { Factory, faker } from 'ember-cli-mirage';
import { provide } from '../utils';
import { DATACENTERS } from '../common';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const NODE_STATUSES = ['initializing', 'ready', 'down'];

export default Factory.extend({
  id: i => (i / 100 >= 1 ? `${UUIDS[i]}-${i}` : UUIDS[i]),
  name: () => `nomad@${faker.internet.ip()}`,

  datacenter: faker.list.random(...DATACENTERS),
  isDraining: faker.random.boolean,
  status: faker.list.random(...NODE_STATUSES),
  tls_enabled: faker.random.boolean,

  http_addr() {
    return this.name.split('@')[1];
  },
});
