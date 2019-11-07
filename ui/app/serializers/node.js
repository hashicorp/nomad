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
    const drivers = hash.Drivers || {};
    hash.Drivers = Object.keys(drivers).map(key => {
      return assign({}, drivers[key], { Name: key });
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
