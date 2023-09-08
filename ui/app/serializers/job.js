/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';
import queryString from 'query-string';
import classic from 'ember-classic-decorator';

@classic
export default class JobSerializer extends ApplicationSerializer {
  attrs = {
    parameterized: 'ParameterizedJob',
  };

  separateNanos = ['SubmitTime'];

  normalize(typeHash, hash) {
    hash.NamespaceID = hash.Namespace;

    // ID is a composite of both the job ID and the namespace the job is in
    hash.PlainId = hash.ID;
    hash.ID = JSON.stringify([hash.ID, hash.NamespaceID || 'default']);

    // ParentID comes in as "" instead of null
    if (!hash.ParentID) {
      hash.ParentID = null;
    } else {
      hash.ParentID = JSON.stringify([
        hash.ParentID,
        hash.NamespaceID || 'default',
      ]);
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

    return super.normalize(typeHash, hash);
  }

  extractRelationships(modelClass, hash) {
    const namespace =
      !hash.NamespaceID || hash.NamespaceID === 'default'
        ? undefined
        : hash.NamespaceID;
    const { modelName } = modelClass;

    const apiNamespace = this.store
      .adapterFor(modelClass.modelName)
      .get('namespace');

    const [jobURL] = this.store
      .adapterFor(modelName)
      .buildURL(modelName, hash.ID, hash, 'findRecord')
      .split('?');

    const variableLookup = hash.ParentID
      ? JSON.parse(hash.ParentID)[0]
      : hash.PlainId;

    return assign(super.extractRelationships(...arguments), {
      allocations: {
        links: {
          related: buildURL(`${jobURL}/allocations`, { namespace }),
        },
      },
      versions: {
        links: {
          related: buildURL(`${jobURL}/versions`, { namespace, diffs: true }),
        },
      },
      deployments: {
        links: {
          related: buildURL(`${jobURL}/deployments`, { namespace }),
        },
      },
      latestDeployment: {
        links: {
          related: buildURL(`${jobURL}/deployment`, { namespace }),
        },
      },
      evaluations: {
        links: {
          related: buildURL(`${jobURL}/evaluations`, { namespace }),
        },
      },
      services: {
        links: {
          related: buildURL(`${jobURL}/services`, { namespace }),
        },
      },
      variables: {
        links: {
          related: buildURL(`/${apiNamespace}/vars`, {
            prefix: `nomad/jobs/${variableLookup}`,
            namespace,
          }),
        },
      },
      scaleState: {
        links: {
          related: buildURL(`${jobURL}/scale`, { namespace }),
        },
      },
      recommendationSummaries: {
        links: {
          related: buildURL(`/${apiNamespace}/recommendations`, {
            job: hash.PlainId,
            namespace: hash.NamespaceID || 'default',
          }),
        },
      },
    });
  }
}

function buildURL(path, queryParams) {
  const qpString = queryString.stringify(queryParams);
  if (qpString) {
    return `${path}?${qpString}`;
  }
  return path;
}
