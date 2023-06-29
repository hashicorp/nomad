/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
  @hasMany('policy') tokenPolicies;
  @hasMany('role') roles;
  @attr() policyNames;
  @attr('date') expirationTime;

  @alias('id') accessor;

  get isExpired() {
    return this.expirationTime && this.expirationTime < new Date();
  }

  get policies() {
    return [
      ...this.tokenPolicies.toArray(),
      ...this.roles.map((role) => role.policies.toArray()).flat(),
    ].uniq();
  }
}
