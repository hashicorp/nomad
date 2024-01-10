/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Model from '@ember-data/model';
import { hasMany, belongsTo, attr } from '@ember-data/model';

export default class JobStatusModel extends Model {
  @hasMany('allocations') allocs;
  @belongsTo('job') job;
  // @attr() allocs;
  @attr('string') foo;
  @attr('string') namespace;
  @attr('string') deploymentID;
}
