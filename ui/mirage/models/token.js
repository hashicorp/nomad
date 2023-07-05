/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Model, hasMany, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  tokenPolicies: hasMany('policy'),
  // policies: hasMany('policy'),
  get policies() {
    console.log('lol, lmao', this.tokenPolicies);
    return this.tokenPolicies;
    // return this.tokenPolicies;

    // return this.tokenPolicies.models;
  },
});
