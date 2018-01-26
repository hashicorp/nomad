import ApplicationSerializer from './application';
import { AdapterError } from 'ember-data/adapters/errors';

export default ApplicationSerializer.extend({
  attrs: {
    datacenter: 'dc',
    address: 'Addr',
    serfPort: 'Port',
  },

  normalize(typeHash, hash) {
    if (!hash) {
      // It's unusual to throw an adapter error from a serializer,
      // but there is no single server end point so the serializer
      // acts like the API in this case.
      const error = new AdapterError([{ status: '404' }]);

      error.message = 'Requested Agent was not found in set of available Agents';
      throw error;
    }

    hash.ID = hash.Name;
    hash.Datacenter = hash.Tags && hash.Tags.dc;
    hash.Region = hash.Tags && hash.Tags.region;
    hash.RpcPort = hash.Tags && hash.Tags.port;

    return this._super(typeHash, hash);
  },

  normalizeResponse(store, typeClass, hash, ...args) {
    return this._super(store, typeClass, hash.Members || [], ...args);
  },

  normalizeSingleResponse(store, typeClass, hash, id, ...args) {
    return this._super(store, typeClass, hash.findBy('Name', id), id, ...args);
  },
});
