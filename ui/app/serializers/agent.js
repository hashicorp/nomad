/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import AdapterError from '@ember-data/adapter/error';
import classic from 'ember-classic-decorator';

@classic
export default class AgentSerializer extends ApplicationSerializer {
  attrs = {
    datacenter: 'dc',
    address: 'Addr',
    serfPort: 'Port',
  };

  normalize(typeHash, hash) {
    if (!hash) {
      // It's unusual to throw an adapter error from a serializer,
      // but there is no single server end point so the serializer
      // acts like the API in this case.
      const error = new AdapterError([{ status: '404' }]);

      error.message =
        'Requested Agent was not found in set of available Agents';
      throw error;
    }

    hash.ID = hash.Name;
    hash.Datacenter = hash.Tags && hash.Tags.dc;
    hash.Region = hash.Tags && hash.Tags.region;
    hash.RpcPort = hash.Tags && hash.Tags.port;

    return super.normalize(typeHash, hash);
  }

  normalizeResponse(store, typeClass, hash, ...args) {
    return super.normalizeResponse(
      store,
      typeClass,
      hash.Members || [],
      ...args
    );
  }

  normalizeSingleResponse(store, typeClass, hash, id, ...args) {
    return super.normalizeSingleResponse(
      store,
      typeClass,
      hash.findBy('Name', id),
      id,
      ...args
    );
  }
}
