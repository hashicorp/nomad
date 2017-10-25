import Ember from 'ember';
import ApplicationSerializer from './application';

const { get, inject } = Ember;

export default ApplicationSerializer.extend({
  system: inject.service(),

  attrs: {
    taskGroupName: 'TaskGroup',
    states: 'TaskStates',
  },

  normalize(typeHash, hash) {
    // Transform the map-based TaskStates object into an array-based
    // TaskState fragment list
    hash.TaskStates = Object.keys(get(hash, 'TaskStates') || {}).map(key => {
      const state = get(hash, `TaskStates.${key}`);
      const summary = { Name: key };
      Object.keys(state).forEach(stateKey => (summary[stateKey] = state[stateKey]));
      summary.Resources = hash.TaskResources && hash.TaskResources[key];
      return summary;
    });

    hash.JobVersion = hash.JobVersion != null ? hash.JobVersion : get(hash, 'Job.Version');

    hash.PlainJobId = hash.JobID;
    hash.Namespace =
      hash.Namespace ||
      get(hash, 'Job.Namespace') ||
      this.get('system.activeNamespace.id') ||
      'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

    // TEMPORARY: https://github.com/emberjs/data/issues/5209
    hash.OriginalJobId = hash.JobID;

    return this._super(typeHash, hash);
  },
});
