/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { belongsTo } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import classic from 'ember-classic-decorator';

@classic
export default class JobSummary extends Model {
  @belongsTo('job') job;

  @fragmentArray('task-group-scale') taskGroupScales;
}
