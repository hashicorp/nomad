/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class Evaluation extends ApplicationSerializer {
  @service system;

  mapToArray = ['FailedTGAllocs'];
  separateNanos = ['CreateTime', 'ModifyTime'];

  normalize(typeHash, hash) {
    hash.PlainJobId = hash.JobID;
    hash.Namespace = hash.Namespace || get(hash, 'Job.Namespace') || 'default';
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);

    const relatedEvals = hash.RelatedEvals;

    const normalizedHash = super.normalize(typeHash, hash);

    if (relatedEvals?.length) {
      this._handleRelatedEvalsRelationshipData(relatedEvals, normalizedHash);
    }

    return normalizedHash;
  }

  _handleRelatedEvalsRelationshipData(relatedEvals, normalizedHash) {
    normalizedHash.data.relationships = normalizedHash.data.relationships || {};

    normalizedHash.data.relationships.relatedEvals = {
      data: relatedEvals.map((evaluationStub) => {
        return { id: evaluationStub.ID, type: 'evaluation-stub' };
      }),
    };

    normalizedHash.included = normalizedHash.included || [];

    const included = relatedEvals.reduce((acc, evaluationStub) => {
      const jsonDocument = this.normalize(
        this.store.modelFor('evaluation-stub'),
        evaluationStub
      );

      return [...acc, jsonDocument.data];
    }, normalizedHash.included);

    normalizedHash.included = included;
  }
}
