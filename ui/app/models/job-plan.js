/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import { hasMany } from '@ember-data/model';

export default class JobPlan extends Model {
  @attr() diff;
  @fragmentArray('placement-failure', { defaultValue: () => [] })
  failedTGAllocs;
  @hasMany('allocation') preemptions;
}
