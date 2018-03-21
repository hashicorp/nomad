import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';
import queryString from 'npm:query-string';

export default ApplicationSerializer.extend({
  attrs: {
    parameterized: 'ParameterizedJob',
  },

  normalize(typeHash, hash) {
    hash.NamespaceID = hash.Namespace;

    // ID is a composite of both the job ID and the namespace the job is in
    hash.PlainId = hash.ID;
    hash.ID = JSON.stringify([hash.ID, hash.NamespaceID || 'default']);

    // ParentID comes in as "" instead of null
    if (!hash.ParentID) {
      hash.ParentID = null;
    } else {
      hash.ParentID = JSON.stringify([hash.ParentID, hash.NamespaceID || 'default']);
    }

    // Job Summary is always at /:job-id/summary, but since it can also come from
    // the job list, it's better for Ember Data to be linked by ID association.
    hash.SummaryID = hash.ID;

    // Periodic is a boolean on list and an object on single
    if (hash.Periodic instanceof Object) {
      hash.PeriodicDetails = hash.Periodic;
      hash.Periodic = true;
    }

    // Parameterized behaves like Periodic
    if (hash.ParameterizedJob instanceof Object) {
      hash.ParameterizedDetails = hash.ParameterizedJob;
      hash.ParameterizedJob = true;
    }

    // If the hash contains summary information, push it into the store
    // as a job-summary model.
    if (hash.JobSummary) {
      this.store.pushPayload('job-summary', {
        'job-summary': [hash.JobSummary],
      });
    }

    return this._super(typeHash, hash);
  },

  extractRelationships(modelClass, hash) {
    const namespace =
      !hash.NamespaceID || hash.NamespaceID === 'default' ? undefined : hash.NamespaceID;
    const { modelName } = modelClass;

    const [jobURL] = this.store
      .adapterFor(modelName)
      .buildURL(modelName, hash.ID, hash, 'findRecord')
      .split('?');

    return assign(this._super(...arguments), {
      allocations: {
        links: {
          related: buildURL(`${jobURL}/allocations`, { namespace: namespace }),
        },
      },
      versions: {
        links: {
          related: buildURL(`${jobURL}/versions`, { namespace: namespace, diffs: true }),
        },
      },
      deployments: {
        links: {
          related: buildURL(`${jobURL}/deployments`, { namespace: namespace }),
        },
      },
      evaluations: {
        links: {
          related: buildURL(`${jobURL}/evaluations`, { namespace: namespace }),
        },
      },
    });
  },
});

function buildURL(path, queryParams) {
  const qpString = queryString.stringify(queryParams);
  if (qpString) {
    return `${path}?${qpString}`;
  }
  return path;
}
