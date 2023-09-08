/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { pickOne } from '../utils';
import { STORAGE_PROVIDERS } from '../common';
import { dasherize } from '@ember/string';

const ACCESS_MODES = ['multi-node-single-writer'];
const ATTACHMENT_MODES = ['file-system'];

export default Factory.extend({
  id: (i) => `${dasherize(faker.hacker.noun())}-${i}`.toLowerCase(),
  name() {
    return this.id;
  },

  externalId: () => `vol-${faker.random.uuid().split('-')[0]}`,

  // Topologies is currently unused by the UI. This should
  // eventually become dynamic.
  topologies: () => [{ foo: 'bar' }],

  accessMode: faker.helpers.randomize(ACCESS_MODES),
  attachmentMode: faker.helpers.randomize(ATTACHMENT_MODES),

  schedulable: faker.random.boolean,
  provider: faker.helpers.randomize(STORAGE_PROVIDERS),
  version: '1.0.1',
  controllerRequired: faker.random.boolean,
  controllersHealthy: () => faker.random.number(10),
  controllersExpected() {
    return this.controllersHealthy + faker.random.number(10);
  },
  nodesHealthy: () => faker.random.number(10),
  nodesExpected() {
    return this.nodesHealthy + faker.random.number(10);
  },

  afterCreate(volume, server) {
    if (!volume.namespaceId) {
      const namespace = server.db.namespaces.length
        ? pickOne(server.db.namespaces).id
        : null;
      volume.update({
        namespace,
        namespaceId: namespace,
      });
    } else {
      volume.update({
        namespace: volume.namespaceId,
      });
    }

    if (!volume.plugin) {
      const plugin = server.db.csiPlugins.length
        ? pickOne(server.db.csiPlugins)
        : null;
      volume.update({
        PluginId: plugin && plugin.id,
      });
    } else {
      volume.update({
        PluginId: volume.plugin.id,
      });
    }
  },
});
