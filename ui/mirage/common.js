import { faker } from 'ember-cli-mirage';
import { provide } from './utils';

// Realistically, resource reservations have a low cardinality
const CPU_RESERVATIONS = [250, 500, 1000, 2000, 2500, 4000];
const MEMORY_RESERVATIONS = [256, 512, 1024, 2048, 4096, 8192];
const DISK_RESERVATIONS = [200, 500, 1000, 2000, 5000, 10000, 100000];
const IOPS_RESERVATIONS = [100000, 250000, 500000, 1000000, 10000000, 20000000];

// There is also a good chance that certain resource restrictions are unbounded
IOPS_RESERVATIONS.push(...Array(1000).fill(0));
DISK_RESERVATIONS.push(...Array(500).fill(0));

export const DATACENTERS = provide(
  15,
  (n, i) => `${faker.address.countryCode().toLowerCase()}${i}`
);

export const HOSTS = provide(100, () => {
  const ip = Math.random() > 0.5 ? faker.internet.ip() : `[${ipv6()}]`;
  return `${ip}:${faker.random.number({ min: 4000, max: 4999 })}`;
});

export function generateResources(options = {}) {
  return {
    CPU: faker.random.arrayElement(CPU_RESERVATIONS),
    MemoryMB: faker.random.arrayElement(MEMORY_RESERVATIONS),
    DiskMB: faker.random.arrayElement(DISK_RESERVATIONS),
    IOPS: faker.random.arrayElement(IOPS_RESERVATIONS),
    Networks: generateNetworks(options.networks),
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
        })),
    }));
}

// Faker v4.0 has a built-in ipv6 function. Once Mirage upgrades,
// this code can be removed.
function ipv6() {
  const subnets = [];
  for (var i = 0; i < 8; i++) {
    var subnet = [];
    for (var char = 0; char < 4; char++) {
      subnet.push(faker.random.number(16).toString(16));
    }
    subnets.push(subnet.join(''));
  }
  return subnets.join(':');
}
