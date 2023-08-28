/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import ApplicationSerializer from './application';

export default class RoleSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.Policies = hash.Policies || []; // null guard
    hash.PolicyIDs = hash.Policies.map((policy) => policy.Name);
    return super.normalize(typeHash, hash);
  }
}
