import { inject as service } from '@ember/service';
import ApplicationSerializer from './application';

export default class NodeSerializer extends ApplicationSerializer {
  @service config;

  attrs = {
    isDraining: 'Drain',
    httpAddr: 'HTTPAddr',
  };

  mapToArray = ['Drivers'];

  normalize(modelClass, hash) {
    const hostVolumes = hash.HostVolumes || {};
    hash.HostVolumes = Object.keys(hostVolumes).map(key => hostVolumes[key]);

    return super.normalize(modelClass, hash);
  }

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
  }
}
