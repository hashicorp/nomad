import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  allDirStats: () => ({
    Available: 0,
    Device: '',
    InodesUsedPercent: 0.1,
    Mountpoint: '',
    Size: 200000000000,
    Used: 100000000000,
    UsedPercent: 50.0,
  }),

  CPU: () => [
    Array(faker.list.random(1, 2, 4, 6, 8, 12, 16, 24, 32)).fill(0).map((cpu, index) => ({
      CPU: `cpu${index}`,
      Idle: 20,
      System: 40,
      Total: 80,
      User: 40,
    })),
  ],

  CPUTicksConsumed: 1000000,

  diskStats: () => [
    Array(faker.random.number({ min: 1, max: 5 })).fill(0).map((disk, index) => ({
      Available: 100000000000,
      Device: `/dev/disk${index}`,
      InodesUsedPercent: 0.10000000001,
      Mountpoint: '/',
      Size: 2000000000000,
      Used: 1000000000000,
      UsedPercent: 50.0,
    })),
  ],

  memory: () => ({
    Available: 60000000000,
    Free: 25000000000,
    Total: 95000000000,
    Used: 10000000000,
  }),

  timestamp: 149000000000,
  uptime: 193838,
});
