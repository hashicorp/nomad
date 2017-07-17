import { faker } from 'ember-cli-mirage';
import { provide } from './utils';

export const DATACENTERS = provide(
  15,
  (n, i) => `${faker.address.countryCode().toLowerCase()}${i}`
);
