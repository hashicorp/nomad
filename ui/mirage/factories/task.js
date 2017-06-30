import { Factory, faker } from 'ember-cli-mirage';

// Realistically, resource reservations have a low cardinality
const CPU_RESERVATIONS = [250, 500, 1000, 2000, 2500, 4000];
const MEMORY_RESERVATIONS = [256, 512, 1024, 2048, 4096, 8192];
const DISK_RESERVATIONS = [200, 500, 1000, 2000, 5000, 10000, 100000];
const IOPS_RESERVATIONS = [100000, 250000, 500000, 1000000, 10000000, 20000000];

// There is also a good chance that certain resource restrictions are unbounded
IOPS_RESERVATIONS.push(...Array(1000).fill(0));
DISK_RESERVATIONS.push(...Array(500).fill(0));

export default Factory.extend({
  // Hidden property used to compute the Summary hash
  groupNames: [],

  JobID: '',

  Resources: () => ({
    CPU: faker.list.random(...CPU_RESERVATIONS)(),
    MemoryMB: faker.list.random(...MEMORY_RESERVATIONS)(),
    DiskMB: faker.list.random(...DISK_RESERVATIONS)(),
    IOPS: faker.list.random(...IOPS_RESERVATIONS)(),
  }),
});
