/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias, equal } from '@ember/object/computed';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';
import sumAggregation from '../utils/properties/sum-aggregation';
import classic from 'ember-classic-decorator';

@classic
export default class Deployment extends Model {
  @shortUUIDProperty('id') shortId;

  @belongsTo('job', { inverse: 'deployments' }) job;
  @belongsTo('job', { inverse: 'latestDeployment' }) jobForLatest;
  @attr('number') versionNumber;

  // If any task group is not promoted yet requires promotion and the deployment
  // is still running, the deployment needs promotion.
  @computed('status', 'taskGroupSummaries.@each.{promoted,requiresPromotion}')
  get requiresPromotion() {
    return (
      this.status === 'running' &&
      this.taskGroupSummaries
        .toArray()
        .some(
          (summary) =>
            summary.get('requiresPromotion') && !summary.get('promoted')
        )
    );
  }

  @computed('taskGroupSummaries.@each.autoPromote')
  get isAutoPromoted() {
    return this.taskGroupSummaries
      .toArray()
      .every((summary) => summary.get('autoPromote'));
  }

  @attr('string') status;
  @attr('string') statusDescription;

  @equal('status', 'running') isRunning;

  @fragmentArray('task-group-deployment-summary') taskGroupSummaries;
  @hasMany('allocations') allocations;

  @computed('versionNumber', 'job.versions.content.@each.number')
  get version() {
    return (this.get('job.versions') || []).findBy(
      'number',
      this.versionNumber
    );
  }

  // Dependent keys can only go one level past an @each so an alias is needed
  @alias('version.submitTime') versionSubmitTime;

  @sumAggregation('taskGroupSummaries', 'placedCanaries') placedCanaries;
  @sumAggregation('taskGroupSummaries', 'desiredCanaries') desiredCanaries;
  @sumAggregation('taskGroupSummaries', 'desiredTotal') desiredTotal;
  @sumAggregation('taskGroupSummaries', 'placedAllocs') placedAllocs;
  @sumAggregation('taskGroupSummaries', 'healthyAllocs') healthyAllocs;
  @sumAggregation('taskGroupSummaries', 'unhealthyAllocs') unhealthyAllocs;

  @computed('status')
  get statusClass() {
    const classMap = {
      running: 'is-running',
      successful: 'is-primary',
      paused: 'is-light',
      failed: 'is-error',
      cancelled: 'is-cancelled',
    };

    return classMap[this.status] || 'is-dark';
  }

  promote() {
    assert(
      'A deployment needs to requirePromotion to be promoted',
      this.requiresPromotion
    );
    return this.store.adapterFor('deployment').promote(this);
  }

  fail() {
    assert('A deployment must be running to be failed', this.isRunning);
    return this.store.adapterFor('deployment').fail(this);
  }
}
