import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { STORAGE_PROVIDERS } from '../common';
const REF_TIME = new Date();

export default Factory.extend({
  provider: faker.helpers.randomize(STORAGE_PROVIDERS),
  providerVersion: '1.0.1',

  healthy: faker.random.boolean,
  healthDescription() {
    this.healthy ? 'healthy' : 'unhealthy';
  },

  updateTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  requiresControllerPlugin: true,
  requiresTopologies: true,

  nodeInfo: () => ({
    MaxVolumes: 51,
    AccessibleTopology: {
      key: 'value',
    },
    RequiresNodeStageVolume: true,
  }),

  afterCreate(storageNode, server) {
    const alloc = server.create('allocation', {
      jobId: storageNode.job.id,
    });

    storageNode.update({
      allocId: alloc.id,
      nodeId: alloc.nodeId,
    });
  },
});
