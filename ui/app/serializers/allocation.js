import Ember from 'ember';
import ApplicationSerializer from './application';

const { get } = Ember;

export default ApplicationSerializer.extend({
  attrs: {
    taskGroupName: 'TaskGroup',
    states: 'TaskStates',
  },

  normalize(typeHash, hash) {
    // Transform the map-based TaskStates object into an array-based
    // TaskState fragment list
    hash.TaskStates = Object.keys(get(hash, 'TaskStates')).map(key => {
      const state = get(hash, `TaskStates.${key}`);
      const summary = { Name: key };
      Object.keys(state).forEach(stateKey => (summary[stateKey] = state[stateKey]));
      summary.Resources = hash.TaskResources && hash.TaskResources[key];
      return summary;
    });

    return this._super(typeHash, hash);
  },
});
