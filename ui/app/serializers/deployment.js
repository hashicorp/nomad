import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    versionNumber: 'JobVersion',
  },

  normalize(typeHash, hash) {
    if (hash) {
      const taskGroups = hash.TaskGroups || {};
      hash.TaskGroupSummaries = Object.keys(taskGroups).map(key => {
        const deploymentStats = taskGroups[key];
        return assign({ Name: key }, deploymentStats);
      });

      hash.PlainJobId = hash.JobID;
      hash.Namespace =
        hash.Namespace ||
        get(hash, 'Job.Namespace') ||
        this.get('system.activeNamespace.id') ||
        'default';

      // Ember Data doesn't support multiple inverses. This means that since jobs have
      // two relationships to a deployment (hasMany deployments, and belongsTo latestDeployment),
      // the deployment must in turn have two relationships to the job, despite it being the
      // same job.
      hash.JobID = hash.JobForLatestID = JSON.stringify([hash.JobID, hash.Namespace]);
    }

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
