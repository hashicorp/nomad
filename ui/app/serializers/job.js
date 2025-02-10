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

    if (hash.UI) {
      hash.Ui = hash.UI;
      delete hash.UI;
    }

    // If the hash contains summary information, push it into the store
    // as a job-summary model.
    if (hash.JobSummary) {
      this.store.pushPayload('job-summary', {
        'job-summary': [hash.JobSummary],
      });
    }

    // job.stop is reserved as a method (points to adapter method) so we rename it here
    if (hash.Stop) {
      hash.Stopped = hash.Stop;
      delete hash.Stop;
    }

    return super.normalize(typeHash, hash);
  }

  normalizeQueryResponse(
    store,
    primaryModelClass,
    payload = [],
    id,
    requestType
  ) {
    // What jobs did we ask for?
    if (payload._requestBody?.jobs) {
      let requestedJobIDs = payload._requestBody.jobs;
      // If they dont match the jobIDs we got back, we need to create an empty one
      // for the ones we didnt get back.
      payload.forEach((job) => {
        job.AssumeGC = false;
      });
      let missingJobIDs = requestedJobIDs.filter(
        (j) =>
          !payload.find((p) => p.ID === j.id && p.Namespace === j.namespace)
      );
      missingJobIDs.forEach((job) => {
        payload.push({
          ID: job.id,
          Namespace: job.namespace,
          Allocs: [],
          AssumeGC: true,
        });

        job.relationships = {
          allocations: {
            data: [],
          },
        };
      });

      // Note: we want our returned jobs to come back in the order we requested them,
      // including jobs that were missing from the initial request.
      payload.sort((a, b) => {
        return (
          requestedJobIDs.findIndex(
            (j) => j.id === a.ID && j.namespace === a.Namespace
          ) -
          requestedJobIDs.findIndex(
            (j) => j.id === b.ID && j.namespace === b.Namespace
          )
        );
      });

      delete payload._requestBody;
    }

    const jobs = payload;
    // Signal that it's a query response at individual normalization level for allocation placement
    // Sort by ModifyIndex, reverse
    jobs.sort((a, b) => b.ModifyIndex - a.ModifyIndex);
    jobs.forEach((job) => {
      if (job.Allocs) {
        job.relationships = {
          allocations: {
            data: job.Allocs.map((alloc) => ({
              id: alloc.id,
              type: 'allocation',
            })),
          },
        };
      }
      if (job.LatestDeployment) {
        job.LatestDeploymentSummary = job.LatestDeployment;
        delete job.LatestDeployment;
      }
      job._aggregate = true;
    });
    return super.normalizeQueryResponse(
      store,
      primaryModelClass,
      jobs,
      id,
      requestType
    );
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

    if (hash._aggregate && hash.Allocs) {
      // Manually push allocations to store
      // These allocations have enough information to be useful on a jobs index page,
      // but less than the /allocations endpoint for an individual job might give us.
      // As such, pages like /optimize require a specific call to the endpoint
      // of any jobs' allocations to get more detailed information.
      hash.Allocs.forEach((alloc) => {
        this.store.push({
          data: {
            id: alloc.ID,
            type: 'allocation',
            attributes: {
              clientStatus: alloc.ClientStatus,
              deploymentStatus: {
                Healthy: alloc.DeploymentStatus.Healthy,
                Canary: alloc.DeploymentStatus.Canary,
              },
              nodeID: alloc.NodeID,
              hasPausedTask: alloc.HasPausedTask,
            },
          },
        });
      });

      delete hash._aggregate;
    }

    return assign(super.extractRelationships(...arguments), {
      allocations: {
        data: hash.Allocs?.map((alloc) => ({
          id: alloc.ID,
          type: 'allocation',
        })),
        links: {
          related: buildURL(`${jobURL}/allocations`, { namespace }),
        },
      },
      versions: {
        links: {
          related: buildURL(`${jobURL}/versions`, {
            namespace,
          }),
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
