/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class NodePool extends Model {
  @attr('string') name;
  @attr('string') description;
  @attr() meta;
  @attr() schedulerConfiguration;

  @computed('schedulerConfiguration.SchedulerAlgorithm')
  get schedulerAlgorithm() {
    return this.get('schedulerConfiguration.SchedulerAlgorithm');
  }

  @computed('schedulerConfiguration.MemoryOversubscriptionEnabled')
  get memoryOversubscriptionEnabled() {
    return this.get('schedulerConfiguration.MemoryOversubscriptionEnabled');
  }
}
