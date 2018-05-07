import { inject as service } from '@ember/service';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  config: service(),

  attrs: {
    httpAddr: 'HTTPAddr',
  },

  normalize(modelClass, hash) {
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
