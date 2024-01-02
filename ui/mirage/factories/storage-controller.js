/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { STORAGE_PROVIDERS } from '../common';
const REF_TIME = new Date();

export default Factory.extend({
  provider: faker.helpers.randomize(STORAGE_PROVIDERS),
  providerVersion: '1.0.1',

  healthy: i => [true, false][i % 2],
  healthDescription() {
    this.healthy ? 'healthy' : 'unhealthy';
  },

  updateTime: () => faker.date.past(2 / 365, REF_TIME),

  requiresControllerPlugin: true,
  requiresTopologies: true,

  shallow: false,

  controllerInfo: () => ({
    SupportsReadOnlyAttach: true,
    SupportsAttachDetach: true,
    SupportsListVolumes: true,
    SupportsListVolumesAttachedNodes: false,
  }),

  afterCreate(storageController, server) {
    const alloc = server.create('allocation', {
      jobId: storageController.job.id,
      forceRunningClientStatus: true,
      modifyTime: storageController.updateTime * 1000000,
      shallow: storageController.shallow,
    });

    storageController.update({
      allocID: alloc.id,
      nodeId: alloc.nodeId,
    });
  },
});
