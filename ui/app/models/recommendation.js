/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';
import { get } from '@ember/object';

export default class Recommendation extends Model {
  @belongsTo('job') job;
  @belongsTo('recommendation-summary', { inverse: 'recommendations' })
  recommendationSummary;

  @attr('date') submitTime;

  get taskGroup() {
    return get(this, 'recommendationSummary.taskGroup');
  }

  @attr('string') taskName;

  get task() {
    return get(this, 'taskGroup.tasks').findBy('name', this.taskName);
  }

  @attr('string') resource;
  @attr('number') value;

  get currentValue() {
    const resourceProperty =
      this.resource === 'CPU' ? 'reservedCPU' : 'reservedMemory';
    return get(this, `task.${resourceProperty}`);
  }

  @attr() stats;
}
