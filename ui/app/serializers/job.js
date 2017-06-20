import Ember from 'ember';
import ApplicationSerializer from './application';

const { get } = Ember;

export default ApplicationSerializer.extend({
  attrs: {
    parameterized: 'ParameterizedJob',
  },

  normalize(typeHash, hash) {
    // Lift the summary cache and children stats out of the deeply
    // nested objects to make them simple attributes.
    const allocStats = get(hash, 'JobSummary.Summary.cache');
    if (allocStats) {
      Object.keys(allocStats).forEach(
        allocKey => (hash[`${allocKey}Allocs`] = allocStats[allocKey])
      );
    }

    const childrenStats = get(hash, 'JobSummary.Children');
    if (childrenStats) {
      Object.keys(childrenStats).forEach(
        childrenKey =>
          (hash[`${childrenKey}Children`] = childrenStats[childrenKey])
      );
    }
    return this._super(typeHash, hash);
  },
});
