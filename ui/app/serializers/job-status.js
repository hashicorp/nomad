/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import classic from 'ember-classic-decorator';
import ApplicationSerializer from './application';

@classic
export default class JobStatusSerializer extends ApplicationSerializer {
  normalizeQueryResponse(store, primaryModelClass, payload, id, requestType) {
    const jobStatuses = Object.values(payload.Jobs);

    jobStatuses.forEach((jobStatus) => {
      if (jobStatus.Allocs) {
        jobStatus.relationships = {
          allocs: {
            data: jobStatus.Allocs.map((alloc) => ({
              id: alloc.id,
              type: 'allocation',
            })),
          },
        };
      }
    });

    return super.normalizeQueryResponse(
      store,
      primaryModelClass,
      jobStatuses,
      id,
      requestType
    );
  }

  extractRelationships(modelClass, resourceHash) {
    const relationships = {};

    if (resourceHash.Allocs) {
      relationships.Allocs = resourceHash.Allocs.map((alloc) => {
        return {
          id: alloc.ID,
          type: 'allocation',
        };
      });

      resourceHash.Allocs.forEach((alloc) => {
        this.store.push({
          data: {
            id: alloc.ID,
            type: 'allocation',
            attributes: {
              clientStatus: alloc.ClientStatus,
            },
            relationships: {
              jobStatus: {
                data: {
                  id: resourceHash.ID,
                  type: 'job-status',
                },
              },
            },
          },
        });
      });

      delete resourceHash.Allocs; //TODO: needed?
    }

    return relationships;
  }
}
