import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    versionNumber: 'JobVersion',
  },

  normalize(typeHash, hash) {
    hash.TaskGroupSummaries = Object.keys(get(hash, 'TaskGroups') || {}).map(key => {
      const deploymentStats = get(hash, `TaskGroups.${key}`);
      return assign({ Name: key }, deploymentStats);
    });

    hash.PlainJobId = hash.JobID;
    hash.Namespace =
      hash.Namespace ||
      get(hash, 'Job.Namespace') ||
      this.get('system.activeNamespace.id') ||
      'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

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
