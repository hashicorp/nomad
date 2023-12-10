/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import { get } from '@ember/object';
import { action } from '@ember/object';

export default class RecommendationSummary extends Model {
  @hasMany('recommendation') recommendations;
  @hasMany('recommendation', { defaultValue: () => [] })
  excludedRecommendations;

  @belongsTo('job') job;
  @attr('string') jobId;
  @attr('string') jobNamespace;

  @attr('date') submitTime;
  @attr('string') taskGroupName;

  // Set in the serialiser upon saving
  @attr('boolean', { defaultValue: false }) isProcessed;

  get taskGroup() {
    const taskGroups = get(this, 'job.taskGroups');

    if (taskGroups) {
      return taskGroups.findBy('name', this.taskGroupName);
    } else {
      return undefined;
    }
  }

  @action
  toggleRecommendation(recommendation) {
    if (this.excludedRecommendations.includes(recommendation)) {
      this.excludedRecommendations =
        this.excludedRecommendations.removeObject(recommendation);
    } else {
      this.excludedRecommendations.pushObject(recommendation);
    }
  }

  @action
  toggleAllRecommendationsForResource(resource, enabled) {
    if (enabled) {
      this.excludedRecommendations = this.excludedRecommendations.rejectBy(
        'resource',
        resource
      );
    } else {
      this.excludedRecommendations.pushObjects(
        this.recommendations.filterBy('resource', resource)
      );
    }
  }

  get slug() {
    return `${get(this, 'job.name')}/${this.taskGroupName}`;
  }
}
