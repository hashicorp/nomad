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
