/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { hasMany } from '@ember-data/model';

export default class Token extends Model {
  @attr('string') secret;
  @attr('string') name;
  @attr('boolean') global;
  @attr('date') createTime;
  @attr('string') type;
  @hasMany('policy') policies;
  @attr() policyNames;
  @attr('date') expirationTime;

  @alias('id') accessor;

  get isExpired() {
    return this.expirationTime && this.expirationTime < new Date();
  }
}
