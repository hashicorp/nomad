/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import faker from 'nomad-ui/mirage/faker';
import { provide } from './utils';

faker.seed(1);

// Realistically, resource reservations have a low cardinality
const CPU_RESERVATIONS = [250, 500, 1000, 2000, 2500, 4000];
const MEMORY_RESERVATIONS = [256, 512, 1024, 2048, 4096, 8192];
const DISK_RESERVATIONS = [200, 500, 1000, 2000, 5000, 10000, 100000];

// There is also a good chance that certain resource restrictions are unbounded
DISK_RESERVATIONS.push(...Array(500).fill(0));

const NETWORK_MODES = ['bridge', 'host'];

export const DATACENTERS = provide(
  15,
  (n, i) => `${faker.address.countryCode().toLowerCase()}${i}`
);

export const HOSTS = provide(100, () => {
  const ip = faker.random.boolean()
    ? faker.internet.ip()
    : `[${faker.internet.ipv6()}]`;
  return `${ip}:${faker.random.number({ min: 4000, max: 4999 })}`;
});

export const STORAGE_PROVIDERS = ['ebs', 'zfs', 'nfs', 'cow', 'moo'];

export function generateResources(options = {}) {
  const resources = {
    Cpu: {
      CpuShares: options.CPU || faker.helpers.randomize(CPU_RESERVATIONS),
    },
    Memory: {
      MemoryMB:
        options.MemoryMB || faker.helpers.randomize(MEMORY_RESERVATIONS),
    },
    Disk: {
      DiskMB: options.DiskMB || faker.helpers.randomize(DISK_RESERVATIONS),
    },
    Networks: generateNetworks(options.networks),
    Ports: generatePorts(options.networks),
  };

  if (faker.random.boolean()) {
    const higherMemoryReservations = MEMORY_RESERVATIONS.filter(
      (mb) => mb > resources.Memory.MemoryMB
    );
    resources.Memory.MemoryMaxMB =
      faker.helpers.randomize(higherMemoryReservations) ||
      resources.Memory.MemoryMB + 1;
  } else {
    resources.Memory.MemoryMaxMB = 0;
  }

  return resources;
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
  return Array(
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
      HostIP: faker.random.boolean()
        ? faker.internet.ip()
        : faker.internet.ipv6(),
    }));
}
