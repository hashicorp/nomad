import { get } from '@ember/object';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  normalize(modelClass, hash) {
    // Transform the map-based Summary object into an array-based
    // TaskGroupSummary fragment list
    hash.PlainJobId = hash.JobID;
    hash.ID = JSON.stringify([hash.JobID, hash.Namespace || 'default']);
    hash.JobID = hash.ID;

    hash.TaskGroupSummaries = Object.keys(get(hash, 'Summary') || {}).map(key => {
      const allocStats = get(hash, `Summary.${key}`) || {};
      const summary = { Name: key };

      Object.keys(allocStats).forEach(
        allocKey => (summary[`${allocKey}Allocs`] = allocStats[allocKey])
      );

      return summary;
    });

    // Lift the children stats out of the Children object
    const childrenStats = get(hash, 'Children');
    if (childrenStats) {
      Object.keys(childrenStats).forEach(
        childrenKey => (hash[`${childrenKey}Children`] = childrenStats[childrenKey])
      );
    }

    return this._super(modelClass, hash);
  },
});
