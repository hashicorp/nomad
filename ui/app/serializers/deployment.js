import Ember from 'ember';
import ApplicationSerializer from './application';

const { get, assign } = Ember;

export default ApplicationSerializer.extend({
  normalize(typeHash, hash) {
    hash.TaskGroupSummaries = Object.keys(get(hash, 'TaskGroups')).map(key => {
      const deploymentStats = get(hash, `TaskGroups.${key}`);
      return assign({ Name: key }, deploymentStats);
    });

    return this._super(typeHash, hash);
  },
});
