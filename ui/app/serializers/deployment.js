/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class DeploymentSerializer extends ApplicationSerializer {
  attrs = {
    versionNumber: 'JobVersion',
  };

  mapToArray = [{ beforeName: 'TaskGroups', afterName: 'TaskGroupSummaries' }];

  normalize(typeHash, hash) {
    if (hash) {
      hash.PlainJobId = hash.JobID;
      hash.Namespace =
        hash.Namespace || get(hash, 'Job.Namespace') || 'default';

      // Ember Data doesn't support multiple inverses. This means that since jobs have
      // two relationships to a deployment (hasMany deployments, and belongsTo latestDeployment),
      // the deployment must in turn have two relationships to the job, despite it being the
      // same job.
      hash.JobID = hash.JobForLatestID = JSON.stringify([
        hash.JobID,
        hash.Namespace,
      ]);
    }

    return super.normalize(typeHash, hash);
  }

  extractRelationships(modelClass, hash) {
    const namespace = this.store
      .adapterFor(modelClass.modelName)
      .get('namespace');
    const id = this.extractId(modelClass, hash);

    return assign(
      {
        allocations: {
          links: {
            related: `/${namespace}/deployment/allocations/${id}`,
          },
        },
      },
      super.extractRelationships(modelClass, hash)
    );
  }
}
