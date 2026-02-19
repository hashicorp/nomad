/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { pickOne } from '../utils';

const REF_TIME = new Date();

export default Factory.extend({
  id: () => `${faker.random.uuid()}`,
  name() {
    return faker.hacker.noun();
  },

  pluginID() {
    return faker.hacker.noun();
  },

  // Nanosecond timestamps matching the Nomad API format
  modifyTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,
  createTime() {
    return (
      faker.date.past(2 / 365, new Date(this.modifyTime / 1000000)) * 1000000
    );
  },

  state() {
    return 'ready';
  },

  capacityBytes() {
    return 10000000;
  },

  requestedCapabilities() {
    return [
      {
        AccessMode: 'single-node-writer',
        AttachmentMode: 'file-system',
      },
      {
        AccessMode: 'single-node-reader-only',
        AttachmentMode: 'block-device',
      },
    ];
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
