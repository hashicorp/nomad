/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationAdapter from './application';
import { pluralize } from 'ember-inflector';

export default class NodePoolAdapter extends ApplicationAdapter {
  urlForFindAll(modelName) {
    let [relationshipResource, resource] = modelName.split('-');
    resource = pluralize(resource);
    return `/v1/${relationshipResource}/${resource}`;
  }

  findAll() {
    return super.findAll(...arguments).catch((error) => {
      // Handle the case where the node pool request is sent to a region that
      // doesn't have node pools and the request is handled by the nodes
      // endpoint.
      const isNodeRequest = error.message.includes(
        'node lookup failed: index error: UUID must be 36 characters',
      );
      if (isNodeRequest) {
        return [];
      }

      // Rethrow to be handled downstream.
      throw error;
    });
  }
}
