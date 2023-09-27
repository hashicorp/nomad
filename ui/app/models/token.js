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
  @hasMany('policy') policies;
  @hasMany('role') roles;
  @attr() policyNames;
  @attr('date') expirationTime;

  // Note on verbatim: updating a token requires passing in its expiration time, where
  // the API performs an equality check. However, we want to display it as a nicely
  // formatted date in the UI. @attr('date') does this for us, but it strips the
  // nanoseconds. Thus, our serializer retains the original value in a separate field
  // that gets used on PUT requests when needed.
  @attr() expirationTimeVerbatim;
  @attr() expirationTTL;

  @alias('id') accessor;

  get isExpired() {
    return this.expirationTime && this.expirationTime < new Date();
  }

  /**
   * Combined policies directly on the token, and policies inferred from token's role[s]
   */
  get combinedPolicies() {
    return [
      ...this.policies.toArray(),
      ...this.roles.map((role) => role.policies.toArray()).flat(),
    ].uniq();
  }
}
