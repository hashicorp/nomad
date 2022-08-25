import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';
import { dasherize } from '@ember/string';
import { DATACENTERS } from '../common';
import { pickOne } from '../utils';

export default Factory.extend({
  id: () => faker.random.uuid(),
  address: () => faker.internet.ip(),
  createIndex: () => faker.random.number(),
  modifyIndex: () => faker.random.number(),
  name: () => faker.random.uuid(),
  serviceName: (id) => `${dasherize(faker.hacker.noun())}-${id}-service`,
  datacenter: faker.helpers.randomize(DATACENTERS),
  port: faker.random.number({ min: 5000, max: 60000 }),
  // jobId: (id) => `${dasherize(faker.hacker.noun())}-${id}`,
  // allocId: (id) => `${dasherize(faker.hacker.noun())}-${id}`,
  tags: () => {
    if (!faker.random.boolean()) {
      return provide(
        faker.random.number({ min: 0, max: 2 }),
        faker.hacker.noun.bind(faker.hacker.noun)
      );
    } else {
      return null;
    }
  },

  afterCreate(service, server) {
    if (!service.namespace) {
      const namespace = pickOne(server.db.jobs)?.namespace || 'default';
      service.update({
        namespace,
      });
    }
    if (!service.jobId) {
      service.update({
        jobId: 'service-haver',
      });
    }
    if (!service.allocId) {
      const servicedAlloc = pickOne(
        server.db.allocations.filter((a) => a.jobId === 'service-haver') || []
      );
      if (servicedAlloc) {
        service.update({
          allocId: servicedAlloc.id,
        });
      }
    }
  },
});
