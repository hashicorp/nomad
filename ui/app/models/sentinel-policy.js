/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class SentinelPolicy extends Model {
  @attr('string') name;
  @attr('string') description;
  @attr('string') scope;
  @attr('string') enforcementLevel;
  @attr('string') policy;
  @attr('string') hash;
  @attr('number') createIndex;
  @attr('number') modifyIndex;
}
