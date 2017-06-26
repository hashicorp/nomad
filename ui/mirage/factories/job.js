import { Factory, faker } from 'ember-cli-mirage';
import { provide, provider } from '../utils';

const JOB_PREFIXES = provide(5, faker.hacker.abbreviation);
const JOB_TYPES = ['service', 'batch', 'system'];
const JOB_STATUSES = ['pending', 'running', 'dead'];
const DATACENTERS = provide(15, (n, i) => `${faker.address.countryCode().toLowerCase()}${i}`);

export default Factory.extend({
  id: i => `job-${i}`,
  name: i => `${faker.list.random(...JOB_PREFIXES)()}-${faker.hacker.noun()}-${i}`,

  region: () => 'global',
  type: faker.list.random(...JOB_TYPES),
  priority: () => faker.random.number(200),
  all_at_once: faker.random.boolean,
  status: faker.list.random(...JOB_STATUSES),
  datacenters: provider(
    () => faker.random.number({ min: 1, max: 4 }),
    faker.list.random(...DATACENTERS)
  ),

  job_summary: () => ({
    Summary: {
      cache: {
        Queued: faker.random.number(10),
        Complete: faker.random.number(10),
        Failed: faker.random.number(10),
        Running: faker.random.number(10),
        Starting: faker.random.number(10),
        Lost: faker.random.number(10),
      },
    },
    Children: {
      Pending: faker.random.number(3),
      Running: faker.random.number(3),
      Dead: faker.random.number(3),
    },
  }),

  task_groups: () => [],

  afterCreate(job, server) {
    job.update({
      task_groups: server.createList('task-group', faker.random.number({ min: 1, max: 10 })),
    });
  },
});
