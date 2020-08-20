import faker from 'nomad-ui/mirage/faker';
import { provide } from './utils';

// Realistically, resource reservations have a low cardinality
const CPU_RESERVATIONS = [250, 500, 1000, 2000, 2500, 4000];
const MEMORY_RESERVATIONS = [256, 512, 1024, 2048, 4096, 8192];
const DISK_RESERVATIONS = [200, 500, 1000, 2000, 5000, 10000, 100000];
const IOPS_RESERVATIONS = [100000, 250000, 500000, 1000000, 10000000, 20000000];

// There is also a good chance that certain resource restrictions are unbounded
IOPS_RESERVATIONS.push(...Array(1000).fill(0));
DISK_RESERVATIONS.push(...Array(500).fill(0));

const NETWORK_MODES = ['bridge', 'host'];

export const DATACENTERS = provide(
  15,
  (n, i) => `${faker.address.countryCode().toLowerCase()}${i}`
);

export const HOSTS = provide(100, () => {
  const ip = faker.random.boolean() ? faker.internet.ip() : `[${faker.internet.ipv6()}]`;
  return `${ip}:${faker.random.number({ min: 4000, max: 4999 })}`;
});

export const STORAGE_PROVIDERS = ['ebs', 'zfs', 'nfs', 'cow', 'moo'];

export function generateResources(options = {}) {
  return {
    CPU: faker.helpers.randomize(CPU_RESERVATIONS),
    MemoryMB: faker.helpers.randomize(MEMORY_RESERVATIONS),
    DiskMB: faker.helpers.randomize(DISK_RESERVATIONS),
    IOPS: faker.helpers.randomize(IOPS_RESERVATIONS),
    Networks: generateNetworks(options.networks),
    Ports: generatePorts(options.networks),
  };
}

export function generateNetworks(options = {}) {
  return Array(faker.random.number({ min: 1, max: 3 }))
    .fill(null)
    .map(() => ({
      Device: `eth${faker.random.number({ max: 5 })}`,
      CIDR: '',
      IP: faker.internet.ip(),
      MBits: 10,
      Mode: faker.helpers.randomize(NETWORK_MODES),
      ReservedPorts: Array(
        faker.random.number({
          min: options.minPorts != null ? options.minPorts : 0,
          max: options.maxPorts != null ? options.maxPorts : 2,
        })
      )
        .fill(null)
        .map(() => ({
          Label: faker.hacker.noun(),
          Value: faker.random.number({ min: 5000, max: 60000 }),
          To: faker.random.number({ min: 5000, max: 60000 }),
        })),
      DynamicPorts: Array(
        faker.random.number({
          min: options.minPorts != null ? options.minPorts : 0,
          max: options.maxPorts != null ? options.maxPorts : 2,
        })
      )
        .fill(null)
        .map(() => ({
          Label: faker.hacker.noun(),
          Value: faker.random.number({ min: 5000, max: 60000 }),
          To: faker.random.number({ min: 5000, max: 60000 }),
        })),
    }));
}

export function generatePorts(options = {}) {
  return Array(faker.random.number({
    min: options.minPorts != null ? options.minPorts : 0,
    max: options.maxPorts != null ? options.maxPorts : 2
  }))
    .fill(null)
    .map(() => ({
      Label: faker.hacker.noun(),
      Value: faker.random.number({ min: 5000, max: 60000 }),
      To: faker.random.number({ min: 5000, max: 60000 }),
      HostIP: faker.random.boolean() ? faker.internet.ip() : faker.internet.ipv6(),
    }))
}