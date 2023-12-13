/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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

  controllersHealthy() {
    if (!this.controllerRequired) return 0;
    return faker.random.number(3);
  },
  controllersExpected() {
    // This property must be read before the conditional
    // or Mirage will incorrectly sort dependent properties.
    const healthy = this.controllersHealthy;
    if (!this.controllerRequired) return 0;
    return healthy + faker.random.number({ min: 1, max: 2 });
  },

  nodesHealthy: () => faker.random.number(3),
  nodesExpected() {
    return this.nodesHealthy + faker.random.number({ min: 1, max: 2 });
  },

  // Internal property to determine whether or not this plugin
  // Should create one or two Jobs to represent Node and
  // Controller plugins.
  isMonolith() {
    if (!this.controllerRequired) return false;
    return faker.random.boolean;
  },

  // When false, the plugin will not make its own volumes
  createVolumes: true,

  // When true, doesn't create any resources, state, or events for associated allocations
  shallow: false,

  afterCreate(plugin, server) {
    let storageNodes;
    let storageControllers;

    if (plugin.isMonolith) {
      const pluginJob = server.create('job', { type: 'service', createAllocations: false });
      const count = plugin.nodesExpected;
      storageNodes = server.createList('storage-node', count, {
        job: pluginJob,
        shallow: plugin.shallow,
      });
      storageControllers = server.createList('storage-controller', count, {
        job: pluginJob,
        shallow: plugin.shallow,
      });
    } else {
      const controllerJob =
        plugin.controllerRequired &&
        server.create('job', {
          type: 'service',
          createAllocations: false,
          shallow: plugin.shallow,
        });
      const nodeJob = server.create('job', {
        type: 'service',
        createAllocations: false,
        shallow: plugin.shallow,
      });
      storageNodes = server.createList('storage-node', plugin.nodesExpected, {
        job: nodeJob,
        shallow: plugin.shallow,
      });
      storageControllers =
        plugin.controllerRequired &&
        server.createList('storage-controller', plugin.controllersExpected, {
          job: controllerJob,
          shallow: plugin.shallow,
        });
    }

    plugin.update({
      controllers: storageControllers,
      nodes: storageNodes,
    });

    if (plugin.createVolumes) {
      server.createList('csi-volume', faker.random.number({ min: 1, max: 5 }), {
        plugin,
        provider: plugin.provider,
      });
    }
  },
});
