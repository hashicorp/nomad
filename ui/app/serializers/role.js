/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import ApplicationSerializer from './application';

export default class RoleSerializer extends ApplicationSerializer {
  // attrs = {
  //   policies: { key: 'PolicyNames', serialize: false },
  // };

  normalize(typeHash, hash) {
    hash.Policies = hash.Policies || [];
    hash.PolicyIDs = hash.Policies.map((policy) => policy.Name);
    return super.normalize(typeHash, hash);
  }
}
