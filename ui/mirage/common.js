import { faker } from 'ember-cli-mirage';
import { provide } from './utils';

export const DATACENTERS = provide(
  15,
  (n, i) => `${faker.address.countryCode().toLowerCase()}${i}`
);

export const HOSTS = provide(
  100,
  () => `${faker.internet.ip()}:${faker.random.number({ min: 4000, max: 4999 })}`
);
