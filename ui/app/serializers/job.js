import Ember from 'ember';
import ApplicationSerializer from './application';

const { get, assign } = Ember;

export default ApplicationSerializer.extend({
  attrs: {
    parameterized: 'ParameterizedJob',
  },

  normalize(typeHash, hash) {
    // Transform the map-based JobSummary object into an array-based
    // JobSummary fragment list
    hash.TaskGroupSummaries = Object.keys(get(hash, 'JobSummary.Summary')).map(key => {
      const allocStats = get(hash, `JobSummary.Summary.${key}`);
      const summary = { Name: key };

      Object.keys(allocStats).forEach(
        allocKey => (summary[`${allocKey}Allocs`] = allocStats[allocKey])
      );

      return summary;
    });

    // Lift the children stats out of the JobSummary object
    const childrenStats = get(hash, 'JobSummary.Children');
    if (childrenStats) {
      Object.keys(childrenStats).forEach(
        childrenKey => (hash[`${childrenKey}Children`] = childrenStats[childrenKey])
      );
    }

    return this._super(typeHash, hash);
  },

  extractRelationships(modelClass, hash) {
    const { modelName } = modelClass;
    const jobURL = this.store
      .adapterFor(modelName)
      .buildURL(modelName, this.extractId(modelClass, hash), hash, 'findRecord');

    return assign(this._super(...arguments), {
      allocations: {
        links: {
          related: `${jobURL}/allocations`,
        },
      },
      versions: {
        links: {
          related: `${jobURL}/versions?diffs=true`,
        },
      },
      deployments: {
        links: {
          related: `${jobURL}/deployments`,
        },
      },
    });
  },
});
