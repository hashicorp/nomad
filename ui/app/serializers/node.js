import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import { inject as service } from '@ember/service';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  config: service(),

  attrs: {
    isDraining: 'Drain',
    httpAddr: 'HTTPAddr',
  },

  normalize(modelClass, hash) {
    // Transform the map-based Drivers object into an array-based NodeDriver fragment list
    hash.Drivers = Object.keys(get(hash, 'Drivers') || {}).map(key => {
      return assign({}, get(hash, `Drivers.${key}`), { Name: key });
    });

    return this._super(modelClass, hash);
  },

  extractRelationships(modelClass, hash) {
    const { modelName } = modelClass;
    const nodeURL = this.store
      .adapterFor(modelName)
      .buildURL(modelName, this.extractId(modelClass, hash), hash, 'findRecord');

    return {
      allocations: {
        links: {
          related: `${nodeURL}/allocations`,
        },
      },
    };
  },
});
