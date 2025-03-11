/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import ApplicationSerializer from './application';

export default class NodeSerializer extends ApplicationSerializer {
  @service config;

  attrs = {
    isDraining: 'Drain',
    httpAddr: 'HTTPAddr',
    resources: 'NodeResources',
    reserved: 'ReservedResources',
  };

  mapToArray = ['Drivers', 'HostVolumes'];

  normalize(modelClass, hash) {
    if (hash.HostVolumes) {
      Object.entries(hash.HostVolumes).forEach(([key, value]) => {
        hash.HostVolumes[key].VolumeID = value.ID || undefined;
      });
    }
    return super.normalize(...arguments);
  }

  extractRelationships(modelClass, hash) {
    const { modelName } = modelClass;
    const nodeURL = this.store
      .adapterFor(modelName)
      .buildURL(
        modelName,
        this.extractId(modelClass, hash),
        hash,
        'findRecord'
      );

    return {
      allocations: {
        links: {
          related: `${nodeURL}/allocations`,
        },
      },
    };
  }
}
