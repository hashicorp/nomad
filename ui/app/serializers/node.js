import { inject as service } from '@ember/service';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  config: service(),

  attrs: {
    httpAddr: 'HTTPAddr',
  },

  normalize(modelClass, hash) {
    // Proxy local agent to the same proxy express server Ember is using
    // to avoid CORS
    if (this.get('config.isDev') && hash.HTTPAddr === '127.0.0.1:4646') {
      hash.HTTPAddr = '127.0.0.1:4200';
    }

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
