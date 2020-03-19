import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { STORAGE_PROVIDERS } from '../common';

export default Factory.extend({
  id: () => faker.random.uuid(),

  // Topologies is currently unused by the UI. This should
  // eventually become dynamic.
  topologies: () => [{ foo: 'bar' }],

  provider: faker.helpers.randomize(STORAGE_PROVIDERS),
  version: '1.0.1',
  controllerRequired: faker.random.boolean,
  controllersHealthy: () => faker.random.number(10),

  nodesHealthy: () => faker.random.number(10),

  // Internal property to determine whether or not this plugin
  // Should create one or two Jobs to represent Node and
  // Controller plugins.
  isMonolith: faker.random.boolean,

  afterCreate(plugin, server) {
    let storageNodes;
    let storageControllers;

    if (plugin.isMonolith) {
      const pluginJob = server.create('job', { type: 'service', createAllocations: false });
      const count = faker.random.number({ min: 1, max: 5 });
      storageNodes = server.createList('storage-node', count, { job: pluginJob });
      storageControllers = server.createList('storage-controller', count, { job: pluginJob });
    } else {
      const controllerJob = server.create('job', { type: 'service', createAllocations: false });
      const nodeJob = server.create('job', { type: 'service', createAllocations: false });
      storageNodes = server.createList('storage-node', faker.random.number({ min: 1, max: 5 }), {
        job: nodeJob,
      });
      storageControllers = server.createList(
        'storage-controller',
        faker.random.number({ min: 1, max: 5 }),
        { job: controllerJob }
      );
    }

    plugin.update({
      controllers: storageControllers,
      nodes: storageNodes,
    });

    server.createList('csi-volume', faker.random.number(5), {
      plugin,
      provider: plugin.provider,
    });
  },
});
