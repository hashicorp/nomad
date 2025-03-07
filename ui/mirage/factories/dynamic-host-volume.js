/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { pickOne } from '../utils';

export default Factory.extend({
  id: () => `${faker.random.uuid()}`,
  name() {
    return faker.hacker.noun();
  },

  pluginID() {
    return faker.hacker.noun();
  },

  state() {
    return 'ready';
  },

  capacityBytes() {
    return 10000000;
  },

  accessModes() {
    return ['single-node-writer'];
  },

  attachmentModes() {
    return ['file-system'];
  },

  path: () => faker.system.filePath(),

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
    if (!volume.nodeId) {
      const node = server.db.nodes.length ? pickOne(server.db.nodes) : null;
      volume.update({
        nodeId: node.id,
      });
    }
  },
});
