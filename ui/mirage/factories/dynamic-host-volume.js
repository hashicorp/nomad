/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { pickOne } from '../utils';
import { dasherize } from '@ember/string';

export default Factory.extend({
  id: (i) => `${dasherize(faker.hacker.noun())}-${i}`.toLowerCase(),
  name() {
    return this.id;
  },

  pluginID() {
    return faker.hacker.noun();
  },

  state() {
    return 'ready';
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
