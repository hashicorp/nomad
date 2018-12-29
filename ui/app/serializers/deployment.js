import Ember from 'ember';
import ApplicationSerializer from './application';

const { get, assign } = Ember;

export default ApplicationSerializer.extend({
  attrs: {
    versionNumber: 'JobVersion',
  },

  normalize(typeHash, hash) {
    hash.TaskGroupSummaries = Object.keys(get(hash, 'TaskGroups') || {}).map(key => {
      const deploymentStats = get(hash, `TaskGroups.${key}`);
      return assign({ Name: key }, deploymentStats);
    });

    return this._super(typeHash, hash);
  },

  extractRelationships(modelClass, hash) {
    const namespace = this.store.adapterFor(modelClass.modelName).get('namespace');
    const id = this.extractId(modelClass, hash);

    return assign(
      {
        allocations: {
          links: {
            related: `/${namespace}/deployment/allocations/${id}`,
          },
        },
      },
      this._super(modelClass, hash)
    );
  },
});
