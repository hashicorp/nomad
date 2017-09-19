import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    datacenter: 'dc',
    address: 'Addr',
    serfPort: 'Port',
  },

  normalize(typeHash, hash) {
    hash.ID = hash.Name;
    hash.Datacenter = hash.Tags && hash.Tags.dc;
    hash.Region = hash.Tags && hash.Tags.region;
    hash.RpcPort = hash.Tags && hash.Tags.port;

    return this._super(typeHash, hash);
  },

  normalizeResponse(store, typeClass, hash, ...args) {
    return this._super(store, typeClass, hash.Members, ...args);
  },

  normalizeSingleResponse(store, typeClass, hash, id, ...args) {
    return this._super(store, typeClass, hash.findBy('Name', id), id, ...args);
  },
});
